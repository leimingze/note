package test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"pulseframe/metrics"
	"pulseframe/server"
)

// TestMetricsEndpoint 验证指标启用时注册 /metrics。
// 输入：测试上下文。
// 输出：指标端点不可用或没有注册时测试失败。
// 功能：确认指标端点随注册表启用，并输出 Prometheus 文本格式。
func TestMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	health := server.NewHealthState()
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      testLogger(t),
		IDGenerator: staticIDGenerator{value: testRequestID},
		Health:      health,
		Metrics:     metrics.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// 先触发一次存活请求，让指标中间件记录 HTTP 指标，便于端点断言。
	performRequest(router, http.MethodGet, "/livez")
	recorder := performRequest(router, http.MethodGet, "/metrics")
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected metrics status: %d", recorder.Code)
	}
	if !contains(recorder.Body.String(), "pulseframe_http_requests_total") {
		t.Fatal("metrics body missing expected metric")
	}
}

// contains 判断字符串是否包含子串。
// 输入：text，被检查字符串；sub，子串。
// 输出：包含时返回 true。
// 功能：辅助指标端点响应断言。
func contains(text, sub string) bool {
	for i := 0; i+len(sub) <= len(text); i++ {
		if text[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
