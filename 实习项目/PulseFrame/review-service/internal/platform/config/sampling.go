package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Sampling 控制已发布候选视频的入选条件。
type Sampling struct {
	Mode            string   `json:"mode"`             // random 抽取全部候选，exposure 先比较曝光增长。
	Percent         *float64 `json:"sample_percent"`   // 合格候选的抽取百分比，取值 0 至 100。
	WindowSeconds   int64    `json:"window_seconds"`   // 相邻统计窗口各自的时长，单位为秒。
	MinimumIncrease int64    `json:"minimum_increase"` // 后一个窗口至少多出的曝光次数。
	GrowthFactor    float64  `json:"growth_factor"`    // 后一个窗口相对前一个窗口的增长倍数。
}

// 输入：抽查配置文件路径。
// 输出：可使用的抽查参数，读取或校验失败时返回错误。
// 功能：启动时校验模式及阈值，例如曝光模式要求两个等长窗口。
func LoadSampling(path string) (Sampling, error) {
	var cfg Sampling
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return cfg, fmt.Errorf("unexpected trailing sampling config")
	}
	if cfg.Mode != "random" && cfg.Mode != "exposure" {
		return cfg, fmt.Errorf("invalid sampling mode %q", cfg.Mode)
	}
	if cfg.Percent == nil || *cfg.Percent < 0 || *cfg.Percent > 100 {
		return cfg, fmt.Errorf("sample_percent must be between 0 and 100")
	}
	if cfg.Mode == "exposure" && (cfg.WindowSeconds <= 0 || cfg.MinimumIncrease <= 0 || cfg.GrowthFactor <= 1) {
		return cfg, fmt.Errorf("invalid exposure window or growth thresholds")
	}
	return cfg, nil
}
