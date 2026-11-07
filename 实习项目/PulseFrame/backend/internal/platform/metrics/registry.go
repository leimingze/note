package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Registry 持有进程内 Prometheus 注册表及所有由基座维护的指标。
type Registry struct {
	// ===== HTTP 请求指标 =====

	// RequestsTotal 记录已完成 HTTP 请求的总数量；每次请求处理完就加1。
	// 用 method（方法）、route（路由模板）、status（状态码）三个维度拆分统计。
	RequestsTotal *prometheus.CounterVec

	// RequestDuration 记录 HTTP 请求处理耗时分布；用于观察延迟，例如 P50/P95/P99。
	// 同样按 method、route、status 拆分。
	RequestDuration *prometheus.HistogramVec

	// RequestsInFlight 记录当前正在处理的 HTTP 请求数量，反映服务并发负载。
	RequestsInFlight prometheus.Gauge

	// ResponseErrorsTotal 记录返回给客户端的统一错误数量；按路由、状态码、错误码统计。
	// 用于观察错误率分布。
	ResponseErrorsTotal *prometheus.CounterVec

	// ===== 外部依赖指标 =====

	// DependencyOperations 记录外部依赖（如 MySQL、Redis）的操作总次数。
	// 用 dependency（依赖名）、operation（操作名）、result（结果）拆分。
	DependencyOperations *prometheus.CounterVec

	// DependencyOperationDuration 记录外部依赖操作的耗时分布，用于观察依赖延迟。
	DependencyOperationDuration *prometheus.HistogramVec

	// DependencyErrors 记录外部依赖错误次数，按依赖名、操作名、错误类型统计。
	DependencyErrors *prometheus.CounterVec

	// DependencyReady 记录外部依赖最近一次可用性状态：1 表示可用，0 表示不可用。
	DependencyReady *prometheus.GaugeVec

	// ===== MySQL 连接池指标 =====

	// MySQLOpenConnections 记录 MySQL 当前打开的连接总数。
	MySQLOpenConnections prometheus.Gauge

	// MySQLInUseConnections 记录 MySQL 当前正被请求使用的连接数。
	MySQLInUseConnections prometheus.Gauge

	// MySQLIdleConnections 记录 MySQL 当前空闲、可被复用的连接数。
	MySQLIdleConnections prometheus.Gauge

	// MySQLWaitCountTotal 记录等待获取 MySQL 连接的累计次数（连接池耗尽时会发生等待）。
	MySQLWaitCountTotal prometheus.Counter

	// MySQLWaitDurationSecondsTotal 记录等待获取 MySQL 连接的累计秒数，反映连接池压力。
	MySQLWaitDurationSecondsTotal prometheus.Counter

	// ===== Redis 连接池指标 =====

	// RedisPoolTotalConnections 记录 Redis 连接池当前的连接总数。
	RedisPoolTotalConnections prometheus.Gauge

	// RedisPoolIdleConnections 记录 Redis 连接池当前的空闲连接数。
	RedisPoolIdleConnections prometheus.Gauge

	// RedisPoolTimeoutsTotal 记录获取 Redis 连接超时的累计次数。
	RedisPoolTimeoutsTotal prometheus.Counter

	// ===== 生命周期指标 =====

	// ProcessReady 记录 HTTP 服务是否已启动就绪：1 表示就绪，0 表示未就绪。
	ProcessReady prometheus.Gauge

	// StartupFailuresTotal 记录启动失败的累计次数（配置或依赖初始化失败）。
	StartupFailuresTotal prometheus.Counter

	// ShutdownTotal 记录服务进入退出流程的累计次数。
	ShutdownTotal prometheus.Counter

	registry *prometheus.Registry
}

// New 创建并注册全部指标。
// 输入：无。
// 输出：持有独立 Prometheus Registry 的指标注册表。
// 功能：集中定义基座各项指标及其标签，避免各模块直接操作全局注册表。
func New() *Registry {
	registry := prometheus.NewRegistry()
	metrics := &Registry{registry: registry}

	metrics.RequestsTotal = prometheus.NewCounterVec( // 创建请求计数指标
		prometheus.CounterOpts{Name: "pulseframe_http_requests_total", Help: "已完成 HTTP 请求数量"},
		[]string{"method", "route", "status"},
	)
	metrics.RequestDuration = prometheus.NewHistogramVec( // 创建请求耗时分布指标
		prometheus.HistogramOpts{Name: "pulseframe_http_request_duration_seconds", Help: "HTTP 请求处理耗时分布"},
		[]string{"method", "route", "status"},
	)
	metrics.RequestsInFlight = prometheus.NewGauge( // 创建进行中请求数指标
		prometheus.GaugeOpts{Name: "pulseframe_http_requests_in_flight", Help: "当前正在处理的 HTTP 请求数量"},
	)
	metrics.ResponseErrorsTotal = prometheus.NewCounterVec( // 创建错误计数指标
		prometheus.CounterOpts{Name: "pulseframe_http_response_errors_total", Help: "已返回的统一错误数量"},
		[]string{"route", "status", "error_code"},
	)

	metrics.DependencyOperations = prometheus.NewCounterVec( // 创建依赖操作计数指标
		prometheus.CounterOpts{Name: "pulseframe_dependency_operations_total", Help: "外部依赖操作数量"},
		[]string{"dependency", "operation", "result"},
	)
	metrics.DependencyOperationDuration = prometheus.NewHistogramVec( // 创建依赖耗时分布指标
		prometheus.HistogramOpts{Name: "pulseframe_dependency_operation_duration_seconds", Help: "外部依赖操作耗时分布"},
		[]string{"dependency", "operation", "result"},
	)
	metrics.DependencyErrors = prometheus.NewCounterVec( // 创建依赖错误计数指标
		prometheus.CounterOpts{Name: "pulseframe_dependency_errors_total", Help: "外部依赖错误数量"},
		[]string{"dependency", "operation", "error_type"},
	)
	metrics.DependencyReady = prometheus.NewGaugeVec( // 创建依赖可用性指标
		prometheus.GaugeOpts{Name: "pulseframe_dependency_ready", Help: "依赖最近一次可用性状态"},
		[]string{"dependency"},
	)

	metrics.MySQLOpenConnections = prometheus.NewGauge( // 创建 MySQL 打开连接数指标
		prometheus.GaugeOpts{Name: "pulseframe_mysql_open_connections", Help: "MySQL 当前打开连接数"},
	)
	metrics.MySQLInUseConnections = prometheus.NewGauge( // 创建 MySQL 使用中连接数指标
		prometheus.GaugeOpts{Name: "pulseframe_mysql_in_use_connections", Help: "MySQL 当前使用中的连接数"},
	)
	metrics.MySQLIdleConnections = prometheus.NewGauge( // 创建 MySQL 空闲连接数指标
		prometheus.GaugeOpts{Name: "pulseframe_mysql_idle_connections", Help: "MySQL 当前空闲连接数"},
	)
	metrics.MySQLWaitCountTotal = prometheus.NewCounter( // 创建 MySQL 等待次数指标
		prometheus.CounterOpts{Name: "pulseframe_mysql_wait_count_total", Help: "等待 MySQL 连接累计次数"},
	)
	metrics.MySQLWaitDurationSecondsTotal = prometheus.NewCounter( // 创建 MySQL 等待时长指标
		prometheus.CounterOpts{Name: "pulseframe_mysql_wait_duration_seconds_total", Help: "等待 MySQL 连接累计时长（秒）"},
	)
	metrics.RedisPoolTotalConnections = prometheus.NewGauge( // 创建 Redis 总连接数指标
		prometheus.GaugeOpts{Name: "pulseframe_redis_pool_total_connections", Help: "Redis 连接池连接总数"},
	)
	metrics.RedisPoolIdleConnections = prometheus.NewGauge( // 创建 Redis 空闲连接数指标
		prometheus.GaugeOpts{Name: "pulseframe_redis_pool_idle_connections", Help: "Redis 连接池空闲连接数"},
	)
	metrics.RedisPoolTimeoutsTotal = prometheus.NewCounter( // 创建 Redis 超时次数指标
		prometheus.CounterOpts{Name: "pulseframe_redis_pool_timeouts_total", Help: "获取 Redis 连接超时次数"},
	)

	metrics.ProcessReady = prometheus.NewGauge( // 创建进程就绪指标
		prometheus.GaugeOpts{Name: "pulseframe_process_ready", Help: "HTTP 服务是否已启动"},
	)
	metrics.StartupFailuresTotal = prometheus.NewCounter( // 创建启动失败次数指标
		prometheus.CounterOpts{Name: "pulseframe_startup_failures_total", Help: "配置或依赖初始化失败次数"},
	)
	metrics.ShutdownTotal = prometheus.NewCounter( // 创建退出次数指标
		prometheus.CounterOpts{Name: "pulseframe_shutdown_total", Help: "服务进入退出流程次数"},
	)

	registry.MustRegister( // 批量注册全部指标
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
	return metrics.registry // 返回进程内注册表
}
