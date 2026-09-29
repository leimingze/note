package application

import (
	"context"
	"time"

	"pulseframe/internal/modules/account/domain"
)

// UserRepository 定义账号用例所需的 MySQL 用户操作。
type UserRepository interface {
	Create(context.Context, string, string) (domain.User, error)
	FindByUsername(context.Context, string) (domain.User, error)
	FindByID(context.Context, int64) (domain.User, error)
}

// SessionManager 定义 Redis 会话的创建、解析和撤销操作。
type SessionManager interface {
	Create(context.Context, int64, time.Duration) (SessionCredentials, error)
	Resolve(context.Context, string) (Session, error)
	Revoke(context.Context, string) error
}

// RateLimiter 定义跨实例共享的原子限流检查。
type RateLimiter interface {
	Allow(context.Context, []Limit) (bool, error)
}

// PasswordHasher 定义密码摘要的生成与验证能力。
type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) error
}

// Session 保存鉴权后可使用的主体和会话 CSRF 信息。
type Session struct {
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
}

// SessionCredentials 保存只返回给登录响应的令牌材料。
type SessionCredentials struct {
	Token     string
	CSRFToken string
}

// Limit 描述一个 Redis 限流桶。
type Limit struct {
	Key    string
	Max    int
	Window time.Duration
}

// RatePolicy 保存登录和注册使用的共享限流策略。
type RatePolicy struct {
	LoginIPMaximum         int
	LoginIdentityIPMaximum int
	RegistrationIPMaximum  int
	LoginWindow            time.Duration
	RegistrationWindow     time.Duration
}
