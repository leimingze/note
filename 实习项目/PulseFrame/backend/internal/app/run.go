package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/logging"
	"pulseframe/internal/platform/metrics"
	"pulseframe/internal/platform/middleware"
	"pulseframe/internal/platform/server"
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
	cfg, err := config.Load(lookup) // 传入 lookup(读环境变量的函数)；返回 cfg(完整配置)、err(错误)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ========== 第 2 步：准备一个能响应退出信号的上下文 ==========
	// signal.NotifyContext 让 ctx 在收到 Ctrl+C 或 SIGTERM 时自动取消。
	// 后面 HTTP 服务和采集器都用这个 ctx，所以退出信号一响，它们就会跟着停止。
	// 返回的 stop 必须 defer 调用，否则信号监听会一直烙在进程里。
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM) // 传入 parent(父上下文)、退出信号集合；返回 ctx(可取消上下文)、stop(取消监听函数)
	defer stop()

	// ========== 第 3 步：创建日志器 ==========
	// 先建日志器，是因为后面任何一步失败，都需要有日志器来记录错误。
	// logging.New 只做最基础的 JSON 日志器，再通过 With 附加公共字段（service、environment），
	// 这样所有日志都自动带上这两个字段，方便采集检索。
	baseLogger, err := logging.New(cfg.LogLevel, os.Stdout) // 传入 cfg.LogLevel(日志级别)、os.Stdout(输出目标)；返回 baseLogger(JSON日志器)、err(错误)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	logger := baseLogger.With("service", "pulseframe-api", "environment", cfg.Environment) // 传入公共字段键值对；返回 logger(带公共字段的日志器)
	// 根据启动环境设置 Gin 的运行模式（调试/测试/发布）。
	gin.SetMode(ginMode(cfg.Environment)) // ginMode 传入 cfg.Environment(环境名)返回 Gin模式；SetMode 用该模式设置 Gin 全局状态

	// ========== 第 4 步：创建外部依赖（MySQL/Redis/指标） ==========
	// NewDependencies 负责连接 MySQL、Redis，并在指标启用时启动连接池采集器。
	// 这一步失败就直接返回；成功后 defer 登记关闭，保证无论后续成功还是失败，资源都会被释放。
	dependencies, err := NewDependencies(ctx, cfg) // 传入 ctx(可取消上下文)、cfg(完整配置)；返回 dependencies(共享依赖)、err(错误)
	if err != nil {
		return fmt.Errorf("create dependencies: %w", err)
	}
	defer func() {
		// 把之前的错误和关闭资源的错误合并，这样退出时如果关闭失败也能带到 main。
		runErr = errors.Join(runErr, dependencies.Close()) // 传入 runErr(旧错误)、dependencies.Close()(关闭错误)；返回合并后的错误
	}()

	// ========== 第 5 步：创建健康状态与路由 ==========
	// 就绪状态由 Health 记录，存活探针通过 Liveness 校验 MySQL/Redis 连通性；
	// 指标启用时把注册表传给路由，挂载指标中间件并注册 /metrics 端点。
	health := server.NewHealthState() // 无参数；返回 health(初始未就绪的健康状态)
	var metricsRegistry *metrics.Registry
	if cfg.Metrics.Enabled {
		metricsRegistry = dependencies.Metrics // 取出共享的指标注册表，传给路由使用
	}
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      logger,                         // 传入带公共字段的日志器，供访问日志和异常恢复使用
		IDGenerator: middleware.CryptoIDGenerator{}, // 传入随机请求 ID 生成器
		Health:      health,                         // 传入共享健康状态
		Metrics:     metricsRegistry,                // 传入指标注册表（未启用时为 nil，不注册指标端点）
		Liveness:    dependencies.Ping,              // 传入依赖探测函数，让 /livez 校验 MySQL 与 Redis 连通性
	}) // 返回 router(配置好的 Gin 路由)、err(错误)
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}

	// ========== 第 6 步：创建 HTTP 服务并运行 ==========
	// 把路由、健康状态、超时配置组装成 HTTP 服务。
	httpServer, err := server.New(optionsFromConfig(cfg, router, health)) // optionsFromConfig 传入 cfg/router/health 返回服务参数；New 传入服务参数返回 httpServer、err
	if err != nil {
		return fmt.Errorf("create http server: %w", err)
	}

	// 打印启动日志，然后阻塞运行。Run 会一直卡在这里，直到收到退出信号或服务出错。
	logger.Info("pulseframe api starting", "address", cfg.HTTP.Address, "environment", cfg.Environment) // 传入启动信息与监听地址、环境
	if err := httpServer.Run(ctx); err != nil {                                                         // 传入 ctx(退出信号上下文)；阻塞运行，退出或出错时返回 err
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

// optionsFromConfig 将应用配置转换为 HTTP Server 参数。
// 输入：cfg，已校验配置；handler，HTTP 路由；health，共享健康状态。
// 输出：可传给 server.New 的服务参数。
// 功能：取出 HTTP 配置，并与路由和健康状态组成创建服务所需的参数。
// 例如配置中的监听地址为 :8080，返回参数中的 Address 也为 :8080。
func optionsFromConfig(cfg config.Config, handler http.Handler, health *server.HealthState) server.Options {
	return server.Options{
		Address:           cfg.HTTP.Address,           // 取监听地址
		Handler:           handler,                    // 装入路由处理器
		Health:            health,                     // 装入健康状态
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout, // 取请求头超时
		ReadTimeout:       cfg.HTTP.ReadTimeout,       // 取读取超时
		WriteTimeout:      cfg.HTTP.WriteTimeout,      // 取写入超时
		IdleTimeout:       cfg.HTTP.IdleTimeout,       // 取空闲超时
		ShutdownTimeout:   cfg.HTTP.ShutdownTimeout,   // 取退出超时
	}
}
