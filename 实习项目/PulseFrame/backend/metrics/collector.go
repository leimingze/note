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

// Collector 定期从 MySQL 和 Redis 连接池读取统计并发布到指标。
type Collector struct {
	provider PoolStatsProvider // 提供连接池统计的依赖对象。
	interval time.Duration     // 两次采集之间的间隔。
	registry *Registry         // 指标发布的注册表。
}

// NewCollector 创建连接池指标采集器。
// 输入：registry，指标注册表；provider，连接池统计来源；interval，采集周期。
// 输出：配置完成的采集器。
// 功能：绑定连接池统计来源，便于后续按周期采集并更新指标。
func NewCollector(registry *Registry, provider PoolStatsProvider, interval time.Duration) *Collector {
	return &Collector{registry: registry, provider: provider, interval: interval}
}

// Run 循环采集连接池指标直到上下文取消。
// 输入：ctx，控制采集周期的上下文。
// 输出：启动周期采集，上下文取消或退出流程开始时停止。
// 功能：使用 ticker 周期采集，避免每次请求都读取统计。
func (collector *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(collector.interval)
	defer ticker.Stop()
	// 启动后立即采集一次，避免指标端点长时间为空。
	collector.collect()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collector.collect()
		}
	}
}

// collect 读取一次连接池统计并更新对应指标。
// 输入：无。
// 输出：更新 MySQL 与 Redis 连接池指标，不写日志。
// 功能：把 sql.DB.Stats 与 Redis PoolStats 的数值同步到指标。
func (collector *Collector) collect() {
	mysql := collector.provider.MySQLStats()
	collector.registry.MySQLOpenConnections.Set(float64(mysql.OpenConnections))
	collector.registry.MySQLInUseConnections.Set(float64(mysql.InUse))
	collector.registry.MySQLIdleConnections.Set(float64(mysql.Idle))
	collector.registry.MySQLWaitCountTotal.Add(float64(mysql.WaitCount))
	collector.registry.MySQLWaitDurationSecondsTotal.Add(mysql.WaitDuration.Seconds())

	redisPool := collector.provider.RedisPoolStats()
	collector.registry.RedisPoolTotalConnections.Set(float64(redisPool.TotalConns))
	collector.registry.RedisPoolIdleConnections.Set(float64(redisPool.IdleConns))
	collector.registry.RedisPoolTimeoutsTotal.Add(float64(redisPool.Timeouts))
}
