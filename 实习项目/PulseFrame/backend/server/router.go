package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

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
	Logger      *slog.Logger
	IDGenerator middleware.IDGenerator
	Health      *HealthState
}

// NewRouter 创建核心 API 的 Gin 路由。
// 输入：dependencies，日志器、请求标识生成器和健康状态。
// 输出：配置完成的 Gin Engine；依赖或中间件初始化失败时返回错误。
// 功能：集中注册全局中间件、基础路由和统一路由错误。
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
	requestID, err := middleware.NewRequestID(dependencies.IDGenerator)
	// 请求 ID 中间件失败时，后续日志无法可靠关联请求。
	if err != nil {
		return nil, err
	}
	accessLog, err := middleware.NewAccessLog(dependencies.Logger)
	// 访问日志中间件失败时停止构造，避免实例缺少审计日志。
	if err != nil {
		return nil, err
	}
	recovery, err := middleware.NewRecovery(dependencies.Logger)
	// 异常恢复中间件失败时不能暴露一个会被 panic 中断的路由器。
	if err != nil {
		return nil, err
	}

	// 不使用 gin.Default，确保中间件及日志格式完全由项目控制。
	router := gin.New()
	// 禁用默认代理信任，确保访问日志中的客户端地址来自实际连接。
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, errors.Join(errConfigureProxyPolicy, err)
	}
	router.HandleMethodNotAllowed = true
	// 请求 ID 必须最先写入，后续日志和异常响应才能引用它。
	router.Use(requestID.Handle, accessLog.Handle, recovery.Handle)
	registerPlatformRoutes(router, dependencies.Health)
	registerRouteErrors(router)
	return router, nil
}

// registerPlatformRoutes 注册工程基础接口。
// 输入：router，Gin 路由；health，共享健康状态。
// 输出：修改路由注册表，无返回值。
// 功能：提供存活和就绪探针，不注册任何业务接口。
func registerPlatformRoutes(router *gin.Engine, health *HealthState) {
	ready := readyHandler{health: health}
	router.GET("/livez", handleLive)
	router.GET("/readyz", ready.handle)
}

// registerRouteErrors 注册未找到路由和方法不允许错误。
// 输入：router，Gin 路由。
// 输出：修改路由错误处理器，无返回值。
// 功能：保证框架级路由错误也使用统一响应结构。
func registerRouteErrors(router *gin.Engine) {
	router.NoRoute(routeNotFound)
	router.NoMethod(methodNotAllowed)
}

// routeNotFound 返回统一的资源不存在错误。
// 输入：c，当前 Gin 请求上下文。
// 输出：写入 404 错误并终止请求链路。
// 功能：避免 Gin 默认文本响应破坏错误契约。
func routeNotFound(c *gin.Context) {
	response.WriteError(c, response.ErrorSpec{
		Status:  http.StatusNotFound,
		Code:    response.CodeNotFound,
		Message: "resource not found",
	})
}

// methodNotAllowed 返回统一的方法不允许错误。
// 输入：c，当前 Gin 请求上下文。
// 输出：写入 405 错误并终止请求链路。
// 功能：让客户端能够稳定区分路径不存在和请求方法错误。
func methodNotAllowed(c *gin.Context) {
	response.WriteError(c, response.ErrorSpec{
		Status:  http.StatusMethodNotAllowed,
		Code:    response.CodeMethodNotAllowed,
		Message: "method not allowed",
	})
}
