package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// 可选配置缺失时使用这些默认值；每个后端进程分别管理自己的 Redis 客户端连接池。
const (
	defaultRedisAddress        = "127.0.0.1:6379" // 本地开发默认连接目标，格式为主机地址:端口。
	defaultRedisDatabase       = 0                // 默认使用 Redis 的逻辑数据库0。
	defaultRedisMaxRetries     = 3                // 单条命令最多自动重试3次；具体写操作仍需由业务判断是否允许重试。
	defaultRedisPoolSize       = 10               // 连接池基础连接数，连接不足时客户端按需创建。
	defaultRedisMinIdleConns   = 2                // 尽量维持的空闲连接数，不代表连接池启动时立即建立。
	defaultRedisMaxActiveConns = 20               // 连接池同时分配的最大连接数，超过后等待或超时。
	defaultRedisDialTimeout    = 3 * time.Second  // 默认建立一次 TCP 连接最多等待3秒。
	defaultRedisIOTimeout      = 5 * time.Second  // 默认单次网络读取或写入最多等待5秒。
	defaultRedisPoolTimeout    = 5 * time.Second  // 连接池耗尽时等待可用连接的最长时间。
	defaultRedisStartupTimeout = 5 * time.Second  // 启动 PING 检查的整体期限。
	defaultRedisConnLifetime   = 30 * time.Minute // 连接默认最长存活30分钟。
	defaultRedisConnIdleTime   = 5 * time.Minute  // 连接默认最多空闲5分钟。
)

// Redis 保存单个进程使用的 Redis 客户端连接池配置。
type Redis struct {
	Address         string        // Redis 连接目标，使用 host:port，IPv6 地址需带方括号。
	Username        string        // Redis ACL 用户名，可为空表示使用服务端默认用户。
	Password        string        // Redis 认证密码，保留原值，不用于日志输出。
	Database        int           // Redis 逻辑数据库编号，不能小于零。
	MaxRetries      int           // 单条命令的自动重试次数，零表示不自动重试。
	PoolSize        int           // 连接池基础连接数。
	MinIdleConns    int           // 连接池希望维持的最小空闲连接数。
	MaxActiveConns  int           // 连接池同时分配的最大连接数。
	DialTimeout     time.Duration // TCP 建连期限。
	ReadTimeout     time.Duration // 单次网络读取期限。
	WriteTimeout    time.Duration // 单次网络写入期限。
	PoolTimeout     time.Duration // 连接池满载时等待连接的期限。
	StartupTimeout  time.Duration // 启动 PING 检查的整体期限，父上下文可提前取消。
	ConnMaxLifetime time.Duration // 连接从创建起可复用的最长时间。
	ConnMaxIdleTime time.Duration // 连接连续空闲的最长时间。
}

// 输入：lookup，已确认非 nil 的环境变量查询函数。
// 输出：Redis 配置；参数非法时返回错误。
// 功能：读取 Redis 地址、认证信息、连接池容量和期限。
// 例如 REDIS_POOL_SIZE 为 "5"、REDIS_MAX_ACTIVE_CONNS 为 "10" 时，得到基础连接数5且最多同时使用10个连接的配置。
func loadRedis(lookup LookupEnv) (Redis, error) {
	cfg := Redis{
		Address:  readValue(lookup, "REDIS_ADDR", defaultRedisAddress),
		Username: readRawValue(lookup, "REDIS_USERNAME"),
		Password: readRawValue(lookup, "REDIS_PASSWORD"),
	}
	var err error
	// 逻辑数据库编号和重试次数先从文字转换为整数，再由 Validate 检查取值关系。
	cfg.Database, err = parseNonNegativeInt(lookup, "REDIS_DATABASE", defaultRedisDatabase)
	if err != nil {
		return Redis{}, err
	}
	cfg.MaxRetries, err = parseNonNegativeInt(lookup, "REDIS_MAX_RETRIES", defaultRedisMaxRetries)
	if err != nil {
		return Redis{}, err
	}
	cfg.PoolSize, err = parseNonNegativeInt(lookup, "REDIS_POOL_SIZE", defaultRedisPoolSize)
	if err != nil {
		return Redis{}, err
	}
	cfg.MinIdleConns, err = parseNonNegativeInt(lookup, "REDIS_MIN_IDLE_CONNS", defaultRedisMinIdleConns)
	if err != nil {
		return Redis{}, err
	}
	cfg.MaxActiveConns, err = parseNonNegativeInt(lookup, "REDIS_MAX_ACTIVE_CONNS", defaultRedisMaxActiveConns)
	if err != nil {
		return Redis{}, err
	}
	durations := []struct {
		key      string
		fallback time.Duration
		target   *time.Duration
	}{
		{"REDIS_DIAL_TIMEOUT", defaultRedisDialTimeout, &cfg.DialTimeout},
		{"REDIS_READ_TIMEOUT", defaultRedisIOTimeout, &cfg.ReadTimeout},
		{"REDIS_WRITE_TIMEOUT", defaultRedisIOTimeout, &cfg.WriteTimeout},
		{"REDIS_POOL_TIMEOUT", defaultRedisPoolTimeout, &cfg.PoolTimeout},
		{"REDIS_STARTUP_TIMEOUT", defaultRedisStartupTimeout, &cfg.StartupTimeout},
		{"REDIS_CONN_MAX_LIFETIME", defaultRedisConnLifetime, &cfg.ConnMaxLifetime},
		{"REDIS_CONN_MAX_IDLE_TIME", defaultRedisConnIdleTime, &cfg.ConnMaxIdleTime},
	}
	// 按表中记录的变量名和配置字段逐项读取，任一时长非法都会终止整份 Redis 配置加载。
	for _, item := range durations {
		// 当前轮读取一项时长；缺失或为空使用该项默认值，再写入对应配置字段。
		value, err := parsePositiveDuration(lookup, item.key, item.fallback)
		if err != nil {
			return Redis{}, err
		}
		// target 保存的是配置字段地址，例如 &cfg.PoolTimeout；解引用后写入解析结果。
		*item.target = value
	}
	return cfg, cfg.Validate()
}

// 输入：cfg，环境读取或调用方直接构造的 Redis 配置。
// 输出：合法时返回 nil，否则返回包含配置名称的错误。
// 功能：在创建客户端前检查地址、数据库编号、重试次数、连接数量及各项期限。
// 例如基础连接数为10、空闲连接数为2、最大连接数为20时通过；最大连接数为1时返回错误。
func (cfg Redis) Validate() error {
	if err := cfg.validateConnection(); err != nil {
		return err
	}
	return cfg.validatePool()
}

// 输入：cfg，Redis 连接配置。
// 输出：地址非法时返回错误。
// 功能：检查 Redis 地址中的主机和端口，确保客户端有明确的连接目标。
// 例如 127.0.0.1:6379 可以通过检查，:6379 会因为主机为空返回错误。
func (cfg Redis) validateConnection() error {
	host, portText, err := net.SplitHostPort(cfg.Address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("REDIS_ADDR must use a nonempty host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("REDIS_ADDR port must be between %d and %d", minimumPort, maximumPort)
	}
	return nil
}

// 输入：cfg，Redis 逻辑数据库、重试、连接池容量和期限配置。
// 输出：参数关系或时长非法时返回错误。
// 功能：确保连接数量有上限、重试次数明确、连接池等待和连接回收均具有期限。
// 例如基础连接数为10、最大连接数为20且所有期限大于零时通过；最大连接数为0时返回错误。
func (cfg Redis) validatePool() error {
	if cfg.Database < 0 {
		return fmt.Errorf("REDIS_DATABASE must be nonnegative")
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("REDIS_MAX_RETRIES must be nonnegative")
	}
	if cfg.MaxActiveConns <= 0 {
		return fmt.Errorf("REDIS_MAX_ACTIVE_CONNS must be greater than zero")
	}
	if cfg.PoolSize <= 0 || cfg.PoolSize > cfg.MaxActiveConns {
		return fmt.Errorf("REDIS_POOL_SIZE must be between one and REDIS_MAX_ACTIVE_CONNS")
	}
	if cfg.MinIdleConns < 0 || cfg.MinIdleConns > cfg.MaxActiveConns {
		return fmt.Errorf("REDIS_MIN_IDLE_CONNS must be between zero and REDIS_MAX_ACTIVE_CONNS")
	}
	for key, value := range map[string]time.Duration{
		"REDIS_DIAL_TIMEOUT":       cfg.DialTimeout,
		"REDIS_READ_TIMEOUT":       cfg.ReadTimeout,
		"REDIS_WRITE_TIMEOUT":      cfg.WriteTimeout,
		"REDIS_POOL_TIMEOUT":       cfg.PoolTimeout,
		"REDIS_STARTUP_TIMEOUT":    cfg.StartupTimeout,
		"REDIS_CONN_MAX_LIFETIME":  cfg.ConnMaxLifetime,
		"REDIS_CONN_MAX_IDLE_TIME": cfg.ConnMaxIdleTime,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", key)
		}
	}
	return nil
}

// 输入：lookup，环境查询函数；key，变量名。
// 输出：环境变量原始值；变量缺失时返回空字符串。
// 功能：读取需要保留原始内容的可选配置，例如密码不能经过 TrimSpace 改写。
// 例如密码为 " pass " 时仍返回前后的空格，变量缺失时返回空字符串。
func readRawValue(lookup LookupEnv, key string) string {
	value, _ := lookup(key)
	return value
}
