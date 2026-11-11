package migration

import (
	"fmt"

	"gorm.io/gorm"

	"pulseframe/internal/model"
)

// Migrate 创建或更新登录、注册与内容发布所需的数据表。
// 输入：db，已经连接的 gorm 数据库实例。
// 输出：执行成功返回 nil；任一表迁移失败时返回错误。
// 功能：用 AutoMigrate 按模型同步 users、password_credentials、video 与 review_task 表结构。
// 例如数据库尚无 video 表时，执行后创建该表并应用模型定义的字段与索引。
func Migrate(db *gorm.DB) error {
	db = db.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4")                                              // 新表使用 InnoDB 与 utf8mb4 字符集
	if err := db.AutoMigrate(&model.User{}, &model.PasswordCredential{}, &model.Video{}, &model.ReviewTask{}); err != nil { // 传入四个模型；同步表结构，失败时返回错误
		return fmt.Errorf("auto migrate auth and video tables: %w", err)
	}
	return nil
}
