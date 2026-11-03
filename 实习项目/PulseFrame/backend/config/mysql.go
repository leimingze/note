package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMySQLAddress        = "127.0.0.1:3306"
	defaultMySQLMaxOpenConns   = 20
	defaultMySQLMaxIdleConns   = 5
	defaultMySQLConnectTimeout = 3 * time.Second
	defaultMySQLIOTimeout      = 5 * time.Second
	defaultMySQLStartupTimeout = 5 * time.Second
	defaultMySQLConnLifetime   = 30 * time.Minute
	defaultMySQLConnIdleTime   = 5 * time.Minute
)

// MySQL 保存单个进程使用的数据库连接池配置。
type MySQL struct {
	Address         string
	User            string
	Password        string
	Database        string
	TLSMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	StartupTimeout  time.Duration
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// 输入：lookup，已确认非 nil 的环境变量查询函数。
// 输出：MySQL 配置；必填项缺失或参数非法时返回错误。
// 功能：读取连接信息和连接池参数，密码始终保留原值。
func loadMySQL(lookup LookupEnv) (MySQL, error) {
	password, exists := lookup("MYSQL_PASSWORD")
	if !exists {
		return MySQL{}, fmt.Errorf("MYSQL_PASSWORD must be explicitly set")
	}
	user, _ := lookup("MYSQL_USER")
	database, _ := lookup("MYSQL_DATABASE")
	cfg := MySQL{
		Address: readValue(lookup, "MYSQL_ADDR", defaultMySQLAddress),
		User:    user, Password: password, Database: database,
		TLSMode: readValue(lookup, "MYSQL_TLS_MODE", "false"),
	}
	var err error
	cfg.MaxOpenConns, err = parseNonNegativeInt(lookup, "MYSQL_MAX_OPEN_CONNS", defaultMySQLMaxOpenConns)
	if err != nil {
		return MySQL{}, err
	}
	cfg.MaxIdleConns, err = parseNonNegativeInt(lookup, "MYSQL_MAX_IDLE_CONNS", defaultMySQLMaxIdleConns)
	if err != nil {
		return MySQL{}, err
	}
	durations := []struct {
		key      string
		fallback time.Duration
		target   *time.Duration
	}{
		{"MYSQL_CONNECT_TIMEOUT", defaultMySQLConnectTimeout, &cfg.ConnectTimeout},
		{"MYSQL_READ_TIMEOUT", defaultMySQLIOTimeout, &cfg.ReadTimeout},
		{"MYSQL_WRITE_TIMEOUT", defaultMySQLIOTimeout, &cfg.WriteTimeout},
		{"MYSQL_STARTUP_TIMEOUT", defaultMySQLStartupTimeout, &cfg.StartupTimeout},
		{"MYSQL_CONN_MAX_LIFETIME", defaultMySQLConnLifetime, &cfg.ConnMaxLifetime},
		{"MYSQL_CONN_MAX_IDLE_TIME", defaultMySQLConnIdleTime, &cfg.ConnMaxIdleTime},
	}
	for _, item := range durations {
		value, err := parsePositiveDuration(lookup, item.key, item.fallback)
		if err != nil {
			return MySQL{}, err
		}
		*item.target = value
	}
	return cfg, cfg.Validate()
}

// 输入：cfg，环境读取或调用方直接构造的 MySQL 配置。
// 输出：合法时返回 nil，否则返回包含配置名称的错误。
// 功能：在建立网络连接前拒绝无效地址、无限连接数量和无效超时。
func (cfg MySQL) Validate() error {
	if err := cfg.validateConnection(); err != nil {
		return err
	}
	return cfg.validatePool()
}

// 输入：cfg，MySQL 连接配置。
// 输出：地址、账号、数据库或 TLS 模式非法时返回错误。
// 功能：校验连接目标，防止驱动自动补全缺失的必要信息。
func (cfg MySQL) validateConnection() error {
	for key, value := range map[string]string{"MYSQL_USER": cfg.User, "MYSQL_DATABASE": cfg.Database} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	host, portText, err := net.SplitHostPort(cfg.Address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("MYSQL_ADDR must use a nonempty host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("MYSQL_ADDR port must be between %d and %d", minimumPort, maximumPort)
	}
	if cfg.TLSMode != "false" && cfg.TLSMode != "true" {
		return fmt.Errorf("MYSQL_TLS_MODE must be false or true")
	}
	return nil
}

// 输入：cfg，连接池容量及期限配置。
// 输出：容量关系或时长非法时返回错误。
// 功能：确保连接数量有上限，等待网络与连接回收均具有明确期限。
func (cfg MySQL) validatePool() error {
	if cfg.MaxOpenConns <= 0 {
		return fmt.Errorf("MYSQL_MAX_OPEN_CONNS must be greater than zero")
	}
	if cfg.MaxIdleConns < 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return fmt.Errorf("MYSQL_MAX_IDLE_CONNS must be between zero and MYSQL_MAX_OPEN_CONNS")
	}
	for key, value := range map[string]time.Duration{
		"MYSQL_CONNECT_TIMEOUT":    cfg.ConnectTimeout,
		"MYSQL_READ_TIMEOUT":       cfg.ReadTimeout,
		"MYSQL_WRITE_TIMEOUT":      cfg.WriteTimeout,
		"MYSQL_STARTUP_TIMEOUT":    cfg.StartupTimeout,
		"MYSQL_CONN_MAX_LIFETIME":  cfg.ConnMaxLifetime,
		"MYSQL_CONN_MAX_IDLE_TIME": cfg.ConnMaxIdleTime,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", key)
		}
	}
	return nil
}

// 输入：lookup，环境查询函数；key，变量名；fallback，缺省整数。
// 输出：非负整数；格式错误、溢出或负数返回错误。
// 功能：读取连接数量，允许空闲连接数为零以关闭空闲连接保留。
func parseNonNegativeInt(lookup LookupEnv, key string, fallback int) (int, error) {
	value, err := strconv.Atoi(readValue(lookup, key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a nonnegative integer", key)
	}
	return value, nil
}
