package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/httpserver/response"
)

var errNilRecoveryLogger = errors.New("recovery logger is required")

// RecoveryMiddleware 保存异常恢复中间件依赖。
type RecoveryMiddleware struct {
	logger *slog.Logger
}

// NewRecovery 创建异常恢复中间件。
// 输入：logger，结构化日志器，不能为 nil。
// 输出：可注册到 Gin 的中间件；依赖缺失时返回错误。
// 功能：将请求处理 panic 转换为可审计且不泄露内部信息的错误响应。
func NewRecovery(logger *slog.Logger) (*RecoveryMiddleware, error) {
	if logger == nil {
		return nil, errNilRecoveryLogger
	}
	return &RecoveryMiddleware{logger: logger}, nil
}

// Handle 捕获当前 HTTP 请求链路中的 panic。
// 输入：c，当前 Gin 请求上下文。
// 输出：无 panic 时继续执行；发生 panic 时记录堆栈并返回统一 500 错误。
// 功能：隔离单次请求异常，避免进程退出或内部细节暴露给客户端。
func (middleware *RecoveryMiddleware) Handle(c *gin.Context) {
	defer middleware.recoverRequest(c)
	c.Next()
}

// recoverRequest 处理已经发生的请求 panic。
// 输入：c，当前 Gin 请求上下文，并通过 recover 读取 panic 值。
// 输出：没有 panic 时不操作；发生 panic 时中止请求并记录错误。
// 功能：集中实现 panic 日志和客户端错误响应。
func (middleware *RecoveryMiddleware) recoverRequest(c *gin.Context) {
	recovered := recover()
	if recovered == nil {
		return
	}
	middleware.logger.Error("http request panic",
		"request_id", c.GetString(response.RequestIDContextKey),
		"panic", recovered,
		"stack", string(debug.Stack()),
	)
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
