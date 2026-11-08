package config

import (
	"fmt"
	"time"
)

// 登录注册功能配置缺失时使用的默认值。
const (
	defaultJWTIssuer          = "pulseframe-auth"  // JWT 默认签发者。
	defaultJWTAudience        = "pulseframe-api"   // JWT 默认受众。
	defaultAccessTokenTTL     = 15 * time.Minute   // 访问令牌默认有效期。
	defaultSessionTTL         = 7 * 24 * time.Hour // 会话默认绝对期限。
	defaultLoginAccountWindow = time.Minute        // 账号登录限流默认窗口。
	defaultLoginAccountLimit  = 5                  // 账号登录窗口内默认允许次数。
	defaultLoginIPWindow      = time.Minute        // IP 登录限流默认窗口。
	defaultLoginIPLimit       = 10                 // IP 登录窗口内默认允许次数。
	defaultRegisterIPWindow   = time.Hour          // IP 注册限流默认窗口。
	defaultRegisterIPLimit    = 20                 // IP 注册窗口内默认允许次数。
)

// Auth 保存登录注册功能所需的认证配置。
type Auth struct {
	JWTPrivateKeyPath  string        // RSA 私钥 PEM 文件路径，用于签发令牌。
	JWTPublicKeyPath   string        // RSA 公钥 PEM 文件路径，用于验证令牌。
	JWTIssuer          string        // 签发者，验签时要求匹配。
	JWTAudience        string        // 受众，验签时要求匹配。
	AllowedOrigin      string        // 允许的前端源，用于校验携带 Cookie 的变更请求；空表示不校验。
	AccessTokenTTL     time.Duration // 访问令牌有效期。
	SessionTTL         time.Duration // 会话绝对期限，不因刷新延长。
	LoginAccountWindow time.Duration // 账号登录滑动窗口。
	LoginAccountLimit  int           // 账号登录窗口内允许次数。
	LoginIPWindow      time.Duration // IP 登录滑动窗口。
	LoginIPLimit       int           // IP 登录窗口内允许次数。
	RegisterIPWindow   time.Duration // IP 注册滑动窗口。
	RegisterIPLimit    int           // IP 注册窗口内允许次数。
}

// loadAuth 从环境变量读取并校验认证配置。
// 输入：lookup，环境变量查询函数。
// 输出：认证配置；密钥路径缺失或参数非法时返回错误。
// 功能：集中加载 JWT 密钥与令牌、会话、限流参数。
// 例如密钥路径有效且默认值未被覆盖时，返回机构期限7天的认证配置。
func loadAuth(lookup LookupEnv) (Auth, error) {
	privateKey, privateOK := lookup("AUTH_JWT_PRIVATE_KEY_PATH")        // 读取私钥路径
	publicKey, publicOK := lookup("AUTH_JWT_PUBLIC_KEY_PATH")           // 读取公钥路径
	if !privateOK || privateKey == "" || !publicOK || publicKey == "" { // 密钥路径缺失
		return Auth{}, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_PATH and AUTH_JWT_PUBLIC_KEY_PATH are required")
	}
	accessTTL, err := parsePositiveDuration(lookup, "AUTH_ACCESS_TOKEN_TTL", defaultAccessTokenTTL) // 读取访问令牌有效期
	if err != nil {
		return Auth{}, err
	}
	sessionTTL, err := parsePositiveDuration(lookup, "AUTH_SESSION_TTL", defaultSessionTTL) // 读取会话期限
	if err != nil {
		return Auth{}, err
	}
	loginAccountWindow, err := parsePositiveDuration(lookup, "AUTH_LOGIN_ACCOUNT_WINDOW", defaultLoginAccountWindow) // 读取账号登录窗口
	if err != nil {
		return Auth{}, err
	}
	loginAccountLimit, err := parsePositiveInt(lookup, "AUTH_LOGIN_ACCOUNT_LIMIT", defaultLoginAccountLimit) // 读取账号登录额度
	if err != nil {
		return Auth{}, err
	}
	loginIPWindow, err := parsePositiveDuration(lookup, "AUTH_LOGIN_IP_WINDOW", defaultLoginIPWindow) // 读取 IP 登录窗口
	if err != nil {
		return Auth{}, err
	}
	loginIPLimit, err := parsePositiveInt(lookup, "AUTH_LOGIN_IP_LIMIT", defaultLoginIPLimit) // 读取 IP 登录额度
	if err != nil {
		return Auth{}, err
	}
	registerIPWindow, err := parsePositiveDuration(lookup, "AUTH_REGISTER_IP_WINDOW", defaultRegisterIPWindow) // 读取注册窗口
	if err != nil {
		return Auth{}, err
	}
	registerIPLimit, err := parsePositiveInt(lookup, "AUTH_REGISTER_IP_LIMIT", defaultRegisterIPLimit) // 读取注册额度
	if err != nil {
		return Auth{}, err
	}
	return Auth{
		JWTPrivateKeyPath:  privateKey,
		JWTPublicKeyPath:   publicKey,
		JWTIssuer:          readValue(lookup, "AUTH_JWT_ISSUER", defaultJWTIssuer),
		JWTAudience:        readValue(lookup, "AUTH_JWT_AUDIENCE", defaultJWTAudience),
		AllowedOrigin:      readValue(lookup, "AUTH_ALLOWED_ORIGIN", ""),
		AccessTokenTTL:     accessTTL,
		SessionTTL:         sessionTTL,
		LoginAccountWindow: loginAccountWindow,
		LoginAccountLimit:  loginAccountLimit,
		LoginIPWindow:      loginIPWindow,
		LoginIPLimit:       loginIPLimit,
		RegisterIPWindow:   registerIPWindow,
		RegisterIPLimit:    registerIPLimit,
	}, nil
}

// parsePositiveInt 读取正整数配置。
// 输入：lookup，环境查询函数；key，变量名；fallback，缺省整数。
// 输出：正数整数；格式错误或小于等于零时返回错误。
// 功能：读取额度配置，确保结果为正值。
// 例如配置为 "5" 时返回5，配置为 "0" 或 "-1" 时返回错误。
func parsePositiveInt(lookup LookupEnv, key string, fallback int) (int, error) {
	value, err := parseNonNegativeInt(lookup, key, fallback) // 先按非负读取
	if err != nil {
		return 0, err
	}
	if value <= 0 { // 额度为零或负
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return value, nil
}
