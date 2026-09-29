package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"pulseframe/internal/modules/account/domain"
)

type memoryUsers struct {
	mu     sync.Mutex
	byName map[string]domain.User
	byID   map[int64]domain.User
	nextID int64
}

// Create 在测试仓储中创建具有唯一用户名的用户。
// 输入：ctx，请求上下文；username，用户名；passwordHash，测试摘要。
// 输出：新用户；用户名重复时返回 ErrUsernameTaken。
// 功能：模拟数据库唯一约束供应用用例验证。
func (users *memoryUsers) Create(_ context.Context, username string, passwordHash string) (domain.User, error) {
	users.mu.Lock()
	defer users.mu.Unlock()
	if _, exists := users.byName[username]; exists {
		return domain.User{}, domain.ErrUsernameTaken
	}
	users.nextID++
	user := domain.User{ID: users.nextID, Username: username, PasswordHash: passwordHash, Status: domain.StatusActive}
	users.byName[username] = user
	users.byID[user.ID] = user
	return user, nil
}

// FindByUsername 按测试用户名读取用户。
// 输入：ctx，请求上下文；username，已校验用户名。
// 输出：测试用户；不存在时返回 ErrUserNotFound。
// 功能：模拟账号凭据查询。
func (users *memoryUsers) FindByUsername(_ context.Context, username string) (domain.User, error) {
	users.mu.Lock()
	defer users.mu.Unlock()
	user, exists := users.byName[username]
	if !exists {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

// FindByID 按测试用户编号读取用户。
// 输入：ctx，请求上下文；userID，内部用户编号。
// 输出：测试用户；不存在时返回 ErrUserNotFound。
// 功能：模拟本人资料查询与账号状态核对。
func (users *memoryUsers) FindByID(_ context.Context, userID int64) (domain.User, error) {
	users.mu.Lock()
	defer users.mu.Unlock()
	user, exists := users.byID[userID]
	if !exists {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

type testPasswords struct{}

type saturatedPasswords struct{ testPasswords }

// Hash 构造可逆测试摘要，不用于生产配置。
// 输入：password，测试密码。
// 输出：带固定前缀的测试值。
// 功能：让应用用例测试不重复执行昂贵的 Argon2id 运算。
func (testPasswords) Hash(password string) (string, error) {
	return "test-hash:" + password, nil
}

// Verify 比较测试密码与测试摘要。
// 输入：password，待验证密码；encoded，测试摘要。
// 输出：匹配返回 nil，否则返回错误。
// 功能：隔离验证应用流程，不代表生产密码算法。
func (testPasswords) Verify(password string, encoded string) error {
	if encoded != "test-hash:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

// Verify 模拟密码哈希容量已满。
// 输入：password，待验证密码；encoded，测试摘要。
// 输出：始终返回密码哈希容量错误。
// 功能：验证应用服务将计算过载映射为依赖不可用而非错误凭据。
func (saturatedPasswords) Verify(string, string) error {
	return ErrPasswordHashCapacity
}

type memorySessions struct {
	mu       sync.Mutex
	byToken  map[string]Session
	nextID   int
	failures error
}

// Create 在测试会话表中生成唯一令牌并设置有效期。
// 输入：ctx，请求上下文；userID，用户编号；ttl，会话有效期。
// 输出：测试令牌与 CSRF 令牌；配置故障时返回错误。
// 功能：模拟 Redis 会话创建与固定过期时间。
func (sessions *memorySessions) Create(_ context.Context, userID int64, ttl time.Duration) (SessionCredentials, error) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.failures != nil {
		return SessionCredentials{}, sessions.failures
	}
	sessions.nextID++
	token := fmt.Sprintf("%064x", sessions.nextID)
	csrfToken := fmt.Sprintf("%064x", sessions.nextID+100)
	sessions.byToken[token] = Session{UserID: userID, CSRFToken: csrfToken, ExpiresAt: time.Now().Add(ttl)}
	return SessionCredentials{Token: token, CSRFToken: csrfToken}, nil
}

// Resolve 读取测试会话并拒绝已过期令牌。
// 输入：ctx，请求上下文；token，原始测试令牌。
// 输出：有效测试会话；缺失或过期时返回 ErrUnauthorized。
// 功能：模拟 Redis TTL 会话读取。
func (sessions *memorySessions) Resolve(_ context.Context, token string) (Session, error) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.failures != nil {
		return Session{}, sessions.failures
	}
	session, exists := sessions.byToken[token]
	if !exists || !session.ExpiresAt.After(time.Now()) {
		return Session{}, ErrUnauthorized
	}
	return session, nil
}

// Revoke 删除测试会话。
// 输入：ctx，请求上下文；token，待撤销测试令牌。
// 输出：撤销成功返回 nil；配置故障时返回错误。
// 功能：模拟当前设备退出。
func (sessions *memorySessions) Revoke(_ context.Context, token string) error {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.failures != nil {
		return sessions.failures
	}
	delete(sessions.byToken, token)
	return nil
}

type memoryLimiter struct {
	allowed bool
	err     error
}

// Allow 返回测试预设的限流结论。
// 输入：ctx，请求上下文；limits，待检查的限流桶。
// 输出：测试预设的允许结果或错误。
// 功能：使服务用例覆盖放行、拒绝和依赖故障路径。
func (limiter memoryLimiter) Allow(context.Context, []Limit) (bool, error) {
	return limiter.allowed, limiter.err
}

// newTestService 创建使用内存仓储和测试摘要器的应用服务。
// 输入：t，测试上下文；limiter，测试限流器。
// 输出：完整账号应用服务及其可检查的依赖。
// 功能：集中构造用例测试依赖且不连接外部系统。
func newTestService(t *testing.T, limiter RateLimiter) (*Service, *memoryUsers, *memorySessions) {
	t.Helper()
	users := &memoryUsers{byName: make(map[string]domain.User), byID: make(map[int64]domain.User)}
	sessions := &memorySessions{byToken: make(map[string]Session)}
	service, err := NewService(Dependencies{
		Users: users, Sessions: sessions, RateLimiter: limiter, Passwords: testPasswords{},
		RatePolicy: RatePolicy{
			LoginIPMaximum: 30, LoginIdentityIPMaximum: 10, RegistrationIPMaximum: 8,
			LoginWindow: 15 * time.Minute, RegistrationWindow: time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("create account service: %v", err)
	}
	return service, users, sessions
}

// TestRegisterLoginAndLogout 验证普通账号从创建到会话撤销的完整用例。
// 输入：Go 测试框架提供的测试上下文。
// 输出：资料、认证和会话撤销不符合预期时使测试失败。
// 功能：保护注册登录闭环和不向公开视图泄露密码摘要。
func TestRegisterLoginAndLogout(t *testing.T) {
	service, _, _ := newTestService(t, memoryLimiter{allowed: true})
	ctx := context.Background()
	registered, err := service.Register(ctx, RegisterInput{RemoteIP: "127.0.0.1", Username: "user_1", Password: "correct horse"})
	if err != nil || registered.ID == 0 || registered.Username != "user_1" {
		t.Fatalf("unexpected registration: %+v, %v", registered, err)
	}
	if _, err := service.Register(ctx, RegisterInput{RemoteIP: "127.0.0.1", Username: "user_1", Password: "correct horse"}); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("duplicate registration error: %v", err)
	}
	login, err := service.Login(ctx, LoginInput{RemoteIP: "127.0.0.1", Username: "user_1", Password: "correct horse"})
	if err != nil || login.Credentials.Token == "" || login.User.ID != registered.ID {
		t.Fatalf("unexpected login: %+v, %v", login, err)
	}
	current, err := service.CurrentUser(ctx, login.User.ID)
	if err != nil || current.Username != registered.Username {
		t.Fatalf("unexpected current user: %+v, %v", current, err)
	}
	if err := service.Logout(ctx, login.Credentials.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.ResolveSession(ctx, login.Credentials.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked session resolved: %v", err)
	}
}

// TestLoginUsesGenericCredentialFailure 验证不存在账号与密码错误统一失败。
// 输入：Go 测试框架提供的测试上下文。
// 输出：失败类型不同或无效账号获得会话时使测试失败。
// 功能：降低登录接口泄露用户名存在性的风险。
func TestLoginUsesGenericCredentialFailure(t *testing.T) {
	service, _, _ := newTestService(t, memoryLimiter{allowed: true})
	ctx := context.Background()
	input := LoginInput{RemoteIP: "127.0.0.1", Username: "missing_user", Password: "correct horse"}
	_, missingErr := service.Login(ctx, input)
	_, _ = service.Register(ctx, RegisterInput{RemoteIP: input.RemoteIP, Username: "known_user", Password: input.Password})
	input.Username = "known_user"
	input.Password = "incorrect horse"
	_, passwordErr := service.Login(ctx, input)
	if !errors.Is(missingErr, ErrInvalidCredentials) || !errors.Is(passwordErr, ErrInvalidCredentials) {
		t.Fatalf("credential errors differ: missing=%v password=%v", missingErr, passwordErr)
	}
}

// TestLoginFailsClosedWhenLimiterUnavailable 验证限流依赖故障不会放行登录。
// 输入：Go 测试框架提供的测试上下文。
// 输出：限流错误未映射为 ErrUnavailable 时使测试失败。
// 功能：确保多实例限流故障不会静默降级。
func TestLoginFailsClosedWhenLimiterUnavailable(t *testing.T) {
	service, _, _ := newTestService(t, memoryLimiter{err: errors.New("Redis unavailable")})
	_, err := service.Login(context.Background(), LoginInput{
		RemoteIP: "127.0.0.1", Username: "user_1", Password: "correct horse",
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unexpected limiter failure mapping: %v", err)
	}
}

// TestLoginReturnsUnavailableWhenHashCapacityIsFull 验证哈希过载不会伪装成密码错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：账号存在或不存在时未返回 ErrUnavailable 使测试失败。
// 功能：确保容量保护产生可诊断的 503 语义并保持账号枚举防护。
func TestLoginReturnsUnavailableWhenHashCapacityIsFull(t *testing.T) {
	service, _, _ := newTestService(t, memoryLimiter{allowed: true})
	ctx := context.Background()
	_, err := service.Register(ctx, RegisterInput{RemoteIP: "127.0.0.1", Username: "known_user", Password: "correct horse"})
	if err != nil {
		t.Fatalf("register test user: %v", err)
	}
	service.passwords = saturatedPasswords{}
	for _, username := range []string{"known_user", "missing_user"} {
		_, err := service.Login(ctx, LoginInput{RemoteIP: "127.0.0.1", Username: username, Password: "correct horse"})
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("login for %q should report unavailable, got %v", username, err)
		}
	}
}
