package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"pulseframe/internal/config"
)

var (
	// errNilHandler 表示 HTTP Server 没有可处理请求的 Handler。
	errNilHandler = errors.New("http handler is required")
	// errNilHealthState 表示 Server 缺少用于发布就绪状态的共享对象。
	errNilHealthState = errors.New("health state is required")
)

// Options 保存 HTTP Server 的构造参数。
type Options struct {
	Address           string
	Handler           http.Handler
	Health            *HealthState
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Server 管理 HTTP 服务和就绪状态的完整生命周期。
type Server struct {
	server          *http.Server
	health          *HealthState
	shutdownTimeout time.Duration
}

// OptionsFromConfig 将应用配置转换为 HTTP Server 参数。
// 输入：cfg，已校验配置；handler，HTTP 路由；health，共享健康状态。
// 输出：可传给 New 的服务参数。
// 功能：隔离配置结构与 HTTP Server 构造细节。
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
func New(options Options) (*Server, error) {
	// Handler 缺失时 Server 无法处理任何路由。
	if options.Handler == nil {
		return nil, errNilHandler
	}
	// 生命周期必须持有健康状态，才能在启动和退出时控制流量。
	if options.Health == nil {
		return nil, errNilHealthState
	}
	// 正关闭期限必须为正，避免 Shutdown 立即取消或无限等待。
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
// 输出：正常关闭时返回 nil，监听或服务失败时返回错误。
// 功能：为生产入口创建监听器并复用可测试的 Serve 生命周期。
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
// 输出：正常关闭时返回 nil，服务或关闭失败时返回错误。
// 功能：支持生产运行，并允许测试通过真实临时端口验证生命周期。
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
// 输出：向 results 写入 Serve 返回值。
// 功能：让 Serve 生命周期可以同时等待服务错误和退出信号。
func (server *Server) serve(listener net.Listener, results chan<- error) {
	results <- server.server.Serve(listener)
}

// shutdown 在期限内关闭服务并等待 Serve 返回。
// 输入：serveErrors，接收 Serve 最终结果的通道。
// 输出：正常关闭返回 nil，超时或异常关闭返回错误。
// 功能：停止接收新请求并等待正在处理的请求完成。
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
func normalizeServeError(err error) error {
	// Shutdown 导致的标准关闭错误属于预期结果，与 nil 一样视为成功。
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
