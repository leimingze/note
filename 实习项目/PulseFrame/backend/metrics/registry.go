package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Registry 持有进程内 Prometheus 注册表及所有由基座维护的指标。
type Registry struct {
	// HTTP 指标
	RequestsTotal       *prometheus.CounterVec
	RequestDuration     *prometheus.HistogramVec
	RequestsInFlight    prometheus.Gauge
	ResponseErrorsTotal *prometheus.CounterVec

	// 依赖指标
	DependencyOperations        *prometheus.CounterVec
	DependencyOperationDuration *prometheus.HistogramVec
	DependencyErrors            *prometheus.CounterVec
	DependencyReady             *prometheus.GaugeVec

	// 连接池指标
	MySQLOpenConnections          prometheus.Gauge
	MySQLInUseConnections         prometheus.Gauge
	MySQLIdleConnections          prometheus.Gauge
	MySQLWaitCountTotal           prometheus.Counter
	MySQLWaitDurationSecondsTotal prometheus.Counter
	RedisPoolTotalConnections     prometheus.Gauge
	RedisPoolIdleConnections      prometheus.Gauge
	RedisPoolTimeoutsTotal        prometheus.Counter

	// 生命周期指标
	ProcessReady         prometheus.Gauge
	StartupFailuresTotal prometheus.Counter
	ShutdownTotal        prometheus.Counter

	registry *prometheus.Registry
}

// New 创建并注册全部指标。
// 输入：无。
// 输出：持有独立 Prometheus Registry 的指标注册表。
// 功能：集中定义基座各项指标及其标签，避免各模块直接操作全局注册表。
func New() *Registry {
	registry := prometheus.NewRegistry()
	metrics := &Registry{registry: registry}

	metrics.RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "pulseframe_http_requests_total", Help: "已完成 HTTP 请求数量"},
		[]string{"method", "route", "status"},
	)
	metrics.RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "pulseframe_http_request_duration_seconds", Help: "HTTP 请求处理耗时分布"},
		[]string{"method", "route", "status"},
	)
	metrics.RequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_http_requests_in_flight", Help: "当前正在处理的 HTTP 请求数量"},
	)
	metrics.ResponseErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "pulseframe_http_response_errors_total", Help: "已返回的统一错误数量"},
		[]string{"route", "status", "error_code"},
	)

	metrics.DependencyOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "pulseframe_dependency_operations_total", Help: "外部依赖操作数量"},
		[]string{"dependency", "operation", "result"},
	)
	metrics.DependencyOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "pulseframe_dependency_operation_duration_seconds", Help: "外部依赖操作耗时分布"},
		[]string{"dependency", "operation", "result"},
	)
	metrics.DependencyErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "pulseframe_dependency_errors_total", Help: "外部依赖错误数量"},
		[]string{"dependency", "operation", "error_type"},
	)
	metrics.DependencyReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "pulseframe_dependency_ready", Help: "依赖最近一次可用性状态"},
		[]string{"dependency"},
	)

	metrics.MySQLOpenConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_mysql_open_connections", Help: "MySQL 当前打开连接数"},
	)
	metrics.MySQLInUseConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_mysql_in_use_connections", Help: "MySQL 当前使用中的连接数"},
	)
	metrics.MySQLIdleConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_mysql_idle_connections", Help: "MySQL 当前空闲连接数"},
	)
	metrics.MySQLWaitCountTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "pulseframe_mysql_wait_count_total", Help: "等待 MySQL 连接累计次数"},
	)
	metrics.MySQLWaitDurationSecondsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "pulseframe_mysql_wait_duration_seconds_total", Help: "等待 MySQL 连接累计时长（秒）"},
	)
	metrics.RedisPoolTotalConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_redis_pool_total_connections", Help: "Redis 连接池连接总数"},
	)
	metrics.RedisPoolIdleConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_redis_pool_idle_connections", Help: "Redis 连接池空闲连接数"},
	)
	metrics.RedisPoolTimeoutsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "pulseframe_redis_pool_timeouts_total", Help: "获取 Redis 连接超时次数"},
	)

	metrics.ProcessReady = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "pulseframe_process_ready", Help: "HTTP 服务是否已启动"},
	)
	metrics.StartupFailuresTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "pulseframe_startup_failures_total", Help: "配置或依赖初始化失败次数"},
	)
	metrics.ShutdownTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "pulseframe_shutdown_total", Help: "服务进入退出流程次数"},
	)

	registry.MustRegister(
		metrics.RequestsTotal, metrics.RequestDuration, metrics.RequestsInFlight, metrics.ResponseErrorsTotal,
		metrics.DependencyOperations, metrics.DependencyOperationDuration, metrics.DependencyErrors, metrics.DependencyReady,
		metrics.MySQLOpenConnections, metrics.MySQLInUseConnections, metrics.MySQLIdleConnections,
		metrics.MySQLWaitCountTotal, metrics.MySQLWaitDurationSecondsTotal,
		metrics.RedisPoolTotalConnections, metrics.RedisPoolIdleConnections, metrics.RedisPoolTimeoutsTotal,
		metrics.ProcessReady, metrics.StartupFailuresTotal, metrics.ShutdownTotal,
	)
	return metrics
}

// Prometheus 返回进程内注册表，供 promhttp 处理器读取。
// 输入：无。
// 输出：Prometheus Registry 实例。
// 功能：让指标端点读取已注册指标，而不依赖全局默认注册表。
func (metrics *Registry) Prometheus() *prometheus.Registry {
	return metrics.registry
}
