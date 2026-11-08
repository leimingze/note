package app

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/metrics"
	"pulseframe/internal/platform/middleware"
	"pulseframe/internal/platform/server"
)

// APIRuntime 持有应用运行所需的 Gin 路由与共享健康状态。
type APIRuntime struct {
	Router *gin.Engine         // 配置完成的 HTTP 路由。
	Health *server.HealthState // 生命周期更新、就绪接口读取的共享状态。
}

// 输入：logger，带公共字段的结构化日志器；cfg，应用配置；dependencies，共享外部依赖。
// 输出：持有路由与健康状态的运行组件；初始化失败时返回错误。
// 功能：集中创建健康状态、在指标启用时选择注册表，并组装核心路由。
// 例如指标配置开启时，返回的运行组件会挂载指标中间件并注册 /metrics 端点。
func NewRuntime(logger *slog.Logger, cfg config.Config, dependencies *Dependencies) (*APIRuntime, error) {
	// 首先创建共享健康状态；后续就绪接口读取同一实例。
	health := server.NewHealthState() // 无参数；返回 health(初始未就绪的健康状态)
	// 指标启用时取出共享注册表传给路由；否则传 nil，不注册指标端点。
	var metricsRegistry *metrics.Registry
	if cfg.Metrics.Enabled {
		metricsRegistry = dependencies.Metrics // 取出共享的指标注册表，传给路由使用
	}
	// 组装核心路由；路由初始化失败时返回带阶段的错误。
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      logger,                         // 传入带公共字段的日志器，供访问日志和异常恢复使用
		IDGenerator: middleware.CryptoIDGenerator{}, // 传入随机请求 ID 生成器
		Health:      health,                         // 传入共享健康状态
		Metrics:     metricsRegistry,                // 传入指标注册表（未启用时为 nil，不注册指标端点）
		Liveness:    dependencies.Ping,              // 传入依赖探测函数，让 /livez 校验 MySQL 与 Redis 连通性
	}) // 返回 router(配置好的 Gin 路由)、err(错误)
	if err != nil {
		return nil, fmt.Errorf("initialize router: %w", err)
	}
	return &APIRuntime{Router: router, Health: health}, nil
}
