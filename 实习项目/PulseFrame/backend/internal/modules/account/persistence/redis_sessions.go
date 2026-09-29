package persistence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"pulseframe/internal/modules/account/application"
)

const (
	sessionKeyPrefix = "pulseframe:auth:session:"
	sessionTokenSize = 32
)

var errSessionTokenCollision = errors.New("session token collision")

// RedisSessionManager 通过 Redis TTL 管理不透明会话。
type RedisSessionManager struct {
	client redis.UniversalClient
}

// redisSession 是 Redis 中序列化的最小会话状态。
type redisSession struct {
	UserID    int64     `json:"user_id"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewRedisSessionManager 创建 Redis 会话管理器。
// 输入：client，已配置 Redis 地址和 ACL 的客户端句柄。
// 输出：会话管理器；客户端为空时返回错误。
// 功能：将会话读写隔离在 Redis 适配层。
func NewRedisSessionManager(client redis.UniversalClient) (*RedisSessionManager, error) {
	if client == nil {
		return nil, errors.New("Redis client is required")
	}
	return &RedisSessionManager{client: client}, nil
}

// Create 生成随机令牌并将令牌摘要对应的会话写入 Redis。
// 输入：ctx，请求上下文；userID，正数用户编号；ttl，正数绝对有效期。
// 输出：仅供响应 Cookie 使用的随机令牌及 CSRF 令牌；写入失败时返回错误。
// 功能：使用 NX 防止令牌碰撞覆盖已有会话，并通过 Redis TTL 限制生命周期。
func (manager *RedisSessionManager) Create(ctx context.Context, userID int64, ttl time.Duration) (application.SessionCredentials, error) {
	if userID <= 0 || ttl <= 0 {
		return application.SessionCredentials{}, errors.New("valid user ID and session TTL are required")
	}
	token, err := randomToken()
	if err != nil {
		return application.SessionCredentials{}, err
	}
	csrfToken, err := application.NewCSRFToken()
	if err != nil {
		return application.SessionCredentials{}, err
	}
	stored := redisSession{UserID: userID, CSRFToken: csrfToken, ExpiresAt: time.Now().UTC().Add(ttl)}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return application.SessionCredentials{}, fmt.Errorf("encode Redis session: %w", err)
	}
	created, err := manager.client.SetNX(ctx, sessionKey(token), encoded, ttl).Result()
	if err != nil {
		return application.SessionCredentials{}, fmt.Errorf("create Redis session: %w", err)
	}
	if !created {
		return application.SessionCredentials{}, errSessionTokenCollision
	}
	return application.SessionCredentials{Token: token, CSRFToken: csrfToken}, nil
}

// Resolve 通过 Cookie 令牌摘要读取并验证 Redis 会话。
// 输入：ctx，请求上下文；token，浏览器提交的十六进制随机令牌。
// 输出：用户编号、CSRF 令牌及过期时间；缺失或过期时返回 application.ErrUnauthorized。
// 功能：不在 Redis 键或会话值中保存原始会话令牌。
func (manager *RedisSessionManager) Resolve(ctx context.Context, token string) (application.Session, error) {
	if _, err := hex.DecodeString(token); err != nil || len(token) != sessionTokenSize*2 {
		return application.Session{}, application.ErrUnauthorized
	}
	encoded, err := manager.client.Get(ctx, sessionKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return application.Session{}, application.ErrUnauthorized
	}
	if err != nil {
		return application.Session{}, fmt.Errorf("read Redis session: %w", err)
	}
	return decodeSession(encoded)
}

// Revoke 删除当前 Cookie 令牌对应的 Redis 会话。
// 输入：ctx，请求上下文；token，浏览器提交的十六进制随机令牌。
// 输出：删除成功返回 nil；Redis 故障时返回错误。
// 功能：让当前设备退出后旧令牌不能继续建立登录态。
func (manager *RedisSessionManager) Revoke(ctx context.Context, token string) error {
	if _, err := hex.DecodeString(token); err != nil || len(token) != sessionTokenSize*2 {
		return application.ErrUnauthorized
	}
	if err := manager.client.Del(ctx, sessionKey(token)).Err(); err != nil {
		return fmt.Errorf("delete Redis session: %w", err)
	}
	return nil
}

// decodeSession 验证 Redis 中的会话载荷结构和绝对过期时间。
// 输入：encoded，Redis 返回的 JSON 字节。
// 输出：有效会话；载荷损坏、缺少字段或已过期时返回错误。
// 功能：让损坏会话显式失败，不把无效值解释成已登录。
func decodeSession(encoded []byte) (application.Session, error) {
	var stored redisSession
	if err := json.Unmarshal(encoded, &stored); err != nil {
		return application.Session{}, fmt.Errorf("decode Redis session: %w", err)
	}
	if stored.UserID <= 0 || stored.CSRFToken == "" || !stored.ExpiresAt.After(time.Now()) {
		return application.Session{}, application.ErrUnauthorized
	}
	return application.Session{UserID: stored.UserID, CSRFToken: stored.CSRFToken, ExpiresAt: stored.ExpiresAt}, nil
}

// randomToken 生成 256 位密码学安全的十六进制令牌。
// 输入：无。
// 输出：64 字符不透明令牌；安全随机源失败时返回错误。
// 功能：创建不可预测且适合 Cookie 传递的会话标识。
func randomToken() (string, error) {
	value := make([]byte, sessionTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(value), nil
}

// sessionKey 生成只包含会话令牌摘要的 Redis 键。
// 输入：token，随机会话令牌。
// 输出：带模块前缀的 SHA-256 摘要键。
// 功能：避免 Redis 键名泄露可直接重放的 Cookie 令牌。
func sessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return sessionKeyPrefix + hex.EncodeToString(digest[:])
}
