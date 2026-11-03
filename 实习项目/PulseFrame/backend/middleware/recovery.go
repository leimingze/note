package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"pulseframe/response"
)

// errNilRecoveryLogger 表示异常恢复中间件缺少记录 panic 的日志器。
var errNilRecoveryLogger = errors.New("recovery logger is required")

// RecoveryMiddleware 保存异常恢复中间件依赖。
type RecoveryMiddleware struct {
	logger *slog.Logger // 记录请求异常及调用位置的日志器。
}

// NewRecovery 创建异常恢复中间件。
// 输入：logger，结构化日志器，不能为 nil。
// 输出：可注册到 Gin 的中间件；依赖缺失时返回错误。
// 功能：保存日志器，供异常恢复中间件记录请求处理中的 panic。
// 例如传入可用日志器时返回中间件，传入 nil 时返回缺少日志器的错误。
func NewRecovery(logger *slog.Logger) (*RecoveryMiddleware, error) {
	// panic 必须留下结构化审计记录，因此日志器是必需依赖。
	if logger == nil {
		return nil, errNilRecoveryLogger
	}
	return &RecoveryMiddleware{logger: logger}, nil
}

// Handle 捕获当前 HTTP 请求链路中的 panic。
// 输入：c，当前 Gin 请求上下文。
// 输出：继续处理请求；发生 panic 时记录调用信息，尚未发送响应时写入统一500错误，否则中止后续处理。
// 功能：为当前请求安排异常恢复，再执行后续中间件和路由。
// 例如路由在发送响应前发生 panic，客户端会收到500错误，异常内容保存在日志中。
func (middleware *RecoveryMiddleware) Handle(c *gin.Context) {
	defer middleware.recoverRequest(c)
	c.Next()
}

// recoverRequest 处理已经发生的请求 panic。
// 输入：c，当前 Gin 请求上下文，并通过 recover 读取 panic 值。
// 输出：没有 panic 时不操作；发生 panic 时中止请求并记录错误。
// 功能：读取当前异常，先记录原因，再根据响应是否已经发送决定能否写入错误正文。
// 例如尚未发送响应时返回 INTERNAL_ERROR；已经发送响应时保留已发送内容并中止后续处理。
func (middleware *RecoveryMiddleware) recoverRequest(c *gin.Context) {
	// 当前函数由 defer 直接调用，在这里读取正在传播的 panic；普通函数调用中读不到该值。
	recovered := recover()
	// 没有 panic 时保持正常响应，不做额外写入。
	if recovered == nil {
		return
	}
	middleware.logger.Error("http request panic",
		"request_id", c.GetString(response.RequestIDContextKey),
		"panic", recovered,
		"stack", string(debug.Stack()),
	)
	// 响应已经发送时不能再改写成 JSON，只能中止链路并保留日志。
	if c.Writer.Written() {
		c.Abort()
		return
	}
	response.WriteError(c, response.ErrorSpec{
		Status:  http.StatusInternalServerError,
		Code:    response.CodeInternal,
		Message: "internal server error",
	})
}
