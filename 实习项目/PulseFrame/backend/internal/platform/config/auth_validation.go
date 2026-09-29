package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// validateMySQLDSN 校验 MySQL 连接必须使用专用非 root 账号和明确数据库。
// 输入：dsn，go-sql-driver/mysql 格式的连接字符串。
// 输出：配置安全且可解析时返回 nil；解析失败、缺少数据库或使用 root 时返回错误。
// 功能：阻止认证服务以高权限账号或未指定业务库的配置启动。
func validateMySQLDSN(dsn string) error {
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("%s must be a valid MySQL DSN", envMySQLDSN)
	}
	if strings.TrimSpace(parsed.User) == "" || strings.EqualFold(parsed.User, "root") || parsed.DBName == "" {
		return fmt.Errorf("%s must use a dedicated non-root user and specify a database", envMySQLDSN)
	}
	return nil
}

// parsePositiveInt 解析大于零的环境变量整数。
// 输入：lookup，环境变量查询函数；key，变量名；fallback，缺省阈值。
// 输出：正整数；格式非法或不大于零时返回错误。
// 功能：校验请求频次阈值。
func parsePositiveInt(lookup LookupEnv, key string, fallback int) (int, error) {
	value, err := strconv.Atoi(readValue(lookup, key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

// parsePositiveDurationValue 解析大于零的限流时间窗口。
// 输入：lookup，环境变量查询函数；key，变量名；fallback，缺省时长。
// 输出：正时长；格式非法或不大于零时返回错误。
// 功能：校验限流桶过期窗口。
func parsePositiveDurationValue(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	value, err := time.ParseDuration(readValue(lookup, key, fallback.String()))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

// requiredValue 读取不能使用隐式默认值的环境配置。
// 输入：lookup，环境查询函数；key，必需变量名称。
// 输出：原始配置值；缺失或仅含空白时返回错误。
// 功能：保留密码等配置的原始字符，同时拒绝空配置。
func requiredValue(lookup LookupEnv, key string) (string, error) {
	value, exists := lookup(key)
	if !exists || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

// validateRedisAddress 校验 Redis TCP 地址。
// 输入：address，host:port 格式的 Redis 地址。
// 输出：地址有效时返回 nil，否则返回配置错误。
// 功能：在创建 Redis 客户端前拒绝无效端点。
func validateRedisAddress(address string) error {
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s must use host:port format: %w", envRedisAddress, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("%s port must be between %d and %d", envRedisAddress, minimumPort, maximumPort)
	}
	return nil
}

// parseRedisDatabase 解析非负 Redis 数据库编号。
// 输入：value，环境变量中的十进制数据库编号。
// 输出：数据库编号；格式非法或为负数时返回错误。
// 功能：避免 Redis 客户端静默选择错误的逻辑数据库。
func parseRedisDatabase(value string) (int, error) {
	database, err := strconv.Atoi(value)
	if err != nil || database < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", envRedisDatabase)
	}
	return database, nil
}

// validateWebOrigin 校验浏览器来源必须是完整 origin 而非 URL 路径。
// 输入：origin，scheme://host[:port] 格式的单一来源；requireHTTPS，是否强制 HTTPS。
// 输出：来源有效时返回 nil，否则返回配置错误。
// 功能：确保 CSRF 来源比较不会因路径、凭据或 URL 片段产生歧义。
func validateWebOrigin(origin string, requireHTTPS bool) error {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (requireHTTPS && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an http or https origin without path or credentials", envWebOrigin)
	}
	return nil
}

// parseCookieSecure 解析 Cookie Secure 策略。
// 输入：lookup，环境查询函数；environment，已经验证的运行环境。
// 输出：Cookie 是否设置 Secure；非法布尔配置时返回错误。
// 功能：生产和预发布默认强制 HTTPS Cookie，开发环境使用独立设置。
func parseCookieSecure(lookup LookupEnv, environment string) (bool, error) {
	value, exists := lookup(envCookieSecure)
	if !exists || strings.TrimSpace(value) == "" {
		return environment == EnvironmentStaging || environment == EnvironmentProduction, nil
	}
	secure, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", envCookieSecure, err)
	}
	if (environment == EnvironmentStaging || environment == EnvironmentProduction) && !secure {
		return false, fmt.Errorf("%s cannot be false outside local and test environments", envCookieSecure)
	}
	return secure, nil
}
