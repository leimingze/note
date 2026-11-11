package cdn

import (
	"context"
	"time"

	"pulseframe/internal/platform/storage"
)

// CDN 定义内容分发接口，包含签发可播放地址与刷新缓存。
type CDN interface {
	// SignURL 生成带签名、限时有效的访问地址。
	SignURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Purge 主动刷新缓存；本地实现为空操作。
	Purge(ctx context.Context, keys ...string) error
}

// Service 为 CDN 接口的本地实现，依赖对象存储签发地址。
type Service struct {
	store storage.Storage // 对象存储，用于签发签名地址。
}

// NewService 创建 CDN 服务。
// 输入：store，对象存储实现。
// 输出：CDN 服务。
// 功能：绑定对象存储，供地址签发使用。
func NewService(store storage.Storage) *Service {
	return &Service{store: store}
}

// SignURL 通过对象存储签发签名地址。
// 输入：ctx，上下文；key，对象键；ttl，有效期。
// 输出：签名地址。
// 功能：委派给 storage.SignedURL。
func (s *Service) SignURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return s.store.SignedURL(ctx, key, ttl)
}

// Purge 为空操作，本地环境不维护缓存。
// 输入：ctx，上下文；keys，对象键列表。
// 输出：始终返回 nil。
// 功能：保留接口形状，生产环境可替换为 CDN 刷新。
func (s *Service) Purge(_ context.Context, _ ...string) error {
	return nil
}
