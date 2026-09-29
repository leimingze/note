package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"pulseframe/internal/modules/account/application"
)

// newRedisTestStores 创建隔离的 Redis 会话与限流适配器。
// 输入：t，Go 测试上下文。
// 输出：两个适配器和测试 Redis；测试结束时自动关闭服务与客户端。
// 功能：验证真实 Redis 命令和 Lua 脚本而不连接共享服务。
func newRedisTestStores(t *testing.T) (*RedisSessionManager, *RedisRateLimiter, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sessions, err := NewRedisSessionManager(client)
	if err != nil {
		t.Fatal(err)
	}
	limiter, err := NewRedisRateLimiter(client)
	if err != nil {
		t.Fatal(err)
	}
	return sessions, limiter, server
}

// TestRedisSessionLifecycle 验证令牌摘要存储、读取、过期和撤销。
// 输入：Go 测试框架提供的测试上下文。
// 输出：会话状态或令牌保密性不符合预期时使测试失败。
// 功能：覆盖网页会话适配器的核心安全行为。
func TestRedisSessionLifecycle(t *testing.T) {
	sessions, _, server := newRedisTestStores(t)
	credentials, err := sessions.Create(context.Background(), 42, time.Minute)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if credentials.Token == "" || credentials.CSRFToken == "" {
		t.Fatal("session credentials are empty")
	}
	for _, key := range server.Keys() {
		if strings.Contains(key, credentials.Token) {
			t.Fatalf("raw token appeared in Redis key %q", key)
		}
	}
	session, err := sessions.Resolve(context.Background(), credentials.Token)
	if err != nil || session.UserID != 42 || session.CSRFToken != credentials.CSRFToken {
		t.Fatalf("unexpected resolved session: %+v, %v", session, err)
	}
	if err := sessions.Revoke(context.Background(), credentials.Token); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := sessions.Resolve(context.Background(), credentials.Token); !errors.Is(err, application.ErrUnauthorized) {
		t.Fatalf("revoked token should be unauthorized: %v", err)
	}
}

// TestRedisSessionExpires 验证 Redis TTL 到期后会话不能恢复。
// 输入：Go 测试框架提供的测试上下文。
// 输出：过期令牌仍能读取时使测试失败。
// 功能：保护固定有效期不会只依赖客户端 Cookie 到期。
func TestRedisSessionExpires(t *testing.T) {
	sessions, _, server := newRedisTestStores(t)
	credentials, err := sessions.Create(context.Background(), 42, time.Second)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	server.FastForward(2 * time.Second)
	if _, err := sessions.Resolve(context.Background(), credentials.Token); !errors.Is(err, application.ErrUnauthorized) {
		t.Fatalf("expired token should be unauthorized: %v", err)
	}
}

// TestRedisRateLimiterAppliesEveryBucket 验证多个限流桶以 Lua 原子递增。
// 输入：Go 测试框架提供的测试上下文。
// 输出：超过任意一个桶仍被放行时使测试失败。
// 功能：保护多实例部署下的 IP 和账号组合限制。
func TestRedisRateLimiterAppliesEveryBucket(t *testing.T) {
	_, limiter, _ := newRedisTestStores(t)
	ctx := context.Background()
	limits := []application.Limit{
		{Key: "ip:one", Max: 3, Window: time.Minute},
		{Key: "identity:one", Max: 2, Window: time.Minute},
	}
	for attempt := 1; attempt <= 3; attempt++ {
		allowed, err := limiter.Allow(ctx, limits)
		if err != nil {
			t.Fatalf("check rate limit: %v", err)
		}
		if allowed != (attempt <= 2) {
			t.Fatalf("attempt %d allowed=%v", attempt, allowed)
		}
	}
}

// TestRedisRateLimiterRejectsInvalidLimits 验证空限流窗口不会执行脚本。
// 输入：Go 测试框架提供的测试上下文。
// 输出：无效限流项未返回错误时使测试失败。
// 功能：防止零窗口或零上限产生无界 Redis 键。
func TestRedisRateLimiterRejectsInvalidLimits(t *testing.T) {
	_, limiter, _ := newRedisTestStores(t)
	if _, err := limiter.Allow(context.Background(), []application.Limit{{Key: "ip", Max: 1}}); err == nil {
		t.Fatal("expected zero window to fail")
	}
}

// TestRedisRateLimiterDoesNotCreateKeysAfterRejection 验证 IP 限额命中后不写入新身份键。
// 输入：Go 测试框架提供的测试上下文。
// 输出：拒绝请求仍新增 Redis 键时使测试失败。
// 功能：防止随机用户名在已限流 IP 下制造高基数短期数据。
func TestRedisRateLimiterDoesNotCreateKeysAfterRejection(t *testing.T) {
	_, limiter, server := newRedisTestStores(t)
	ctx := context.Background()
	ipLimit := application.Limit{Key: "login:ip:one", Max: 1, Window: time.Minute}
	first, err := limiter.Allow(ctx, []application.Limit{
		ipLimit, {Key: "login:identity:one", Max: 10, Window: time.Minute},
	})
	if err != nil || !first {
		t.Fatalf("first request should pass: allowed=%v error=%v", first, err)
	}
	second, err := limiter.Allow(ctx, []application.Limit{
		ipLimit, {Key: "login:identity:random", Max: 10, Window: time.Minute},
	})
	if err != nil || second {
		t.Fatalf("second request should be rejected: allowed=%v error=%v", second, err)
	}
	if keys := server.Keys(); len(keys) != 2 {
		t.Fatalf("rejected request created Redis keys: %v", keys)
	}
}
