package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"pulseframe/metrics"
	"pulseframe/response"
)

// MetricsMiddleware 记录 HTTP 请求指标。
type MetricsMiddleware struct {
	registry *metrics.Registry // 指标注册表，提供 HTTP 请求指标。
}

// NewMetrics 创建指标中间件。
// 输入：registry，指标注册表。
// 输出：可注册到 Gin 的中间件。
// 功能：保存指标注册表，供每次请求更新请求数量、耗时和进行中请求数。
func NewMetrics(registry *metrics.Registry) *MetricsMiddleware {
	return &MetricsMiddleware{registry: registry}
}

// Handle 记录当前 HTTP 请求的指标。
// 输入：c，当前 Gin 请求上下文。
// 输出：更新请求指标并继续请求链路。
// 功能：在请求进入时累加进行中数量，完成时更新数量、耗时、错误计数并衰减进行中数量。
func (middleware *MetricsMiddleware) Handle(c *gin.Context) {
	middleware.registry.RequestsInFlight.Inc()
	startedAt := time.Now()
	c.Next()

	path := c.FullPath()
	if path == "" {
		path = "/unknown"
	}
	status := c.Writer.Status()
	labels := []string{c.Request.Method, path, strconv.Itoa(status)}
	middleware.registry.RequestsTotal.WithLabelValues(labels...).Inc()
	middleware.registry.RequestDuration.WithLabelValues(labels...).Observe(time.Since(startedAt).Seconds())
	middleware.registry.RequestsInFlight.Dec()

	// 统一错误中间件已在上下文中记录错误码，这里据此累加错误指标。
	if errCode := c.GetString(response.ErrorCodeContextKey); errCode != "" {
		middleware.registry.ResponseErrorsTotal.WithLabelValues(path, strconv.Itoa(status), errCode).Inc()
	}
}
