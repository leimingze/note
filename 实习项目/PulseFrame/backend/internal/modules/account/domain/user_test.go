package domain

import (
	"strings"
	"testing"
)

// TestValidateUsernameEnforcesCanonicalForm 验证用户名只接受规范小写 ASCII 字符。
// 输入：Go 测试框架提供的测试上下文。
// 输出：非法用户名未被拒绝时使测试失败。
// 功能：保护应用校验与 MySQL 大小写敏感唯一键的一致性。
func TestValidateUsernameEnforcesCanonicalForm(t *testing.T) {
	for _, username := range []string{"ab", "Upper", "has-dash", "含中文", strings.Repeat("a", maximumUsernameLength+1)} {
		if err := ValidateUsername(username); err == nil {
			t.Errorf("expected username %q to be rejected", username)
		}
	}
	if err := ValidateUsername("user_123"); err != nil {
		t.Fatalf("valid username rejected: %v", err)
	}
}

// TestValidatePasswordPreservesWhitespace 验证密码按原字符校验而不裁剪。
// 输入：Go 测试框架提供的测试上下文。
// 输出：合法边界被拒绝或非法长度被接受时使测试失败。
// 功能：保护密码输入一致性并拒绝空白密码与过大输入。
func TestValidatePasswordPreservesWhitespace(t *testing.T) {
	if err := ValidatePassword("  long pass  "); err != nil {
		t.Fatalf("password with meaningful whitespace rejected: %v", err)
	}
	for _, password := range []string{"short", "        ", strings.Repeat("a", maximumPasswordBytes+1)} {
		if err := ValidatePassword(password); err == nil {
			t.Errorf("expected password of %d bytes to be rejected", len(password))
		}
	}
}
