package auth

import (
	"fmt"

	"gorm.io/gorm"
)

// Migrate 创建或更新登录与注册所需的数据表。
// 输入：db，已经连接的 gorm 数据库实例。
// 输出：执行成功返回 nil；任一表迁移失败时返回错误。
// 功能：用 AutoMigrate 按模型同步 users 与 password_credentials 表结构。
// 例如数据库尚无 users 表时，执行后创建该表并应用模型定义的约束与索引。
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&User{}, &PasswordCredential{}); err != nil { // 传入两个模型；同步表结构，失败时返回错误
		return fmt.Errorf("auto migrate auth tables: %w", err)
	}
	return nil
}
