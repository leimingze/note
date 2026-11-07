package response

import "github.com/gin-gonic/gin"

const (
	RequestIDHeader     = "X-Request-ID" // 请求和响应中传递请求标识的 HTTP 头名称。
	RequestIDContextKey = "request_id"   // Gin 上下文保存请求标识时使用的键。
	ErrorCodeContextKey = "error_code"   // Gin 上下文保存本次统一错误码时使用的键。

	CodeInternal              = "INTERNAL_ERROR"         // 服务内部错误，不向客户端暴露内部原因。
	CodeInvalidRequest        = "INVALID_REQUEST"        // 请求参数或格式非法。
	CodeUnauthorized          = "UNAUTHORIZED"           // 未认证或凭证无效。
	CodeForbidden             = "FORBIDDEN"              // 已认证但无权限。
	CodeNotFound              = "RESOURCE_NOT_FOUND"     // 请求路径没有对应资源。
	CodeMethodNotAllowed      = "METHOD_NOT_ALLOWED"     // 路径存在，但不支持当前请求方法。
	CodeConflict              = "CONFLICT"               // 与当前资源状态冲突。
	CodeTooManyRequests       = "TOO_MANY_REQUESTS"      // 请求过于频繁，触发限流。
	CodeDependencyUnavailable = "DEPENDENCY_UNAVAILABLE" // 外部依赖不可用。
)

// ErrorSpec 描述对外稳定的 HTTP 错误。
type ErrorSpec struct {
	Status  int    // HTTP 状态码，例如404。
	Code    string // 客户端用于判断错误类别的稳定标识。
	Message string // 可以公开给客户端的错误说明。
}

// errorDetail 是错误响应中的稳定错误信息。
type errorDetail struct {
	Code    string `json:"code"`    // 写入 JSON 的错误类别。
	Message string `json:"message"` // 写入 JSON 的公开说明。
}

// errorBody 是统一 HTTP 错误响应体。
type errorBody struct {
	RequestID string      `json:"request_id"` // 用于关联服务端日志的请求标识。
	Error     errorDetail `json:"error"`      // 本次失败的错误码和说明。
}

// HealthResponse 是存活和就绪接口的响应体。
type HealthResponse struct {
	Status string `json:"status"` // 对外状态文本，使用 ok、not_ready 或 unhealthy。
}

// WriteError 写入统一错误响应并终止当前 Gin 请求链路。
// 输入：c，当前 Gin 上下文；spec，对外 HTTP 状态码、错误码和说明。
// 输出：写入响应并调用 Abort，不返回值。
// 功能：避免各 Handler 自行拼装不一致或泄露内部信息的错误响应。
// 例如传入404和 RESOURCE_NOT_FOUND 时，返回包含 request_id、error.code 和 error.message 的 JSON。
func WriteError(c *gin.Context, spec ErrorSpec) {
	// 请求 ID 与错误码分别写入上下文，供日志和指标中间件读取。
	c.Set(ErrorCodeContextKey, spec.Code)         // 将错误码写入上下文
	c.AbortWithStatusJSON(spec.Status, errorBody{ // 中断链路并按状态码写入错误 JSON
		RequestID: c.GetString(RequestIDContextKey), // 读取请求标识
		Error: errorDetail{
			Code:    spec.Code,    // 写入错误码
			Message: spec.Message, // 写入公开说明
		},
	})
}
