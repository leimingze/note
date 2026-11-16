package token

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestKeys 生成测试用 RSA 密钥对。
// 输入：无。
// 输出：私钥与公钥；生成失败时 panic。
// 功能：为令牌测试提供独立密钥，避免依赖外部文件。
// 例如调用后得到可用于签名和验证的密钥对。
func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048) // 生成 RSA 密钥对
	if err != nil {
		t.Fatal(err)
	}
	return key, &key.PublicKey
}

// TestSignAndVerifyRoundTrip 验证签发的令牌可被正确验证。
// 输入：Go 测试框架提供的测试上下文。
// 输出：验证失败或载荷不一致时使测试失败。
// 功能：验证签名、签发者、受众与载荷往返。
// 例如签发载荷后验证返回相同用户与会话标识。
func TestSignAndVerifyRoundTrip(t *testing.T) {
	priv, pub := generateTestKeys(t)                                                                       // 生成测试密钥
	signer := &Signer{key: priv, keyID: "test-key", issuer: "pulseframe-auth", audience: "pulseframe-api"} // 构造签发器
	verifier := &Verifier{key: pub, issuer: "pulseframe-auth", audience: "pulseframe-api"}                 // 构造验证器
	now := time.Now().Unix()                                                                               // 构造签发时间
	tokenString, err := signer.Sign("user-uuid", "session-uuid", now, 15*time.Minute)                      // 签发令牌
	if err != nil {
		t.Fatal(err)
	}
	got, err := verifier.Verify(tokenString) // 验证令牌
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "user-uuid" || got.SessionID != "session-uuid" { // 核对载荷
		t.Fatalf("unexpected claims: %+v", got)
	}
}

// TestVerifyExpiredToken 验证过期令牌返回分类错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：未返回过期错误时使测试失败。
// 功能：验证 exp 校验逻辑。
// 例如签发明文到期的令牌返回 ErrTokenExpired。
func TestVerifyExpiredToken(t *testing.T) {
	priv, pub := generateTestKeys(t)                                                                       // 生成测试密钥
	signer := &Signer{key: priv, keyID: "test-key", issuer: "pulseframe-auth", audience: "pulseframe-api"} // 构造签发器
	verifier := &Verifier{key: pub, issuer: "pulseframe-auth", audience: "pulseframe-api"}                 // 构造验证器
	now := time.Now().Unix()                                                                               // 构造签发时间
	tokenString, err := signer.Sign("user", "session", now-2000, -time.Minute)                             // 签发已过期令牌
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(tokenString); err != ErrTokenExpired { // 期望过期错误
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

// TestVerifyWrongAudience 验证受众不匹配导致令牌无效。
// 输入：Go 测试框架提供的测试上下文。
// 输出：未返回无效错误时使测试失败。
// 功能：验证受众校验逻辑。
// 例如授予其他受众的令牌返回 ErrTokenInvalid。
func TestVerifyWrongAudience(t *testing.T) {
	priv, pub := generateTestKeys(t)                                                                  // 生成测试密钥
	signer := &Signer{key: priv, keyID: "test-key", issuer: "pulseframe-auth", audience: "other-api"} // 不同受众签发器
	verifier := &Verifier{key: pub, issuer: "pulseframe-auth", audience: "pulseframe-api"}            // 原受众验证器
	now := time.Now().Unix()                                                                          // 构造签发时间
	tokenString, err := signer.Sign("user", "session", now, 15*time.Minute)                           // 签发令牌
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(tokenString); err != ErrTokenInvalid { // 期望无效错误
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

// TestPayloadImplementsInterface 验证 Payload 满足 jwt.Claims 接口。
// 输入：Go 测试框架提供的测试上下文。
// 输出：Payload 未实现接口时编译失败。
// 功能：确保嵌套 RegisteredClaims 满足库接口要求。
// 例如声明变量可用于 NewWithClaims。
func TestPayloadImplementsInterface(t *testing.T) {
	var _ jwt.Claims = Payload{} // 编译期断言接口实现
}
