package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	EnvironmentLocal      = "local"
	EnvironmentTest       = "test"
	EnvironmentStaging    = "staging"
	EnvironmentProduction = "production"

	envAppEnvironment     = "APP_ENV"
	envHTTPAddress        = "HTTP_ADDR"
	envLogLevel           = "LOG_LEVEL"
	envReadHeaderTimeout  = "HTTP_READ_HEADER_TIMEOUT"
	envReadTimeout        = "HTTP_READ_TIMEOUT"
	envWriteTimeout       = "HTTP_WRITE_TIMEOUT"
	envIdleTimeout        = "HTTP_IDLE_TIMEOUT"
	envShutdownTimeout    = "HTTP_SHUTDOWN_TIMEOUT"
	defaultHTTPAddress    = ":8080"
	defaultLogLevel       = "info"
	minimumPort           = 1
	maximumPort           = 65535
	defaultReadHeaderTime = 5 * time.Second
	defaultReadTime       = 15 * time.Second
	defaultWriteTime      = 15 * time.Second
	defaultIdleTime       = 60 * time.Second
	defaultShutdownTime   = 10 * time.Second
)

var errNilEnvironmentLookup = errors.New("environment lookup is required")

// LookupEnv 表示读取单个环境变量的函数。
// 输入：环境变量名称。
// 输出：环境变量值以及该变量是否存在。
// 功能：允许配置加载在测试中注入确定的环境来源。
type LookupEnv func(string) (string, bool)

// HTTP 保存 HTTP 服务生命周期配置。
type HTTP struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Config 保存核心 API 启动所需的工程配置。
type Config struct {
	Environment string
	LogLevel    string
	HTTP        HTTP
	MySQL       MySQL
	Redis       Redis
	Auth        Auth
}

// Load 从环境变量读取并校验配置。
// 输入：lookup，环境变量查询函数，不能为 nil。
// 输出：HTTP、MySQL、Redis 和网页认证的类型化配置；来源缺失或取值非法时返回错误。
// 功能：解析并校验环境变量；缺失或空值使用默认值，校验通过后将解析结果写入 Config 返回，失败时返回错误。
func Load(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errNilEnvironmentLookup
	}

	environment, err := parseEnvironment(readValue(lookup, envAppEnvironment, EnvironmentLocal))
	if err != nil {
		return Config{}, err
	}
	logLevel, err := parseLogLevel(readValue(lookup, envLogLevel, defaultLogLevel))
	if err != nil {
		return Config{}, err
	}
	httpConfig, err := loadHTTP(lookup)
	if err != nil {
		return Config{}, err
	}
	mysql, redis, auth, err := loadDependencies(lookup, environment)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Environment: environment,
		LogLevel:    logLevel,
		HTTP:        httpConfig,
		MySQL:       mysql,
		Redis:       redis,
		Auth:        auth,
	}, nil
}

// loadHTTP 读取并校验 HTTP 服务配置。
// 输入：lookup，已经确认非 nil 的环境变量查询函数。
// 输出：HTTP 配置；地址或超时非法时返回错误。
// 功能：将 HTTP 相关配置集中构造成不可变的值对象。
func loadHTTP(lookup LookupEnv) (HTTP, error) {
	address := readValue(lookup, envHTTPAddress, defaultHTTPAddress)
	if err := validateAddress(address); err != nil {
		return HTTP{}, err
	}

	readHeader, err := parsePositiveDuration(lookup, envReadHeaderTimeout, defaultReadHeaderTime)
	if err != nil {
		return HTTP{}, err
	}
	read, err := parsePositiveDuration(lookup, envReadTimeout, defaultReadTime)
	if err != nil {
		return HTTP{}, err
	}
	write, err := parsePositiveDuration(lookup, envWriteTimeout, defaultWriteTime)
	if err != nil {
		return HTTP{}, err
	}
	idle, err := parsePositiveDuration(lookup, envIdleTimeout, defaultIdleTime)
	if err != nil {
		return HTTP{}, err
	}
	shutdown, err := parsePositiveDuration(lookup, envShutdownTimeout, defaultShutdownTime)
	if err != nil {
		return HTTP{}, err
	}

	return HTTP{
		Address:           address,
		ReadHeaderTimeout: readHeader,
		ReadTimeout:       read,
		WriteTimeout:      write,
		IdleTimeout:       idle,
		ShutdownTimeout:   shutdown,
	}, nil
}

// readValue 获取经过空白清理的配置值。
// 输入：lookup，环境查询函数；key，变量名；fallback，变量缺失或为空时的默认值。
// 输出：清理后的环境变量值或明确的默认值。
// 功能：统一环境变量的空值处理规则。
func readValue(lookup LookupEnv, key string, fallback string) string {
	value, exists := lookup(key)
	value = strings.TrimSpace(value)
	if !exists || value == "" {
		return fallback
	}
	return value
}

// parseEnvironment 校验运行环境名称。
// 输入：value，待校验的环境名称。
// 输出：规范化环境名称；不支持的环境返回错误。
// 功能：限制运行环境集合，避免拼写错误改变服务行为。
func parseEnvironment(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case EnvironmentLocal, EnvironmentTest, EnvironmentStaging, EnvironmentProduction:
		return value, nil
	default:
		return "", fmt.Errorf("%s has unsupported value %q", envAppEnvironment, value)
	}
}

// parseLogLevel 校验结构化日志级别。
// 输入：value，待校验的日志级别。
// 输出：规范化日志级别；不支持的级别返回错误。
// 功能：在日志器创建前阻止错误配置进入运行阶段。
func parseLogLevel(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "debug", "info", "warn", "error":
		return value, nil
	default:
		return "", fmt.Errorf("%s has unsupported value %q", envLogLevel, value)
	}
}

// parsePositiveDuration 读取正数时长配置。
// 输入：lookup，环境查询函数；key，变量名；fallback，变量缺失时使用的时长。
// 输出：正数时长；格式非法或不大于零时返回错误。
// 功能：统一 HTTP 超时配置的解析与边界校验。
func parsePositiveDuration(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	raw := readValue(lookup, key, fallback.String())
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return value, nil
}

// validateAddress 校验 HTTP 监听地址和端口。
// 输入：address，符合 host:port 形式的监听地址。
// 输出：地址合法时返回 nil，否则返回包含配置名称的错误。
// 功能：在监听端口前暴露地址格式和端口范围问题。
func validateAddress(address string) error {
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s must use host:port format: %w", envHTTPAddress, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("%s port must be between %d and %d", envHTTPAddress, minimumPort, maximumPort)
	}
	return nil
}
