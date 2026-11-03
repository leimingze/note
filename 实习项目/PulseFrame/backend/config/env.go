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
	// 调用传入的 lookup 查询 key 指定的变量，得到原始文本及变量是否存在。
	value, exists := lookup(key)
	// TrimSpace 去除文本首尾空白，例如将 " 5s " 变为 "5s"，保留中间字符。
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
	// fallback.String() 将默认时长转为文本，例如 5*time.Second 转为 "5s"。
	// readValue 用 lookup 查询 key；缺失或为空时返回默认文本，raw 保存尚未解析的字符串。
	raw := readValue(lookup, key, fallback.String())
	// ParseDuration 将 "5s"、"250ms" 等文本转换为 time.Duration，无法解析时返回错误。
	value, err := time.ParseDuration(raw)
	// Errorf 将变量名加入错误信息；%w 保留原始错误，方便调用方继续检查原因。
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
	// Itoa 将默认整数转为文本；readValue 读取 key 对应的文本，缺失或为空时采用默认值。
	// Atoi 再将读取结果转为 int，例如 "20" 转为 20；格式错误或溢出时返回错误。
	value, err := strconv.Atoi(readValue(lookup, key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a nonnegative integer", key)
	}
	return value, nil
}
