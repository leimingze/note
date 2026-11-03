package main

import (
	"context"
	"database/sql"
	"fmt"

	"pulseframe/config"
	"pulseframe/infra"
)

// Dependencies 持有当前 API 进程创建的外部依赖，使用方借用连接池。
type Dependencies struct {
	MySQL *sql.DB
}

// 输入：ctx，启动上下文；cfg，应用配置。
// 输出：拥有 MySQL 连接池的依赖对象；初始化失败时返回错误。
// 功能：集中创建进程共享资源，失败资源由创建函数清理。
func NewDependencies(ctx context.Context, cfg config.Config) (*Dependencies, error) {
	db, err := infra.NewMySQL(ctx, cfg.MySQL)
	if err != nil {
		return nil, fmt.Errorf("initialize mysql: %w", err)
	}
	return &Dependencies{MySQL: db}, nil
}

// 输入：dependencies，已成功创建的依赖对象。
// 输出：关闭其 MySQL 连接池；关闭失败时返回错误。
// 功能：在 HTTP 请求处理完成后统一释放进程拥有的连接资源。
func (dependencies *Dependencies) Close() error {
	if err := dependencies.MySQL.Close(); err != nil {
		return fmt.Errorf("close mysql: %w", err)
	}
	return nil
}
