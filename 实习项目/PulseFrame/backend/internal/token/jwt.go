package token

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 令牌验证失败的分类错误，供调用方区分过期与其他无效令牌。
var (
	ErrTokenExpired = errors.New("token expired") // 令牌已过期。
	ErrTokenInvalid = errors.New("token invalid") // 令牌无效。
)

// Claims 是服务端签发的访问令牌载荷。
type Claims struct {
	SessionID            string `json:"sid"` // 会话标识，关联刷新会话。
	jwt.RegisteredClaims        // 标准声明，包含用户标识、签发者、受众、签发与到期时间。
}

// Signer 使用 RSA 私钥签发访问令牌。
type Signer struct {
	key      *rsa.PrivateKey // 私钥。
	keyID    string          // 密钥标识，写入令牌头。
	issuer   string          // 签发者。
	audience string          // 受众。
}

// NewSigner 加载 RSA 私钥并创建签发器。
// 输入：privateKeyPath，PEM 格式 RSA 私钥路径；kid，密钥标识；iss，签发者；aud，受众。
// 输出：签发器；私钥加载或解析失败时返回错误。
// 功能：读取私钥用于签发令牌，校验算法为 RS256。
// 例如私钥文件有效时返回可用签发器，文件缺失时返回错误。
func NewSigner(privateKeyPath, kid, iss, aud string) (*Signer, error) {
	key, err := loadPrivateKey(privateKeyPath) // 加载 RSA 私钥
	if err != nil {
		return nil, err
	}
	return &Signer{key: key, keyID: kid, issuer: iss, audience: aud}, nil
}

// Sign 签发访问令牌。
// 输入：userID，用户标识 UUID；sessionID，会话标识；now，签发时间的 Unix 秒数；ttl，令牌有效期。
// 输出：签名的 JWT 字符串；令牌生成失败时返回错误。
// 功能：按 RS256 签名并携带密钥标识。
// 例如传入用户、会话与有效期时返回带签名的令牌字符串。
func (s *Signer) Sign(userID, sessionID string, now int64, ttl time.Duration) (string, error) {
	claims := Claims{ // 组装令牌载荷
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			IssuedAt:  jwt.NewNumericDate(time.Unix(now, 0)),
			ExpiresAt: jwt.NewNumericDate(time.Unix(now, 0).Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims) // 按 RS256 创建令牌
	token.Header["kid"] = s.keyID                              // 写入密钥标识
	return token.SignedString(s.key)                           // 使用私钥签名并返回令牌
}

// Verifier 使用 RSA 公钥验证访问令牌。
type Verifier struct {
	key      *rsa.PublicKey // 公钥。
	issuer   string         // 期望签发者。
	audience string         // 期望受众。
}

// NewVerifier 加载 RSA 公钥并创建验证器。
// 输入：publicKeyPath，PEM 格式 RSA 公钥路径；iss，期望签发者；aud，期望受众。
// 输出：验证器；公钥加载或解析失败时返回错误。
// 功能：读取公钥用于验证令牌签名。
// 例如公钥文件有效时返回可用验证器，文件缺失时返回错误。
func NewVerifier(publicKeyPath, iss, aud string) (*Verifier, error) {
	key, err := loadPublicKey(publicKeyPath) // 加载 RSA 公钥
	if err != nil {
		return nil, err
	}
	return &Verifier{key: key, issuer: iss, audience: aud}, nil
}

// Verify 验证令牌并返回载荷。
// 输入：tokenString，待验证的 JWT。
// 输出：验证通过返回载荷；过期或无效令牌返回对应分类错误。
// 功能：校验签名、签发者、受众与有效期。
// 例如令牌正确未过期时返回载荷，过期时返回 ErrTokenExpired。
func (v *Verifier) Verify(tokenString string) (Claims, error) {
	var claims Claims // 载入结果
	parsed, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 { // 算法必须是 RS256
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.key, nil
	}, jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) { // 区分过期与其他无效
			return Claims{}, ErrTokenExpired
		}
		return Claims{}, ErrTokenInvalid
	}
	if !parsed.Valid { // 签名或校验失败
		return Claims{}, ErrTokenInvalid
	}
	return claims, nil
}

// loadPrivateKey 从 PEM 文件加载 RSA 私钥。
// 输入：path，PEM 文件路径。
// 输出：RSA 私钥；读取或解析失败时返回错误。
// 功能：兼容 PKCS1 与 PKCS8 两种私钥格式。
// 例如 PKCS8 格式私钥文件可成功解析，非 RSA 密钥返回错误。
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path) // 读取私钥文件
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(data) // 解码 PEM 块
	if block == nil {
		return nil, errors.New("private key is not valid PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil { // 尝试 PKCS8
		rsaKey, ok := key.(*rsa.PrivateKey) // 断言 RSA 私钥
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes) // 尝试 PKCS1
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return rsaKey, nil
}

// loadPublicKey 从 PEM 文件加载 RSA 公钥。
// 输入：path，PEM 文件路径。
// 输出：RSA 公钥；读取或解析失败时返回错误。
// 功能：兼容 PKIX 与 PKCS1 两种公钥格式。
// 例如 PKIX 格式公钥文件可成功解析，非 RSA 密钥返回错误。
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path) // 读取公钥文件
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(data) // 解码 PEM 块
	if block == nil {
		return nil, errors.New("public key is not valid PEM")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil { // 尝试 PKIX
		rsaKey, ok := key.(*rsa.PublicKey) // 断言 RSA 公钥
		if !ok {
			return nil, errors.New("public key is not RSA")
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes) // 尝试 PKCS1
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return rsaKey, nil
}
