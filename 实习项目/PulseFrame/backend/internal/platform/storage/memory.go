package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory 是基于进程内 map 的对象存储实现，用于单元测试。
type Memory struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMemory 创建空的内存对象存储。
// 输入：无。
// 输出：内存对象存储实例。
// 功能：内部用并发安全的 map 保存对象内容。
func NewMemory() *Memory {
	return &Memory{objects: make(map[string][]byte)}
}

// PresignPost 返回固定的假凭证，字段与输入参数一致。
// 输入：ctx，上下文；key，对象键；maxBytes，长度上限；contentType，内容类型；ttl，有效期。
// 输出：假凭证，不访问存储。
// 功能：满足接口约定，供测试断言返回参数。
func (m *Memory) PresignPost(_ context.Context, key string, maxBytes int64, contentType string, ttl time.Duration) (*UploadCredential, error) {
	return &UploadCredential{
		Method: "POST",
		URL:    fmt.Sprintf("http://memory.invalid/%s", key),
		FormFields: map[string]string{
			"key":                  key,
			"content-length-range": fmt.Sprintf("%d-%d", 1, maxBytes),
			"Content-Type":         contentType,
			"expires":              fmt.Sprintf("%d", time.Now().Add(ttl).Unix()),
		},
	}, nil
}

// Head 返回对象元数据；对象不存在返回 ErrNotFound。
// 输入：ctx，上下文；key，对象键。
// 输出：对象信息；不存在时返回 ErrNotFound。
// 功能：从内存读取对象内容并计算大小。
func (m *Memory) Head(_ context.Context, key string) (*ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &ObjectInfo{Key: key, Size: int64(len(data))}, nil
}

// Copy 在内存内复制对象；源不存在返回 ErrNotFound。
// 输入：ctx，上下文；src，源键；dst，目标键。
// 输出：复制成功返回 nil。
// 功能：深拷贝字节内容到目标键。
func (m *Memory) Copy(_ context.Context, src, dst string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[src]
	if !ok {
		return ErrNotFound
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.objects[dst] = cp
	return nil
}

// Move 复制后删除源对象。
// 输入：ctx，上下文；src，源键；dst，目标键。
// 输出：移动成功返回 nil。
// 功能：等价于 Copy 加删除源。
func (m *Memory) Move(ctx context.Context, src, dst string) error {
	if err := m.Copy(ctx, src, dst); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.objects, src)
	m.mu.Unlock()
	return nil
}

// DeletePrefix 删除以 prefix 开头的所有对象。
// 输入：ctx，上下文；prefix，键前缀。
// 输出：删除成功返回 nil。
// 功能：过滤并删除匹配前缀的键。
func (m *Memory) DeletePrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.objects))
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		delete(m.objects, k)
	}
	return nil
}

// SignedURL 返回带假签名的 URL。
// 输入：ctx，上下文；key，对象键；ttl，有效期。
// 输出：形如 http://memory.invalid/{key}?sign=... 的地址。
// 功能：模拟签名地址的形状。
func (m *Memory) SignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("http://memory.invalid/%s?sign=mem&expires=%d", key, time.Now().Add(ttl).Unix()), nil
}

// Put 写入对象内容。
// 输入：ctx，上下文；key，对象键；data，内容字节。
// 输出：写入成功返回 nil。
// 功能：覆盖写入对象。
func (m *Memory) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = data
	return nil
}

// Keys 返回按字典序排序的对象键列表，仅供测试断言。
// 输入：无。
// 输出：对象键切片。
// 功能：导出内存对象键集合。
func (m *Memory) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.objects))
	for k := range m.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
