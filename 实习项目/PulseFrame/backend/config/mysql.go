package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// 可选配置缺失时使用这些默认值；每个后端进程分别计算连接数量。
const (
	defaultMySQLAddress        = "127.0.0.1:3306" // 本地开发默认连接目标，格式为主机地址:端口。
	defaultMySQLMaxOpenConns   = 20               // 单进程默认连接总上限，包含使用中和空闲连接。
	defaultMySQLMaxIdleConns   = 5                // 单进程默认最多保留的空闲连接数，不代表预先创建数量。
	defaultMySQLConnectTimeout = 3 * time.Second  // 默认建立一次 TCP 连接最多等待3秒。
	defaultMySQLIOTimeout      = 5 * time.Second  // 默认单次网络读取或写入最多等待5秒，SQL 整体期限由调用方的上下文控制。
	defaultMySQLStartupTimeout = 5 * time.Second  // 启动 PingContext 的默认整体期限，包含连接建立和检查。
	defaultMySQLConnLifetime   = 30 * time.Minute // 连接默认最长存活30分钟，到期后按连接池规则回收。
	defaultMySQLConnIdleTime   = 5 * time.Minute  // 连接默认最多持续空闲5分钟，之后由连接池清理。
)

// MySQL 保存单个进程使用的数据库连接池配置。
type MySQL struct {
	Address         string        // 数据库连接目标，使用 host:port，IPv6 地址需带方括号。
	User            string        // 数据库认证账号，保留配置中的原始字符。
	Password        string        // 数据库认证密码，保留空白，不用于日志输出。
	Database        string        // 连接后使用的数据库名称，启动前须已存在。
	TLSMode         string        // "true" 启用 TLS 并校验证书；"false" 使用普通连接。
	MaxOpenConns    int           // 当前进程连接池的总容量上限，必须大于零。
	MaxIdleConns    int           // 最多保留的空闲连接数，零表示归还后不保留空闲连接。
	ConnectTimeout  time.Duration // TCP 建连期限，不包含连接池满载时的排队时间。
	ReadTimeout     time.Duration // 单次网络读取的等待期限。
	WriteTimeout    time.Duration // 单次网络写入的等待期限。
	StartupTimeout  time.Duration // 启动连通性检查的整体期限，父上下文可提前取消。
	ConnMaxLifetime time.Duration // 连接从创建起可复用的最长时间，不强行中断正在执行的操作。
	ConnMaxIdleTime time.Duration // 连接连续空闲的最长时间，清理由连接池调度。
}

// 输入：lookup，已确认非 nil 的环境变量查询函数。
// 输出：MySQL 配置；必填项缺失或参数非法时返回错误。
// 功能：读取连接信息和连接池参数，密码始终保留原值。
// 例如必填项有效，连接总上限配置为 "3"、空闲上限为 "1" 时，得到最多3个连接且保留至多1个空闲连接的配置。
func loadMySQL(lookup LookupEnv) (MySQL, error) {
	// 先读取密码，并确认密码变量有没有设置；没有设置时停止加载，显式空密码则允许继续。
	password, exists := lookup("MYSQL_PASSWORD")
	if !exists {
		return MySQL{}, fmt.Errorf("MYSQL_PASSWORD must be explicitly set")
	}
	// 再读取账号和数据库名称，保留原来的内容；是否为空会在最后统一检查。
	user, _ := lookup("MYSQL_USER")
	database, _ := lookup("MYSQL_DATABASE")
	// 继续读取连接地址和加密模式，这两项没有设置或内容为空时使用各自的默认值。
	cfg := MySQL{
		Address: readValue(lookup, "MYSQL_ADDR", defaultMySQLAddress),
		User:    user, Password: password, Database: database,
		TLSMode: readValue(lookup, "MYSQL_TLS_MODE", "false"),
	}
	var err error
	// 接下来读取连接总上限和空闲连接上限，将配置中的文字转换为非负整数。
	// 先确认每项能表示一个数量，最后再检查空闲连接上限有没有超过总上限。
	cfg.MaxOpenConns, err = parseNonNegativeInt(lookup, "MYSQL_MAX_OPEN_CONNS", defaultMySQLMaxOpenConns)
	if err != nil {
		return MySQL{}, err
	}
	cfg.MaxIdleConns, err = parseNonNegativeInt(lookup, "MYSQL_MAX_IDLE_CONNS", defaultMySQLMaxIdleConns)
	if err != nil {
		return MySQL{}, err
	}
	// 为六项时长分别列出环境变量名、默认值和保存位置，下面按这张表逐项读取。
	durations := []struct {
		key      string         // 本轮要读取的环境变量名称。
		fallback time.Duration  // 没有设置或内容为空时使用的默认时长。
		target   *time.Duration // 保存转换结果的配置字段地址。
	}{
		{"MYSQL_CONNECT_TIMEOUT", defaultMySQLConnectTimeout, &cfg.ConnectTimeout},
		{"MYSQL_READ_TIMEOUT", defaultMySQLIOTimeout, &cfg.ReadTimeout},
		{"MYSQL_WRITE_TIMEOUT", defaultMySQLIOTimeout, &cfg.WriteTimeout},
		{"MYSQL_STARTUP_TIMEOUT", defaultMySQLStartupTimeout, &cfg.StartupTimeout},
		{"MYSQL_CONN_MAX_LIFETIME", defaultMySQLConnLifetime, &cfg.ConnMaxLifetime},
		{"MYSQL_CONN_MAX_IDLE_TIME", defaultMySQLConnIdleTime, &cfg.ConnMaxIdleTime},
	}
	for _, item := range durations {
		// 每轮读取一项配置；没有设置或内容为空时使用它的默认值，再换算成时间长度。
		value, err := parsePositiveDuration(lookup, item.key, item.fallback)
		// 任一时长非法就终止加载，防止只配置了一部分期限仍继续启动。
		if err != nil {
			return MySQL{}, err
		}
		// 根据前面记录的字段地址，把得到的时间长度存入对应配置项，例如连接超时。
		*item.target = value
	}
	// 所有内容读取完后，统一检查连接信息和参数之间的关系，再交给上层决定能否继续启动。
	return cfg, cfg.Validate()
}

// 输入：cfg，环境读取或调用方直接构造的 MySQL 配置。
// 输出：合法时返回 nil，否则返回包含配置名称的错误。
// 功能：在建立网络连接前拒绝无效地址、无限连接数量和无效超时。
// 例如其他参数有效但连接总上限为0时返回错误，避免连接数量不受限制。
func (cfg MySQL) Validate() error {
	// 先确认账号、数据库名称、地址和加密模式可用，发现问题就停止本次检查。
	if err := cfg.validateConnection(); err != nil {
		return err
	}
	// 连接信息通过后，再检查连接数量和各项期限，完成整份配置的检查。
	return cfg.validatePool()
}

// 输入：cfg，MySQL 连接配置。
// 输出：地址、账号、数据库或 TLS 模式非法时返回错误。
// 功能：校验连接目标，防止驱动自动补全缺失的必要信息。
// 例如账号和数据库名称有效时，127.0.0.1:3306 可通过地址检查，缺少主机的 :3306 会报错。
func (cfg MySQL) validateConnection() error {
	// 逐项检查账号和数据库名称，完全没填或只填空白都算缺失；检查过程保留原来的内容。
	for key, value := range map[string]string{"MYSQL_USER": cfg.User, "MYSQL_DATABASE": cfg.Database} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	// 接着从连接地址中取出主机和端口，例如 "localhost:3306" 对应主机 localhost 和端口3306。
	// 连接数据库必须知道具体主机，所以地址格式不正确或主机没填时，都停止检查并报错。
	host, portText, err := net.SplitHostPort(cfg.Address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("MYSQL_ADDR must use a nonempty host:port")
	}
	// 端口读出来还是文字，要先转换成整数，再检查数值是否有效。
	port, err := strconv.Atoi(portText)
	if err != nil || port < minimumPort || port > maximumPort {
		return fmt.Errorf("MYSQL_ADDR port must be between %d and %d", minimumPort, maximumPort)
	}
	// 启用 TLS 时使用驱动的证书校验，不接受跳过验证等其他模式。
	if cfg.TLSMode != "false" && cfg.TLSMode != "true" {
		return fmt.Errorf("MYSQL_TLS_MODE must be false or true")
	}
	return nil
}

// 输入：cfg，连接池容量及期限配置。
// 输出：容量关系或时长非法时返回错误。
// 功能：确保连接数量有上限，等待网络与连接回收均具有明确期限。
// 例如总上限为20、空闲上限为5且各项时长大于零时通过；空闲上限为21时返回错误。
func (cfg MySQL) validatePool() error {
	// sql.DB 将非正上限视为不限制连接数量，此处要求显式限制资源使用。
	if cfg.MaxOpenConns <= 0 {
		return fmt.Errorf("MYSQL_MAX_OPEN_CONNS must be greater than zero")
	}
	// 零表示不保留空闲连接；超过总上限的配置直接报错，不依赖连接池自动调整。
	if cfg.MaxIdleConns < 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return fmt.Errorf("MYSQL_MAX_IDLE_CONNS must be between zero and MYSQL_MAX_OPEN_CONNS")
	}
	// 对直接构造的配置也检查所有期限，防止绕过环境变量解析时的校验。
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
