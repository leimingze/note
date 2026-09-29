package secure

import (
	"errors"
	"strings"
	"testing"

	"pulseframe/internal/modules/account/application"
)

// TestArgon2idRoundTrip 验证摘要不等于明文且正确密码能够通过。
// 输入：Go 测试框架提供的测试上下文。
// 输出：摘要格式、验证或错误密码行为不符合预期时使测试失败。
// 功能：验证注册与登录共用的 Argon2id 编码契约。
func TestArgon2idRoundTrip(t *testing.T) {
	hasher, err := NewArgon2id(1)
	if err != nil {
		t.Fatalf("create Argon2id hasher: %v", err)
	}
	password := "correct horse battery staple"
	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if strings.Contains(encoded, password) || !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("unexpected password encoding: %q", encoded)
	}
	if err := hasher.Verify(password, encoded); err != nil {
		t.Fatalf("verify correct password: %v", err)
	}
	if err := hasher.Verify("wrong password", encoded); err == nil {
		t.Fatal("wrong password unexpectedly verified")
	}
}

// TestArgon2idRejectsMalformedAndExpensiveParameters 验证存储摘要解析上限。
// 输入：Go 测试框架提供的测试上下文。
// 输出：损坏格式或超上限参数未被拒绝时使测试失败。
// 功能：确保数据库内容不能将验证内存或迭代成本放大到无界。
func TestArgon2idRejectsMalformedAndExpensiveParameters(t *testing.T) {
	for _, encoded := range []string{
		"not-a-hash",
		"$argon2id$v=19$m=999999999,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4",
		"$argon2id$v=19$m=20480,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4",
		"$argon2id$v=19$m=19456,t=3,p=1$c2FsdHNhbHRzYWx0c2FsdA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4",
	} {
		hasher, err := NewArgon2id(1)
		if err != nil {
			t.Fatal(err)
		}
		if err := hasher.Verify("password-long-enough", encoded); err == nil {
			t.Errorf("expected malformed hash to fail: %q", encoded)
		}
	}
}

// TestArgon2idRejectsSaturatedCapacity 验证满载时哈希请求快速失败。
// 输入：Go 测试框架提供的测试上下文。
// 输出：容量耗尽未返回明确错误时使测试失败。
// 功能：保护密码哈希达到并发上限时不排队消耗额外内存。
func TestArgon2idRejectsSaturatedCapacity(t *testing.T) {
	hasher, err := NewArgon2id(1)
	if err != nil {
		t.Fatalf("create Argon2id hasher: %v", err)
	}
	hasher.active <- struct{}{}
	if _, err := hasher.Hash("password-long-enough"); !errors.Is(err, application.ErrPasswordHashCapacity) {
		t.Fatalf("unexpected saturated Hash error: %v", err)
	}
	if err := hasher.Verify("password-long-enough", "invalid-hash"); err == nil {
		t.Fatal("malformed hash should be rejected before capacity check")
	}
}

// TestArgon2idRejectsInvalidConcurrency 验证超出边界的并发容量不能创建。
// 输入：Go 测试框架提供的测试上下文。
// 输出：零或超过实现上限时仍创建成功使测试失败。
// 功能：避免错误配置导致无界 Argon2id 并发内存分配。
func TestArgon2idRejectsInvalidConcurrency(t *testing.T) {
	for _, concurrency := range []int{0, maximumHashConcurrency + 1} {
		if _, err := NewArgon2id(concurrency); err == nil {
			t.Errorf("expected concurrency %d to fail", concurrency)
		}
	}
}
