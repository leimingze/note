package storage

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 表示对象不存在；调用方可用 errors.Is 判断。
var ErrNotFound = errors.New("storage: object not found")

// UploadCredential 表示客户端直传使用的预签名凭证。
type UploadCredential struct {
	Method     string            // 提交方式，通常为 POST。
	URL        string            // 上传目标地址。
	FormFields map[string]string // 表单字段，包含签名与策略。
}

// ObjectInfo 描述对象元数据。
type ObjectInfo struct {
	Key         string // 对象键。
	Size        int64  // 对象大小，单位字节。
	ContentType string // 内容类型。
}

// Storage 定义对象存储接口，对应 P0 需要的六个操作。
type Storage interface {
	// PresignPost 生成限制大小与类型、带过期时间的直传 POST 凭证。
	PresignPost(ctx context.Context, key string, maxBytes int64, contentType string, ttl time.Duration) (*UploadCredential, error)
	// Head 读取对象元数据；对象不存在返回 ErrNotFound。
	Head(ctx context.Context, key string) (*ObjectInfo, error)
	// Copy 在存储内部把对象从 src 复制到 dst。
	Copy(ctx context.Context, src, dst string) error
	// Move 等价于复制后删除源对象。
	Move(ctx context.Context, src, dst string) error
	// DeletePrefix 删除指定前缀下的所有对象。
	DeletePrefix(ctx context.Context, prefix string) error
	// SignedURL 生成带签名、限时有效的 GET 地址。
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Putter 是供测试注入数据用的可选接口，不属于业务接口集合。
type Putter interface {
	// Put 写入对象内容，覆盖已有对象。
	Put(ctx context.Context, key string, data []byte) error
}
