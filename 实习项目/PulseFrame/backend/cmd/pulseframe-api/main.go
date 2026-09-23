package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/httpserver"
	httpmiddleware "pulseframe/internal/platform/httpserver/middleware"
	"pulseframe/internal/platform/logging"
)

// main 是 PulseFrame 核心 API 的进程入口。
// 输入：进程环境变量和操作系统退出信号。
// 输出：启动 HTTP 服务；初始化或运行失败时记录错误并以非零状态退出。
// 功能：装配工程基础依赖并运行核心 API。
func main() {
	if err := run(context.Background(), os.LookupEnv); err != nil {
		slog.Error("pulseframe api stopped with error", "error", err)
		os.Exit(1)
	}
}

// run 构建并运行核心 API。
// 输入：parent，服务生命周期父上下文；lookup，环境变量查询函数。
// 输出：服务正常退出时返回 nil，配置、初始化或运行失败时返回错误。
// 功能：完成配置、日志、路由、信号监听和 HTTP 服务的依赖装配。
func run(parent context.Context, lookup config.LookupEnv) error {
	cfg, err := config.Load(lookup)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := logging.New(cfg.LogLevel, os.Stdout)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	gin.SetMode(ginMode(cfg.Environment))

	health := httpserver.NewHealthState()
	router, err := httpserver.NewRouter(httpserver.RouterDependencies{
		Logger:      logger,
		IDGenerator: httpmiddleware.CryptoIDGenerator{},
		Health:      health,
	})
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}

	server, err := httpserver.New(httpserver.OptionsFromConfig(cfg, router, health))
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info("pulseframe api starting", "address", cfg.HTTP.Address, "environment", cfg.Environment)
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}
	logger.Info("pulseframe api stopped")
	return nil
}

// ginMode 将运行环境映射为 Gin 模式。
// 输入：environment，已经通过配置校验的运行环境名称。
// 输出：Gin 支持的 debug、test 或 release 模式。
// 功能：避免 Gin 的运行模式判断散落在启动流程中。
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
