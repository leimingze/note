package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"pulseframe/config"
)

// 输入：ctx，启动上下文；cfg，Redis 连接信息及连接池配置。
// 输出：访问 Redis 并返回已检查连通性的客户端；失败时关闭已创建的客户端并返回错误。
// 功能：根据配置创建可供进程内并发复用的 Redis 客户端，并在启动阶段执行 PING 检查。
// 例如提供有效地址和数据库编号0时，返回一个可执行 Get、Set 等命令的共享客户端。
func NewRedis(ctx context.Context, cfg config.Redis) (*redis.Client, error) {
	// 调用方也可能直接构造配置，因此创建客户端前仍检查完整约束。
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate redis config: %w", err)
	}
	// go-redis 用 -1 表示关闭自动重试，而项目配置用0表达同一意图，先完成一次取值转换。
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = -1
	}
	// 将项目配置逐项映射到 go-redis，显式设置连接池、超时和重试行为。
	client := redis.NewClient(&redis.Options{
		Network:         "tcp",
		Addr:            cfg.Address,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DB:              cfg.Database,
		MaxRetries:      maxRetries,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		MaxActiveConns:  cfg.MaxActiveConns,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		PoolTimeout:     cfg.PoolTimeout,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	})
	// 启动检查同时受自身期限和父上下文取消控制；返回时释放计时资源。
	checkContext, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	// 检查失败时不会将客户端交给调用方，因此这里关闭已创建的客户端。
	if err := client.Ping(checkContext).Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("ping redis: %w", err), client.Close())
	}
	return client, nil
}
