package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/response"
)

const (
	// requestIDBytes 决定新请求标识包含的安全随机字节数。
	requestIDBytes = 16
	// maxRequestIDLength 限制允许透传的上游请求标识长度。
	maxRequestIDLength = 128
)

var (
	// requestIDPattern 只允许适合日志和 HTTP 响应头的可打印字符。
	requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	// errNilIDGenerator 表示请求标识中间件缺少随机 ID 生成器。
	errNilIDGenerator = errors.New("request id generator is required")
)

// IDGenerator 定义请求标识生成能力。
// 输入：无。
// 输出：新的请求标识；安全随机源不可用时返回错误。
// 功能：隔离随机源，使请求中间件可以确定性测试。
type IDGenerator interface {
	NewID() (string, error)
}

// CryptoIDGenerator 使用系统安全随机源生成请求标识。
type CryptoIDGenerator struct{}

// NewID 生成十六进制请求标识。
// 输入：无显式参数，读取操作系统安全随机源。
// 输出：固定长度的十六进制标识；随机源失败时返回错误。
// 功能：为没有合法上游请求标识的请求创建唯一关联标识。
func (CryptoIDGenerator) NewID() (string, error) {
	content := make([]byte, requestIDBytes)
	// 安全随机源失败时不能生成可能重复或可预测的关联标识。
	if _, err := rand.Read(content); err != nil {
		return "", err
	}
	return hex.EncodeToString(content), nil
}

// RequestIDMiddleware 保存请求标识中间件依赖。
type RequestIDMiddleware struct {
	generator IDGenerator
}

// NewRequestID 创建请求标识中间件。
// 输入：generator，请求标识生成器，不能为 nil。
// 输出：可注册到 Gin 的中间件；依赖缺失时返回错误。
// 功能：在路由处理前建立请求、响应和日志之间的关联标识。
func NewRequestID(generator IDGenerator) (*RequestIDMiddleware, error) {
	// 生成器是所有无合法上游 ID 请求的必需依赖。
	if generator == nil {
		return nil, errNilIDGenerator
	}
	return &RequestIDMiddleware{generator: generator}, nil
}

// Handle 解析或生成请求标识并写入上下文与响应头。
// 输入：c，当前 Gin 请求上下文。
// 输出：继续请求链路；生成失败时返回统一 500 错误并终止链路。
// 功能：保证所有进入业务路由的请求都拥有合法请求标识。
func (middleware *RequestIDMiddleware) Handle(c *gin.Context) {
	// 只透传长度和字符合法的上游 ID，避免控制字符进入日志或响应头。
	requestID := strings.TrimSpace(c.GetHeader(response.RequestIDHeader))
	// 缺失或不安全的上游值必须替换成本服务生成的新标识。
	if !validRequestID(requestID) {
		generated, err := middleware.generator.NewID()
		// 生成失败时无法建立日志关联，返回统一内部错误并终止请求。
		if err != nil {
			response.WriteError(c, response.ErrorSpec{
				Status:  http.StatusInternalServerError,
				Code:    response.CodeInternal,
				Message: "internal server error",
			})
			return
		}
		requestID = generated
	}
	// 在进入后续中间件前同时写入上下文和响应头。
	c.Set(response.RequestIDContextKey, requestID)
	c.Header(response.RequestIDHeader, requestID)
	c.Next()
}

// validRequestID 判断上游请求标识是否可以安全透传。
// 输入：value，已经清理首尾空白的请求标识。
// 输出：长度和字符集合均合法时返回 true。
// 功能：阻止超长或包含控制字符的标识进入日志和响应头。
func validRequestID(value string) bool {
	return value != "" && len(value) <= maxRequestIDLength && requestIDPattern.MatchString(value)
}
