package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"pulseframe/internal/modules/account/application"
	"pulseframe/internal/modules/account/httpapi"
	"pulseframe/internal/modules/account/persistence"
	"pulseframe/internal/modules/account/secure"
	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/httpserver"
	httpmiddleware "pulseframe/internal/platform/httpserver/middleware"
	"pulseframe/internal/platform/logging"
	"pulseframe/internal/platform/storage"
)

const schemaCheckTimeout = 5 * time.Second

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
// 输出：服务正常退出且依赖关闭成功时返回 nil，否则返回配置、初始化、运行或清理错误。
// 功能：完成配置、MySQL/Redis、模块路由、信号监听和 HTTP 服务的依赖装配。
func run(parent context.Context, lookup config.LookupEnv) (runErr error) {
	cfg, err := config.Load(lookup)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger, err := logging.New(cfg.LogLevel, os.Stdout)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	gin.SetMode(ginMode(cfg.Environment))

	runtime, err := newRuntime(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect authentication dependencies: %w", err)
	}
	defer func() {
		runErr = errors.Join(runErr, runtime.Close())
	}()

	health := httpserver.NewHealthState()
	router, err := newBusinessRouter(ctx, businessRouterOptions{
		Config: cfg, Logger: logger, Health: health, Runtime: runtime,
	})
	if err != nil {
		return fmt.Errorf("create application router: %w", err)
	}

	server, err := httpserver.New(httpserver.OptionsFromConfig(cfg, router, health))
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	logger.Info("pulseframe api starting", "address", cfg.HTTP.Address, "environment", cfg.Environment)
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}
	logger.Info("pulseframe api stopped")
	return nil
}

type runtimeDependencies struct {
	database *gorm.DB
	redis    *redis.Client
}

// newRuntime 连接认证模块使用的 MySQL 和 Redis。
// 输入：ctx，服务启动上下文；cfg，已完成边界校验的应用配置。
// 输出：持有可用外部连接的运行时依赖；连接或 Ping 失败时关闭已创建资源并返回错误。
// 功能：让服务在外部认证依赖未就绪时拒绝启动。
func newRuntime(ctx context.Context, cfg config.Config) (*runtimeDependencies, error) {
	database, err := storage.OpenMySQL(ctx, cfg.MySQL.DSN)
	if err != nil {
		return nil, err
	}
	redisClient, err := storage.OpenRedis(ctx, cfg.Redis)
	if err != nil {
		return nil, errors.Join(err, storage.CloseMySQL(database))
	}
	return &runtimeDependencies{database: database, redis: redisClient}, nil
}

// Close 释放运行时持有的外部连接。
// 输入：无。
// 输出：MySQL 或 Redis 关闭失败时返回组合错误。
// 功能：在 API 退出时关闭认证模块连接池。
func (runtime *runtimeDependencies) Close() error {
	return errors.Join(storage.CloseRedis(runtime.redis), storage.CloseMySQL(runtime.database))
}

// businessRouterOptions 保存账号 HTTP 路由组装所需的配置和运行依赖。
type businessRouterOptions struct {
	Config  config.Config
	Logger  *slog.Logger
	Health  *httpserver.HealthState
	Runtime *runtimeDependencies
}

// newBusinessRouter 装配账号应用和 HTTP 路由。
// 输入：ctx，启动生命周期上下文；options，认证配置、日志器、健康状态和已连接外部依赖。
// 输出：注册平台及账号路由的 Gin Engine；装配失败时返回错误。
// 功能：在进程入口明确组合账号领域、持久化与 HTTP 适配实现。
func newBusinessRouter(ctx context.Context, options businessRouterOptions) (*gin.Engine, error) {
	users, err := persistence.NewUserRepository(options.Runtime.database)
	if err != nil {
		return nil, err
	}
	schemaContext, cancel := context.WithTimeout(ctx, schemaCheckTimeout)
	defer cancel()
	if err := users.CheckSchema(schemaContext); err != nil {
		return nil, err
	}
	sessions, err := persistence.NewRedisSessionManager(options.Runtime.redis)
	if err != nil {
		return nil, err
	}
	rateLimiter, err := persistence.NewRedisRateLimiter(options.Runtime.redis)
	if err != nil {
		return nil, err
	}
	passwords, err := secure.NewArgon2id(options.Config.Auth.HashConcurrency)
	if err != nil {
		return nil, err
	}
	service, err := application.NewService(application.Dependencies{
		Users: users, Sessions: sessions, RateLimiter: rateLimiter, Passwords: passwords,
		RatePolicy: application.RatePolicy{
			LoginIPMaximum: options.Config.Auth.LoginIPLimit, LoginIdentityIPMaximum: options.Config.Auth.LoginIdentityIPLimit,
			RegistrationIPMaximum: options.Config.Auth.RegistrationIPLimit,
			LoginWindow:           options.Config.Auth.LoginLimitWindow, RegistrationWindow: options.Config.Auth.RegistrationLimitWindow,
		},
	})
	if err != nil {
		return nil, err
	}
	handler, err := httpapi.NewHandler(service, options.Config.Auth.WebOrigin, options.Config.Auth.CookieSecure)
	if err != nil {
		return nil, err
	}
	return httpserver.NewRouter(httpserver.RouterDependencies{
		Logger: options.Logger, IDGenerator: httpmiddleware.CryptoIDGenerator{}, Health: options.Health,
		Modules: []httpserver.RouteRegistrar{handler},
	})
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
