package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/modules/account/application"
)

const (
	productionCookieName = "__Host-pf_session"
	localCookieName      = "pf_session"
	csrfHeaderName       = "X-CSRF-Token"
	maxCredentialsBytes  = 4096
)

// Service 定义 HTTP 适配层所需的账号用例。
type Service interface {
	Register(context.Context, application.RegisterInput) (application.PublicUser, error)
	Login(context.Context, application.LoginInput) (application.LoginResult, error)
	ResolveSession(context.Context, string) (application.Session, error)
	CurrentUser(context.Context, int64) (application.PublicUser, error)
	Logout(context.Context, string) error
}

// Handler 负责网页账号接口和 Cookie/CSRF 适配。
type Handler struct {
	service      Service
	webOrigin    string
	cookieSecure bool
	cookieName   string
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type csrfResponse struct {
	Token string `json:"csrf_token"`
}

// NewHandler 创建账号 HTTP 适配器。
// 输入：service，应用用例；webOrigin，唯一允许的网页来源；cookieSecure，是否启用 Secure。
// 输出：账号路由处理器；依赖或来源配置缺失时返回错误。
// 功能：集中设置生产/开发 Cookie 名称，避免客户端令牌进入业务用例。
func NewHandler(service Service, webOrigin string, cookieSecure bool) (*Handler, error) {
	if service == nil || webOrigin == "" {
		return nil, errors.New("account HTTP service and web origin are required")
	}
	name := localCookieName
	if cookieSecure {
		name = productionCookieName
	}
	return &Handler{service: service, webOrigin: webOrigin, cookieSecure: cookieSecure, cookieName: name}, nil
}

// RegisterRoutes 注册第一版网页账号接口。
// 输入：router，核心 Gin Engine。
// 输出：修改路由表，无返回值。
// 功能：将凭据型 CORS、匿名认证接口和需要会话及 CSRF 的接口分组挂载。
func (handler *Handler) RegisterRoutes(router *gin.Engine) {
	group := router.Group("/api/v1/auth")
	group.Use(handler.cors())
	group.OPTIONS("/register", handler.preflight)
	group.OPTIONS("/login", handler.preflight)
	group.OPTIONS("/me", handler.preflight)
	group.OPTIONS("/csrf", handler.preflight)
	group.OPTIONS("/logout", handler.preflight)
	group.POST("/register", handler.requireOrigin(), handler.register)
	group.POST("/login", handler.requireOrigin(), handler.login)
	group.GET("/me", handler.requireSession(), handler.currentUser)
	group.GET("/csrf", handler.requireSession(), handler.csrf)
	group.POST("/logout", handler.requireOrigin(), handler.requireSession(), handler.requireCSRF(), handler.logout)
}

// register 解析注册凭据并创建用户。
// 输入：c，Gin 当前请求上下文。
// 输出：201 用户资料，或统一参数、冲突、限流和依赖错误响应。
// 功能：将可信远端地址和请求凭据交给注册用例，不返回密码摘要。
func (handler *Handler) register(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialsBytes)
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	user, err := handler.service.Register(c.Request.Context(), application.RegisterInput{
		RemoteIP: c.ClientIP(), Username: request.Username, Password: request.Password,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

// login 验证用户名密码并通过 Set-Cookie 建立网页会话。
// 输入：c，Gin 当前请求上下文。
// 输出：200 用户资料并设置 HttpOnly Cookie；错误时返回统一响应。
// 功能：不把可重放会话令牌放入 JSON 响应体。
func (handler *Handler) login(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialsBytes)
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	result, err := handler.service.Login(c.Request.Context(), application.LoginInput{
		RemoteIP: c.ClientIP(), Username: request.Username, Password: request.Password,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	handler.setSessionCookie(c, result.Credentials.Token)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result.User)
}

// currentUser 从 MySQL 读取当前会话用户资料。
// 输入：c，已通过会话中间件认证的 Gin 上下文。
// 输出：200 用户资料；账号无效或存储不可用时返回统一错误。
// 功能：向网页提供本人身份摘要，不暴露密码或会话凭据。
func (handler *Handler) currentUser(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
		return
	}
	user, err := handler.service.CurrentUser(c.Request.Context(), userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, user)
}

// csrf 返回与当前 Redis 会话绑定的 CSRF 令牌。
// 输入：c，已通过会话中间件认证的 Gin 上下文。
// 输出：禁止缓存的 CSRF 令牌响应；上下文缺少会话时返回 401。
// 功能：允许网页重新加载后读取令牌并放入修改请求头。
func (handler *Handler) csrf(c *gin.Context) {
	token, exists := c.Get(contextCSRFToken)
	if !exists {
		writeError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, csrfResponse{Token: token.(string)})
}

// logout 撤销当前 Redis 会话并清除浏览器 Cookie。
// 输入：c，已通过会话和 CSRF 校验的 Gin 上下文。
// 输出：成功返回 204 并过期 Cookie；Redis 故障时返回 503。
// 功能：只注销当前浏览器会话，不影响其他设备。
func (handler *Handler) logout(c *gin.Context) {
	token, exists := c.Get(contextSessionToken)
	if !exists {
		writeError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
		return
	}
	if err := handler.service.Logout(c.Request.Context(), token.(string)); err != nil {
		writeServiceError(c, err)
		return
	}
	handler.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// setSessionCookie 设置不含业务资料的不透明网页会话 Cookie。
// 输入：c，当前响应上下文；token，由应用层随机生成的会话令牌。
// 输出：写入 Set-Cookie 响应头，无返回值。
// 功能：为生产启用 Secure、Host 前缀、HttpOnly 和 SameSite=Lax。
func (handler *Handler) setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(handler.cookieName, token, int(application.SessionLifetime.Seconds()), "/", "", handler.cookieSecure, true)
}

// clearSessionCookie 让浏览器删除当前网页会话 Cookie。
// 输入：c，当前响应上下文。
// 输出：写入相同属性但 Max-Age 为负数的 Set-Cookie 响应头。
// 功能：在服务端会话撤销成功后同步清除客户端令牌。
func (handler *Handler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(handler.cookieName, "", -1, "/", "", handler.cookieSecure, true)
}
