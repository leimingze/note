package test

import (
	"strings"
	"testing"
	"time"

	"pulseframe/config"
)

// 输入：无。
// 输出：包含 Redis 必填或可选配置的独立环境映射，不访问 Redis 服务。
// 功能：为 Redis 配置测试提供有效起点，避免真实进程环境影响断言。
// 例如返回的映射把 REDIS_PASSWORD 设置为空字符串，表示允许未启用密码认证的本地 Redis。
func redisEnvironment() envMap {
	return envMap{
		"MYSQL_USER":     "config-test",
		"MYSQL_PASSWORD": "",
		"MYSQL_DATABASE": "config-test",
	}
}

// 输入：t，测试上下文。
// 输出：默认 Redis 配置不符合预期时使测试失败。
// 功能：保护 Redis 地址、连接数量、重试次数和期限的默认值。
// 例如未设置 Redis 参数时，检查地址为127.0.0.1:6379、最大连接数为20、重试次数为3。
func TestRedisDefaults(t *testing.T) {
	cfg, err := config.Load(redisEnvironment().Lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Address != "127.0.0.1:6379" || cfg.Redis.Database != 0 {
		t.Fatalf("unexpected redis target: %+v", cfg.Redis)
	}
	if cfg.Redis.MaxRetries != 3 || cfg.Redis.PoolSize != 10 || cfg.Redis.MinIdleConns != 2 || cfg.Redis.MaxActiveConns != 20 {
		t.Fatalf("unexpected redis pool defaults: %+v", cfg.Redis)
	}
	if cfg.Redis.DialTimeout != 3*time.Second || cfg.Redis.PoolTimeout != 5*time.Second {
		t.Fatalf("unexpected redis timeout defaults: %+v", cfg.Redis)
	}
}

// 输入：t，测试上下文。
// 输出：覆盖值没有按原意读取时使测试失败。
// 功能：验证 Redis 地址、认证信息、逻辑数据库、连接池和期限均可配置。
// 例如密码含有前后空格时，加载后的 Password 仍保留这些字符。
func TestRedisOverrides(t *testing.T) {
	values := redisEnvironment()
	values["REDIS_ADDR"] = "[::1]:6380"
	values["REDIS_USERNAME"] = "app"
	values["REDIS_PASSWORD"] = " pass "
	values["REDIS_DATABASE"] = "2"
	values["REDIS_MAX_RETRIES"] = "1"
	values["REDIS_POOL_SIZE"] = "5"
	values["REDIS_MIN_IDLE_CONNS"] = "1"
	values["REDIS_MAX_ACTIVE_CONNS"] = "8"
	values["REDIS_POOL_TIMEOUT"] = "250ms"
	cfg, err := config.Load(values.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Address != values["REDIS_ADDR"] || cfg.Redis.Username != "app" || cfg.Redis.Password != " pass " {
		t.Fatalf("redis credentials changed: %+v", cfg.Redis)
	}
	if cfg.Redis.Database != 2 || cfg.Redis.MaxRetries != 1 || cfg.Redis.PoolSize != 5 || cfg.Redis.MaxActiveConns != 8 {
		t.Fatalf("redis pool overrides ignored: %+v", cfg.Redis)
	}
	if cfg.Redis.PoolTimeout != 250*time.Millisecond {
		t.Fatalf("redis pool timeout override ignored: %s", cfg.Redis.PoolTimeout)
	}
}

// 输入：t，测试上下文。
// 输出：非法 Redis 配置未返回包含对应变量名称的错误时使测试失败。
// 功能：验证地址、容量、重试次数和逻辑数据库的边界。
// 例如 REDIS_POOL_SIZE 为0时，要求配置加载指出连接池基础数量无效。
func TestRedisRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		key   string
		value string
	}{
		{"REDIS_ADDR", "6379"},
		{"REDIS_ADDR", ":6379"},
		{"REDIS_ADDR", "127.0.0.1:0"},
		{"REDIS_ADDR", "127.0.0.1:65536"},
		{"REDIS_DATABASE", "-1"},
		{"REDIS_MAX_RETRIES", "-1"},
		{"REDIS_POOL_SIZE", "0"},
		{"REDIS_POOL_SIZE", "21"},
		{"REDIS_MIN_IDLE_CONNS", "21"},
		{"REDIS_MAX_ACTIVE_CONNS", "0"},
	}
	for _, item := range cases {
		values := redisEnvironment()
		values[item.key] = item.value
		_, err := config.Load(values.Lookup)
		if err == nil || !strings.Contains(err.Error(), item.key) {
			t.Fatalf("invalid %s: %v", item.key, err)
		}
	}
}

// 输入：t，测试上下文。
// 输出：非法 Redis 时长未被拒绝时使测试失败。
// 功能：确保连接建立、读写、排队、启动检查和连接回收都具有正数期限。
// 例如 REDIS_POOL_TIMEOUT 为 "0s" 时，要求配置加载返回错误。
func TestRedisRejectsInvalidDurations(t *testing.T) {
	keys := []string{
		"REDIS_DIAL_TIMEOUT", "REDIS_READ_TIMEOUT", "REDIS_WRITE_TIMEOUT",
		"REDIS_POOL_TIMEOUT", "REDIS_STARTUP_TIMEOUT", "REDIS_CONN_MAX_LIFETIME",
		"REDIS_CONN_MAX_IDLE_TIME",
	}
	for _, key := range keys {
		values := redisEnvironment()
		values[key] = "0s"
		_, err := config.Load(values.Lookup)
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("invalid %s: %v", key, err)
		}
	}
}
