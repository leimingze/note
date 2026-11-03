package middleware

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"pulseframe/response"
)

// errNilAccessLogger 表示访问日志中间件缺少结构化日志器。
var errNilAccessLogger = errors.New("access logger is required")

// AccessLogMiddleware 保存访问日志中间件依赖。
type AccessLogMiddleware struct {
	logger *slog.Logger // 请求处理完成后写入访问记录的日志器。
}

// NewAccessLog 创建访问日志中间件。
// 输入：logger，结构化日志器，不能为 nil。
// 输出：可注册到 Gin 的中间件；依赖缺失时返回错误。
// 功能：保存日志器，供后续每次请求完成后记录访问结果。
// 例如传入 JSON 日志器后，Handle 会用它记录 GET /livez 的状态码和耗时。
func NewAccessLog(logger *slog.Logger) (*AccessLogMiddleware, error) {
	// 没有日志器就无法记录请求结果，构造阶段直接拒绝。
	if logger == nil {
		return nil, errNilAccessLogger
	}
	return &AccessLogMiddleware{logger: logger}, nil
}

// Handle 记录一次 HTTP 请求的结构化访问日志。
// 输入：c，当前 Gin 请求上下文。
// 输出：继续请求链路，并在完成后写入一条日志。
// 功能：让请求结果和性能能够通过请求标识追踪。
// 例如 GET /livez 返回200后，日志包含 GET、/livez、200、耗时和本次请求标识。
func (middleware *AccessLogMiddleware) Handle(c *gin.Context) {
	startedAt := time.Now()
	// 先执行完整请求链，返回后才能得到最终状态码和耗时。
	c.Next()

	path := c.FullPath()
	// 未匹配路由没有模板路径，退回实际 URL 便于定位 404。
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
