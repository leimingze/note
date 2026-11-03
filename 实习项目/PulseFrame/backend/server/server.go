package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"pulseframe/config"
)

var (
	// errNilHandler 表示 HTTP Server 没有可处理请求的 Handler。
	errNilHandler = errors.New("http handler is required")
	// errNilHealthState 表示 Server 缺少用于发布就绪状态的共享对象。
	errNilHealthState = errors.New("health state is required")
)

// Options 保存 HTTP Server 的构造参数。
type Options struct {
	Address           string        // HTTP 监听地址，例如 :8080。
	Handler           http.Handler  // 接收 HTTP 请求的路由处理器。
	Health            *HealthState  // 与就绪接口共享的服务状态。
	ReadHeaderTimeout time.Duration // 完成请求头读取的最长等待时间。
	ReadTimeout       time.Duration // 完成整个请求读取的最长等待时间，包含请求体。
	WriteTimeout      time.Duration // 响应写入的期限。
	IdleTimeout       time.Duration // 复用连接等待下一个请求的最长时间。
	ShutdownTimeout   time.Duration // 退出时等待正在处理的请求结束的期限，必须大于零。
}

// Server 管理 HTTP 服务和就绪状态的完整生命周期。
type Server struct {
	server          *http.Server  // 接收请求并管理 HTTP 连接的标准库服务对象。
	health          *HealthState  // 启动和退出时更新的共享就绪状态。
	shutdownTimeout time.Duration // 发起关闭后最多等待多久。
}

// OptionsFromConfig 将应用配置转换为 HTTP Server 参数。
// 输入：cfg，已校验配置；handler，HTTP 路由；health，共享健康状态。
// 输出：可传给 New 的服务参数。
// 功能：取出 HTTP 配置，并与路由和健康状态组成创建服务所需的参数。
// 例如配置中的监听地址为 :8080，返回参数中的 Address 也为 :8080。
func OptionsFromConfig(cfg config.Config, handler http.Handler, health *HealthState) Options {
	return Options{
		Address:           cfg.HTTP.Address,
		Handler:           handler,
		Health:            health,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ShutdownTimeout:   cfg.HTTP.ShutdownTimeout,
	}
}

// New 创建 HTTP 服务生命周期对象。
// 输入：options，监听地址、Handler、健康状态和超时配置。
// 输出：Server；必要依赖缺失或关闭超时非法时返回错误。
// 功能：集中构造具有资源保护超时的标准库 HTTP Server。
// 例如提供有效路由、健康状态和10秒退出期限时，返回服务对象；端口会在后续 Run 中开始监听。
func New(options Options) (*Server, error) {
	// Handler 缺失时 Server 无法处理任何路由。
	if options.Handler == nil {
		return nil, errNilHandler
	}
	// 生命周期必须持有健康状态，才能在启动和退出时控制流量。
	if options.Health == nil {
		return nil, errNilHealthState
	}
	// 退出等待时间必须大于零，否则无法给正在处理的请求留出完成时间。
	if options.ShutdownTimeout <= 0 {
		return nil, errors.New("shutdown timeout must be greater than zero")
	}
	// 所有网络超时都显式传入，避免慢连接无限占用服务资源。
	return &Server{
		server: &http.Server{
			Addr:              options.Address,
			Handler:           options.Handler,
			ReadHeaderTimeout: options.ReadHeaderTimeout,
			ReadTimeout:       options.ReadTimeout,
			WriteTimeout:      options.WriteTimeout,
			IdleTimeout:       options.IdleTimeout,
		},
		health:          options.Health,
		shutdownTimeout: options.ShutdownTimeout,
	}, nil
}

// Run 在配置地址监听并运行服务直到上下文取消。
// 输入：ctx，控制服务生命周期的上下文。
// 输出：监听网络端口并处理请求，正常关闭时返回 nil，监听或服务失败时返回错误。
// 功能：按配置地址启动 HTTP 服务，并等待退出通知或服务错误。
// 例如地址为 :8080 时监听8080端口；该端口已被占用时返回监听错误。
func (server *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.server.Addr)
	// 端口绑定失败时服务尚未就绪，直接返回明确监听错误。
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.server.Addr, err)
	}
	return server.Serve(ctx, listener)
}

// Serve 使用已有监听器运行服务直到上下文取消。
// 输入：ctx，服务生命周期上下文；listener，已经成功绑定的网络监听器。
// 输出：在已有监听器上处理请求并更新就绪状态；正常关闭时返回 nil，服务或关闭失败时返回错误。
// 功能：接管已建立的监听器，启动请求处理，并在收到取消通知后安排关闭。
// 例如调用方取消上下文后，服务先标记为未就绪，再停止接收新请求并等待现有请求结束。
func (server *Server) Serve(ctx context.Context, listener net.Listener) error {
	// 监听器已经成功创建，此时才能对外报告服务就绪。
	server.health.SetReady()
	serveErrors := make(chan error, 1)
	// http.Server.Serve 会阻塞，放入 goroutine 后才能同时监听退出信号。
	go server.serve(listener, serveErrors)

	// 服务自身报错和外部退出信号，谁先发生就处理谁。
	select {
	case err := <-serveErrors:
		server.health.SetNotReady()
		return normalizeServeError(err)
	case <-ctx.Done():
		server.health.SetNotReady()
		return server.shutdown(serveErrors)
	}
}

// serve 执行阻塞的 HTTP Serve 并传回结果。
// 输入：listener，网络监听器；results，容量至少为一的结果通道。
// 输出：在监听器上处理 HTTP 请求，结束后向 results 写入 Serve 返回值。
// 功能：让 Serve 生命周期可以同时等待服务错误和退出信号。
// 例如 HTTP 服务关闭后，把 http.ErrServerClosed 放入结果通道，通知等待方服务已经结束。
func (server *Server) serve(listener net.Listener, results chan<- error) {
	results <- server.server.Serve(listener)
}

// shutdown 在期限内关闭服务并等待 Serve 返回。
// 输入：serveErrors，接收 Serve 最终结果的通道。
// 输出：关闭监听器和空闲连接，等待正在处理的请求；正常关闭返回 nil，超时或异常关闭返回错误。
// 功能：停止接收新请求并等待正在处理的请求完成。
// 例如退出期限为10秒时，请求在2秒后完成则正常返回；超过10秒仍未结束则报告超时。
func (server *Server) shutdown(serveErrors <-chan error) error {
	// Shutdown 停止接收新请求，并在超时前等待正在处理的请求完成。
	ctx, cancel := context.WithTimeout(context.Background(), server.shutdownTimeout)
	defer cancel()
	// 超时或连接关闭失败时向入口报告，不能假装优雅退出成功。
	if err := server.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return normalizeServeError(<-serveErrors)
}

// normalizeServeError 统一 HTTP Server 的正常关闭结果。
// 输入：err，标准库 Serve 返回的错误。
// 输出：正常关闭映射为 nil，其他错误原样返回。
// 功能：避免把 http.ErrServerClosed 误判为运行故障。
// 例如传入 http.ErrServerClosed 返回 nil，传入其他网络错误则原样返回。
func normalizeServeError(err error) error {
	// Shutdown 导致的标准关闭错误属于预期结果，与 nil 一样视为成功。
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
