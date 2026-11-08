package model

import "time"

// User 表示系统用户主表的一行记录。
// 所有登录方式共用同一用户标识，chars 承担鉴权职责。
type User struct {
	// ID 为用户唯一标识，应用生成规范小写 UUIDv4，作为主键。
	ID string `gorm:"type:char(36);primaryKey"`
	// Status 表示账号状态，值为 active 或 disabled。
	Status string `gorm:"type:varchar(16);not null;default:active"`
	// AuthVersion 在封禁或撤销全部登录时递增，用于判断会话是否失效。
	AuthVersion uint64 `gorm:"type:bigint unsigned;not null;default:1"`
	// CreatedAt 为用户创建时的 UTC 时间。
	CreatedAt time.Time `gorm:"type:datetime(3);not null"`
	// UpdatedAt 为最近修改账号状态或版本时的 UTC 时间。
	UpdatedAt time.Time `gorm:"type:datetime(3);not null"`
}

// TableName 返回用户表名。
// 输入：无。
// 输出：users。
// 功能：固定表名避免 gorm 复数化歧义。
// 例如该用户模型始终映射到 users 表。
func (User) TableName() string {
	return "users"
}

// PasswordCredential 表示用户名密码凭据表的一行记录。
// 一个用户最多一组本地密码凭据，密码只保存 Argon2id 哈希。
type PasswordCredential struct {
	// UserID 与 users.id 相等，作为主键和外键连接用户。
	UserID string `gorm:"type:char(36);primaryKey"`
	// Username 为保留原始大小写的登录名，使用 ascii_bin 比较使 abc 与 ABC 可分别注册。
	Username string `gorm:"type:varchar(24) character set ascii collate ascii_bin;not null;uniqueIndex:uk_password_username"`
	// PasswordHash 保存 Argon2id 完整 PHC 编码，含参数、盐与哈希。
	PasswordHash string `gorm:"type:varchar(255);not null"`
	// CreatedAt 为凭据创建时的 UTC 时间。
	CreatedAt time.Time `gorm:"type:datetime(3);not null"`
	// UpdatedAt 为最近修改凭据时的 UTC 时间。
	UpdatedAt time.Time `gorm:"type:datetime(3);not null"`
	// User 声明外键关联，删除与更新均限制为 RESTRICT。
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

// TableName 返回密码凭据表名。
// 输入：无。
// 输出：password_credentials。
// 功能：固定表名避免 gorm 复数化歧义。
// 例如该凭据模型始终映射到 password_credentials 表。
func (PasswordCredential) TableName() string {
	return "password_credentials"
}
