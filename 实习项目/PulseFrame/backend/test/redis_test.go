//go:build redisintegration

package test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"pulseframe/config"
	"pulseframe/infra"
)

// 输入：t，测试上下文；进程环境需提供独立 Redis 配置。
// 输出：完整 Redis 配置；缺失配置或参数非法时测试失败。
// 功能：让 Redis 集成测试连接真实服务，不跳过缺失依赖。
// 例如进程设置 REDIS_ADDR=127.0.0.1:6379 时返回可用于启动检查的配置。
func integrationRedisConfig(t *testing.T) config.Redis {
	t.Helper()
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Redis
}

// 输入：t，测试上下文；cfg，Redis 配置。
// 输出：连接真实 Redis 并返回客户端；连接失败时测试失败，调用方负责关闭客户端。
// 功能：统一创建 Redis 集成测试客户端，保证后续断言使用已经完成 PING 检查的连接池。
// 例如客户端创建成功后可执行 Ping 和 Set 命令，测试完成时由调用方关闭连接池。
func openRedis(t *testing.T, cfg config.Redis) *redis.Client {
	t.Helper()
	client, err := infra.NewRedis(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// TestRedisClientLifecycle 验证 PING、读写和关闭行为。
// 输入：Go 测试框架提供的测试上下文和独立 Redis 服务。
// 输出：命令失败、值不一致或关闭后仍能访问时使测试失败。
// 功能：验证 Redis 客户端连接池能建立连接、复用连接并在关闭后拒绝访问。
// 例如写入 redis-pool-test-key 后读取应得到同样的值，关闭后 Ping 应返回错误。
func TestRedisClientLifecycle(t *testing.T) {
	client := openRedis(t, integrationRedisConfig(t))
	ctx := t.Context()
	key := "pulseframe:redis:pool:test"
	if err := client.Set(ctx, key, "ok", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	value, err := client.Get(ctx, key).Result()
	if err != nil || value != "ok" {
		t.Fatalf("unexpected redis value %q: %v", value, err)
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(ctx).Err(); err == nil || !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("expected closed client error, got %v", err)
	}
}
