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
	if err := cfg.Validate(); err != nil { // 传入 Redis 配置；校验非法时返回错误
		return nil, fmt.Errorf("validate redis config: %w", err)
	}
	// Redis 配置直接透传给 go-redis，重试次数遵循库约定：-1 关闭重试，0 使用默认（3次），正数为指定次数。
	client := redis.NewClient(&redis.Options{ // 传入连接选项；返回共享客户端连接池
		Network:         "tcp",
		Addr:            cfg.Address,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DB:              cfg.Database,
		MaxRetries:      cfg.MaxRetries,
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
	checkContext, cancel := context.WithTimeout(ctx, cfg.StartupTimeout) // 传入父上下文与期限；返回检查上下文、取消函数
	defer cancel()
	// 检查失败时不会将客户端交给调用方，因此这里关闭已创建的客户端。
	if err := client.Ping(checkContext).Err(); err != nil { // 传入带期限上下文执行 PING；失败时返回错误
		return nil, errors.Join(fmt.Errorf("ping redis: %w", err), client.Close()) // 合并 Ping 与关闭错误后返回
	}
	return client, nil
}
