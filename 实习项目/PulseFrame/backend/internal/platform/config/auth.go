package config

import (
	"fmt"
	"time"
)

const (
	envMySQLDSN            = "MYSQL_DSN"
	envRedisAddress        = "REDIS_ADDR"
	envRedisUsername       = "REDIS_USERNAME"
	envRedisPassword       = "REDIS_PASSWORD"
	envRedisDatabase       = "REDIS_DATABASE"
	envWebOrigin           = "WEB_ORIGIN"
	envCookieSecure        = "AUTH_COOKIE_SECURE"
	envHashConcurrency     = "AUTH_HASH_CONCURRENCY"
	envLoginIPLimit        = "AUTH_LOGIN_IP_LIMIT"
	envLoginIdentityLimit  = "AUTH_LOGIN_IDENTITY_IP_LIMIT"
	envRegisterIPLimit     = "AUTH_REGISTER_IP_LIMIT"
	envLoginLimitWindow    = "AUTH_LOGIN_LIMIT_WINDOW"
	envRegisterLimitWindow = "AUTH_REGISTER_LIMIT_WINDOW"
	defaultHashConcurrency = 4
	maximumHashConcurrency = 8
	defaultLoginIPLimit    = 30
	defaultLoginIDLimit    = 10
	defaultRegisterLimit   = 8
	defaultLoginWindow     = 15 * time.Minute
	defaultRegisterWindow  = time.Hour
)

// MySQL 保存账号模块所需的数据库连接配置。
type MySQL struct {
	DSN string
}

// Redis 保存会话和共享限流所需的 Redis 连接配置。
type Redis struct {
	Address  string
	Username string
	Password string
	Database int
}

// Auth 保存网页登录来源、Cookie 安全策略、密码哈希容量和请求限流配置。
type Auth struct {
	WebOrigin               string
	CookieSecure            bool
	HashConcurrency         int
	LoginIPLimit            int
	LoginIdentityIPLimit    int
	RegistrationIPLimit     int
	LoginLimitWindow        time.Duration
	RegistrationLimitWindow time.Duration
}

// rateLimits 保存登录和注册使用的共享限流阈值。
type rateLimits struct {
	LoginIPLimit            int
	LoginIdentityIPLimit    int
	RegistrationIPLimit     int
	LoginLimitWindow        time.Duration
	RegistrationLimitWindow time.Duration
}

// loadDependencies 读取并校验数据库、Redis 和网页认证配置。
// 输入：lookup，环境变量查询函数；environment，已验证的运行环境。
// 输出：三类依赖配置；必需配置缺失或格式非法时返回错误。
// 功能：阻止服务在没有真实认证依赖或来源策略时启动。
func loadDependencies(lookup LookupEnv, environment string) (MySQL, Redis, Auth, error) {
	dsn, err := requiredValue(lookup, envMySQLDSN)
	if err != nil {
		return MySQL{}, Redis{}, Auth{}, err
	}
	if err := validateMySQLDSN(dsn); err != nil {
		return MySQL{}, Redis{}, Auth{}, err
	}
	redisConfig, err := loadRedisConfig(lookup)
	if err != nil {
		return MySQL{}, Redis{}, Auth{}, err
	}
	authConfig, err := loadAuthConfig(lookup, environment)
	if err != nil {
		return MySQL{}, Redis{}, Auth{}, err
	}
	return MySQL{DSN: dsn}, redisConfig, authConfig, nil
}

// loadRedisConfig 读取 Redis 地址、ACL 和逻辑库配置。
// 输入：lookup，环境变量查询函数。
// 输出：已验证的 Redis 连接配置；字段缺失、地址无效或数据库编号非法时返回错误。
// 功能：确保会话和限流连接始终使用具名 ACL 凭据。
func loadRedisConfig(lookup LookupEnv) (Redis, error) {
	address, err := requiredValue(lookup, envRedisAddress)
	if err != nil {
		return Redis{}, err
	}
	if err := validateRedisAddress(address); err != nil {
		return Redis{}, err
	}
	database, err := parseRedisDatabase(readValue(lookup, envRedisDatabase, "0"))
	if err != nil {
		return Redis{}, err
	}
	redisUsername, err := requiredValue(lookup, envRedisUsername)
	if err != nil {
		return Redis{}, err
	}
	redisPassword, err := requiredValue(lookup, envRedisPassword)
	if err != nil {
		return Redis{}, err
	}
	return Redis{Address: address, Username: redisUsername, Password: redisPassword, Database: database}, nil
}

// loadAuthConfig 读取网页来源、Cookie、哈希并发与限流配置。
// 输入：lookup，环境变量查询函数；environment，已验证的运行环境。
// 输出：已校验的网页认证策略；来源、安全标记或容量配置非法时返回错误。
// 功能：集中构造浏览器安全边界与认证资源上限。
func loadAuthConfig(lookup LookupEnv, environment string) (Auth, error) {
	origin, err := requiredValue(lookup, envWebOrigin)
	if err != nil {
		return Auth{}, err
	}
	requireHTTPS := environment == EnvironmentStaging || environment == EnvironmentProduction
	if err := validateWebOrigin(origin, requireHTTPS); err != nil {
		return Auth{}, err
	}
	secure, err := parseCookieSecure(lookup, environment)
	if err != nil {
		return Auth{}, err
	}
	hashConcurrency, err := parsePositiveInt(lookup, envHashConcurrency, defaultHashConcurrency)
	if err != nil {
		return Auth{}, err
	}
	if hashConcurrency > maximumHashConcurrency {
		return Auth{}, fmt.Errorf("%s must not exceed %d", envHashConcurrency, maximumHashConcurrency)
	}
	limits, err := loadRateLimits(lookup)
	if err != nil {
		return Auth{}, err
	}
	return Auth{
		WebOrigin: origin, CookieSecure: secure, HashConcurrency: hashConcurrency,
		LoginIPLimit: limits.LoginIPLimit, LoginIdentityIPLimit: limits.LoginIdentityIPLimit,
		RegistrationIPLimit: limits.RegistrationIPLimit,
		LoginLimitWindow:    limits.LoginLimitWindow, RegistrationLimitWindow: limits.RegistrationLimitWindow,
	}, nil
}

// loadRateLimits 解析登录和注册的限流次数与窗口。
// 输入：lookup，环境变量查询函数。
// 输出：正数阈值和时长；任一配置非法时返回错误。
// 功能：允许按部署环境和测量结果调整共享防滥用策略。
func loadRateLimits(lookup LookupEnv) (rateLimits, error) {
	loginIP, err := parsePositiveInt(lookup, envLoginIPLimit, defaultLoginIPLimit)
	if err != nil {
		return rateLimits{}, err
	}
	loginIdentity, err := parsePositiveInt(lookup, envLoginIdentityLimit, defaultLoginIDLimit)
	if err != nil {
		return rateLimits{}, err
	}
	registerIP, err := parsePositiveInt(lookup, envRegisterIPLimit, defaultRegisterLimit)
	if err != nil {
		return rateLimits{}, err
	}
	loginWindow, err := parsePositiveDurationValue(lookup, envLoginLimitWindow, defaultLoginWindow)
	if err != nil {
		return rateLimits{}, err
	}
	registerWindow, err := parsePositiveDurationValue(lookup, envRegisterLimitWindow, defaultRegisterWindow)
	if err != nil {
		return rateLimits{}, err
	}
	return rateLimits{loginIP, loginIdentity, registerIP, loginWindow, registerWindow}, nil
}
