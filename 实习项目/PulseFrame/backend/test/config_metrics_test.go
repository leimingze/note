package test

import (
	"strings"
	"testing"
	"time"

	"pulseframe/internal/platform/config"
)

// TestMetricsDefaults 验证默认指标配置。
// 输入：测试上下文。
// 输出：默认值不符合预期时测试失败。
// 功能：保护指标开关、暴露路径和采集周期的默认值。
func TestMetricsDefaults(t *testing.T) {
	cfg, err := config.Load(redisEnvironment().Lookup)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Metrics.Enabled || cfg.Metrics.Path != "/metrics" || cfg.Metrics.CollectInterval != 5*time.Second {
		t.Fatalf("unexpected metrics defaults: %+v", cfg.Metrics)
	}
}

// TestMetricsOverrides 验证指标配置覆盖。
// 输入：测试上下文。
// 输出：覆盖值未读取时测试失败。
// 功能：确认指标开关、路径和采集周期均可配置。
func TestMetricsOverrides(t *testing.T) {
	values := redisEnvironment()
	values["METRICS_ENABLED"] = "false"
	values["METRICS_PATH"] = "/custom-metrics"
	values["METRICS_COLLECT_INTERVAL"] = "10s"
	cfg, err := config.Load(values.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metrics.Enabled || cfg.Metrics.Path != "/custom-metrics" || cfg.Metrics.CollectInterval != 10*time.Second {
		t.Fatalf("unexpected metrics overrides: %+v", cfg.Metrics)
	}
}

// TestMetricsRejectsInvalid 验证非法指标配置被拒绝。
// 输入：测试上下文。
// 输出：非法值未被拒绝时测试失败。
// 功能：确保路径必须以斜杠开头、采集周期大于零且布尔值合法。
func TestMetricsRejectsInvalid(t *testing.T) {
	cases := []struct {
		key   string
		value string
	}{
		{"METRICS_PATH", "metrics"},
		{"METRICS_COLLECT_INTERVAL", "0s"},
		{"METRICS_ENABLED", "yes"},
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
