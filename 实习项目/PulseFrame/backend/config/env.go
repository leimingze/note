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
// 例如传入 os.LookupEnv，且环境变量 APP_ENV 的值为 local 时，查询 APP_ENV 会得到 "local" 和 true。
type LookupEnv func(string) (string, bool)

// 输入：lookup，环境查询函数；key，变量名；fallback，变量缺失或为空时的默认值。
// 输出：清理空白后的环境变量值或默认值。
// 功能：读取配置文本并去掉首尾空白，没有设置或内容为空时使用默认值。
// 例如配置为 " 5s " 时返回 "5s"；未设置且默认值为 "3s" 时返回 "3s"。
func readValue(lookup LookupEnv, key string, fallback string) string {
	// 按指定的名称读取环境变量，同时确认这个变量有没有设置。
	value, exists := lookup(key)
	// 去掉内容首尾的空白，例如把 " 5s " 整理成 "5s"，方便后面识别。
	value = strings.TrimSpace(value)
	// 如果没有设置，或者去掉空白后没有内容，就使用事先给定的默认值。
	if !exists || value == "" {
		return fallback
	}
	return value
}

// 输入：lookup，环境查询函数；key，变量名；fallback，可选值缺失时的默认时长。
// 输出：正数时长；格式非法、溢出或不大于零时返回错误。
// 功能：读取时长配置，将文本转换为时长，并确保结果大于零。
// 例如配置为 "5s" 时返回5秒的时长，配置为 "0s" 或 "abc" 时返回错误。
func parsePositiveDuration(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	// 环境变量读出来都是文字，所以先把默认时长也变成文字，例如把5秒变成 "5s"。
	// 然后读取需要的环境变量；如果没有设置或内容为空，就采用这份默认值。
	raw := readValue(lookup, key, fallback.String())
	// 此时拿到的仍是 "5s" 这样的文字，这里将它转换为程序能计算的时间长度。
	value, err := time.ParseDuration(raw)
	// 如果写成了 "abc" 等无法识别的内容，就指出哪个环境变量有问题，并保留原因。
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	// 能识别为时长还不够，结果必须大于零，才能作为有效的超时或连接回收期限。
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return value, nil
}

// 输入：lookup，环境查询函数；key，变量名；fallback，缺省整数。
// 输出：非负整数；格式错误、溢出或负数返回错误。
// 功能：读取整数配置，将文本转换为整数，并确保结果不小于零。
// 例如配置为 "20" 时返回整数20，配置为 "0" 时返回0，配置为 "-1" 或 "abc" 时返回错误。
func parseNonNegativeInt(lookup LookupEnv, key string, fallback int) (int, error) {
	// 先把默认整数写成文字，再读取需要的环境变量；没有设置或内容为空时采用默认值。
	// 最后将取得的文字转换成整数，例如把 "20" 变成数字20，供后面设置连接数量。
	value, err := strconv.Atoi(readValue(lookup, key, strconv.Itoa(fallback)))
	// 内容无法转换为整数、超出整数范围或小于零时，直接报告配置错误。
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a nonnegative integer", key)
	}
	return value, nil
}
