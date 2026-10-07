package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

const duplicateKeyCode = 1062

// ErrUsernameExists 表示数据库唯一索引拒绝重复用户名。
var ErrUsernameExists = errors.New("username already exists")

// Users 通过 MySQL 保存账号。
type Users struct{ db *sql.DB }

// NewUsers 构造账号数据访问对象。
// 输入：db，已连接的 MySQL 连接池，不能为 nil。
// 输出：账号数据访问对象；db 为 nil 时返回错误。
// 功能：将连接池注入账号持久化逻辑。
func NewUsers(db *sql.DB) (*Users, error) {
	if db == nil {
		return nil, errors.New("mysql connection is required")
	}
	return &Users{db: db}, nil
}

// Create 插入一条新账号记录。
// 输入：ctx，请求上下文；username，已校验用户名；passwordHash，bcrypt 哈希。
// 输出：插入成功返回 nil；用户名重复返回 ErrUsernameExists；其余数据库错误原样包装。
// 功能：由 MySQL 唯一索引保证并发注册时只有一个请求成功。
func (users *Users) Create(ctx context.Context, username, passwordHash string) error {
	_, err := users.db.ExecContext(ctx,
		"INSERT INTO users (username, password_hash) VALUES (?, ?)", username, passwordHash)
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateKeyCode {
		return ErrUsernameExists
	}
	return fmt.Errorf("insert user: %w", err)
}
