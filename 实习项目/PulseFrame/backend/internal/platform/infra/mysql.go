package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"pulseframe/internal/platform/config"
)

// 输入：ctx，启动上下文；cfg，MySQL 连接信息及连接池配置。
// 输出：访问数据库并返回已检查连通性的 gorm 实例；失败时关闭已创建的连接池并返回错误。
// 功能：创建可供进程内并发复用的 gorm 数据库实例，成功后由调用方负责关闭。
// 例如提供有效数据库配置且总上限为20时，返回最多同时持有20个连接的实例，不会预先建立20个连接。
func NewMySQL(ctx context.Context, cfg config.MySQL) (*gorm.DB, error) {
	// 调用方也可能直接构造配置，因此创建连接前仍检查完整约束。
	if err := cfg.Validate(); err != nil { // 传入 MySQL 配置；校验非法时返回错误
		return nil, fmt.Errorf("validate mysql config: %w", err)
	}
	// 使用 go-sql-driver 生成 DSN，用户、密码及数据库名不会拼接进字符串。
	driverConfig := mysql.NewConfig()    // 无参数；返回带默认空值的驱动配置
	driverConfig.Net = "tcp"             // 使用 TCP 网络协议连接 MySQL
	driverConfig.Addr = cfg.Address      // 传入主机地址，如 127.0.0.1:3306
	driverConfig.User = cfg.User         // 传入数据库用户名
	driverConfig.Passwd = cfg.Password   // 传入数据库密码
	driverConfig.DBName = cfg.Database   // 传入要连接的数据库名
	driverConfig.TLSConfig = cfg.TLSMode // 传入 TLS 模式
	// 日期时间字段按 time.Time 读取，便于后续模型字段映射。
	driverConfig.ParseTime = true                // 开启时间解析，返回 time.Time 而非字符串
	driverConfig.Timeout = cfg.ConnectTimeout    // 传入建连超时
	driverConfig.ReadTimeout = cfg.ReadTimeout   // 传入读取超时
	driverConfig.WriteTimeout = cfg.WriteTimeout // 传入写入超时
	// gorm 使用数据库驱动生成的 DSN 打开连接，读取每条 SQL 时静默执行。
	db, err := gorm.Open(gmysql.Open(driverConfig.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}) // 传入 DSN 与静默日志配置；返回 gorm 实例、错误
	if err != nil {
		return nil, fmt.Errorf("create mysql gorm session: %w", err)
	}
	// 从 gorm 取出底层连接池，设置数量和回收期限；连通性检查才会建立真实连接。
	sqlDB, err := db.DB() // 从 gorm 实例取出底层 sql.DB 连接池
	if err != nil {
		return nil, fmt.Errorf("get mysql sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)       // 传入连接总上限
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)       // 传入空闲连接上限
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime) // 传入连接最长复用时间
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime) // 传入空闲清理时间
	// 启动检查同时受自身期限和父上下文取消控制；返回时释放计时资源。
	checkContext, cancel := context.WithTimeout(ctx, cfg.StartupTimeout) // 传入父上下文与期限；返回检查上下文、取消函数
	// 函数返回时取消计时器，即使 Ping 已经结束，也释放上下文相关资源。
	defer cancel()
	// 检查失败时不会将连接池交给调用方，因此这里关闭并保留检查及关闭错误。
	if err := sqlDB.PingContext(checkContext); err != nil { // 传入带期限上下文；连通失败时返回错误
		return nil, errors.Join(fmt.Errorf("ping mysql: %w", err), sqlDB.Close()) // 合并 Ping 与关闭错误后返回
	}
	return db, nil
}
