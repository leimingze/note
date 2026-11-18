//go:build ffprobeintegration

package mediaprobe

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rangeHandler 提供支持 Range 请求的对象下载服务，用于模拟对象存储的头部读取。
// 输入：data，对象字节。
// 输出：http.Handler，按 Range 头返回对应字节区间。
// 功能：完整返回或按 bytes=start-end 返回部分内容，验证探测仅读取头部。
func rangeHandler(data []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := r.Header.Get("Range"); h != "" {
			var start, end int64
			if _, err := fmt.Sscanf(h, "bytes=%d-%d", &start, &end); err != nil {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if end >= int64(len(data)) || end < start {
				end = int64(len(data)) - 1
			}
			body := data[start : end+1]
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(body)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
}

// TestMediaProbeIntegration 在真实 ffprobe 与样本文件上验证端到端探测。
// 输入：t，测试上下文；需通过 MEDIA_SAMPLE_DIR 提供样本目录或使用默认路径。
// 输出：ffprobe 缺失或样本缺失时跳过；断言失败时测试失败。
// 功能：对 ok_* 与 bad_* 样本分别断言通过或拒绝。
func TestMediaProbeIntegration(t *testing.T) {
	if err := CheckAvailable(); err != nil {
		t.Skip("ffprobe not found")
	}
	sampleDir := os.Getenv("MEDIA_SAMPLE_DIR")
	if sampleDir == "" {
		sampleDir = ".agent/work/20261118-mediaprobe/samples"
	}
	entries, err := os.ReadDir(sampleDir)
	if err != nil {
		t.Skipf("samples dir not found: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		expectOK := strings.HasPrefix(name, "ok_")
		if !expectOK && !strings.HasPrefix(name, "bad_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sampleDir, name))
		if err != nil {
			t.Fatalf("read sample %s: %v", name, err)
		}
		srv := httptest.NewServer(rangeHandler(data))
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, probeErr := Probe(ctx, srv.URL)
		cancel()
		srv.Close()
		if expectOK && probeErr != nil {
			t.Errorf("%s: expected ok, got %v", name, probeErr)
		}
		if !expectOK && probeErr == nil {
			t.Errorf("%s: expected rejection, got ok", name)
		}
	}
}
