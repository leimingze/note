package storage

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	mysqlMaxOpenConnections = 25
	mysqlMaxIdleConnections = 5
	mysqlConnectionLifetime = 3 * time.Minute
	dependencyPingTimeout   = 5 * time.Second
)

// OpenMySQL 创建经过连接池配置和 Ping 验证的 GORM 句柄。
// 输入：ctx，服务启动上下文；dsn，含账号、密码和库名的 MySQL DSN。
// 输出：可用 GORM 句柄；初始化、Ping 或池配置失败时返回错误。
// 功能：在服务接流量前暴露数据库连接问题，并避免 SQL 日志泄露认证数据。
func OpenMySQL(ctx context.Context, dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("MySQL DSN is required")
	}
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: dsn, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent), DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get MySQL pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(mysqlMaxOpenConnections)
	sqlDB.SetMaxIdleConns(mysqlMaxIdleConnections)
	sqlDB.SetConnMaxLifetime(mysqlConnectionLifetime)
	pingContext, cancel := context.WithTimeout(ctx, dependencyPingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingContext); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	return db, nil
}

// CloseMySQL 关闭 GORM 句柄对应的底层连接池。
// 输入：db，已创建的 GORM 句柄。
// 输出：连接池关闭结果；句柄为空或底层池不可用时返回错误。
// 功能：在 HTTP 服务退出时释放 MySQL 连接。
func CloseMySQL(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("MySQL database is required")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get MySQL pool: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close MySQL pool: %w", err)
	}
	return nil
}
