package middleware

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/httpserver/response"
)

var errNilAccessLogger = errors.New("access logger is required")

// AccessLogMiddleware 保存访问日志中间件依赖。
type AccessLogMiddleware struct {
	logger *slog.Logger
}

// NewAccessLog 创建访问日志中间件。
// 输入：logger，结构化日志器，不能为 nil。
// 输出：可注册到 Gin 的中间件；依赖缺失时返回错误。
// 功能：统一记录请求方法、路由、状态码、耗时和请求标识。
func NewAccessLog(logger *slog.Logger) (*AccessLogMiddleware, error) {
	if logger == nil {
		return nil, errNilAccessLogger
	}
	return &AccessLogMiddleware{logger: logger}, nil
}

// Handle 记录一次 HTTP 请求的结构化访问日志。
// 输入：c，当前 Gin 请求上下文。
// 输出：继续请求链路，并在完成后写入一条日志。
// 功能：让请求结果和性能能够通过请求标识追踪。
func (middleware *AccessLogMiddleware) Handle(c *gin.Context) {
	startedAt := time.Now()
	c.Next()

	path := c.FullPath()
	if path == "" {
		path = c.Request.URL.Path
	}
	middleware.logger.Info("http request completed",
		"request_id", c.GetString(response.RequestIDContextKey),
		"method", c.Request.Method,
		"path", path,
		"status", c.Writer.Status(),
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
}
