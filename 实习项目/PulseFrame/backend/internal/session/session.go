package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// 会话或刷新映射操作失败返回的错误。
var (
	ErrNotFound        = errors.New("login expired")    // 会话或映射不存在或已过期。
	ErrVersionMismatch = errors.New("version mismatch") // 账号版本与会话记录不一致。
	ErrTokenMismatch   = errors.New("token mismatch")   // 刷新令牌摘要不匹配当前会话。
)

// Session 表示一次刷新会话。
type Session struct {
	SessionID          string // 会话标识。
	UserID             string // 会话所属用户。
	AuthVersion        uint64 // 创建会话时的账号版本。
	CurrentTokenDigest string // 当前刷新令牌摘要。
	AbsoluteExpiresAt  int64  // 会话不可延长的到期 Unix 秒数。
}

// Manager 负责会话与刷新令牌映射的原子读写。
type Manager struct {
	client *redis.Client // Redis 客户端。
}

// NewManager 创建会话管理器。
// 输入：client，go-redis 客户端。
// 输出：会话管理器。
// 功能：绑定 Redis 客户端，供会话读写脚本使用。
// 例如传入共享 Redis 客户端后返回会话管理器。
func NewManager(client *redis.Client) *Manager {
	return &Manager{client: client}
}

// Create 原子创建会话与刷新令牌映射。
// 输入：ctx，上下文；sessionID，会话标识；userID，用户；authVersion，账号版本；tokenDigest，刷新令牌摘要；absoluteExpiresAt，绝对到期时间。
// 输出：创建成功返回 nil；Redis 写入失败返回错误。
// 功能：保证会话与映射同时存在并共享同一到期时间。
// 例如登录时调用创建会话，之后刷新与退出均可定位。
func (m *Manager) Create(ctx context.Context, sessionID, userID string, authVersion uint64, tokenDigest string, absoluteExpiresAt int64) error {
	result, err := createScript.Run(ctx, m.client,
		[]string{sessionKey(sessionID), refreshKey(tokenDigest)},
		userID, strconv.FormatUint(authVersion, 10), tokenDigest, absoluteExpiresAt,
	).Result()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	if result == nil {
		return fmt.Errorf("create session: unexpected result")
	}
	return nil
}

// Get 按刷新令牌摘要读取会话与原始刷新令牌摘要。
// 输入：ctx，上下文；tokenDigest，刷新令牌摘要。
// 输出：会话及登录令牌摘要；映射或会话缺失时返回 ErrNotFound。
// 功能：读取会话并核对当前刷新令牌摘要。
// 例如刷新时传入请求令牌摘要，返回会话信息。
func (m *Manager) Get(ctx context.Context, tokenDigest string) (*Session, error) {
	sessionID, err := m.client.Get(ctx, refreshKey(tokenDigest)).Result() // 读取映射定位会话
	if errors.Is(err, redis.Nil) {                                        // 映射不存在
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh mapping: %w", err)
	}
	fields, err := m.client.HGetAll(ctx, sessionKey(sessionID)).Result() // 读取会话哈希
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if len(fields) == 0 { // 会话不存在
		return nil, ErrNotFound
	}
	authVersion, err := strconv.ParseUint(fields["authVersion"], 10, 64) // 解析账号版本
	if err != nil {
		return nil, fmt.Errorf("parse auth version: %w", err)
	}
	absoluteExpiresAt, err := strconv.ParseInt(fields["absoluteExpiresAt"], 10, 64) // 解析绝对到期时间
	if err != nil {
		return nil, fmt.Errorf("parse absolute expiry: %w", err)
	}
	return &Session{
		SessionID:          sessionID,
		UserID:             fields["userId"],
		AuthVersion:        authVersion,
		CurrentTokenDigest: fields["currentTokenDigest"],
		AbsoluteExpiresAt:  absoluteExpiresAt,
	}, nil
}

// Rotate 原子轮换刷新令牌、更新会话当前摘要并返回新会话标识。
// 输入：ctx，上下文；oldDigest，旧刷新令牌摘要；sessionID，会话标识；authVersion，当前账号版本；newDigest，新刷新令牌摘要；absoluteExpiresAt，绝对到期时间。
// 输出：轮换成功返回 nil；映射、会话或摘要校验失败时返回对应错误。
// 功能：保证并发刷新只有一个成功，失败不修改会话。
// 例如刷新时传入旧摘要与新摘要，成功后旧映射被删除。
func (m *Manager) Rotate(ctx context.Context, oldDigest, sessionID string, authVersion uint64, newDigest string, absoluteExpiresAt int64) error {
	result, err := rotateScript.Run(ctx, m.client,
		[]string{sessionKey(sessionID), refreshKey(oldDigest), refreshKey(newDigest)},
		strconv.FormatUint(authVersion, 10), absoluteExpiresAt, newDigest,
	).Int64()
	if err != nil {
		return fmt.Errorf("rotate session: %w", err)
	}
	switch result {
	case 0:
		return ErrNotFound
	case 1:
		return ErrVersionMismatch
	case 2:
		return ErrTokenMismatch
	default:
		return nil
	}
}

// Delete 原子删除当前刷新令牌映射与会话。
// 输入：ctx，上下文；tokenDigest，刷新令牌摘要；sessionID，会话标识。
// 输出：删除成功返回 nil；摘要不匹配返回 ErrTokenMismatch。
// 功能：仅在当前摘要匹配时删除会话，避免误删其他令牌对应会话。
// 例如退出时删除当前会话，重复退出返回成功。
func (m *Manager) Delete(ctx context.Context, tokenDigest, sessionID string) error {
	result, err := deleteScript.Run(ctx, m.client,
		[]string{sessionKey(sessionID), refreshKey(tokenDigest)},
	).Int64()
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if result == 0 {
		return ErrTokenMismatch
	}
	return nil
}

func sessionKey(sessionID string) string { return "session:" + sessionID }
func refreshKey(digest string) string    { return "refresh:" + digest }
