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

// Run 是服务的总入口：负责把配置、日志、外部依赖、路由和 HTTP 服务按顺序组装起来，并一直运行到收到退出信号。
// 输入：parent，控制服务生命周期的父上下文；lookup，读环境变量的函数（生产传 os.LookupEnv）。
// 输出：正常启动并运行时会阻塞，直到收到退出信号才返回 nil；任一阶段失败会返回带阶段的错误。
// 功能：整个启动过程按“先配置、再日志、再依赖、再路由、再运行、最后清理”的顺序推进，
// 确保任何一步失败都能立刻停下来并清理已经创建的资源。
// 例如收到 SIGTERM 时，服务会先等正在处理的 HTTP 请求完成，再关闭 MySQL/Redis，最后返回。
func Run(parent context.Context, lookup config.LookupEnv) (runErr error) {

	// ========== 第 1 步：读取并校验配置 ==========
	// 配置是后面所有东西的基础，先把它全部读出来并校验。
	// 如果配置有错，立刻返回，不创建任何资源。
	cfg, err := config.Load(lookup)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ========== 第 2 步：准备一个能响应退出信号的上下文 ==========
	// signal.NotifyContext 让 ctx 在收到 Ctrl+C 或 SIGTERM 时自动取消。
	// 后面 HTTP 服务和采集器都用这个 ctx，所以退出信号一响，它们就会跟着停止。
	// 返回的 stop 必须 defer 调用，否则信号监听会一直烙在进程里。
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ========== 第 3 步：创建日志器 ==========
	// 先建日志器，是因为后面任何一步失败，都需要有日志器来记录错误。
	// logging.New 只做最基础的 JSON 日志器，再通过 With 附加公共字段（service、environment），
	// 这样所有日志都自动带上这两个字段，方便采集检索。
	baseLogger, err := logging.New(cfg.LogLevel, os.Stdout)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	logger := baseLogger.With("service", "pulseframe-api", "environment", cfg.Environment)
	// 根据启动环境设置 Gin 的运行模式（调试/测试/发布）。
	gin.SetMode(ginMode(cfg.Environment))

	// ========== 第 4 步：创建外部依赖（MySQL/Redis/指标） ==========
	// NewDependencies 负责连接 MySQL、Redis，并在指标启用时启动连接池采集器。
	// 这一步失败就直接返回；成功后 defer 登记关闭，保证无论后续成功还是失败，资源都会被释放。
	dependencies, err := NewDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create dependencies: %w", err)
	}
	defer func() {
		// 把之前的错误和关闭资源的错误合并，这样退出时如果关闭失败也能带到 main。
		runErr = errors.Join(runErr, dependencies.Close())
	}()

	// ========== 第 5 步：创建健康状态与路由 ==========
	// 健康状态 /livez、/readyz 共用；指标启用时把注册表传给路由，
	// 让路由挂载指标中间件并注册 /metrics 端点。
	health := server.NewHealthState()
	var metricsRegistry *metrics.Registry
	if cfg.Metrics.Enabled {
		metricsRegistry = dependencies.Metrics
	}
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      logger,
		IDGenerator: middleware.CryptoIDGenerator{},
		Health:      health,
		Metrics:     metricsRegistry,
	})
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}

	// ========== 第 6 步：创建 HTTP 服务并运行 ==========
	// 把路由、健康状态、超时配置组装成 HTTP 服务。
	httpServer, err := server.New(server.OptionsFromConfig(cfg, router, health))
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	// 打印启动日志，然后阻塞运行。Run 会一直卡在这里，直到收到退出信号或服务出错。
	logger.Info("pulseframe api starting", "address", cfg.HTTP.Address, "environment", cfg.Environment)
	if err := httpServer.Run(ctx); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}
	logger.Info("pulseframe api stopped")
	return nil
}

// ginMode 把应用环境转换成 Gin 的运行模式。
// 输入：environment，已校验的环境名（local/test/staging/production）。
// 输出：对应的 Gin 模式（debug/test/release）。
// 功能：test 用测试模式，staging/production 用发布模式，其余（local）用调试模式。
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
