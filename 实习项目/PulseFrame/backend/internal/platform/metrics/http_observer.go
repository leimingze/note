package metrics

import (
	"strconv"
	"time"
)

// HTTPObserver 抽象 HTTP 请求指标观察入口，隐藏具体指标写入细节。
type HTTPObserver interface {
	// RequestStarted 通知一次请求开始，用于累计进行中请求数。
	RequestStarted()
	// RequestFinished 通知一次请求结束并记录耗时、状态码与错误码。
	RequestFinished(method string, path string, status int, duration time.Duration, errorCode string)
}

// httpObserver 是把 HTTP 请求事件写入 Registry 的观察器实现。
type httpObserver struct {
	registry *Registry // 指标写入目标。
}

// NewHTTPObserver 创建 HTTP 请求指标观察器。
// 输入：registry，指标注册表。
// 输出：可注入中间件的 HTTPObserver。
// 功能：集中把请求开始与结束事件换算为 Prometheus 指标更新。
// 例如 GET /ok 返回200且耗时100ms 时，累加请求计数并记录0.1秒耗时。
func NewHTTPObserver(registry *Registry) HTTPObserver {
	return &httpObserver{registry: registry}
}

// RequestStarted 在请求进入时增加进行中请求数。
// 输入：无。
// 输出：原子更新进行中请求 Gauge。
// 功能：配合 RequestFinished 保持进行中请求数准确。
func (observer *httpObserver) RequestStarted() {
	observer.registry.RequestsInFlight.Inc()
}

// RequestFinished 记录一次请求的计数、耗时与错误，并减少进行中请求数。
// 输入：method，请求方法；path，路由模板；status，响应状态码；duration，处理耗时；errorCode，统一错误码（可为空）。
// 输出：更新请求计数、耗时、错误指标及进行中请求数。
// 功能：把一次完整请求的运行结果同步到指标注册表。
// 例如 status=200 时累加对应标签请求计数；errorCode 非空时额外累加错误计数。
func (observer *httpObserver) RequestFinished(method string, path string, status int, duration time.Duration, errorCode string) {
	labels := []string{method, path, strconv.Itoa(status)}                                   // 组装 method/route/status 标签值
	observer.registry.RequestsTotal.WithLabelValues(labels...).Inc()                         // 给对应组合请求计数 +1
	observer.registry.RequestDuration.WithLabelValues(labels...).Observe(duration.Seconds()) // 记录本次请求耗时（秒）
	observer.registry.RequestsInFlight.Dec()                                                 // 进行中请求数 -1
	if errorCode != "" {
		observer.registry.ResponseErrorsTotal.WithLabelValues(path, strconv.Itoa(status), errorCode).Inc() // 按路由/状态/错误码累加错误计数
	}
}
