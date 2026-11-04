package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"pulseframe/config"
	"pulseframe/infra"
)

// Dependencies 持有当前 API 进程创建的外部依赖，使用方借用连接池。
type Dependencies struct {
	MySQL     *sql.DB       // 当前 API 进程共享的连接池，由 Dependencies 统一关闭。
	Redis     *redis.Client // 当前 API 进程共享的 Redis 客户端，由 Dependencies 统一关闭。
	closeOnce sync.Once     // 确保重复或并发调用 Close 时只释放一次资源。
	closeErr  error         // 保存第一次关闭的结果，供后续 Close 调用返回相同结果。
}

// 输入：ctx，启动上下文；cfg，应用配置。
// 输出：连接 MySQL 和 Redis，并返回拥有共享客户端的依赖对象；初始化失败时返回错误。
// 功能：集中创建进程共享资源，Redis 初始化失败时清理已创建的 MySQL 连接池。
// 例如程序启动时调用一次，后续数据库和缓存访问都使用返回对象中的共享客户端。
func NewDependencies(ctx context.Context, cfg config.Config) (*Dependencies, error) {
	// 仅在程序启动时创建共享连接池；创建函数负责清理初始化失败的资源。
	db, err := infra.NewMySQL(ctx, cfg.MySQL)
	if err != nil {
		return nil, fmt.Errorf("initialize mysql: %w", err)
	}
	// MySQL 成功后继续创建 Redis；Redis 失败时先关闭已创建的 MySQL 连接池。
	client, err := infra.NewRedis(ctx, cfg.Redis)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("initialize redis: %w", err), db.Close())
	}
	// 成功后由 Dependencies 持有关闭责任，业务模块只借用这些共享对象。
	return &Dependencies{MySQL: db, Redis: client}, nil
}

// 输入：dependencies，已成功创建的依赖对象。
// 输出：关闭 MySQL 连接池和 Redis 客户端；任一关闭失败时返回错误，重复调用返回第一次结果。
// 功能：在 HTTP 请求处理完成后统一释放进程拥有的外部连接资源，并让退出流程可以安全重复清理。
// 例如服务退出时调用 Close，之后通过这些客户端执行新的访问会返回错误；再次调用不会重复关闭客户端。
func (dependencies *Dependencies) Close() error {
	dependencies.closeOnce.Do(func() {
		// 关闭整个共享连接池会影响所有使用方，调用入口统一安排在服务退出阶段。
		dependencies.closeErr = errors.Join(
			closeDependency("mysql", dependencies.MySQL.Close()),
			closeDependency("redis", dependencies.Redis.Close()),
		)
	})
	return dependencies.closeErr
}

// 输入：name，依赖名称；err，关闭依赖时返回的错误。
// 输出：带有依赖名称的错误；err 为 nil 时返回 nil。
// 功能：统一标记关闭失败的依赖，便于退出入口定位资源问题。
// 例如关闭 Redis 返回网络错误时，结果会带有 "close redis" 前缀。
func closeDependency(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close %s: %w", name, err)
}
