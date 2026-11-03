package config

import (
	"errors"
	"fmt"
	"strings"
)

const (
	EnvironmentLocal      = "local"      // 本地开发环境，入口使用 Gin 调试模式。
	EnvironmentTest       = "test"       // 自动化测试环境，入口使用 Gin 测试模式。
	EnvironmentStaging    = "staging"    // 预发布环境，入口使用 Gin 发布模式。
	EnvironmentProduction = "production" // 生产环境，入口使用 Gin 发布模式。

	envAppEnvironment = "APP_ENV"   // 选择应用运行环境的变量名。
	envLogLevel       = "LOG_LEVEL" // 设置结构化日志级别的变量名。
	defaultLogLevel   = "info"      // 未配置日志级别时记录 info 及以上级别。
)

// errNilEnvironmentLookup 表示调用方没有提供环境变量读取函数。
var errNilEnvironmentLookup = errors.New("environment lookup is required")

// Config 保存核心 API 基座启动所需的工程配置。
type Config struct {
	Environment string // 经过校验的运行环境，供启动入口选择 Gin 模式。
	LogLevel    string // 经过校验的日志级别，供日志器初始化使用。
	HTTP        HTTP   // HTTP 模块加载的监听地址和生命周期期限。
	MySQL       MySQL  // MySQL 模块加载的连接信息和连接池参数。
}

// Load 从环境变量读取并校验配置。
// 输入：lookup，环境变量查询函数，不能为 nil。
// 输出：环境、日志、HTTP 和 MySQL 配置；必填项缺失或取值非法时返回错误。
// 功能：加载启动配置，可选参数使用默认值，数据库认证信息按原值读取。
func Load(lookup LookupEnv) (Config, error) {
	// 配置来源是必需依赖，缺失时无法区分“变量不存在”和读取失败。
	if lookup == nil {
		return Config{}, errNilEnvironmentLookup
	}

	// readValue 通过 lookup 读取 APP_ENV，缺失或为空时采用 local。
	// parseEnvironment 将读取的文本规范化并检查允许的环境名称，返回环境名或错误。
	environment, err := parseEnvironment(readValue(lookup, envAppEnvironment, EnvironmentLocal))
	// 环境非法时立即停止，避免框架进入错误模式。
	if err != nil {
		return Config{}, err
	}
	// 读取 LOG_LEVEL，默认使用 info；parseLogLevel 返回通过校验的日志级别文本。
	logLevel, err := parseLogLevel(readValue(lookup, envLogLevel, defaultLogLevel))
	// 日志级别错误也属于启动配置错误，不在运行期静默修正。
	if err != nil {
		return Config{}, err
	}
	// loadHTTP 使用同一个 lookup 读取地址和超时，返回完整 HTTP 配置或校验错误。
	httpConfig, err := loadHTTP(lookup)
	// HTTP 配置不完整时服务不能安全监听端口。
	if err != nil {
		return Config{}, err
	}
	// loadMySQL 读取认证信息和连接池参数，返回 MySQL 配置；此调用不会连接数据库。
	mysqlConfig, err := loadMySQL(lookup)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Environment: environment,
		LogLevel:    logLevel,
		HTTP:        httpConfig,
		MySQL:       mysqlConfig,
	}, nil
}

// parseEnvironment 校验运行环境名称。
// 输入：value，待校验的环境名称。
// 输出：规范化环境名称；不支持的环境返回错误。
// 功能：限制运行环境集合，避免拼写错误改变服务行为。
func parseEnvironment(value string) (string, error) {
	// TrimSpace 去除首尾空白，ToLower 转为小写，使 " LOCAL " 可以匹配 local。
	value = strings.ToLower(strings.TrimSpace(value))
	// 只接受显式支持的环境集合，default 分支负责暴露拼写错误。
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
	// 先去除首尾空白再转为小写，使 " INFO " 可以匹配 info。
	value = strings.ToLower(strings.TrimSpace(value))
	// 日志级别使用固定白名单，避免未知值进入 slog 配置。
	switch value {
	case "debug", "info", "warn", "error":
		return value, nil
	default:
		return "", fmt.Errorf("%s has unsupported value %q", envLogLevel, value)
	}
}
