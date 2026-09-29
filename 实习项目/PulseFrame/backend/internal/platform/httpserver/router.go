package httpserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	httpmiddleware "pulseframe/internal/platform/httpserver/middleware"
	"pulseframe/internal/platform/httpserver/response"
)

var (
	errNilRouterLogger      = errors.New("router logger is required")
	errNilRouterHealth      = errors.New("router health state is required")
	errConfigureProxyPolicy = errors.New("configure trusted proxy policy")
)

// RouterDependencies 保存创建核心路由所需依赖。
type RouterDependencies struct {
	Logger      *slog.Logger
	IDGenerator httpmiddleware.IDGenerator
	Health      *HealthState
	Modules     []RouteRegistrar
}

// RouteRegistrar 定义业务模块注册 Gin HTTP 路由的边界。
type RouteRegistrar interface {
	RegisterRoutes(*gin.Engine)
}

// NewRouter 创建核心 API 的 Gin 路由。
// 输入：dependencies，日志器、请求标识生成器、健康状态和业务路由注册器。
// 输出：配置完成的 Gin Engine；依赖或中间件初始化失败时返回错误。
// 功能：集中注册全局中间件、基础路由和统一路由错误。
func NewRouter(dependencies RouterDependencies) (*gin.Engine, error) {
	if dependencies.Logger == nil {
		return nil, errNilRouterLogger
	}
	if dependencies.Health == nil {
		return nil, errNilRouterHealth
	}
	requestID, err := httpmiddleware.NewRequestID(dependencies.IDGenerator)
	if err != nil {
		return nil, err
	}
	accessLog, err := httpmiddleware.NewAccessLog(dependencies.Logger)
	if err != nil {
		return nil, err
	}
	recovery, err := httpmiddleware.NewRecovery(dependencies.Logger)
	if err != nil {
		return nil, err
	}

	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, errors.Join(errConfigureProxyPolicy, err)
	}
	router.HandleMethodNotAllowed = true
	router.Use(requestID.Handle, accessLog.Handle, recovery.Handle)
	registerPlatformRoutes(router, dependencies.Health)
	registerModuleRoutes(router, dependencies.Modules)
	registerRouteErrors(router)
	return router, nil
}

// registerModuleRoutes 按装配顺序注册业务模块 HTTP 路由。
// 输入：router，Gin 路由器；modules，启动时创建的模块注册器集合。
// 输出：修改 Gin 路由表，无返回值。
// 功能：避免平台路由包直接依赖具体业务模块。
func registerModuleRoutes(router *gin.Engine, modules []RouteRegistrar) {
	for _, module := range modules {
		module.RegisterRoutes(router)
	}
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
