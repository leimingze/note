package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/metrics"
	"pulseframe/internal/platform/response"
)

// MetricsMiddleware 记录 HTTP 请求指标。
type MetricsMiddleware struct {
	observer metrics.HTTPObserver // 记录请求指标的观察器。
}

// NewMetrics 创建指标中间件。
// 输入：observer，HTTP 请求指标观察器。
// 输出：可注册到 Gin 的中间件。
// 功能：保存观察器，供每次请求调用记录数量、耗时和进行中请求数。
func NewMetrics(observer metrics.HTTPObserver) *MetricsMiddleware {
	return &MetricsMiddleware{observer: observer} // 传入观察器；返回指标中间件
}

// Handle 记录当前 HTTP 请求的指标。
// 输入：c，当前 Gin 请求上下文。
// 输出：通过观察器更新请求指标并继续请求链路。
// 功能：在请求进入时通知观察器记录开始，完成时传递耗时、状态与错误码记录结束。
func (middleware *MetricsMiddleware) Handle(c *gin.Context) {
	middleware.observer.RequestStarted() // 通知观察器请求开始，进行中请求数 +1
	startedAt := time.Now()              // 记录请求开始时间
	c.Next()                             // 交给后续中间件和路由处理请求

	path := c.FullPath() // 获取注册的路由模板
	if path == "" {
		path = "/unknown" // 未匹配路由统一记为 /unknown
	}
	// 统一错误中间件已在上下文中记录错误码，这里读取后传给观察器。
	errCode := c.GetString(response.ErrorCodeContextKey)                                                           // 读取上下文错误码；非空表示返回统一错误
	middleware.observer.RequestFinished(c.Request.Method, path, c.Writer.Status(), time.Since(startedAt), errCode) // 通知观察器请求结束并记录指标
}
