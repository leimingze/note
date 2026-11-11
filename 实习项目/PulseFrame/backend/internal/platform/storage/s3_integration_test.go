//go:build s3integration

package storage

import (
	"os"
	"testing"
)

// TestS3IntegrationSuite 在真实 S3 兼容端点（如 SeaweedFS）上运行一致性用例。
// 输入：t，测试上下文；需通过环境变量提供连接信息。
// 输出：配置缺失时跳过；断言失败时测试失败。
// 功能：验证生产实现与内存实现行为一致。
func TestS3IntegrationSuite(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	bucket := os.Getenv("S3_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("S3_ENDPOINT or S3_BUCKET not set")
	}
	s3, err := NewS3(endpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		t.Fatal(err)
	}
	runStorageSuite(t, s3)
}
