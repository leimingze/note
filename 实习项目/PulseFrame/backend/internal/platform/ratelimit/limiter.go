package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter 使用 Redis ZSET 实现滑动窗口限流。
type Limiter struct {
	client  *redis.Client // Redis 客户端。
	window  time.Duration // 滑动窗口长度。
	limit   int64         // 窗口内允许进入处理的次数。
	enabled bool          // 是否启用限流，关闭时总是放行。
}

// NewLimiter 创建滑动窗口限流器。
// 输入：client，Redis 客户端；window，窗口长度；limit，窗口内额度；enabled，是否启用。
// 输出：限流器。
// 功能：绑定计数维度并使用同一窗口与额度。
// 例如窗口为1分钟且额度为5时，窗口内第6个请求被拒绝。
func NewLimiter(client *redis.Client, window time.Duration, limit int, enabled bool) *Limiter {
	return &Limiter{client: client, window: window, limit: int64(limit), enabled: enabled}
}

// Allow 判断当前计数维度是否允许继续处理。
// 输入：ctx，上下文；key，计数维度键（调用方提供唯一标识）。
// 输出：允许返回 true，拒绝返回 false；Redis 不可用时返回错误。
// 功能：清理窗口外成员，检查并累加本次请求。
// 例如未超过额度时返回 true，超额返回 false。
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	if !l.enabled { // 未启用限流
		return true, nil
	}
	member, err := randomMember() // 生成唯一请求标识
	if err != nil {
		return false, err
	}
	result, err := allowScript.Run(ctx, l.client, []string{key}, l.window.Milliseconds(), l.limit, member).Int64() // 执行滑动窗口脚本
	if err != nil {
		return false, fmt.Errorf("rate limit check: %w", err)
	}
	return result == 1, nil
}

func randomMember() (string, error) {
	content := make([]byte, 16)                   // 分配随机字节
	if _, err := rand.Read(content); err != nil { // 读取安全随机字节
		return "", err
	}
	return hex.EncodeToString(content), nil // 编码为十六进制字符串
}

// allowScript 原子清理窗口外成员并检查额度，通过后加入本次请求。
// 返回 1 表示允许；0 表示已超额度。
var allowScript = redis.NewScript(`
local key = KEYS[1]
local window_ms = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local member = ARGV[3]
local now = redis.call('TIME')
local now_ms = now[1] * 1000 + math.floor(now[2] / 1000)
local cutoff = now_ms - window_ms
redis.call('ZREMRANGEBYSCORE', key, '-inf', cutoff)
local count = redis.call('ZCARD', key)
if count >= limit then
  return 0
end
redis.call('ZADD', key, now_ms, member)
redis.call('PEXPIRE', key, window_ms)
return 1
`)
