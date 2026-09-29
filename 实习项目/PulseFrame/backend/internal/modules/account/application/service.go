package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"pulseframe/internal/modules/account/domain"
)

const (
	SessionLifetime = 7 * 24 * time.Hour
	tokenHexLength  = 64
	csrfRandomBytes = 32
)

var errCreateDummyPassword = errors.New("create dummy password hash")

// Dependencies 收纳账号应用服务使用的可替换依赖。
type Dependencies struct {
	Users       UserRepository
	Sessions    SessionManager
	RateLimiter RateLimiter
	Passwords   PasswordHasher
	RatePolicy  RatePolicy
}

// Service 编排注册、登录、身份解析和退出用例。
type Service struct {
	users       UserRepository
	sessions    SessionManager
	rateLimiter RateLimiter
	passwords   PasswordHasher
	dummyHash   string
	ratePolicy  RatePolicy
}

// PublicUser 是不含密码摘要的用户响应视图。
type PublicUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// LoginResult 保存登录响应所需的用户和 Cookie 令牌。
type LoginResult struct {
	User        PublicUser
	Credentials SessionCredentials
}

// RegisterInput 保存注册请求和可信连接来源。
type RegisterInput struct {
	RemoteIP string
	Username string
	Password string
}

// LoginInput 保存登录请求和可信连接来源。
type LoginInput struct {
	RemoteIP string
	Username string
	Password string
}

// NewService 校验依赖并准备账号枚举防护所需的虚拟密码摘要。
// 输入：dependencies，MySQL、Redis 和密码哈希实现，均不能为空。
// 输出：可处理账号用例的服务；依赖缺失或摘要创建失败时返回错误。
// 功能：在服务启动时暴露配置错误，并让未知用户名走同类密码验证成本。
func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Users == nil || dependencies.Sessions == nil || dependencies.RateLimiter == nil || dependencies.Passwords == nil ||
		!validRatePolicy(dependencies.RatePolicy) {
		return nil, errors.New("account service dependencies are required")
	}
	dummyHash, err := dependencies.Passwords.Hash("pulseframe-invalid-account-password")
	if err != nil {
		return nil, errors.Join(errCreateDummyPassword, err)
	}
	return &Service{
		users: dependencies.Users, sessions: dependencies.Sessions,
		rateLimiter: dependencies.RateLimiter, passwords: dependencies.Passwords,
		dummyHash: dummyHash, ratePolicy: dependencies.RatePolicy,
	}, nil
}

// Register 校验限流与凭据后创建普通用户。
// 输入：ctx，请求上下文；input，来源 IP、用户名和原始密码。
// 输出：公开用户资料；字段非法、用户名冲突、限流或依赖故障时返回错误。
// 功能：编排注册校验、密码摘要和 MySQL 持久化。
func (service *Service) Register(ctx context.Context, input RegisterInput) (PublicUser, error) {
	if err := validateInput(input.Username, input.Password); err != nil {
		return PublicUser{}, err
	}
	allowed, err := service.rateLimiter.Allow(ctx, []Limit{{
		Key: "register:ip:" + input.RemoteIP, Max: service.ratePolicy.RegistrationIPMaximum,
		Window: service.ratePolicy.RegistrationWindow,
	}})
	if err != nil {
		return PublicUser{}, ErrUnavailable
	}
	if !allowed {
		return PublicUser{}, ErrRateLimited
	}
	hash, err := service.passwords.Hash(input.Password)
	if err != nil {
		return PublicUser{}, ErrUnavailable
	}
	user, err := service.users.Create(ctx, input.Username, hash)
	if err != nil {
		if errors.Is(err, domain.ErrUsernameTaken) {
			return PublicUser{}, err
		}
		return PublicUser{}, ErrUnavailable
	}
	return publicUser(user), nil
}

// Login 验证账号后创建 Redis 会话。
// 输入：ctx，请求上下文；input，来源 IP、用户名和原始密码。
// 输出：公开用户与一次性返回的会话材料；认证、限流或依赖故障时返回错误。
// 功能：统一凭据错误响应并签发网页会话。
func (service *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if err := validateInput(input.Username, input.Password); err != nil {
		return LoginResult{}, err
	}
	limits := []Limit{
		{Key: "login:ip:" + input.RemoteIP, Max: service.ratePolicy.LoginIPMaximum, Window: service.ratePolicy.LoginWindow},
		{Key: "login:identity-ip:" + input.RemoteIP + ":" + input.Username,
			Max: service.ratePolicy.LoginIdentityIPMaximum, Window: service.ratePolicy.LoginWindow},
	}
	allowed, err := service.rateLimiter.Allow(ctx, limits)
	if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	if !allowed {
		return LoginResult{}, ErrRateLimited
	}
	return service.authenticateAndCreateSession(ctx, input)
}

// authenticateAndCreateSession 校验 MySQL 用户并创建短期会话。
// 输入：ctx，请求上下文；input，已经通过格式校验和限流的登录请求。
// 输出：登录结果；无效凭据统一返回 ErrInvalidCredentials，依赖或哈希容量故障返回 ErrUnavailable。
// 功能：对不存在用户执行虚拟哈希验证，并只为有效账号创建 Redis 会话。
func (service *Service) authenticateAndCreateSession(ctx context.Context, input LoginInput) (LoginResult, error) {
	user, err := service.users.FindByUsername(ctx, input.Username)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			if err := service.passwords.Verify(input.Password, service.dummyHash); errors.Is(err, ErrPasswordHashCapacity) {
				return LoginResult{}, ErrUnavailable
			}
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, ErrUnavailable
	}
	passwordErr := service.passwords.Verify(input.Password, user.PasswordHash)
	if errors.Is(passwordErr, ErrPasswordHashCapacity) {
		return LoginResult{}, ErrUnavailable
	}
	if user.Status != domain.StatusActive || passwordErr != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	credentials, err := service.sessions.Create(ctx, user.ID, SessionLifetime)
	if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	return LoginResult{User: publicUser(user), Credentials: credentials}, nil
}

// ResolveSession 解析随机令牌并检查服务端会话。
// 输入：ctx，请求上下文；token，Cookie 中的原始不透明令牌。
// 输出：通过 Redis 验证的主体与 CSRF 信息；令牌无效或依赖故障时返回错误。
// 功能：不信任客户端携带的用户编号，统一执行服务端会话查找。
func (service *Service) ResolveSession(ctx context.Context, token string) (Session, error) {
	if len(token) != tokenHexLength {
		return Session{}, ErrUnauthorized
	}
	session, err := service.sessions.Resolve(ctx, token)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return Session{}, ErrUnauthorized
		}
		return Session{}, ErrUnavailable
	}
	return session, nil
}

// CurrentUser 读取仍处于有效状态的本人资料。
// 输入：ctx，请求上下文；userID，已由 Redis 会话认证的用户编号。
// 输出：公开用户资料；账号不存在或已停用时返回未认证错误。
// 功能：从 MySQL 读取权威用户状态，不返回密码摘要。
func (service *Service) CurrentUser(ctx context.Context, userID int64) (PublicUser, error) {
	user, err := service.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return PublicUser{}, ErrUnauthorized
		}
		return PublicUser{}, ErrUnavailable
	}
	if user.Status != domain.StatusActive {
		return PublicUser{}, ErrUnauthorized
	}
	return publicUser(user), nil
}

// Logout 撤销当前 Cookie 对应的服务端会话。
// 输入：ctx，请求上下文；token，Cookie 中的原始不透明令牌。
// 输出：撤销成功时返回 nil；Redis 故障时返回 ErrUnavailable。
// 功能：保证成功响应后当前令牌不能再次建立登录态。
func (service *Service) Logout(ctx context.Context, token string) error {
	if len(token) != tokenHexLength {
		return ErrUnauthorized
	}
	if err := service.sessions.Revoke(ctx, token); err != nil {
		return ErrUnavailable
	}
	return nil
}

// validateInput 校验登录与注册共用的凭据格式。
// 输入：username，用户名；password，原始密码。
// 输出：格式合法时返回 nil，否则返回领域校验错误。
// 功能：保持注册和登录接受完全相同的凭据范围。
func validateInput(username string, password string) error {
	if err := domain.ValidateUsername(username); err != nil {
		return err
	}
	return domain.ValidatePassword(password)
}

// publicUser 构造不含密码摘要的接口视图。
// 输入：user，MySQL 返回的领域用户。
// 输出：仅含用户编号和用户名的公开结构。
// 功能：集中阻止凭据字段进入 HTTP 响应。
func publicUser(user domain.User) PublicUser {
	return PublicUser{ID: user.ID, Username: user.Username}
}

// validRatePolicy 确认服务创建时限流阈值和窗口均有效。
// 输入：policy，来自已校验环境配置的限流策略。
// 输出：所有阈值和窗口均为正数时返回 true。
// 功能：防止应用服务绕过配置校验后以零限制运行。
func validRatePolicy(policy RatePolicy) bool {
	return policy.LoginIPMaximum > 0 && policy.LoginIdentityIPMaximum > 0 &&
		policy.RegistrationIPMaximum > 0 && policy.LoginWindow > 0 && policy.RegistrationWindow > 0
}

// NewCSRFToken 生成会话绑定的高熵 CSRF 令牌。
// 输入：无。
// 输出：32 字节密码学安全随机数的十六进制表示；随机源失败时返回错误。
// 功能：避免使用可预测值保护 Cookie 自动携带的状态修改请求。
func NewCSRFToken() (string, error) {
	value := make([]byte, csrfRandomBytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
