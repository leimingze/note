package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"pulseframe/dao"
)

const (
	maxUsernameRunes = 24
	maxPasswordBytes = 72
)

var (
	// ErrInvalidUsername 表示用户名为空或超过字段长度。
	ErrInvalidUsername = errors.New("username must contain 1 to 24 characters")
	// ErrInvalidPassword 表示密码为空或超出 bcrypt 接受的字节数。
	ErrInvalidPassword = errors.New("password must contain 1 to 72 bytes")
)

// Registration 实现新账号注册。
type Registration struct{ users *dao.Users }

// NewRegistration 构造注册服务。
// 输入：users，已连接数据库的账号持久化对象，不能为 nil。
// 输出：注册服务；依赖缺失时返回错误。
// 功能：将注册规则与数据库操作组合成单一业务入口。
func NewRegistration(users *dao.Users) (*Registration, error) {
	if users == nil {
		return nil, errors.New("user repository is required")
	}
	return &Registration{users: users}, nil
}

// Register 校验输入并创建账号。
// 输入：ctx，请求上下文；username，原始用户名；password，原始密码。
// 输出：成功时返回 nil；输入、唯一索引或数据库失败时返回对应错误。
// 功能：生成 bcrypt 哈希后只向 MySQL 插入账号，不写 Redis。
func (registration *Registration) Register(ctx context.Context, username, password string) error {
	if strings.TrimSpace(username) == "" || utf8.RuneCountInString(username) > maxUsernameRunes {
		return ErrInvalidUsername
	}
	if len(password) == 0 || len(password) > maxPasswordBytes {
		return ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return registration.users.Create(ctx, username, string(hash))
}
