package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"pulseframe/config"
)

const (
	connectTimeout  = 5 * time.Second
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
)

// OpenMySQL 打开并确认账号数据库连接。
// 输入：ctx，启动上下文；cfg，已校验的 MySQL 配置。
// 输出：可复用的连接池；连接失败时关闭连接池并返回错误。
// 功能：集中配置数据库驱动、连接期限及启动检查。
func OpenMySQL(ctx context.Context, cfg config.MySQL) (*sql.DB, error) {
	driverConfig := mysql.NewConfig()
	driverConfig.Net = "tcp"
	driverConfig.Addr = cfg.Address
	driverConfig.User = cfg.User
	driverConfig.Passwd = cfg.Password
	driverConfig.DBName = cfg.Database
	if cfg.TLS {
		driverConfig.TLSConfig = "true"
	}
	driverConfig.Timeout = connectTimeout
	driverConfig.ReadTimeout = connectTimeout
	driverConfig.WriteTimeout = connectTimeout
	db, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	checkCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := db.PingContext(checkCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}
