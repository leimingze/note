package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"pulseframe/internal/modules/account/application"
)

const rateLimitPrefix = "pulseframe:auth:limit:"

var rateLimitScript = redis.NewScript(`
for index, key in ipairs(KEYS) do
  local count = tonumber(redis.call('GET', key) or '0')
  local maximum = tonumber(ARGV[(index - 1) * 2 + 2])
  if count >= maximum then
    return 0
  end
end
for index, key in ipairs(KEYS) do
  local count = redis.call('INCR', key)
  if count == 1 then
    redis.call('PEXPIRE', key, ARGV[(index - 1) * 2 + 1])
  end
end
return 1
`)

// RedisRateLimiter 通过 Lua 脚本原子检查多个限流桶。
type RedisRateLimiter struct {
	client redis.UniversalClient
}

// NewRedisRateLimiter 创建 Redis 限流器。
// 输入：client，已配置 Redis 地址和 ACL 的客户端句柄。
// 输出：限流器；客户端为空时返回错误。
// 功能：确保多 API 实例共享同一组登录和注册请求计数。
func NewRedisRateLimiter(client redis.UniversalClient) (*RedisRateLimiter, error) {
	if client == nil {
		return nil, errors.New("Redis client is required")
	}
	return &RedisRateLimiter{client: client}, nil
}

// Allow 原子递增请求桶并判断是否仍在限额内。
// 输入：ctx，请求上下文；limits，非空限流桶集合，每项需有正上限和窗口。
// 输出：全部桶未超限返回 true；无效配置或 Redis 故障返回错误。
// 功能：避免并发请求在分布式 API 实例间绕过注册或登录限流。
func (limiter *RedisRateLimiter) Allow(ctx context.Context, limits []application.Limit) (bool, error) {
	if len(limits) == 0 {
		return false, errors.New("at least one rate limit is required")
	}
	keys, arguments, err := rateLimitArguments(limits)
	if err != nil {
		return false, err
	}
	result, err := rateLimitScript.Run(ctx, limiter.client, keys, arguments...).Int64()
	if err != nil {
		return false, fmt.Errorf("check Redis rate limit: %w", err)
	}
	return result == 1, nil
}

// rateLimitArguments 校验限流桶并生成不含原始身份的 Redis 键。
// 输入：limits，调用方提供的限流桶。
// 输出：摘要键列表和 Lua 参数；上限、窗口或键格式非法时返回错误。
// 功能：隔离用户标识并避免 Redis 命令参数拼接。
func rateLimitArguments(limits []application.Limit) ([]string, []any, error) {
	keys := make([]string, 0, len(limits))
	arguments := make([]any, 0, len(limits)*2)
	for _, limit := range limits {
		windowMilliseconds := limit.Window.Milliseconds()
		if limit.Key == "" || limit.Max <= 0 || windowMilliseconds <= 0 {
			return nil, nil, errors.New("rate limit key, maximum, and window must be positive")
		}
		digest := sha256.Sum256([]byte(limit.Key))
		keys = append(keys, rateLimitPrefix+hex.EncodeToString(digest[:]))
		arguments = append(arguments, windowMilliseconds, limit.Max)
	}
	return keys, arguments, nil
}
