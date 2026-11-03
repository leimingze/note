package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"pulseframe/middleware"
	"pulseframe/response"
	"pulseframe/server"
)

// testRequestID 是路由测试预设并用于断言的请求关联标识。
const testRequestID = "request-test-001"

// staticIDGenerator 保存路由测试传入的标识和错误，不访问随机源。
type staticIDGenerator struct {
	value string // 调用 NewID 时直接返回的标识。
	err   error  // 调用 NewID 时直接返回的错误，用于已有失败场景。
}

// NewID 返回测试预设的请求标识或错误。
// 输入：无。
// 输出：构造时指定的标识和错误。
// 功能：让请求标识中间件测试不依赖随机源。
// 例如 value 为 request-test-001 且 err 为 nil 时，调用后得到该标识和 nil。
func (generator staticIDGenerator) NewID() (string, error) {
	return generator.value, generator.err
}

// errorEnvelope 表示测试需要读取的统一错误响应字段。
type errorEnvelope struct {
	RequestID string `json:"request_id"` // 从响应中读取的请求标识。
	Error     struct {
		Code    string `json:"code"`    // 用于核对错误类别。
		Message string `json:"message"` // 用于确认客户端能收到错误说明。
	} `json:"error"` // 响应正文中的错误对象。
}

// errorExpectation 保存统一错误响应的测试期望。
type errorExpectation struct {
	Status           int    // 期望的 HTTP 状态码。
	Code             string // 期望的公开错误码。
	RequireRequestID bool   // 是否要求错误正文中存在非空请求标识。
}

// newTestRouter 创建使用固定依赖的测试路由。
// 输入：t，测试上下文；health，测试控制的健康状态。
// 输出：切换 Gin 为测试模式，返回 Gin Engine 和日志缓冲区；路由构造失败时立即结束测试。
// 功能：减少路由行为测试中的重复依赖装配。
// 例如传入未就绪的健康状态，返回路由的 /readyz 会响应503，访问日志写入返回的缓冲区。
func newTestRouter(t *testing.T, health *server.HealthState) (*gin.Engine, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      logger,
		IDGenerator: staticIDGenerator{value: testRequestID},
		Health:      health,
	})
	// 基础路由装配失败时后续行为断言没有意义。
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	return router, &logs
}

// performRequest 执行一条无请求体的路由测试请求。
// 输入：router，待测路由；method，请求方法；path，请求路径。
// 输出：记录完整 HTTP 响应的 Recorder。
// 功能：用 httptest 验证请求经过真实 Gin 中间件和路由链路。
// 例如传入 GET 和 /livez，返回的记录对象可用于检查200状态码和响应正文。
func performRequest(router http.Handler, method string, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestHealthEndpoints 验证存活和就绪接口状态变化。
// 输入：Go 测试框架提供的测试上下文。
// 输出：状态码或响应内容不符合健康语义时使测试失败。
// 功能：检查存活接口和就绪接口是否随健康状态返回正确结果。
// 例如调用 SetReady 前 /readyz 返回503，调用后同一接口返回200。
func TestHealthEndpoints(t *testing.T) {
	health := server.NewHealthState()
	router, _ := newTestRouter(t, health)

	// 存活探针不依赖就绪状态，进程运行时始终返回 200。
	if responseRecorder := performRequest(router, http.MethodGet, "/livez"); responseRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected live status: %d", responseRecorder.Code)
	}
	// 初始化未完成时，就绪探针必须返回 503。
	if responseRecorder := performRequest(router, http.MethodGet, "/readyz"); responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected not-ready status: %d", responseRecorder.Code)
	}
	health.SetReady()
	// 发布就绪状态后，同一路由必须切换为 200。
	if responseRecorder := performRequest(router, http.MethodGet, "/readyz"); responseRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected ready status: %d", responseRecorder.Code)
	}
}

// TestRequestIDPropagation 验证合法上游请求标识被透传。
// 输入：Go 测试框架提供的测试上下文。
// 输出：响应头未返回相同请求标识时使测试失败。
// 功能：保护跨服务请求追踪所需的标识传递行为。
// 例如请求头携带 upstream-123 时，检查响应头也返回 upstream-123。
func TestRequestIDPropagation(t *testing.T) {
	health := server.NewHealthState()
	router, _ := newTestRouter(t, health)
	// 为存活请求设置合法的上游标识，随后检查响应头是否原样返回。
	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	request.Header.Set(response.RequestIDHeader, "upstream-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	// 合法上游标识必须原样返回，不能被随机生成器覆盖。
	if value := recorder.Header().Get(response.RequestIDHeader); value != "upstream-123" {
		t.Fatalf("unexpected request id: %q", value)
	}
}

// TestRequestIDGenerationFailure 验证随机源失败会显式返回统一错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：状态码或错误码不符合预期时使测试失败。
// 功能：确保请求标识失败不会被静默降级掩盖。
// 例如生成器返回 random source unavailable 时，检查请求返回500和 INTERNAL_ERROR。
func TestRequestIDGenerationFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	health := server.NewHealthState()
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	router, err := server.NewRouter(server.RouterDependencies{
		Logger:      logger,
		IDGenerator: staticIDGenerator{err: errors.New("random source unavailable")},
		Health:      health,
	})
	// 路由本身应正常构造，随机源故障只在处理请求时触发。
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	recorder := performRequest(router, http.MethodGet, "/livez")
	assertErrorResponse(t, recorder, errorExpectation{
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternal,
	})
}

// TestRecoveryReturnsInternalError 验证请求 panic 被记录并转换为统一错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：panic 泄漏、状态码错误或日志缺失时使测试失败。
// 功能：保护单次请求异常不会导致进程退出或暴露内部信息。
// 例如访问会触发 panic 的 /panic，应收到500，日志保留异常信息，响应正文不包含异常原文。
func TestRecoveryReturnsInternalError(t *testing.T) {
	health := server.NewHealthState()
	router, logs := newTestRouter(t, health)
	// 让真实 Recovery 中间件捕获 panic，验证测试进程不会被 Handler 终止。
	router.GET("/panic", panicHandler)
	recorder := performRequest(router, http.MethodGet, "/panic")

	assertErrorResponse(t, recorder, errorExpectation{
		Status:           http.StatusInternalServerError,
		Code:             response.CodeInternal,
		RequireRequestID: true,
	})
	// 客户端只看到统一错误，详细 panic 只能进入服务日志。
	if bytes.Contains(recorder.Body.Bytes(), []byte("sensitive panic detail")) {
		t.Fatalf("panic detail leaked to client: %s", recorder.Body.String())
	}
	// 服务日志必须记录 panic 事件以便排查根因。
	if !bytes.Contains(logs.Bytes(), []byte("http request panic")) {
		t.Fatalf("panic log missing: %s", logs.String())
	}
	// 访问日志还必须记录最终 500 状态，保持请求审计完整。
	if !bytes.Contains(logs.Bytes(), []byte(`"status":500`)) {
		t.Fatalf("panic status missing from access log: %s", logs.String())
	}
}

// TestRouteErrorsUseEnvelope 验证路由级错误使用统一结构。
// 输入：Go 测试框架提供的测试上下文。
// 输出：404或405响应不符合约定的 JSON 结构时使测试失败。
// 功能：保证框架默认错误不会绕过项目响应规范。
// 例如 GET /missing 返回 RESOURCE_NOT_FOUND，POST /livez 返回 METHOD_NOT_ALLOWED。
func TestRouteErrorsUseEnvelope(t *testing.T) {
	health := server.NewHealthState()
	router, _ := newTestRouter(t, health)
	notFound := performRequest(router, http.MethodGet, "/missing")
	assertErrorResponse(t, notFound, errorExpectation{
		Status:           http.StatusNotFound,
		Code:             response.CodeNotFound,
		RequireRequestID: true,
	})

	methodNotAllowed := performRequest(router, http.MethodPost, "/livez")
	assertErrorResponse(t, methodNotAllowed, errorExpectation{
		Status:           http.StatusMethodNotAllowed,
		Code:             response.CodeMethodNotAllowed,
		RequireRequestID: true,
	})
}

// panicHandler 制造请求处理 panic。
// 输入：当前 Gin 上下文，本测试不读取其中内容。
// 输出：抛出固定 panic，不写正常响应。
// 功能：验证异常恢复中间件的真实行为。
// 例如注册到 /panic 后，请求该路径会触发 panic，供中间件捕获并记录。
func panicHandler(*gin.Context) {
	panic("sensitive panic detail")
}

// assertErrorResponse 断言统一错误响应的关键字段。
// 输入：t，测试上下文；recorder，HTTP 响应；expected，状态码、错误码和请求标识要求。
// 输出：响应不符合期望状态码和 JSON 字段时使测试失败。
// 功能：集中校验状态码、请求标识和稳定错误码。
// 例如期望404和 RESOURCE_NOT_FOUND，响应却为200时，立即报告测试失败。
func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, expected errorExpectation) {
	t.Helper()
	// 先检查 HTTP 状态是否符合当前场景，再读取 JSON 正文并核对错误字段。
	if recorder.Code != expected.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, expected.Status)
	}
	var body errorEnvelope
	// 错误正文必须是统一 JSON 包装结构。
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	// 稳定错误码和可读消息必须同时存在。
	if body.Error.Code != expected.Code || body.Error.Message == "" {
		t.Fatalf("unexpected error response: %+v", body)
	}
	// 要求链路追踪的场景必须携带非空请求标识。
	if expected.RequireRequestID && body.RequestID == "" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

// 编译期确认测试生成器持续满足生产中间件依赖的接口。
var _ middleware.IDGenerator = staticIDGenerator{}
