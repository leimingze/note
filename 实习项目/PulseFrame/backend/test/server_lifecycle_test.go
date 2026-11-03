package test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pulseframe/server"
)

const (
	// testHTTPTimeout 限制真实端口请求和就绪等待时间。
	testHTTPTimeout = 2 * time.Second
	// testShutdownTimeout 限制测试服务的优雅退出时间。
	testShutdownTimeout = time.Second
)

// serverTestRun 保存后台服务测试所需参数。
type serverTestRun struct {
	server   *server.Server  // 在后台接收请求的服务。
	ctx      context.Context // 测试通过取消此上下文通知服务退出。
	listener net.Listener    // 已绑定本机临时端口的监听器。
	results  chan<- error    // 服务退出后写入结果，供测试等待。
}

// TestServerLifecycle 验证真实监听端口下的请求和优雅退出。
// 输入：Go 测试框架提供的测试上下文。
// 输出：启动本机 HTTP 服务并在检查后关闭；无法接收请求、更新就绪状态或按时退出时使测试失败。
// 功能：通过本机网络请求检查服务从启动、就绪到退出的完整过程。
// 例如 /readyz 返回200后取消上下文，检查服务结束且健康状态恢复为未就绪。
func TestServerLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	health := server.NewHealthState()
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
		IDGenerator: staticIDGenerator{value: testRequestID},
		Health:      health,
	})
	// 路由创建失败时无法继续验证真实监听生命周期。
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	httpServer, err := server.New(lifecycleTestOptions(router, health))
	// Server 构造失败说明测试参数未满足生命周期约束。
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	// 使用系统分配的临时端口，测试真实网络生命周期且避免端口冲突。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	// 临时端口绑定失败时没有可供客户端访问的测试目标。
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan error, 1)
	run := serverTestRun{server: httpServer, ctx: ctx, listener: listener, results: results}
	// 服务在后台接收请求；测试等待就绪状态出现后才发送请求。
	go run.Serve()
	waitForReady(t, health)

	client := &http.Client{Timeout: testHTTPTimeout}
	responseValue, err := client.Get("http://" + listener.Addr().String() + "/readyz")
	// 就绪后真实 HTTP 请求必须能够连通。
	if err != nil {
		t.Fatalf("request ready endpoint: %v", err)
	}
	_ = responseValue.Body.Close()
	// 已发布就绪状态的实例必须返回 200。
	if responseValue.StatusCode != http.StatusOK {
		t.Fatalf("unexpected ready status: %d", responseValue.StatusCode)
	}

	// 取消上下文触发优雅退出，并等待后台 Serve 返回最终结果。
	cancel()
	// 同时等待服务结束和超时通知，超过期限仍未结束就报告测试失败。
	select {
	case err := <-results:
		// 主动取消上下文应使服务正常关闭，因此返回值应为 nil。
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(testHTTPTimeout):
		t.Fatal("server did not stop before timeout")
	}
	// 服务退出后必须撤销就绪状态，防止平台继续分流。
	if health.Ready() {
		t.Fatal("server remained ready after shutdown")
	}
}

// lifecycleTestOptions 创建生命周期测试使用的 HTTP Server 参数。
// 输入：handler，测试路由；health，共享健康状态。
// 输出：使用短超时和临时地址的 Options。
// 功能：为生命周期测试统一设置监听地址和各项等待期限。
// 例如返回127.0.0.1:0作为地址，由系统分配端口，退出等待期限设置为1秒。
func lifecycleTestOptions(handler http.Handler, health *server.HealthState) server.Options {
	return server.Options{
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
// 输出：运行 HTTP 服务，并将服务的 Serve 返回值写入结果通道。
// 功能：让测试能够并发发送真实 HTTP 请求并触发退出。
// 例如测试取消上下文后，服务正常退出并向结果通道写入 nil。
func (run serverTestRun) Serve() {
	run.results <- run.server.Serve(run.ctx, run.listener)
}

// waitForReady 等待服务进入就绪状态。
// 输入：t，测试上下文；health，共享健康状态。
// 输出：服务在期限内就绪则返回，否则使测试失败。
// 功能：以显式超时同步服务启动，避免使用固定睡眠时间。
// 例如2秒内观察到 Ready 为 true 就返回；一直未就绪则报告测试失败。
func waitForReady(t *testing.T, health *server.HealthState) {
	t.Helper()
	deadline := time.NewTimer(testHTTPTimeout)
	ticker := time.NewTicker(time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	// 每毫秒读取一次并发安全的就绪标记，同时用整体期限限制等待时间。
	for {
		// 每次循环同时监听状态检查节拍和整体截止时间。
		select {
		case <-ticker.C:
			// 一旦观察到就绪即可继续测试真实 HTTP 请求。
			if health.Ready() {
				return
			}
		case <-deadline.C:
			t.Fatal("server did not become ready before timeout")
		}
	}
}
