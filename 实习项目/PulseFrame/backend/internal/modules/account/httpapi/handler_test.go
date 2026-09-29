package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/modules/account/application"
	"pulseframe/internal/modules/account/domain"
	"pulseframe/internal/platform/httpserver"
	httpmiddleware "pulseframe/internal/platform/httpserver/middleware"
)

const testWebOrigin = "http://127.0.0.1:5173"

type testService struct {
	users    map[int64]application.PublicUser
	byName   map[string]application.PublicUser
	sessions map[string]application.Session
}

// Register 在 HTTP 测试服务中创建唯一用户名。
// 输入：ctx，请求上下文；input，注册用户名、密码和来源 IP。
// 输出：测试用户；重复用户名返回领域冲突错误。
// 功能：隔离 Handler 测试，不连接真实数据库。
func (service *testService) Register(_ context.Context, input application.RegisterInput) (application.PublicUser, error) {
	if _, exists := service.byName[input.Username]; exists {
		return application.PublicUser{}, domain.ErrUsernameTaken
	}
	user := application.PublicUser{ID: int64(len(service.users) + 1), Username: input.Username}
	service.users[user.ID] = user
	service.byName[user.Username] = user
	return user, nil
}

// Login 在 HTTP 测试服务中签发固定会话凭据。
// 输入：ctx，请求上下文；input，登录用户名、密码和来源 IP。
// 输出：测试用户、令牌和 CSRF 令牌；凭据错误时返回统一认证错误。
// 功能：验证 Handler Cookie 与响应行为。
func (service *testService) Login(_ context.Context, input application.LoginInput) (application.LoginResult, error) {
	user, exists := service.byName[input.Username]
	if !exists || input.Password != "correct horse" {
		return application.LoginResult{}, application.ErrInvalidCredentials
	}
	token := strings.Repeat("a", 64)
	csrfToken := strings.Repeat("b", 64)
	service.sessions[token] = application.Session{UserID: user.ID, CSRFToken: csrfToken, ExpiresAt: time.Now().Add(time.Hour)}
	return application.LoginResult{User: user, Credentials: application.SessionCredentials{Token: token, CSRFToken: csrfToken}}, nil
}

// ResolveSession 读取 HTTP 测试服务的会话映射。
// 输入：ctx，请求上下文；token，Cookie 中的测试令牌。
// 输出：已保存会话；令牌不存在时返回 ErrUnauthorized。
// 功能：验证认证中间件从 Cookie 获取身份。
func (service *testService) ResolveSession(_ context.Context, token string) (application.Session, error) {
	session, exists := service.sessions[token]
	if !exists {
		return application.Session{}, application.ErrUnauthorized
	}
	return session, nil
}

// CurrentUser 返回 HTTP 测试服务中的本人资料。
// 输入：ctx，请求上下文；userID，已认证主体编号。
// 输出：测试用户；主体不存在时返回 ErrUnauthorized。
// 功能：验证 `/me` 只使用中间件提供的主体编号。
func (service *testService) CurrentUser(_ context.Context, userID int64) (application.PublicUser, error) {
	user, exists := service.users[userID]
	if !exists {
		return application.PublicUser{}, application.ErrUnauthorized
	}
	return user, nil
}

// Logout 删除 HTTP 测试服务中的当前会话。
// 输入：ctx，请求上下文；token，当前 Cookie 令牌。
// 输出：删除成功返回 nil。
// 功能：验证退出后旧 Cookie 不能再次访问受保护路由。
func (service *testService) Logout(_ context.Context, token string) error {
	delete(service.sessions, token)
	return nil
}

type requestInput struct {
	Method           string
	Path             string
	Origin           string
	Body             string
	Cookie           *http.Cookie
	CSRF             string
	PreflightMethod  string
	PreflightHeaders string
}

// newTestHTTPRouter 创建装配账号 Handler 的真实 Gin 路由。
// 输入：t，测试上下文；secure，是否测试安全 Cookie 属性。
// 输出：Gin Router 和可检查的测试应用服务。
// 功能：验证请求经过平台中间件、模块路由及账号 HTTP 适配。
func newTestHTTPRouter(t *testing.T, secure bool) (*gin.Engine, *testService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &testService{
		users: make(map[int64]application.PublicUser), byName: make(map[string]application.PublicUser),
		sessions: make(map[string]application.Session),
	}
	handler, err := NewHandler(service, testWebOrigin, secure)
	if err != nil {
		t.Fatalf("create account Handler: %v", err)
	}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	router, err := httpserver.NewRouter(httpserver.RouterDependencies{
		Logger: logger, IDGenerator: httpmiddleware.CryptoIDGenerator{}, Health: httpserver.NewHealthState(),
		Modules: []httpserver.RouteRegistrar{handler},
	})
	if err != nil {
		t.Fatalf("create HTTP router: %v", err)
	}
	return router, service
}

// performJSON 执行带可选 Cookie、来源、CSRF 和 CORS 预检数据的测试请求。
// 输入：router，待测 HTTP Handler；input，请求、Cookie、来源及预检数据。
// 输出：完整 HTTP 响应记录。
// 功能：减少端到端账号请求测试的重复装配。
func performJSON(router http.Handler, input requestInput) *httptest.ResponseRecorder {
	request := httptest.NewRequest(input.Method, input.Path, strings.NewReader(input.Body))
	if input.Body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if input.Origin != "" {
		request.Header.Set("Origin", input.Origin)
	}
	if input.Cookie != nil {
		request.AddCookie(input.Cookie)
	}
	if input.CSRF != "" {
		request.Header.Set(csrfHeaderName, input.CSRF)
	}
	if input.PreflightMethod != "" {
		request.Header.Set("Access-Control-Request-Method", input.PreflightMethod)
	}
	if input.PreflightHeaders != "" {
		request.Header.Set("Access-Control-Request-Headers", input.PreflightHeaders)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestWebRegisterLoginCSRFAndLogout 验证浏览器认证链路与会话撤销。
// 输入：Go 测试框架提供的测试上下文。
// 输出：接口状态、Cookie 属性或 CSRF 行为不符合预期时使测试失败。
// 功能：覆盖网页注册、登录、本人查询、CSRF 获取、拒绝及通过和退出。
func TestWebRegisterLoginCSRFAndLogout(t *testing.T) {
	router, _ := newTestHTTPRouter(t, true)
	register := performJSON(router, requestInput{
		Method: http.MethodPost, Path: "/api/v1/auth/register", Origin: testWebOrigin,
		Body: `{"username":"user_1","password":"correct horse"}`,
	})
	if register.Code != http.StatusCreated || bytes.Contains(register.Body.Bytes(), []byte("password")) {
		t.Fatalf("unexpected registration response: %d %s", register.Code, register.Body.String())
	}
	login := performJSON(router, requestInput{
		Method: http.MethodPost, Path: "/api/v1/auth/login", Origin: testWebOrigin,
		Body: `{"username":"user_1","password":"correct horse"}`,
	})
	if login.Code != http.StatusOK || bytes.Contains(login.Body.Bytes(), []byte(strings.Repeat("a", 64))) {
		t.Fatalf("unexpected login response: %d %s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if login.Header().Get("Access-Control-Allow-Origin") != testWebOrigin ||
		login.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("login response lacks credentialed CORS headers: %v", login.Header())
	}
	if cookie.Name != productionCookieName || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected session Cookie: %+v", cookie)
	}
	assertStatus(t, performJSON(router, requestInput{Method: http.MethodGet, Path: "/api/v1/auth/me", Cookie: cookie}), http.StatusOK)
	csrfResult := performJSON(router, requestInput{Method: http.MethodGet, Path: "/api/v1/auth/csrf", Cookie: cookie})
	if csrfResult.Code != http.StatusOK || csrfResult.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected CSRF response: %d %s", csrfResult.Code, csrfResult.Body.String())
	}
	var csrf csrfResponse
	if err := json.Unmarshal(csrfResult.Body.Bytes(), &csrf); err != nil || csrf.Token == "" {
		t.Fatalf("invalid CSRF response: %v", err)
	}
	logoutInput := requestInput{Method: http.MethodPost, Path: "/api/v1/auth/logout", Origin: testWebOrigin, Cookie: cookie}
	assertStatus(t, performJSON(router, logoutInput), http.StatusForbidden)
	logoutInput.CSRF = csrf.Token
	assertStatus(t, performJSON(router, logoutInput), http.StatusNoContent)
	assertStatus(t, performJSON(router, requestInput{Method: http.MethodGet, Path: "/api/v1/auth/me", Cookie: cookie}), http.StatusUnauthorized)
}

// TestCredentialedCORSPreflight 验证网页来源和请求头白名单。
// 输入：Go 测试框架提供的测试上下文。
// 输出：合法预检未放行或外部来源、未授权请求头被接受时使测试失败。
// 功能：保护浏览器跨来源携带 Cookie 的请求边界。
func TestCredentialedCORSPreflight(t *testing.T) {
	router, _ := newTestHTTPRouter(t, false)
	allowed := performJSON(router, requestInput{
		Method: http.MethodOptions, Path: "/api/v1/auth/logout", Origin: testWebOrigin,
		PreflightMethod: http.MethodPost, PreflightHeaders: "content-type, x-csrf-token",
	})
	if allowed.Code != http.StatusNoContent || allowed.Header().Get("Access-Control-Allow-Origin") != testWebOrigin ||
		allowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("unexpected allowed preflight: %d %v", allowed.Code, allowed.Header())
	}
	for _, input := range []requestInput{
		{Method: http.MethodOptions, Path: "/api/v1/auth/login", Origin: "https://attacker.example", PreflightMethod: http.MethodPost},
		{Method: http.MethodOptions, Path: "/api/v1/auth/login", Origin: testWebOrigin, PreflightMethod: http.MethodPut},
		{Method: http.MethodOptions, Path: "/api/v1/auth/login", Origin: testWebOrigin, PreflightMethod: http.MethodPost, PreflightHeaders: "x-admin"},
	} {
		response := performJSON(router, input)
		assertStatus(t, response, http.StatusForbidden)
	}
}

// TestMutationRejectsForeignOrigin 验证匿名写接口也检查精确网页来源。
// 输入：Go 测试框架提供的测试上下文。
// 输出：跨站或缺少 Origin 的注册请求未返回 403 时使测试失败。
// 功能：阻止伪造注册或登录请求绕过网页来源策略。
func TestMutationRejectsForeignOrigin(t *testing.T) {
	router, service := newTestHTTPRouter(t, false)
	for _, origin := range []string{"https://attacker.example", ""} {
		response := performJSON(router, requestInput{
			Method: http.MethodPost, Path: "/api/v1/auth/register", Origin: origin,
			Body: `{"username":"user_1","password":"correct horse"}`,
		})
		assertStatus(t, response, http.StatusForbidden)
	}
	if len(service.users) != 0 {
		t.Fatalf("foreign origin created a user: %+v", service.users)
	}
}

// assertStatus 检查请求响应状态码。
// 输入：t，测试上下文；response，HTTP 响应记录；expected，预期状态码。
// 输出：状态不匹配时使测试失败。
// 功能：统一断言账号 HTTP 状态行为。
func assertStatus(t *testing.T, response *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if response.Code != expected {
		t.Fatalf("unexpected status: got %d want %d body=%s", response.Code, expected, response.Body.String())
	}
}
