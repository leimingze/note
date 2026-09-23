package config

import (
	"testing"
	"time"
)

type envMap map[string]string

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
// 功能：保护本地启动所依赖的默认监听地址和超时。
func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(envMap{}.Lookup)
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if cfg.Environment != EnvironmentLocal || cfg.LogLevel != defaultLogLevel {
		t.Fatalf("unexpected base config: %+v", cfg)
	}
	if cfg.HTTP.Address != defaultHTTPAddress || cfg.HTTP.ShutdownTimeout != defaultShutdownTime {
		t.Fatalf("unexpected http defaults: %+v", cfg.HTTP)
	}
}

// TestLoadOverrides 验证合法环境变量能够覆盖默认配置。
// 输入：Go 测试框架提供的测试上下文。
// 输出：覆盖值没有正确解析时使测试失败。
// 功能：确认运行环境、监听地址、日志级别和时长均可配置。
func TestLoadOverrides(t *testing.T) {
	values := envMap{
		envAppEnvironment:    EnvironmentStaging,
		envHTTPAddress:       "127.0.0.1:18080",
		envLogLevel:          "debug",
		envReadHeaderTimeout: "2s",
		envReadTimeout:       "3s",
		envWriteTimeout:      "4s",
		envIdleTimeout:       "5s",
		envShutdownTimeout:   "6s",
	}
	cfg, err := Load(values.Lookup)
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}
	if cfg.Environment != EnvironmentStaging || cfg.HTTP.Address != "127.0.0.1:18080" {
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
	_, err := Load(envMap{envHTTPAddress: "8080"}.Lookup)
	if err == nil {
		t.Fatal("expected invalid address error")
	}
}

// TestLoadRejectsInvalidDuration 验证非法或非正数时长会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：非法时长未返回错误时使测试失败。
// 功能：避免关闭和连接超时被配置为无效值。
func TestLoadRejectsInvalidDuration(t *testing.T) {
	_, err := Load(envMap{envShutdownTimeout: "0s"}.Lookup)
	if err == nil {
		t.Fatal("expected invalid duration error")
	}
}

// TestLoadRejectsUnsupportedEnvironment 验证未知运行环境会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：未知环境未返回错误时使测试失败。
// 功能：防止环境名称拼写错误改变 Gin 或日志行为。
func TestLoadRejectsUnsupportedEnvironment(t *testing.T) {
	_, err := Load(envMap{envAppEnvironment: "prod"}.Lookup)
	if err == nil {
		t.Fatal("expected unsupported environment error")
	}
}
