package domain

import (
	"strings"
	"unicode/utf8"
)

const (
	minimumUsernameLength = 3
	maximumUsernameLength = 32
	minimumPasswordLength = 8
	maximumPasswordBytes  = 1024
	StatusActive          = "active"
	StatusDisabled        = "disabled"
)

// User 保存用户身份、密码摘要和权威账号状态。
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Status       string
}

// ValidateUsername 校验用户名格式。
// 输入：username，3 至 32 位小写 ASCII 字母、数字或下划线。
// 输出：符合规则时返回 nil，否则返回 ErrInvalidUsername。
// 功能：保证数据库唯一键与登录时的用户名匹配规则一致。
func ValidateUsername(username string) error {
	if len(username) < minimumUsernameLength || len(username) > maximumUsernameLength {
		return ErrInvalidUsername
	}
	for _, character := range username {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return ErrInvalidUsername
	}
	return nil
}

// ValidatePassword 校验密码长度且不改写密码内容。
// 输入：password，至少 8 个 Unicode 字符且 UTF-8 编码不超过 1024 字节。
// 输出：符合规则时返回 nil，否则返回 ErrInvalidPassword。
// 功能：限制密码哈希资源消耗，同时保留用户输入的原始字符。
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < minimumPasswordLength ||
		len(password) > maximumPasswordBytes || strings.TrimSpace(password) == "" {
		return ErrInvalidPassword
	}
	return nil
}
