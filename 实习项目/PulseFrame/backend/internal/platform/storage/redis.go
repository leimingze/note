package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"pulseframe/internal/platform/config"
)

// OpenRedis 创建 Redis 客户端并在启动期限内验证连接。
// 输入：ctx，服务启动上下文；settings，地址、ACL 用户、密码和逻辑库配置。
// 输出：已通过 PING 的客户端；连接失败时关闭客户端并返回错误。
// 功能：在服务接流量前确认会话和限流后端可用。
func OpenRedis(ctx context.Context, settings config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: settings.Address, Username: settings.Username,
		Password: settings.Password, DB: settings.Database,
		DialTimeout: dependencyPingTimeout, ReadTimeout: dependencyPingTimeout,
		WriteTimeout: dependencyPingTimeout,
	})
	pingContext, cancel := context.WithTimeout(ctx, dependencyPingTimeout)
	defer cancel()
	if err := client.Ping(pingContext).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return client, nil
}

// CloseRedis 关闭 Redis 客户端连接池。
// 输入：client，已经创建的 Redis 客户端。
// 输出：客户端关闭结果；客户端为空或关闭失败时返回错误。
// 功能：在 HTTP 服务退出时释放 Redis 网络连接。
func CloseRedis(client *redis.Client) error {
	if client == nil {
		return fmt.Errorf("Redis client is required")
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close Redis client: %w", err)
	}
	return nil
}
