package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	envMetricsEnabled         = "METRICS_ENABLED"          // 是否启用指标端口的环境变量名。
	envMetricsPath            = "METRICS_PATH"             // 指标暴露路径的环境变量名。
	envMetricsCollectInterval = "METRICS_COLLECT_INTERVAL" // 连接池统计采集周期环境变量名。

	defaultMetricsEnabled         = true            // 默认启用指标端口，便于直接观察服务状态。
	defaultMetricsPath            = "/metrics"      // 默认指标暴露路径，符合常见约定。
	defaultMetricsCollectInterval = 5 * time.Second // 默认每5秒采集一次连接池统计。
)

// Metrics 保存指标服务配置。
type Metrics struct {
	Enabled         bool          // 是否注册指标端口并启动采集器。
	Path            string        // 指标暴露路径，例如 /metrics，必须以斜杠开头。
	CollectInterval time.Duration // 连接池统计采集周期，必须大于零。
}

// 输入：lookup，已确认非 nil 的环境变量查询函数。
// 输出：Metrics 配置；参数非法时返回错误。
// 功能：读取指标开关、暴露路径和采集周期，并校验取值合法。
// 例如 METRICS_COLLECT_INTERVAL 为 "10s" 时，得到每10秒采集一次的配置。
func loadMetrics(lookup LookupEnv) (Metrics, error) {
	cfg := Metrics{
		Enabled:         defaultMetricsEnabled,
		Path:            defaultMetricsPath,
		CollectInterval: defaultMetricsCollectInterval,
	}
	// 先读取是否启用指标；布尔值无法解析时配置加载失败。
	enabled, err := parseBool(lookup, envMetricsEnabled, defaultMetricsEnabled)
	if err != nil {
		return Metrics{}, err
	}
	cfg.Enabled = enabled
	// 读取指标暴露路径，未设置时沿用默认的 /metrics。
	cfg.Path = readValue(lookup, envMetricsPath, defaultMetricsPath)
	// 读取采集周期，时长非法或小于等于零时返回错误。
	interval, err := parsePositiveDuration(lookup, envMetricsCollectInterval, defaultMetricsCollectInterval)
	if err != nil {
		return Metrics{}, err
	}
	cfg.CollectInterval = interval
	return cfg, cfg.validate()
}

// 输入：cfg，指标配置。
// 输出：路径和周期合法时返回 nil，否则返回包含配置名称的错误。
// 功能：检查指标暴露路径与非斜杠开头以及采集周期大于零。
// 例如路径为 /metrics、周期为5秒时通过；路径为 metrics 或周期为零时返回错误。
func (cfg Metrics) validate() error {
	// 指标路径必须从斜杠开始，避免与业务路由冲突或无法解析。
	if !strings.HasPrefix(cfg.Path, "/") {
		return fmt.Errorf("%s must start with a slash", envMetricsPath)
	}
	// 采集周期为零或负数时无法定期刷新连接池统计，直接拒绝。
	if cfg.CollectInterval <= 0 {
		return fmt.Errorf("%s must be greater than zero", envMetricsCollectInterval)
	}
	return nil
}
