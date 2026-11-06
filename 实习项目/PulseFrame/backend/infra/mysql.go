package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"pulseframe/config"
)

// 输入：ctx，启动上下文；cfg，MySQL 连接信息及连接池配置。
// 输出：访问数据库并返回已检查连通性的连接池；失败时关闭已创建的连接池并返回错误。
// 功能：创建可供进程内并发复用的 sql.DB，成功后由调用方负责关闭。
// 例如提供有效数据库配置且总上限为20时，返回最多同时持有20个连接的连接池，不会预先建立20个连接。
func NewMySQL(ctx context.Context, cfg config.MySQL) (*sql.DB, error) {
	// 调用方也可能直接构造配置，因此创建连接前仍检查完整约束。
	if err := cfg.Validate(); err != nil { // 传入 MySQL 配置；校验非法时返回错误
		return nil, fmt.Errorf("validate mysql config: %w", err)
	}
	// 保留驱动的默认行为，仅覆盖项目明确管理的连接选项。
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
	// 使用驱动配置 API，用户名、密码及数据库名称无需拼接进 DSN。
	connector, err := mysql.NewConnector(driverConfig) // 传入驱动配置；返回连接器、错误
	if err != nil {
		return nil, fmt.Errorf("create mysql connector: %w", err)
	}
	// 先创建管理连接的对象，再设置数量和回收期限；后面的连通性检查才会建立数据库连接。
	db := sql.OpenDB(connector)                // 传入连接器；返回 sql.DB 连接池对象
	db.SetMaxOpenConns(cfg.MaxOpenConns)       // 传入连接总上限
	db.SetMaxIdleConns(cfg.MaxIdleConns)       // 传入空闲连接上限
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime) // 传入连接最长复用时间
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime) // 传入空闲清理时间
	// 启动检查同时受自身期限和父上下文取消控制；返回时释放计时资源。
	checkContext, cancel := context.WithTimeout(ctx, cfg.StartupTimeout) // 传入父上下文与期限；返回检查上下文、取消函数
	// 函数返回时取消计时器，即使 Ping 已经结束，也释放上下文相关资源。
	defer cancel()
	// 检查失败时不会将连接池交给调用方，因此这里关闭并保留检查及关闭错误。
	if err := db.PingContext(checkContext); err != nil { // 传入带期限上下文；连通失败时返回错误
		return nil, errors.Join(fmt.Errorf("ping mysql: %w", err), db.Close()) // 合并 Ping 与关闭错误后返回
	}
	return db, nil
}
