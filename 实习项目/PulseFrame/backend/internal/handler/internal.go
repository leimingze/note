package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// 输入校验常量。
const (
	usernameMinLength = 3   // 用户名最短长度。
	usernameMaxLength = 24  // 用户名最长长度。
	passwordMinRunes  = 12  // 密码最短字符数。
	passwordMaxRunes  = 128 // 密码最长字符数。
	passwordMaxBytes  = 512 // 密码 UTF-8 编码最大字节数。
	sessionIDBytes    = 16  // 会话标识随机字节数。
	refreshTokenBytes = 48  // 刷新令牌随机字节数。
)

// usernamePattern 只允许 ASCII 字母、数字与下划线。
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// normalizeIP 把原始 IP 规范化为去 IPv4 映射的统一形式。
// 输入：raw，可能带端口或 IPv6 映射地址的 IP 文本。
// 输出：规范化 IP 字符串；解析失败时返回空串。
// 功能：让同一地址的不同文本共用同一计数键。
// 例如 ::ffff:192.0.2.1 返回 192.0.2.1。
func normalizeIP(raw string) string {
	raw = strings.TrimSpace(raw)      // 清理首尾空白
	addr, err := netip.ParseAddr(raw) // 解析 IP
	if err != nil {
		return ""
	}
	return addr.Unmap().String() // 去除 IPv4 映射并序列化
}

// validUsername 校验用户名格式。
// 输入：username，待校验的用户名。
// 输出：长度与字符集合合法时返回 true。
// 功能：限制用户名长度并拒绝非法字符。
// 例如 alpha_123 返回 true，含空格或超过24字符返回 false。
func validUsername(username string) bool {
	if len(username) < usernameMinLength || len(username) > usernameMaxLength { // 长度越界
		return false
	}
	return usernamePattern.MatchString(username) // 字符集合合法
}

// validPassword 校验密码格式。
// 输入：password，待校验的密码。
// 输出：字符数与字节数合法时返回 true。
// 功能：限制密码长度，避免超长输入滥用资源。
// 例如12个 Unicode 字符返回 true，仅1字符返回 false。
func validPassword(password string) bool {
	runes := utf8.RuneCountInString(password)                 // 统计字符数
	if runes < passwordMinRunes || runes > passwordMaxRunes { // 字符数越界
		return false
	}
	return len(password) <= passwordMaxBytes // 字节数不超限
}

// randomToken 生成指定字节数的安全随机 token。
// 输入：size，随机字节数。
// 输出：字节数与错误；随机源不可用时返回错误。
// 功能：为会话标识或刷新令牌提供高熵值。
func randomToken(size int) ([]byte, error) {
	content := make([]byte, size)                 // 分配随机字节
	if _, err := rand.Read(content); err != nil { // 读取安全随机字节
		return nil, err
	}
	return content, nil
}

// newRandomID 生成随机十六进制标识。
// 输入：无。
// 输出：32位十六进制随机标识或错误。
// 功能：生成随机标识，用于用户或会话等需要唯一标识的场景。
func newRandomID() (string, error) {
	content, err := randomToken(sessionIDBytes) // 生成随机字节
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(content), nil // 编码为十六进制
}

// newRefreshToken 生成刷新令牌与摘要。
// 输入：无。
// 输出：无填充 base64url 令牌、SHA-256 十六进制摘要及错误。
// 功能：生成仅保存摘要、原始令牌只下发客户端的刷新凭据。
func newRefreshToken() (string, string, error) {
	content, err := randomToken(refreshTokenBytes) // 生成随机字节
	if err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(content) // 编码令牌
	digest := sha256.Sum256([]byte(token))                 // 计算摘要
	return token, hex.EncodeToString(digest[:]), nil       // 返回令牌与摘要
}

// refreshTokenDigest 计算刷新令牌的 SHA-256 摘要。
// 输入：token，刷新令牌。
// 输出：十六进制摘要。
// 功能：用于定位 Redis 中的刷新映射。
func refreshTokenDigest(token string) string {
	digest := sha256.Sum256([]byte(token)) // 计算摘要
	return hex.EncodeToString(digest[:])   // 返回十六进制
}

// setRefreshCookie 下发 HttpOnly 刷新令牌 Cookie。
// 输入：c，Gin 上下文；token，刷新令牌；maxAge，Cookie 存活秒数。
// 输出：修改响应头，写入刷新 Cookie。
// 功能：让浏览器只通过 Cookie 携带刷新令牌，脚本不可读。
// 例如登录成功后调用，下发带绝对期限的刷新 Cookie。
func setRefreshCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refreshToken",
		Value:    token,
		Path:     "/api",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	}) // 写入刷新 Cookie
}

// clearRefreshCookie 清除刷新令牌 Cookie。
// 输入：c，Gin 上下文。
// 输出：修改响应头，清除刷新 Cookie。
// 功能：退出时让浏览器移除刷新令牌。
// 例如退出成功后调用，浏览器随即丢弃刷新 Cookie。
func clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		Path:     "/api",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}) // 清除刷新 Cookie
}

// originAllowed 校验请求 Origin 是否在允许列表。
// 输入：c，Gin 上下文。
// 输出：允许返回 true，拒绝返回 false。
// 功能：配置了允许源时，拒绝跨站变更请求。
// 例如 Origin 匹配配置源返回 true，缺失或不匹配返回 false。
func (h *AuthHandler) originAllowed(c *gin.Context) bool {
	if h.cfg.AllowedOrigin == "" { // 未配置允许源
		return true
	}
	return c.GetHeader("Origin") == h.cfg.AllowedOrigin // 精确匹配 Origin
}
