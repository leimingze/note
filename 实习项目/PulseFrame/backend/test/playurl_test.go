package test

import (
	"context"
	"strings"
	"testing"
	"time"

	"pulseframe/internal/content/playurl"
	"pulseframe/internal/content/storage"
)

// TestServiceSignURL 验证播放地址签名委派给对象存储。
// 输入：t，测试上下文。
// 输出：断言通过时返回。
// 功能：用内存存储检查签发结果非空且包含对象键。
func TestServiceSignURL(t *testing.T) {
	store := storage.NewMemory()
	svc := playurl.NewService(store)
	u, err := svc.SignURL(context.Background(), "videos/1/media.mp4", time.Hour)
	if err != nil || !strings.Contains(u, "media.mp4") {
		t.Fatalf("sign url: %q %v", u, err)
	}
}

// TestServicePurge 验证本地刷新为空操作。
// 输入：t，测试上下文。
// 输出：断言通过时返回。
// 功能：调用 Purge 应返回 nil。
func TestServicePurge(t *testing.T) {
	if err := playurl.NewService(storage.NewMemory()).Purge(context.Background(), "a", "b"); err != nil {
		t.Fatal(err)
	}
}
