package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"
)

// PoolStatsProvider 抽象外部依赖连接池统计的来源。
type PoolStatsProvider interface {
	// MySQLStats 返回当前 MySQL 连接池快照。
	MySQLStats() sql.DBStats
	// RedisPoolStats 返回当前 Redis 客户端连接池快照。
	RedisPoolStats() *redis.PoolStats
}

// PoolStatsObserver 定期从 MySQL 和 Redis 连接池读取统计并发布到指标。
type PoolStatsObserver struct {
	provider PoolStatsProvider // 提供连接池统计的依赖对象。
	interval time.Duration     // 两次采集之间的间隔。
	registry *Registry         // 指标发布的注册表。
}

// NewPoolStatsObserver 创建连接池指标观察器。
// 输入：registry，指标注册表；provider，连接池统计来源；interval，采集周期。
// 输出：配置完成的观察器。
// 功能：绑定连接池统计来源，便于后续按周期采集并更新指标。
func NewPoolStatsObserver(registry *Registry, provider PoolStatsProvider, interval time.Duration) *PoolStatsObserver {
	return &PoolStatsObserver{registry: registry, provider: provider, interval: interval} // 组装注册表、来源与周期
}

// Run 循环采集连接池指标直到上下文取消。
// 输入：ctx，控制采集周期的上下文。
// 输出：启动周期采集，上下文取消或退出流程开始时停止。
// 功能：使用 ticker 周期采集，避免每次请求都读取统计。
func (observer *PoolStatsObserver) Run(ctx context.Context) {
	ticker := time.NewTicker(observer.interval) // 传入采集周期；返回定时器
	defer ticker.Stop()                         // 函数返回时停止定时器
	// 启动后立即采集一次，避免指标端点长时间为空。
	observer.sample() // 立即执行一次采集
	for {
		select {
		case <-ctx.Done(): // 上下文取消时停止采集
			return
		case <-ticker.C: // 每次定时器触发时采集
			observer.sample()
		}
	}
}

// sample 读取一次连接池统计并更新对应指标。
// 输入：无。
// 输出：更新 MySQL 与 Redis 连接池指标，不写日志。
// 功能：把 sql.DB.Stats 与 Redis PoolStats 的数值同步到指标。
func (observer *PoolStatsObserver) sample() {
	mysql := observer.provider.MySQLStats()                                           // 读取 MySQL 连接池统计
	observer.registry.MySQLOpenConnections.Set(float64(mysql.OpenConnections))        // 写入打开连接数
	observer.registry.MySQLInUseConnections.Set(float64(mysql.InUse))                 // 写入使用中连接数
	observer.registry.MySQLIdleConnections.Set(float64(mysql.Idle))                   // 写入空闲连接数
	observer.registry.MySQLWaitCountTotal.Add(float64(mysql.WaitCount))               // 累加等待次数
	observer.registry.MySQLWaitDurationSecondsTotal.Add(mysql.WaitDuration.Seconds()) // 累加等待秒数

	redisPool := observer.provider.RedisPoolStats()                                // 读取 Redis 连接池统计
	observer.registry.RedisPoolTotalConnections.Set(float64(redisPool.TotalConns)) // 写入总连接数
	observer.registry.RedisPoolIdleConnections.Set(float64(redisPool.IdleConns))   // 写入空闲连接数
	observer.registry.RedisPoolTimeoutsTotal.Add(float64(redisPool.Timeouts))      // 累加超时次数
}
