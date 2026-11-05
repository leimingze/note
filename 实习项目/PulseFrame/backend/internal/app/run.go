package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"pulseframe/config"
	"pulseframe/logging"
	"pulseframe/metrics"
	"pulseframe/middleware"
	"pulseframe/server"
)

// Run 构建并运行核心 API。
// 输入：parent，服务生命周期父上下文；lookup，环境变量查询函数。
// 输出：创建 MySQL、Redis 客户端与指标、设置 Gin 模式并运行 HTTP 服务；退出时关闭外部连接，失败时返回错误。
// 功能：组织配置、依赖、基础路由及 HTTP 生命周期，保证初始化后的资源得到释放。
// 例如收到 SIGTERM 后，先等待 HTTP 请求处理结束，再关闭共享的 MySQL 和 Redis 客户端。
func Run(parent context.Context, lookup config.LookupEnv) (runErr error) {
	// 第一阶段只读取和校验配置，配置错误时不创建服务资源。
	cfg, err := config.Load(lookup)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 创建带公共字段的日志器，供路由和依赖模块复用。
	baseLogger, err := logging.New(cfg.LogLevel, os.Stdout)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	logger := baseLogger.With("service", "pulseframe-api", "environment", cfg.Environment)
	gin.SetMode(ginMode(cfg.Environment))

	dependencies, err := NewDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create dependencies: %w", err)
	}
	defer func() {
		runErr = errors.Join(runErr, dependencies.Close())
	}()

	health := server.NewHealthState()
	var metricsRegistry *metrics.Registry
	if cfg.Metrics.Enabled {
		metricsRegistry = dependencies.Metrics
	}
	router, err := server.NewRouter(server.RouterDependencies{
		Logger: logger, IDGenerator: middleware.CryptoIDGenerator{}, Health: health,
		Metrics: metricsRegistry,
	})
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}

	httpServer, err := server.New(server.OptionsFromConfig(cfg, router, health))
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	logger.Info("pulseframe api starting", "address", cfg.HTTP.Address, "environment", cfg.Environment)
	if err := httpServer.Run(ctx); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}
	logger.Info("pulseframe api stopped")
	return nil
}

// ginMode 将应用环境转换成 Gin 运行模式。
// 输入：environment，已经校验过的环境名称。
// 输出：Gin 支持的 debug、test 或 release 模式。
// 功能：集中维护项目环境与 Gin 模式之间的映射。
// 例如 test 对应测试模式，production 对应发布模式，local 对应调试模式。
func ginMode(environment string) string {
	switch environment {
	case config.EnvironmentTest:
		return gin.TestMode
	case config.EnvironmentStaging, config.EnvironmentProduction:
		return gin.ReleaseMode
	default:
		return gin.DebugMode
	}
}
