package test

import (
	"testing"
	"time"

	"pulseframe/config"
)

const (
	envAppEnvironment    = "APP_ENV"               // 测试中设置运行环境的变量名。
	envHTTPAddress       = "HTTP_ADDR"             // 测试中设置监听地址的变量名。
	envLogLevel          = "LOG_LEVEL"             // 测试中设置日志级别的变量名。
	envReadTimeout       = "HTTP_READ_TIMEOUT"     // 测试中设置请求读取期限的变量名。
	envShutdownTimeout   = "HTTP_SHUTDOWN_TIMEOUT" // 测试中设置退出等待期限的变量名。
	expectedHTTPAddress  = ":8080"                 // 未配置监听地址时应采用的默认值。
	expectedLogLevel     = "info"                  // 未配置日志级别时应采用的默认值。
	expectedShutdownTime = 10 * time.Second        // 未配置退出等待时间时应采用的默认时长。
)

// envMap 是不修改真实进程环境的测试配置源。
type envMap map[string]string

// Lookup 查询测试环境变量。
// 输入：key，待查询的环境变量名称。
// 输出：测试值以及该名称是否存在。
// 功能：为配置测试提供不修改进程全局状态的环境来源。
// 例如映射中 APP_ENV 的值为 test 时返回 "test" 和 true；查询不存在的名称返回空字符串和 false。
func (values envMap) Lookup(key string) (string, bool) {
	value, exists := values[key]
	return value, exists
}

// TestLoadDefaults 验证仅提供数据库必填项时的基础默认配置。
// 输入：Go 测试框架提供的测试上下文。
// 输出：默认配置不符合预期时使测试失败。
// 功能：保护本地启动所需的基础默认值。
// 例如只提供数据库必填项时，检查监听地址为 :8080、日志级别为 info。
func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(mysqlEnvironment().Lookup)
	// 未指定基础可选参数时采用明确的本地默认值。
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if cfg.Environment != config.EnvironmentLocal || cfg.LogLevel != expectedLogLevel {
		t.Fatalf("unexpected environment or log level: %s %s", cfg.Environment, cfg.LogLevel)
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
// 例如设置 HTTP_READ_TIMEOUT 为 "3s" 后，检查读取期限为3秒。
func TestLoadOverrides(t *testing.T) {
	values := envMap{
		"MYSQL_USER": "config-test", "MYSQL_PASSWORD": "", "MYSQL_DATABASE": "config-test",
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
		t.Fatal("environment, log level or HTTP address did not match overrides")
	}
	// 各 HTTP 时长保持独立解析。
	if cfg.HTTP.ReadTimeout != 3*time.Second || cfg.HTTP.ShutdownTimeout != 6*time.Second {
		t.Fatalf("unexpected timeout config: %+v", cfg.HTTP)
	}
}

// TestLoadRejectsInvalidValues 验证非法基础配置会阻止启动。
// 输入：Go 测试框架提供的测试上下文。
// 输出：任一非法配置未返回错误时使测试失败。
// 功能：在 MySQL 必填配置有效时覆盖基础配置的校验边界。
// 例如设置 HTTP_SHUTDOWN_TIMEOUT 为 "0s" 后，要求配置加载返回错误。
func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := []envMap{
		{envAppEnvironment: "prod"},
		{envHTTPAddress: "8080"},
		{envShutdownTimeout: "0s"},
	}
	// 每组输入只包含一个错误，便于定位校验失效位置。
	for _, values := range cases {
		// 补齐有效的数据库配置，确保失败来自本轮待验证的基础配置项。
		complete := mysqlEnvironment()
		for key, value := range values {
			complete[key] = value
		}
		if _, err := config.Load(complete.Lookup); err == nil {
			t.Fatalf("expected invalid config to fail: %+v", values)
		}
	}
}

// TestLoadRequiresLookup 验证环境读取函数是必要依赖。
// 输入：Go 测试框架提供的测试上下文。
// 输出：nil 读取函数未返回错误时使测试失败。
// 功能：防止配置加载因缺少来源而发生 panic。
// 例如调用 config.Load(nil)，应返回缺少配置来源的错误。
func TestLoadRequiresLookup(t *testing.T) {
	// nil 无法区分变量缺失与读取异常，因此必须明确拒绝。
	if _, err := config.Load(nil); err == nil {
		t.Fatal("expected nil lookup to fail")
	}
}
