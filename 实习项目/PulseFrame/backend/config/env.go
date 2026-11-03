package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	minimumPort = 1     // HTTP 和 MySQL 显式配置端口的最小值，不接受自动分配端口的零值。
	maximumPort = 65535 // TCP 端口的最大有效值，供各模块地址校验共用。
)

// LookupEnv 表示读取单个环境变量的函数。
// 输入：环境变量名称。
// 输出：环境变量值以及该变量是否存在。
// 功能：让各配置模块使用传入的环境来源，生产入口可传入 os.LookupEnv。
type LookupEnv func(string) (string, bool)

// 输入：lookup，环境查询函数；key，变量名；fallback，变量缺失或为空时的默认值。
// 输出：清理空白后的环境变量值或默认值。
// 功能：统一可选配置的读取规则；需要保留原值的密码由模块直接调用 lookup。
func readValue(lookup LookupEnv, key string, fallback string) string {
	value, exists := lookup(key)
	value = strings.TrimSpace(value)
	// 缺失、空字符串和仅含空白的可选值都采用调用方提供的默认值。
	if !exists || value == "" {
		return fallback
	}
	return value
}

// 输入：lookup，环境查询函数；key，变量名；fallback，可选值缺失时的默认时长。
// 输出：正数时长；格式非法、溢出或不大于零时返回错误。
// 功能：为 HTTP 和 MySQL 等模块提供统一的时长解析与边界校验。
func parsePositiveDuration(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	raw := readValue(lookup, key, fallback.String())
	value, err := time.ParseDuration(raw)
	// 保留配置名称与解析错误，让调用方能够定位具体的环境变量。
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	// 统一要求正数，防止零值关闭超时或回收机制，以及立即触发启动取消。
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return value, nil
}

// 输入：lookup，环境查询函数；key，变量名；fallback，缺省整数。
// 输出：非负整数；格式错误、溢出或负数返回错误。
// 功能：提供通用整数解析，零值是否允许及字段间的约束由各模块校验。
func parseNonNegativeInt(lookup LookupEnv, key string, fallback int) (int, error) {
	// 可选值缺失时使用默认值，已提供但格式错误或为负数时明确返回错误。
	value, err := strconv.Atoi(readValue(lookup, key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a nonnegative integer", key)
	}
	return value, nil
}
