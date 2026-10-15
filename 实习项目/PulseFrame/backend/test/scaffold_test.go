package test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"pulseframe/middleware"
	"pulseframe/response"
	"pulseframe/server"
)

// 输入：Go 测试框架提供的测试上下文。
// 输出：业务路由出现或未知路径响应异常时使测试失败。
// 功能：验证工程基座仅提供健康探针，并使用真实请求标识生成器处理请求。
func TestScaffoldRoutes(t *testing.T) {
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      slog.New(slog.NewJSONHandler(io.Discard, nil)),
		IDGenerator: middleware.CryptoIDGenerator{},
		Health:      server.NewHealthState(),
	})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	expected := map[string]string{"/livez": http.MethodGet, "/readyz": http.MethodGet}
	routes := router.Routes()
	if len(routes) != len(expected) {
		t.Fatalf("unexpected route count: got %d want %d", len(routes), len(expected))
	}
	for _, route := range routes {
		method, ok := expected[route.Path]
		if !ok || route.Method != method {
			t.Fatalf("unexpected route: %s %s", route.Method, route.Path)
		}
		delete(expected, route.Path)
	}
	if len(expected) != 0 {
		t.Fatalf("missing routes: %v", expected)
	}
	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/register")
	assertErrorResponse(t, recorder, errorExpectation{
		Status:           http.StatusNotFound,
		Code:             response.CodeNotFound,
		RequireRequestID: true,
	})
}
