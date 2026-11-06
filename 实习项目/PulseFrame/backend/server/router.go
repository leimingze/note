package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"pulseframe/metrics"
	"pulseframe/middleware"
	"pulseframe/response"
)

var (
	// errNilRouterLogger 表示路由缺少访问日志和异常恢复共用的日志器。
	errNilRouterLogger = errors.New("router logger is required")
	// errNilRouterHealth 表示路由缺少就绪探针读取的健康状态。
	errNilRouterHealth = errors.New("router health state is required")
	// errConfigureProxyPolicy 标记 Gin 可信代理策略配置失败。
	errConfigureProxyPolicy = errors.New("configure trusted proxy policy")
)

// RouterDependencies 保存创建核心路由所需依赖。
type RouterDependencies struct {
	Logger      *slog.Logger           // 访问日志和异常记录共用的日志器。
	IDGenerator middleware.IDGenerator // 请求未携带合法标识时，用它生成新标识。
	Health      *HealthState           // 生命周期更新、就绪接口读取的共享状态。
	Metrics     *metrics.Registry      // 指标注册表，非空时注册指标端点与中间件。
}

// NewRouter 创建核心 API 的 Gin 路由。
// 输入：dependencies，日志器、请求标识生成器、健康状态和指标注册表。
// 输出：配置完成的 Gin Engine；依赖或中间件初始化失败时返回错误。
// 功能：集中注册全局中间件、基础路由、指标端点和统一路由错误。
// 例如请求 GET /livez 会经过请求标识、日志、异常恢复和指标中间件，再返回存活状态。
func NewRouter(dependencies RouterDependencies) (*gin.Engine, error) {
	// 全局访问日志和异常恢复都依赖同一个结构化日志器。
	if dependencies.Logger == nil {
		return nil, errNilRouterLogger
	}
	// 就绪路由必须读取共享生命周期状态，缺失时不能构造路由。
	if dependencies.Health == nil {
		return nil, errNilRouterHealth
	}
	// 先构造所有中间件，避免路由注册一半后才发现依赖缺失。
	requestID, err := middleware.NewRequestID(dependencies.IDGenerator) // 传入 ID 生成器；返回请求标识中间件、错误
	// 请求 ID 中间件失败时，后续日志无法可靠关联请求。
	if err != nil {
		return nil, err
	}
	accessLog, err := middleware.NewAccessLog(dependencies.Logger) // 传入日志器；返回访问日志中间件、错误
	// 访问日志中间件失败时停止构造，避免实例缺少审计日志。
	if err != nil {
		return nil, err
	}
	recovery, err := middleware.NewRecovery(dependencies.Logger) // 传入日志器；返回异常恢复中间件、错误
	// 异常恢复中间件失败时不能暴露一个会被 panic 中断的路由器。
	if err != nil {
		return nil, err
	}

	// 不使用 gin.Default，确保中间件及日志格式完全由项目控制。
	router := gin.New() // 无参数；返回空的 Gin 引擎
	// 不信任请求头中代理转发的 IP；后续读取客户端地址时以连接来源为准。
	if err := router.SetTrustedProxies(nil); err != nil { // 传入空可信代理列表；失败时返回错误
		return nil, errors.Join(errConfigureProxyPolicy, err)
	}
	router.HandleMethodNotAllowed = true // 开启未允许方法的统一处理
	// 请求 ID 必须最先写入，后续日志和异常响应才能引用它。
	handlers := []gin.HandlerFunc{requestID.Handle, accessLog.Handle, recovery.Handle} // 组装基础中间件列表
	if dependencies.Metrics != nil {
		// 指标中间件紧邻日志中间件，记录请求数量、耗时和错误。
		handlers = append(handlers, middleware.NewMetrics(dependencies.Metrics).Handle) // 传入注册表创建指标中间件并追加
	}
	router.Use(handlers...)                                                   // 按顺序挂载所有全局中间件
	registerPlatformRoutes(router, dependencies.Health, dependencies.Metrics) // 传入路由、健康状态、注册表注册基础路由
	registerRouteErrors(router)                                               // 传入路由注册统一错误处理
	return router, nil
}

// registerPlatformRoutes 注册工程基础接口。
// 输入：router，Gin 路由；health，共享健康状态；metricsRegistry，指标注册表（可为空）。
// 输出：修改路由注册表，无返回值。
// 功能：提供存活、就绪探针和指标端点，不注册任何业务接口。
// 例如注册后可通过 GET /livez 查询进程存活状态，通过 GET /readyz 查询就绪状态。
func registerPlatformRoutes(router *gin.Engine, health *HealthState, metricsRegistry *metrics.Registry) {
	ready := readyHandler{health: health} // 组装就绪处理器
	router.GET("/livez", handleLive)      // 注册存活探针路由
	router.GET("/readyz", ready.handle)   // 注册就绪探针路由
	if metricsRegistry != nil {
		// 指标端点使用进程内注册表，不依赖全局默认注册表。
		router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metricsRegistry.Prometheus(), promhttp.HandlerOpts{}))) // 注册指标端点，读取注册表输出 Prometheus 格式
	}
}

// registerRouteErrors 注册未找到路由和方法不允许错误。
// 输入：router，Gin 路由。
// 输出：修改路由错误处理器，无返回值。
// 功能：保证框架级路由错误也使用统一响应结构。
// 例如 GET /missing 返回 JSON 格式的404错误，POST /livez 返回 JSON 格式的405错误。
func registerRouteErrors(router *gin.Engine) {
	router.NoRoute(routeNotFound)     // 注册未匹配路由的 404 处理
	router.NoMethod(methodNotAllowed) // 注册方法不允许的 405 处理
}

// routeNotFound 返回统一的资源不存在错误。
// 输入：c，当前 Gin 请求上下文。
// 输出：写入 404 错误并终止请求链路。
// 功能：把未注册路径的请求转换为统一的 JSON 错误响应。
// 例如 GET /missing 返回404，错误码为 RESOURCE_NOT_FOUND。
func routeNotFound(c *gin.Context) {
	response.WriteError(c, response.ErrorSpec{ // 传入上下文与错误规格，写入 404 响应
		Status:  http.StatusNotFound,
		Code:    response.CodeNotFound,
		Message: "resource not found",
	})
}

// methodNotAllowed 返回统一的方法不允许错误。
// 输入：c，当前 Gin 请求上下文。
// 输出：写入 405 错误并终止请求链路。
// 功能：让客户端能够稳定区分路径不存在和请求方法错误。
// 例如 /livez 只支持 GET，使用 POST 访问时返回405和 METHOD_NOT_ALLOWED。
func methodNotAllowed(c *gin.Context) {
	response.WriteError(c, response.ErrorSpec{ // 传入上下文与错误规格，写入 405 响应
		Status:  http.StatusMethodNotAllowed,
		Code:    response.CodeMethodNotAllowed,
		Message: "method not allowed",
	})
}
