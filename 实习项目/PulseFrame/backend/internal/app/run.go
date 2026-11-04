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
	"pulseframe/middleware"
	"pulseframe/server"
)

// Run 构建并运行核心 API。
// 输入：parent，服务生命周期父上下文；lookup，环境变量查询函数。
// 输出：创建 MySQL 和 Redis 客户端、设置 Gin 模式并运行 HTTP 服务；退出时关闭外部连接，失败时返回错误。
// 功能：组织配置、依赖、基础路由及 HTTP 生命周期，保证初始化后的资源得到释放。
// 例如收到 SIGTERM 后，先等待 HTTP 请求处理结束，再关闭共享的 MySQL 和 Redis 客户端。
func Run(parent context.Context, lookup config.LookupEnv) (runErr error) {
	// 第一阶段只读取和校验配置，配置错误时不创建服务资源。
	cfg, err := config.Load(lookup)
	// 配置不完整时立即返回，避免用错误参数启动监听。
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	// 退出信号或父上下文取消都会通知服务结束；函数返回时停止接收这些信号。
	defer stop()

	// 日志器和 Gin 模式都只依赖基础配置。
	logger, err := logging.New(cfg.LogLevel, os.Stdout)
	// 日志器无效时不继续启动，确保后续故障都有可靠输出。
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	gin.SetMode(ginMode(cfg.Environment))
	// 数据库启动检查通过后才创建 HTTP 服务，失败时直接结束启动流程。
	dependencies, err := NewDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create dependencies: %w", err)
	}
	// 立即登记清理，覆盖后续启动失败和正常退出；正常退出时 HTTP 已完成请求处理。
	defer func() {
		// 写入命名返回值，同时保留运行错误和关闭错误，关闭失败也能传回 main。
		runErr = errors.Join(runErr, dependencies.Close())
	}()

	// 基座只注册健康检查和通用 HTTP 中间件。
	health := server.NewHealthState()
	router, err := server.NewRouter(server.RouterDependencies{
		Logger: logger, IDGenerator: middleware.CryptoIDGenerator{}, Health: health,
	})
	// 路由依赖无效时不创建 HTTP Server。
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}

	httpServer, err := server.New(server.OptionsFromConfig(cfg, router, health))
	// Server 参数非法时仍处于未监听状态，可以安全终止启动。
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	// Run 会阻塞到服务异常或进程收到退出信号。
	logger.Info("pulseframe api starting", "address", cfg.HTTP.Address, "environment", cfg.Environment)
	// 正常信号退出返回 nil，只有监听或关闭故障才作为启动命令失败。
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
	// 测试和部署环境使用各自模式，本地环境保留调试输出。
	switch environment {
	case config.EnvironmentTest:
		return gin.TestMode
	case config.EnvironmentStaging, config.EnvironmentProduction:
		return gin.ReleaseMode
	default:
		return gin.DebugMode
	}
}
