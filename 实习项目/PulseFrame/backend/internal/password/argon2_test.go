package password

import "testing"

// TestHashAndVerifyRoundTrip 验证密码哈希可成功验证且错误密码被拒绝。
// 输入：Go 测试框架提供的测试上下文。
// 输出：哈希生成失败、正确密码被拒或错误密码通过时使测试失败。
// 功能：验证 Hash 与 Verify 往返行为。
// 例如对正确密码生成的哈希，Verify 返回 true；错误密码返回 false。
func TestHashAndVerifyRoundTrip(t *testing.T) {
	params := DefaultParams() // 获取默认哈希参数
	hashed, err := Hash([]byte("correct horse battery staple"), params) // 生成密码哈希
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(hashed, []byte("correct horse battery staple")) // 验证正确密码
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("valid password was rejected")
	}
	ok, err = Verify(hashed, []byte("wrong password")) // 验证错误密码
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("wrong password was accepted")
	}
}

// TestVerifyMalformedHash 验证非法哈希格式返回错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：非法哈希未返回错误时使测试失败。
// 功能：防止格式错误的哈希被静默当成不匹配。
// 例如传入非 PHC 字符串时返回错误。
func TestVerifyMalformedHash(t *testing.T) {
	if _, err := Verify("not-a-hash", []byte("x")); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}
