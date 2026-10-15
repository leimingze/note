// 程序功能：读取基础配置，并启动 PulseFrame 核心 API。
// 启动命令：在 backend 目录执行 go run ./cmd/pulseframe-api；环境变量见 backend/README.md。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"pulseframe/config"
	"pulseframe/logging"
	"pulseframe/middleware"
	"pulseframe/server"
)

// main 是 PulseFrame 核心 API 的进程入口。
// 输入：进程环境变量和操作系统退出信号。
// 输出：启动 HTTP 服务；初始化或运行失败时记录错误并以非零状态退出。
// 功能：装配工程基础依赖并运行核心 API。
func main() {
	// 入口无法向调用方返回错误，因此记录根因并用非零状态通知进程管理器。
	if err := run(context.Background(), os.LookupEnv); err != nil {
		slog.Error("pulseframe api stopped with error", "error", err)
		os.Exit(1)
	}
}

// run 构建并运行核心 API。
// 输入：parent，服务生命周期父上下文；lookup，环境变量查询函数。
// 输出：服务正常退出时返回 nil，否则返回配置、初始化或运行错误。
// 功能：完成配置、基础路由、信号监听和 HTTP 服务的依赖装配。
func run(parent context.Context, lookup config.LookupEnv) error {
	// 第一阶段只读取和校验配置，配置错误时不创建服务资源。
	cfg, err := config.Load(lookup)
	// 配置不完整时立即返回，避免用错误参数启动监听。
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 日志器和 Gin 模式都只依赖基础配置。
	logger, err := logging.New(cfg.LogLevel, os.Stdout)
	// 日志器无效时不继续启动，确保后续故障都有可靠输出。
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	gin.SetMode(ginMode(cfg.Environment))

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
