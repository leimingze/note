package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"pulseframe/internal/platform/response"
)

const (
	// requestIDBytes 决定新请求标识包含的安全随机字节数。
	requestIDBytes = 16
)

var (
	// errNilIDGenerator 表示请求标识中间件缺少随机 ID 生成器。
	errNilIDGenerator = errors.New("request id generator is required")
)

// IDGenerator 定义请求标识生成能力。
// 输入：无。
// 输出：新的请求标识；安全随机源不可用时返回错误。
// 功能：约定生成请求标识的方法，让中间件通过传入的生成器取得标识。
// 例如传入 CryptoIDGenerator 后，中间件调用 NewID 可得到32个十六进制字符组成的随机标识。
type IDGenerator interface {
	NewID() (string, error)
}

// CryptoIDGenerator 使用系统安全随机源生成请求标识。
type CryptoIDGenerator struct{}

// NewID 生成十六进制请求标识。
// 输入：无显式参数，读取操作系统安全随机源。
// 输出：固定长度的十六进制标识；随机源失败时返回错误。
// 功能：读取16个安全随机字节，再转换为方便写入日志和响应头的32个十六进制字符。
// 例如一次调用可能得到 "7baf01e9c2d3465089abcdef0123456789"，后续调用会重新读取随机字节。
func (CryptoIDGenerator) NewID() (string, error) {
	content := make([]byte, requestIDBytes) // 分配16字节切片
	// 安全随机源失败时不能生成可能重复或可预测的关联标识。
	if _, err := rand.Read(content); err != nil { // 读取安全随机字节；失败时返回错误
		return "", err
	}
	return hex.EncodeToString(content), nil // 将字节编码为十六进制字符串
}

// RequestIDMiddleware 保存请求标识中间件依赖。
type RequestIDMiddleware struct {
	generator IDGenerator // 为每次请求生成新标识的生成器。
}

// NewRequestID 创建请求标识中间件。
// 输入：generator，请求标识生成器，不能为 nil。
// 输出：可注册到 Gin 的中间件；依赖缺失时返回错误。
// 功能：保存请求标识生成器，为后续请求准备关联请求、响应和日志的中间件。
// 例如传入 CryptoIDGenerator 后，可把返回对象的 Handle 注册到 Gin 路由。
func NewRequestID(generator IDGenerator) (*RequestIDMiddleware, error) {
	// 生成器是所有无合法上游 ID 请求的必需依赖。
	if generator == nil {
		return nil, errNilIDGenerator
	}
	return &RequestIDMiddleware{generator: generator}, nil // 传入生成器；返回请求标识中间件
}

// Handle 解析或生成请求标识并写入上下文与响应头。
// 输入：c，当前 Gin 请求上下文。
// 输出：继续请求链路；生成失败时返回统一 500 错误并终止链路。
// 功能：无视上游请求标识，为每次请求统一生成新标识，避免重复值污染日志。
// 例如生成器返回 abc123 时，响应头和请求上下文都会保存 abc123。
func (middleware *RequestIDMiddleware) Handle(c *gin.Context) {
	requestID, err := middleware.generator.NewID() // 每次请求调用生成器取得新标识
	// 生成失败时无法建立日志关联，返回统一内部错误并终止请求。
	if err != nil {
		response.WriteError(c, response.ErrorSpec{ // 传入上下文与 500 错误规格
			Status:  http.StatusInternalServerError,
			Code:    response.CodeInternal,
			Message: "internal server error",
		})
		return
	}
	// 在进入后续中间件前同时写入上下文和响应头。
	c.Set(response.RequestIDContextKey, requestID) // 将请求标识写入上下文
	c.Header(response.RequestIDHeader, requestID)  // 将请求标识写入响应头
	c.Next()                                       // 交给后续中间件和路由处理请求
}
