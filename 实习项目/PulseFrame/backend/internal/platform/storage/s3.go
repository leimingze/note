package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3 是基于 minio-go 的实现，兼容 AWS S3、MinIO 与 SeaweedFS S3 端点。
type S3 struct {
	client   *minio.Client // 共享客户端。
	bucket   string        // 私有桶名。
	endpoint string        // 服务地址，用于拼接上传目标。
}

// NewS3 创建 S3 兼容客户端。
// 输入：endpoint，服务地址 host:port；accessKey、secretKey，凭证；bucket，桶名；secure，是否启用 TLS。
// 输出：S3 实现；创建失败返回错误。
// 功能：初始化并校验客户端，供后续对象操作使用。
func NewS3(endpoint, accessKey, secretKey, bucket string, secure bool) (*S3, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}
	return &S3{client: client, bucket: bucket, endpoint: endpoint}, nil
}

// PresignPost 生成直传 POST 凭证，限定大小、类型与有效期。
// 输入：ctx，上下文；key，对象键；maxBytes，长度上限；contentType，内容类型；ttl，有效期。
// 输出：预签名表单凭证。
// 功能：把请求参数写入 PostPolicy 并签名。
func (s *S3) PresignPost(ctx context.Context, key string, maxBytes int64, contentType string, ttl time.Duration) (*UploadCredential, error) {
	policy := minio.NewPostPolicy()
	policy.SetBucket(s.bucket)                // 指定桶
	policy.SetKey(key)                        // 指定对象键
	policy.SetContentLengthRange(1, maxBytes) // 限定字节范围
	policy.SetContentType(contentType)        // 限定内容类型
	policy.SetExpires(time.Now().Add(ttl))    // 设置过期时间
	u, form, err := s.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("presign post: %w", err)
	}
	return &UploadCredential{
		Method:     "POST",
		URL:        u.String(),
		FormFields: form,
	}, nil
}

// Head 返回对象元数据；对象不存在返回 ErrNotFound。
// 输入：ctx，上下文；key，对象键。
// 输出：对象信息；不存在时返回 ErrNotFound。
// 功能：用 StatObject 读取对象大小与类型。
func (s *S3) Head(ctx context.Context, key string) (*ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKey(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return &ObjectInfo{Key: key, Size: info.Size, ContentType: info.ContentType}, nil
}

// Copy 在存储内部复制对象。
// 输入：ctx，上下文；src，源键；dst，目标键。
// 输出：复制成功返回 nil。
// 功能：服务端复制，不经过本地进程。
func (s *S3) Copy(ctx context.Context, src, dst string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dst},
		minio.CopySrcOptions{Bucket: s.bucket, Object: src},
	)
	if err != nil {
		return fmt.Errorf("copy object: %w", err)
	}
	return nil
}

// Move 复制后删除源对象。
// 输入：ctx，上下文；src，源键；dst，目标键。
// 输出：移动成功返回 nil。
// 功能：先复制，成功后删除源。
func (s *S3) Move(ctx context.Context, src, dst string) error {
	if err := s.Copy(ctx, src, dst); err != nil {
		return err
	}
	return s.client.RemoveObject(ctx, s.bucket, src, minio.RemoveObjectOptions{})
}

// DeletePrefix 删除指定前缀下的所有对象。
// 输入：ctx，上下文；prefix，键前缀。
// 输出：删除成功返回 nil。
// 功能：递归列出前缀对象并批量删除。
func (s *S3) DeletePrefix(ctx context.Context, prefix string) error {
	keys := s.listKeys(ctx, prefix)
	if len(keys) == 0 {
		return nil
	}
	objectsCh := make(chan minio.ObjectInfo, len(keys))
	go func() {
		defer close(objectsCh)
		for _, k := range keys {
			objectsCh <- minio.ObjectInfo{Key: k}
		}
	}()
	for e := range s.client.RemoveObjects(ctx, s.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if e.Err != nil {
			return fmt.Errorf("remove objects: %w", e.Err)
		}
	}
	return nil
}

// SignedURL 生成带签名、限时有效的 GET 地址。
// 输入：ctx，上下文；key，对象键；ttl，有效期。
// 输出：预签名 GET 地址。
// 功能：使用 PresignedGetObject 签发地址。
func (s *S3) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, url.Values{})
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return u.String(), nil
}

// Put 写入对象内容，测试注入数据用。
// 输入：ctx，上下文；key，对象键；data，内容字节。
// 输出：写入成功返回 nil。
// 功能：使用 PutObject 覆盖写入。
func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

// listKeys 返回前缀下的对象键列表。
func (s *S3) listKeys(ctx context.Context, prefix string) []string {
	var keys []string
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		keys = append(keys, obj.Key)
	}
	return keys
}

// isNoSuchKey 判断错误是否为对象不存在。
func isNoSuchKey(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.Code == "NoSuchKey"
}
