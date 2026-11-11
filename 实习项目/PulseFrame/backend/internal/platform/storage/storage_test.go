package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// seed 把数据写入存储，无法写入时让测试失败。
func seed(t *testing.T, s Storage, key string, data []byte) {
	t.Helper()
	p, ok := s.(Putter)
	if !ok {
		t.Fatal("storage does not implement Putter")
	}
	if err := p.Put(context.Background(), key, data); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// runStorageSuite 运行内存与 S3 实现共用的一致性用例。
// 输入：t，测试上下文；s，对象存储实现。
// 输出：断言全部通过时返回。
// 功能：验证六个接口的可观察行为。
func runStorageSuite(t *testing.T, s Storage) {
	t.Helper()
	ctx := context.Background()
	seed(t, s, "upload/a/media.mp4", []byte("media-bytes"))
	seed(t, s, "upload/a/cover.jpg", []byte("cover-bytes"))

	t.Run("Head", func(t *testing.T) {
		info, err := s.Head(ctx, "upload/a/media.mp4")
		if err != nil {
			t.Fatal(err)
		}
		if info.Size != int64(len("media-bytes")) {
			t.Fatalf("size=%d", info.Size)
		}
		if _, err := s.Head(ctx, "upload/missing"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Copy", func(t *testing.T) {
		if err := s.Copy(ctx, "upload/a/media.mp4", "videos/a/media.mp4"); err != nil {
			t.Fatal(err)
		}
		info, err := s.Head(ctx, "videos/a/media.mp4")
		if err != nil || info.Size != int64(len("media-bytes")) {
			t.Fatalf("copy result not visible: %v", err)
		}
	})

	t.Run("Move", func(t *testing.T) {
		if err := s.Move(ctx, "upload/a/cover.jpg", "videos/a/cover_1.jpg"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Head(ctx, "upload/a/cover.jpg"); !errors.Is(err, ErrNotFound) {
			t.Fatal("source still exists after move")
		}
	})

	t.Run("DeletePrefix", func(t *testing.T) {
		seed(t, s, "stale/a/1.bin", []byte("x"))
		seed(t, s, "keep/a/1.bin", []byte("y"))
		if err := s.DeletePrefix(ctx, "stale/"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Head(ctx, "stale/a/1.bin"); !errors.Is(err, ErrNotFound) {
			t.Fatal("prefix object not deleted")
		}
		if _, err := s.Head(ctx, "keep/a/1.bin"); err != nil {
			t.Fatal("non-prefix object deleted")
		}
	})

	t.Run("SignedURL", func(t *testing.T) {
		u, err := s.SignedURL(ctx, "videos/a/media.mp4", time.Hour)
		if err != nil || !strings.Contains(u, "media.mp4") {
			t.Fatalf("signed url invalid: %q %v", u, err)
		}
	})

	t.Run("PresignPost", func(t *testing.T) {
		cred, err := s.PresignPost(ctx, "upload/b/media.mp4", 200*1024*1024, "video/mp4", 30*time.Minute)
		if err != nil || cred == nil {
			t.Fatalf("presign post failed: %v", err)
		}
		if cred.Method != "POST" {
			t.Fatalf("method=%s", cred.Method)
		}
	})
}

// TestMemoryStorageSuite 在内存实现上运行一致性用例。
// 输入：t，测试上下文。
// 输出：断言语通过时返回。
// 功能：验证内存实现功能正确。
func TestMemoryStorageSuite(t *testing.T) {
	runStorageSuite(t, NewMemory())
}
