package httpserver

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	testHTTPTimeout     = 2 * time.Second
	testShutdownTimeout = time.Second
)

// serverTestRun 保存后台服务测试所需参数。
type serverTestRun struct {
	server   *Server
	ctx      context.Context
	listener net.Listener
	results  chan<- error
}

// TestServerLifecycle 验证真实监听端口下的请求和优雅退出。
// 输入：Go 测试框架提供的测试上下文。
// 输出：服务无法接收请求、更新就绪状态或按时退出时使测试失败。
// 功能：使用真实网络生命周期验证工程骨架，而非只测试 Handler。
func TestServerLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	health := NewHealthState()
	router, err := NewRouter(RouterDependencies{
		Logger:      slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
		IDGenerator: staticIDGenerator{value: testRequestID},
		Health:      health,
	})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	server, err := New(lifecycleTestOptions(router, health))
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan error, 1)
	run := serverTestRun{server: server, ctx: ctx, listener: listener, results: results}
	go run.Serve()
	waitForReady(t, health)

	client := &http.Client{Timeout: testHTTPTimeout}
	responseValue, err := client.Get("http://" + listener.Addr().String() + "/readyz")
	if err != nil {
		t.Fatalf("request ready endpoint: %v", err)
	}
	_ = responseValue.Body.Close()
	if responseValue.StatusCode != http.StatusOK {
		t.Fatalf("unexpected ready status: %d", responseValue.StatusCode)
	}

	cancel()
	select {
	case err := <-results:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(testHTTPTimeout):
		t.Fatal("server did not stop before timeout")
	}
	if health.Ready() {
		t.Fatal("server remained ready after shutdown")
	}
}

// lifecycleTestOptions 创建生命周期测试使用的 HTTP Server 参数。
// 输入：handler，测试路由；health，共享健康状态。
// 输出：使用短超时和临时地址的 Options。
// 功能：集中测试配置，避免生命周期测试散落魔法数。
func lifecycleTestOptions(handler http.Handler, health *HealthState) Options {
	return Options{
		Address:           "127.0.0.1:0",
		Handler:           handler,
		Health:            health,
		ReadHeaderTimeout: testHTTPTimeout,
		ReadTimeout:       testHTTPTimeout,
		WriteTimeout:      testHTTPTimeout,
		IdleTimeout:       testHTTPTimeout,
		ShutdownTimeout:   testShutdownTimeout,
	}
}

// Serve 在后台运行测试服务并返回结果。
// 输入：使用 serverTestRun 中的服务、上下文、监听器和结果通道。
// 输出：将服务的 Serve 返回值写入结果通道。
// 功能：让测试能够并发发送真实 HTTP 请求并触发退出。
func (run serverTestRun) Serve() {
	run.results <- run.server.Serve(run.ctx, run.listener)
}

// waitForReady 等待服务进入就绪状态。
// 输入：t，测试上下文；health，共享健康状态。
// 输出：服务在期限内就绪则返回，否则使测试失败。
// 功能：以显式超时同步服务启动，避免使用固定睡眠时间。
func waitForReady(t *testing.T, health *HealthState) {
	t.Helper()
	deadline := time.NewTimer(testHTTPTimeout)
	ticker := time.NewTicker(time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if health.Ready() {
				return
			}
		case <-deadline.C:
			t.Fatal("server did not become ready before timeout")
		}
	}
}
