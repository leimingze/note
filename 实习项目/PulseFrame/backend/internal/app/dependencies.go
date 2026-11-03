package app

import (
	"context"
	"database/sql"
	"fmt"

	"pulseframe/config"
	"pulseframe/infra"
)

// Dependencies 持有当前 API 进程创建的外部依赖，使用方借用连接池。
type Dependencies struct {
	MySQL *sql.DB // 当前 API 进程共享的连接池，由 Dependencies 统一关闭。
}

// 输入：ctx，启动上下文；cfg，应用配置。
// 输出：连接数据库并返回拥有 MySQL 连接池的依赖对象；初始化失败时返回错误。
// 功能：集中创建进程共享资源，失败资源由创建函数清理。
// 例如程序启动时调用一次，后续数据库访问都可使用返回对象中的同一个 MySQL 连接池。
func NewDependencies(ctx context.Context, cfg config.Config) (*Dependencies, error) {
	// 仅在程序启动时创建共享连接池；创建函数负责清理初始化失败的资源。
	db, err := infra.NewMySQL(ctx, cfg.MySQL)
	if err != nil {
		return nil, fmt.Errorf("initialize mysql: %w", err)
	}
	// 成功后由 Dependencies 持有关闭责任，DAO 等使用方只借用该对象。
	return &Dependencies{MySQL: db}, nil
}

// 输入：dependencies，已成功创建的依赖对象。
// 输出：关闭其 MySQL 连接池；关闭失败时返回错误。
// 功能：在 HTTP 请求处理完成后统一释放进程拥有的连接资源。
// 例如服务退出时调用 Close，之后通过这个连接池执行新的查询会返回错误。
func (dependencies *Dependencies) Close() error {
	// 关闭整个共享连接池会影响所有使用方，调用入口统一安排在服务退出阶段。
	if err := dependencies.MySQL.Close(); err != nil {
		return fmt.Errorf("close mysql: %w", err)
	}
	return nil
}
