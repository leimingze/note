//go:build mysqlintegration

package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"pulseframe/config"
)

const (
	appTestTimeout  = 5 * time.Second       // 限制测试中的启动等待、HTTP 请求和退出等待。
	appPollInterval = 10 * time.Millisecond // 等待应用就绪时两次检查之间的间隔。
)

// 输入：t，测试上下文；进程环境需提供独立测试数据库配置。
// 输出：依赖无法创建、共享或关闭时测试失败。
// 功能：使用真实 MySQL 验证启动层持有并关闭连接池。
func TestDependenciesLifecycle(t *testing.T) {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := NewDependencies(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dependencies.Close(); err != nil {
			t.Error(err)
		}
	})
	shared := dependencies.MySQL
	if err := shared.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := dependencies.Close(); err != nil {
		t.Fatal(err)
	}
	if err := shared.PingContext(t.Context()); err == nil || shared.Stats().OpenConnections != 0 {
		t.Fatal("dependency owner did not close the shared pool")
	}
}

// 输入：t，测试上下文。
// 输出：监听失败未向启动入口传播时测试失败。
// 功能：验证 MySQL 初始化成功后的 HTTP 启动失败路径。
func TestRunListenFailure(t *testing.T) {
	// 先占用真实监听端口，使应用完成数据库初始化后在 HTTP 监听阶段失败。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(t.Context(), appTestTimeout)
	defer cancel()
	err = run(ctx, appEnvironment(listener.Addr().String()))
	if err == nil || !strings.Contains(err.Error(), "run http server: listen") {
		t.Fatalf("expected HTTP listen failure after MySQL initialization: %v", err)
	}
}

// 输入：t，测试上下文。
// 输出：服务不能就绪或不能响应退出请求时测试失败。
// 功能：通过真实数据库和 HTTP 请求验证完整启动退出流程。
func TestRunGracefulShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), appTestTimeout)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- run(ctx, appEnvironment(address)) }()
	readyErr := waitForApplication(ctx, address)
	// 无论就绪检查成功与否，都通知后台服务退出并等待最终结果。
	cancel()
	select {
	case err := <-result:
		if err != nil || readyErr != nil {
			t.Fatalf("startup=%v shutdown=%v", readyErr, err)
		}
	case <-time.After(appTestTimeout):
		t.Fatal("application did not stop")
	}
}

// 输入：address，真实测试监听地址。
// 输出：基于进程环境的配置读取函数，覆盖 HTTP 地址和运行模式。
// 功能：保留真实数据库配置并隔离每个测试的 HTTP 监听端口。
func appEnvironment(address string) config.LookupEnv {
	return func(key string) (string, bool) {
		switch key {
		case "HTTP_ADDR":
			return address, true
		case "APP_ENV":
			return config.EnvironmentTest, true
		default:
			return os.LookupEnv(key)
		}
	}
}

// 输入：ctx，等待期限；address，真实 HTTP 地址。
// 输出：就绪时返回 nil，超时或取消时返回上下文错误。
// 功能：同步后台服务启动，只接受真实 /readyz 成功响应。
func waitForApplication(ctx context.Context, address string) error {
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: appTestTimeout}
	ticker := time.NewTicker(appPollInterval)
	defer ticker.Stop()
	// 等待后台启动期间允许连接尚未建立，只在真实就绪响应或期限结束时返回。
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/readyz", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			closeErr := response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return closeErr
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
