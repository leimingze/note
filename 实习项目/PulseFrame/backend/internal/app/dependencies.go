package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"pulseframe/internal/auth"
	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/infra"
	"pulseframe/internal/platform/metrics"
)

// Dependencies 持有当前 API 进程创建的外部依赖，使用方借用连接池。
type Dependencies struct {
	MySQL     *gorm.DB                   // 当前 API 进程共享的 gorm 数据库实例，由 Dependencies 统一关闭。
	Redis     *redis.Client              // 当前 API 进程共享的 Redis 客户端，由 Dependencies 统一关闭。
	Metrics   *metrics.Registry          // 指标注册表及观察器所需的统计来源。
	observer  *metrics.PoolStatsObserver // 连接池统计观察器，投入时持有，退出时随上下文停止。
	closeOnce sync.Once                  // 确保重复或并发调用 Close 时只释放一次资源。
	closeErr  error                      // 保存第一次关闭的结果，供后续 Close 调用返回相同结果。
}

// MySQLStats 返回 MySQL 连接池当前统计快照。
// 输入：无。
// 输出：sql.DBStats 统计。
// 功能：向指标采集器提供 MySQL 连接池数据。
func (dependencies *Dependencies) MySQLStats() sql.DBStats {
	sqlDB, err := dependencies.MySQL.DB() // 从 gorm 实例取出底层连接池
	if err != nil {
		return sql.DBStats{} // 底层连接池不可用时返回空统计，不中断采集
	}
	return sqlDB.Stats() // 无参数；返回 MySQL 连接池统计快照
}

// RedisPoolStats 返回 Redis 客户端连接池当前统计快照。
// 输入：无。
// 输出：redis.PoolStats 统计。
// 功能：向指标采集器提供 Redis 连接池数据。
func (dependencies *Dependencies) RedisPoolStats() *redis.PoolStats {
	return dependencies.Redis.PoolStats() // 无参数；返回 Redis 客户端连接池统计快照
}

// Ping 探测 MySQL 与 Redis 连通性，任一不可用时返回错误。
// 输入：ctx，控制探测期限的上下文。
// 输出：全部依赖可用时返回 nil；存在不可用依赖时返回带名称的错误。
// 功能：供存活探针校验共享连接池对应的外部组件状态。
// 例如 MySQL 停止响应时，返回包含 mysql unreachable 的错误。
func (dependencies *Dependencies) Ping(ctx context.Context) error {
	sqlDB, err := dependencies.MySQL.DB() // 从 gorm 实例取出底层连接池
	if err != nil {
		return fmt.Errorf("mysql pool unavailable: %w", err)
	}
	return errors.Join(
		pingDependency("mysql", sqlDB.PingContext(ctx)),             // 探测 MySQL 并将失败标记依赖名
		pingDependency("redis", dependencies.Redis.Ping(ctx).Err()),  // 探测 Redis 并将失败标记依赖名
	)
}

// pingDependency 把探测失败转换为带名称的错误。
// 输入：name，依赖名称；err，探测返回的错误。
// 输出：err 为 nil 时返回 nil；否则返回带依赖名称的错误。
// 功能：统一标记不可用依赖，便于存活响应定位故障组件。
// 例如探测 Redis 返回网络错误时，结果会带有 redis unreachable 前缀。
func pingDependency(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s unreachable: %w", name, err)
}

// 输入：ctx，启动上下文；cfg，应用配置。
// 输出：连接 MySQL、Redis 并组装指标与采集器，返回持有共享依赖的对象；初始化失败时返回错误。
// 功能：集中创建进程共享资源，Redis 初始化失败时清理已创建的 MySQL 连接池。
// 例如程序启动时调用一次，后续数据库、缓存和指标访问都使用返回对象中的共享客户端。
func NewDependencies(ctx context.Context, cfg config.Config) (*Dependencies, error) {
	// 仅在程序启动时创建共享连接池；创建函数负责清理初始化失败的资源。
	db, err := infra.NewMySQL(ctx, cfg.MySQL) // 传入上下文与 MySQL 配置；返回 gorm 实例、错误
	if err != nil {
		return nil, fmt.Errorf("initialize mysql: %w", err)
	}
	// MySQL 连接创建成功后同步表结构；迁移失败时关闭连接池并返回错误。
	if err := auth.Migrate(db); err != nil { // 传入 gorm 实例；同步登录注册表结构，失败时返回错误
		return nil, errors.Join(fmt.Errorf("migrate mysql: %w", err), closeDB(db)) // 合并迁移错误与连接池关闭错误
	}
	// MySQL 成功后继续创建 Redis；Redis 失败时先关闭已创建的 MySQL 连接池。
	client, err := infra.NewRedis(ctx, cfg.Redis) // 传入上下文与 Redis 配置；返回客户端、错误
	if err != nil {
		return nil, errors.Join(fmt.Errorf("initialize redis: %w", err), closeDB(db)) // 合并 Redis 错误与 MySQL 关闭错误
	}
	// 组装依赖对象；指标注册表独立创建，观察器仅在启用指标时启动。
	dependencies := &Dependencies{MySQL: db, Redis: client, Metrics: metrics.New()} // 传入连接池与客户端，创建指标注册表
	if cfg.Metrics.Enabled {
		// 指标启用时把 SQL 观测插件挂到 gorm，激活依赖操作指标。
		metrics.AttachDBCallbacks(db, dependencies.Metrics) // 传入 gorm 实例与注册表；注册数据库操作回调
		// 观察器读取共享连接池统计并周期发布指标，随 ctx 取消而停止。
		dependencies.observer = metrics.NewPoolStatsObserver(dependencies.Metrics, dependencies, cfg.Metrics.CollectInterval) // 传入注册表、统计来源、采集周期；返回观察器
		go dependencies.observer.Run(ctx)                                                                                     // 传入上下文，在后台协程中周期采集
	}
	return dependencies, nil
}

// closeDB 关闭 gorm 实例持有的底连接连接池。
// 输入：db，需要关闭的 gorm 数据库实例。
// 输出：关闭失败时返回错误。
// 功能：统一释放 gorm 底层的 sql.DB 连接池。
// 例如服务退出时调用，关闭该实例持有的全部数据库连接。
func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB() // 取出底层连接池
	if err != nil {
		return err
	}
	return sqlDB.Close() // 关闭连接池
}

// 输入：dependencies，已成功创建的依赖对象。
// 输出：关闭 MySQL 连接池和 Redis 客户端；任一关闭失败时返回错误，重复调用返回第一次结果。
// 功能：在 HTTP 请求处理完成后统一释放进程拥有的外部连接资源，并让退出流程可以安全重复清理。
// 例如服务退出时调用 Close，之后通过这些客户端执行新的访问会返回错误；再次调用不会重复关闭客户端。
func (dependencies *Dependencies) Close() error {
	dependencies.closeOnce.Do(func() { // 保证只执行一次关闭逻辑，并发调用安全
		// 关闭整个共享连接池会影响所有使用方，调用入口统一安排在服务退出阶段。
		dependencies.closeErr = errors.Join(
			closeDependency("mysql", closeDB(dependencies.MySQL)), // 关闭 MySQL 并标依赖名
			closeDependency("redis", dependencies.Redis.Close()), // 关闭 Redis 并标记依赖名
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
