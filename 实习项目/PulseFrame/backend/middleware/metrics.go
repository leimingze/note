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
	return &MetricsMiddleware{registry: registry} // 传入注册表；返回指标中间件
}

// Handle 记录当前 HTTP 请求的指标。
// 输入：c，当前 Gin 请求上下文。
// 输出：更新请求指标并继续请求链路。
// 功能：在请求进入时累加进行中数量，完成时更新数量、耗时、错误计数并衰减进行中数量。
func (middleware *MetricsMiddleware) Handle(c *gin.Context) {
	middleware.registry.RequestsInFlight.Inc() // 进行中请求数 +1
	startedAt := time.Now()                    // 记录请求开始时间
	c.Next()                                   // 交给后续中间件和路由处理请求

	path := c.FullPath() // 获取注册的路由模板
	if path == "" {
		path = "/unknown" // 未匹配路由统一记为 /unknown
	}
	status := c.Writer.Status()                                                                             // 读取响应状态码
	labels := []string{c.Request.Method, path, strconv.Itoa(status)}                                        // 组装 method/route/status 标签值
	middleware.registry.RequestsTotal.WithLabelValues(labels...).Inc()                                      // 给对应组合请求计数 +1
	middleware.registry.RequestDuration.WithLabelValues(labels...).Observe(time.Since(startedAt).Seconds()) // 记录本次请求耗时（秒）
	middleware.registry.RequestsInFlight.Dec()                                                              // 进行中请求数 -1

	// 统一错误中间件已在上下文中记录错误码，这里据此累加错误指标。
	if errCode := c.GetString(response.ErrorCodeContextKey); errCode != "" { // 读取上下文错误码；非空表示返回统一错误
		middleware.registry.ResponseErrorsTotal.WithLabelValues(path, strconv.Itoa(status), errCode).Inc() // 按路由/状态/错误码累加错误计数
	}
}
