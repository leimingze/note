package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/modules/account/application"
	"pulseframe/internal/modules/account/domain"
	"pulseframe/internal/platform/httpserver/response"
)

const (
	contextUserID       = "account_user_id"
	contextSessionToken = "account_session_token"
	contextCSRFToken    = "account_csrf_token"
)

// cors 为唯一配置网页来源返回凭据型 CORS 响应并校验预检请求。
// 输入：c，正在处理的账号 API 请求。
// 输出：合法来源继续路由并附加 CORS 响应头；无效预检返回 403。
// 功能：允许网页跨端口携带 HttpOnly Cookie，同时不使用通配来源。
func (handler *Handler) cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin == handler.webOrigin {
			c.Header("Access-Control-Allow-Origin", handler.webOrigin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
		}
		if c.Request.Method != http.MethodOptions {
			c.Next()
			return
		}
		if origin != handler.webOrigin || !validPreflightMethod(c.GetHeader("Access-Control-Request-Method")) ||
			!validPreflightHeaders(c.GetHeader("Access-Control-Request-Headers")) {
			writeError(c, http.StatusForbidden, response.CodeCSRFRejected, "request origin rejected")
			return
		}
		c.Next()
	}
}

// preflight 结束已经通过 cors 中间件验证的浏览器预检请求。
// 输入：c，来源、目标方法和请求头均已通过校验的 OPTIONS 请求。
// 输出：返回 204，无响应正文。
// 功能：让浏览器在发送带 JSON 或 CSRF 自定义头的请求前取得允许策略。
func (handler *Handler) preflight(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// validPreflightMethod 校验网页需要的目标请求方法。
// 输入：method，浏览器 Access-Control-Request-Method 的值。
// 输出：GET 或 POST 时返回 true。
// 功能：限制凭据型跨来源请求只能使用账号 API 已定义的方法。
func validPreflightMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodPost
}

// validPreflightHeaders 校验网页预检声明的请求头集合。
// 输入：raw，逗号分隔的 Access-Control-Request-Headers 值，可为空。
// 输出：仅含 Content-Type 和 X-CSRF-Token 时返回 true。
// 功能：避免凭据型 CORS 策略无意间开放未设计的自定义请求头。
func validPreflightHeaders(raw string) bool {
	for _, header := range strings.Split(raw, ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "", "content-type", "x-csrf-token":
			continue
		default:
			return false
		}
	}
	return true
}

// requireOrigin 拒绝来源与配置网页不完全一致的写请求。
// 输入：c，正在处理的浏览器请求。
// 输出：来源不匹配时写入 403 并中止；匹配时继续请求链。
// 功能：保护注册、登录和退出免受跨站请求伪造。
func (handler *Handler) requireOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") != handler.webOrigin {
			writeError(c, http.StatusForbidden, response.CodeCSRFRejected, "request origin rejected")
			return
		}
		c.Next()
	}
}

// requireSession 从 Cookie 解析 Redis 会话并填入可信主体上下文。
// 输入：c，正在处理的受保护请求。
// 输出：会话无效时返回 401，依赖不可用时返回 503；有效时继续请求链。
// 功能：只信任 Redis 验证过的身份，不采纳客户端自报用户 ID。
func (handler *Handler) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(handler.cookieName)
		if err != nil {
			writeError(c, http.StatusUnauthorized, response.CodeUnauthenticated, "authentication required")
			return
		}
		session, err := handler.service.ResolveSession(c.Request.Context(), token)
		if err != nil {
			writeSessionError(c, err)
			return
		}
		c.Set(contextUserID, session.UserID)
		c.Set(contextSessionToken, token)
		c.Set(contextCSRFToken, session.CSRFToken)
		c.Next()
	}
}

// requireCSRF 比较自定义请求头与 Redis 会话绑定令牌。
// 输入：c，已通过会话认证的写请求。
// 输出：令牌缺失或不匹配时返回 403 并中止。
// 功能：要求网页显式证明状态修改请求由已加载本站页面发起。
func (handler *Handler) requireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := c.GetString(contextCSRFToken)
		provided := c.GetHeader(csrfHeaderName)
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			writeError(c, http.StatusForbidden, response.CodeCSRFRejected, "CSRF token rejected")
			return
		}
		c.Next()
	}
}

// UserIDFromContext 读取会话中间件写入的内部用户编号。
// 输入：c，已经执行过 requireSession 的 Gin 上下文。
// 输出：有效正数用户编号及存在标记。
// 功能：供当前和后续 HTTP 适配器获取可信认证主体。
func UserIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(contextUserID)
	if !exists {
		return 0, false
	}
	userID, validType := value.(int64)
	return userID, validType && userID > 0
}

// writeSessionError 映射会话解析错误且不泄露 Redis 内部信息。
// 输入：c，当前响应上下文；err，会话解析结果。
// 输出：写入 401 或 503 统一错误并终止当前链路。
// 功能：区分无效凭据和认证依赖故障，避免故障时放行。
func writeSessionError(c *gin.Context, err error) {
	if errors.Is(err, application.ErrUnauthorized) {
		writeError(c, http.StatusUnauthorized, response.CodeUnauthenticated, "authentication required")
		return
	}
	writeServiceError(c, err)
}

// writeServiceError 将应用层稳定错误映射为统一 HTTP 契约。
// 输入：c，当前响应上下文；err，应用用例返回的错误。
// 输出：写入业务错误状态码、稳定错误码和安全说明。
// 功能：隔离内部错误文本并保持客户端可依赖的响应语义。
func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidCredentials), errors.Is(err, application.ErrUnauthorized):
		writeError(c, http.StatusUnauthorized, response.CodeUnauthenticated, "invalid credentials")
	case errors.Is(err, application.ErrRateLimited):
		writeError(c, http.StatusTooManyRequests, response.CodeRateLimited, "too many requests")
	case errors.Is(err, application.ErrUnavailable):
		writeError(c, http.StatusServiceUnavailable, response.CodeDependencyUnavailable, "authentication temporarily unavailable")
	case errors.Is(err, domain.ErrInvalidUsername) || errors.Is(err, domain.ErrInvalidPassword):
		writeError(c, http.StatusBadRequest, response.CodeInvalidRequest, "invalid request")
	case errors.Is(err, domain.ErrUsernameTaken):
		writeError(c, http.StatusConflict, response.CodeUsernameTaken, "username already exists")
	default:
		writeError(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
	}
}

// writeError 写入统一错误结构。
// 输入：c，当前响应上下文；status、code、message，对外稳定错误信息。
// 输出：写入错误响应并终止请求链。
// 功能：通过公共响应适配器携带请求标识。
func writeError(c *gin.Context, status int, code string, message string) {
	response.WriteError(c, response.ErrorSpec{Status: status, Code: code, Message: message})
}
