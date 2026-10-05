package response

import "github.com/gin-gonic/gin"

const (
	// RequestIDHeader 和 RequestIDContextKey 分别用于 HTTP 传递和 Gin 上下文保存请求标识。
	RequestIDHeader     = "X-Request-ID"
	RequestIDContextKey = "request_id"

	// Code* 是客户端可以稳定依赖的公开错误码，不包含内部错误详情。
	CodeInternal         = "INTERNAL_ERROR"
	CodeNotFound         = "RESOURCE_NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
)

// ErrorSpec 描述对外稳定的 HTTP 错误。
type ErrorSpec struct {
	Status  int
	Code    string
	Message string
}

// errorDetail 是错误响应中的稳定错误信息。
type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorBody 是统一 HTTP 错误响应体。
type errorBody struct {
	RequestID string      `json:"request_id"`
	Error     errorDetail `json:"error"`
}

// WriteError 写入统一错误响应并终止当前 Gin 请求链路。
// 输入：c，当前 Gin 上下文；spec，对外 HTTP 状态码、错误码和说明。
// 输出：写入响应并调用 Abort，不返回值。
// 功能：避免各 Handler 自行拼装不一致或泄露内部信息的错误响应。
func WriteError(c *gin.Context, spec ErrorSpec) {
	// 请求 ID 用来把客户端错误与服务端日志对应起来，Abort 会终止后续 Handler。
	c.AbortWithStatusJSON(spec.Status, errorBody{
		RequestID: c.GetString(RequestIDContextKey),
		Error: errorDetail{
			Code:    spec.Code,
			Message: spec.Message,
		},
	})
}
