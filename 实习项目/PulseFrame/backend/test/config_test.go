package test

import (
	"testing"
	"time"

	"pulseframe/config"
)

const (
	// env* 镜像生产配置名称，测试通过它们构造独立输入。
	envAppEnvironment    = "APP_ENV"
	envHTTPAddress       = "HTTP_ADDR"
	envLogLevel          = "LOG_LEVEL"
	envReadTimeout       = "HTTP_READ_TIMEOUT"
	envShutdownTimeout   = "HTTP_SHUTDOWN_TIMEOUT"
	expectedHTTPAddress  = ":8080"
	expectedLogLevel     = "info"
	expectedShutdownTime = 10 * time.Second
)

// envMap 是不修改真实进程环境的测试配置源。
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
// 功能：保护本地启动所需的基础默认值。
func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(envMap{}.Lookup)
	// 空环境应采用明确的本地默认值。
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if cfg.Environment != config.EnvironmentLocal || cfg.LogLevel != expectedLogLevel {
		t.Fatalf("unexpected base config: %+v", cfg)
	}
	// HTTP 地址和关闭期限是本地运行的关键默认项。
	if cfg.HTTP.Address != expectedHTTPAddress || cfg.HTTP.ShutdownTimeout != expectedShutdownTime {
		t.Fatalf("unexpected http defaults: %+v", cfg.HTTP)
	}
}

// TestLoadOverrides 验证合法环境变量能够覆盖默认配置。
// 输入：Go 测试框架提供的测试上下文。
// 输出：覆盖值没有正确解析时使测试失败。
// 功能：确认运行环境、监听地址、日志级别和时长均可配置。
func TestLoadOverrides(t *testing.T) {
	values := envMap{
		envAppEnvironment:  config.EnvironmentStaging,
		envHTTPAddress:     "127.0.0.1:18080",
		envLogLevel:        "debug",
		envReadTimeout:     "3s",
		envShutdownTimeout: "6s",
	}
	cfg, err := config.Load(values.Lookup)
	// 合法覆盖组合必须能够完整加载。
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}
	if cfg.Environment != config.EnvironmentStaging || cfg.LogLevel != "debug" || cfg.HTTP.Address != "127.0.0.1:18080" {
		t.Fatalf("unexpected override config: %+v", cfg)
	}
	// 各 HTTP 时长保持独立解析。
	if cfg.HTTP.ReadTimeout != 3*time.Second || cfg.HTTP.ShutdownTimeout != 6*time.Second {
		t.Fatalf("unexpected timeout config: %+v", cfg.HTTP)
	}
}

// TestLoadRejectsInvalidValues 验证非法基础配置会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：任一非法配置未返回错误时使测试失败。
// 功能：覆盖环境名称、监听地址和正数时长的校验边界。
func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := []envMap{
		{envAppEnvironment: "prod"},
		{envHTTPAddress: "8080"},
		{envShutdownTimeout: "0s"},
	}
	// 每组输入只包含一个错误，便于定位校验失效位置。
	for _, values := range cases {
		if _, err := config.Load(values.Lookup); err == nil {
			t.Fatalf("expected invalid config to fail: %+v", values)
		}
	}
}

// TestLoadRequiresLookup 验证环境读取函数是必要依赖。
// 输入：Go 测试框架提供的测试上下文。
// 输出：nil 读取函数未返回错误时使测试失败。
// 功能：防止配置加载因缺少来源而发生 panic。
func TestLoadRequiresLookup(t *testing.T) {
	// nil 无法区分变量缺失与读取异常，因此必须明确拒绝。
	if _, err := config.Load(nil); err == nil {
		t.Fatal("expected nil lookup to fail")
	}
}
