package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/httpserver/response"
)

const (
	requestIDBytes     = 16
	maxRequestIDLength = 128
)

var (
	requestIDPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
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
	requestID := strings.TrimSpace(c.GetHeader(response.RequestIDHeader))
	if !validRequestID(requestID) {
		generated, err := middleware.generator.NewID()
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
