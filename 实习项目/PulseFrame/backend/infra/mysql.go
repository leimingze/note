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
// 输出：已检查连通性的连接池；失败时关闭已创建的连接池并返回错误。
// 功能：创建可供进程内并发复用的 sql.DB，成功后由调用方负责关闭。
func NewMySQL(ctx context.Context, cfg config.MySQL) (*sql.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate mysql config: %w", err)
	}
	driverConfig := mysql.NewConfig()
	driverConfig.Net = "tcp"
	driverConfig.Addr = cfg.Address
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.DBName = cfg.Database
	driverConfig.TLSConfig = cfg.TLSMode
	driverConfig.ParseTime = true
	driverConfig.Timeout = cfg.ConnectTimeout
	driverConfig.ReadTimeout = cfg.ReadTimeout
	driverConfig.WriteTimeout = cfg.WriteTimeout
	// 使用驱动配置 API，用户名、密码及数据库名称无需拼接进 DSN。
	connector, err := mysql.NewConnector(driverConfig)
	if err != nil {
		return nil, fmt.Errorf("create mysql connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	checkContext, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	if err := db.PingContext(checkContext); err != nil {
		return nil, errors.Join(fmt.Errorf("ping mysql: %w", err), db.Close())
	}
	return db, nil
}
