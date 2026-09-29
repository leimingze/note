package config

import (
	"testing"
	"time"
)

type envMap map[string]string

// validEnvironment 构造包含外部依赖的测试配置并应用覆盖项。
// 输入：overrides，要替换的环境变量；允许为 nil。
// 输出：包含必需 MySQL、Redis 和网页来源值的独立环境映射。
// 功能：让配置测试只覆盖当前关注的变量。
func validEnvironment(overrides envMap) envMap {
	values := envMap{
		envMySQLDSN:      "app:secret@tcp(127.0.0.1:3306)/pulseframe?parseTime=true",
		envRedisAddress:  "127.0.0.1:6379",
		envRedisUsername: "pulseframe_api",
		envRedisPassword: "test-redis-password",
		envWebOrigin:     "http://127.0.0.1:5173",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return values
}

// Lookup 查询测试环境变量。
// 输入：key，待查询的环境变量名称。
// 输出：测试值以及该名称是否存在。
// 功能：为配置测试提供不修改进程全局状态的环境来源。
func (values envMap) Lookup(key string) (string, bool) {
	value, exists := values[key]
	return value, exists
}

// TestLoadDefaults 验证无环境变量时的默认配置。
// 输入：Go 测试框架提供的测试上下文。
// 输出：默认配置不符合预期时使测试失败。
// 功能：保护本地启动所依赖的默认 HTTP、认证限流和哈希并发配置。
func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(validEnvironment(nil).Lookup)
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if cfg.Environment != EnvironmentLocal || cfg.LogLevel != defaultLogLevel {
		t.Fatalf("unexpected base config: %+v", cfg)
	}
	if cfg.HTTP.Address != defaultHTTPAddress || cfg.HTTP.ShutdownTimeout != defaultShutdownTime {
		t.Fatalf("unexpected http defaults: %+v", cfg.HTTP)
	}
	if cfg.Auth.LoginIPLimit != defaultLoginIPLimit || cfg.Auth.LoginIdentityIPLimit != defaultLoginIDLimit ||
		cfg.Auth.RegistrationIPLimit != defaultRegisterLimit || cfg.Auth.LoginLimitWindow != defaultLoginWindow ||
		cfg.Auth.HashConcurrency != defaultHashConcurrency {
		t.Fatalf("unexpected authentication rate defaults: %+v", cfg.Auth)
	}
}

// TestLoadOverrides 验证合法环境变量能够覆盖默认配置。
// 输入：Go 测试框架提供的测试上下文。
// 输出：覆盖值没有正确解析时使测试失败。
// 功能：确认运行环境、监听地址、日志级别和时长均可配置。
func TestLoadOverrides(t *testing.T) {
	values := validEnvironment(envMap{
		envAppEnvironment:    EnvironmentStaging,
		envWebOrigin:         "https://app.example.test",
		envHTTPAddress:       "127.0.0.1:18080",
		envLogLevel:          "debug",
		envReadHeaderTimeout: "2s",
		envReadTimeout:       "3s",
		envWriteTimeout:      "4s",
		envIdleTimeout:       "5s",
		envShutdownTimeout:   "6s",
	})
	cfg, err := Load(values.Lookup)
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}
	if cfg.Environment != EnvironmentStaging || cfg.HTTP.Address != "127.0.0.1:18080" || !cfg.Auth.CookieSecure {
		t.Fatalf("unexpected override config: %+v", cfg)
	}
	if cfg.HTTP.ReadTimeout != 3*time.Second || cfg.HTTP.ShutdownTimeout != 6*time.Second {
		t.Fatalf("unexpected timeout config: %+v", cfg.HTTP)
	}
}

// TestLoadRejectsInvalidAddress 验证非法监听地址会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：非法地址未返回错误时使测试失败。
// 功能：保证地址问题在创建 HTTP Server 前暴露。
func TestLoadRejectsInvalidAddress(t *testing.T) {
	_, err := Load(validEnvironment(envMap{envHTTPAddress: "8080"}).Lookup)
	if err == nil {
		t.Fatal("expected invalid address error")
	}
}

// TestLoadRejectsInvalidDuration 验证非法或非正数时长会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：非法时长未返回错误时使测试失败。
// 功能：避免关闭和连接超时被配置为无效值。
func TestLoadRejectsInvalidDuration(t *testing.T) {
	_, err := Load(validEnvironment(envMap{envShutdownTimeout: "0s"}).Lookup)
	if err == nil {
		t.Fatal("expected invalid duration error")
	}
}

// TestLoadRejectsUnsupportedEnvironment 验证未知运行环境会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：未知环境未返回错误时使测试失败。
// 功能：防止环境名称拼写错误改变 Gin 或日志行为。
func TestLoadRejectsUnsupportedEnvironment(t *testing.T) {
	_, err := Load(validEnvironment(envMap{envAppEnvironment: "prod"}).Lookup)
	if err == nil {
		t.Fatal("expected unsupported environment error")
	}
}

// TestLoadRequiresExternalDependencies 验证数据库、Redis 和网页来源不能缺省。
// 输入：Go 测试框架提供的测试上下文。
// 输出：缺少任一必需配置仍能加载时使测试失败。
// 功能：防止服务以空依赖启动并伪装为可用。
func TestLoadRequiresExternalDependencies(t *testing.T) {
	for _, key := range []string{envMySQLDSN, envRedisAddress, envRedisUsername, envRedisPassword, envWebOrigin} {
		values := validEnvironment(nil)
		delete(values, key)
		if _, err := Load(values.Lookup); err == nil {
			t.Errorf("expected %s to be required", key)
		}
	}
}

// TestLoadRejectsRootMySQLUser 验证认证应用不能使用 MySQL root 账号。
// 输入：Go 测试框架提供的测试上下文。
// 输出：root DSN 未被拒绝时使测试失败。
// 功能：将数据库最小权限要求落实到启动配置校验。
func TestLoadRejectsRootMySQLUser(t *testing.T) {
	values := validEnvironment(envMap{envMySQLDSN: "root:secret@tcp(127.0.0.1:3306)/pulseframe?parseTime=true"})
	if _, err := Load(values.Lookup); err == nil {
		t.Fatal("expected MySQL root user to be rejected")
	}
}

// TestLoadRejectsInsecureProductionCookie 验证线上环境不能关闭安全 Cookie。
// 输入：Go 测试框架提供的测试上下文。
// 输出：生产环境接受 AUTH_COOKIE_SECURE=false 时使测试失败。
// 功能：阻止线上会话令牌通过明文 HTTP 传输。
func TestLoadRejectsInsecureProductionCookie(t *testing.T) {
	values := validEnvironment(envMap{
		envAppEnvironment: EnvironmentProduction,
		envCookieSecure:   "false",
	})
	if _, err := Load(values.Lookup); err == nil {
		t.Fatal("expected insecure production cookie configuration to fail")
	}
}

// TestLoadParsesAuthConfiguration 验证认证来源和 Redis 数据库配置。
// 输入：Go 测试框架提供的测试上下文。
// 输出：解析值不符合预期时使测试失败。
// 功能：保护 Redis 逻辑库和允许网页来源的显式配置。
func TestLoadParsesAuthConfiguration(t *testing.T) {
	values := validEnvironment(envMap{
		envRedisDatabase: "2",
		envRedisUsername: "pulseframe",
		envRedisPassword: " secret ",
		envCookieSecure:  "true",
	})
	cfg, err := Load(values.Lookup)
	if err != nil {
		t.Fatalf("load auth config: %v", err)
	}
	if cfg.Redis.Database != 2 || cfg.Redis.Username != "pulseframe" || cfg.Redis.Password != " secret " {
		t.Fatal("Redis ACL settings were not preserved")
	}
	if !cfg.Auth.CookieSecure || cfg.Auth.WebOrigin != "http://127.0.0.1:5173" {
		t.Fatalf("unexpected auth config: %+v", cfg.Auth)
	}
}

// TestLoadRejectsNonHTTPSProductionOrigin 验证线上网页来源必须使用 HTTPS。
// 输入：Go 测试框架提供的测试上下文。
// 输出：生产环境接受 HTTP 来源时使测试失败。
// 功能：保护 Secure Cookie 不被配置到明文网页来源。
func TestLoadRejectsNonHTTPSProductionOrigin(t *testing.T) {
	values := validEnvironment(envMap{envAppEnvironment: EnvironmentProduction})
	if _, err := Load(values.Lookup); err == nil {
		t.Fatal("expected production HTTP origin to fail")
	}
}

// TestLoadValidatesRateLimitConfiguration 验证限流阈值支持合法覆盖并拒绝零值。
// 输入：Go 测试框架提供的测试上下文。
// 输出：覆盖值未解析或零阈值未拒绝时使测试失败。
// 功能：确保限流策略按环境显式校准。
func TestLoadValidatesRateLimitConfiguration(t *testing.T) {
	values := validEnvironment(envMap{envLoginIPLimit: "42", envLoginLimitWindow: "20m"})
	cfg, err := Load(values.Lookup)
	if err != nil {
		t.Fatalf("load rate limit override: %v", err)
	}
	if cfg.Auth.LoginIPLimit != 42 || cfg.Auth.LoginLimitWindow != 20*time.Minute {
		t.Fatalf("unexpected rate limit config: %+v", cfg.Auth)
	}
	if _, err := Load(validEnvironment(envMap{envRegisterIPLimit: "0"}).Lookup); err == nil {
		t.Fatal("expected zero rate limit to fail")
	}
}

// TestLoadValidatesHashConcurrency 验证 Argon2id 并发容量可配置且必须为正数。
// 输入：Go 测试框架提供的测试上下文。
// 输出：配置覆盖或非法并发数处理不符合预期时使测试失败。
// 功能：确保密码哈希资源上限能够通过部署配置调整。
func TestLoadValidatesHashConcurrency(t *testing.T) {
	values := validEnvironment(envMap{envHashConcurrency: "2"})
	cfg, err := Load(values.Lookup)
	if err != nil || cfg.Auth.HashConcurrency != 2 {
		t.Fatalf("unexpected hash concurrency config: %+v, %v", cfg.Auth, err)
	}
	if _, err := Load(validEnvironment(envMap{envHashConcurrency: "0"}).Lookup); err == nil {
		t.Fatal("expected zero hash concurrency to fail")
	}
	if _, err := Load(validEnvironment(envMap{envHashConcurrency: "9"}).Lookup); err == nil {
		t.Fatal("expected excessive hash concurrency to fail")
	}
}
