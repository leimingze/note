package store

import (
	"context"

	"gorm.io/gorm"

	"pulseframe/internal/model"
)

// CreateUserWithCredential 在同一事务中创建用户及其密码凭据。
// 输入：ctx，请求或启动上下文；db，gorm 实例；user，待创建用户；credential，待创建凭据。
// 输出：全部写入成功返回 nil；任何一步失败时回滚整笔事务并返回错误。
// 功能：保证用户与凭据要么同时存在，要么同时不存在。
// 例如用户名并发冲突导致凭据创建失败时，用户记录也一并回滚。
func CreateUserWithCredential(ctx context.Context, db *gorm.DB, user *model.User, credential *model.PasswordCredential) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { // 开启事务，失败自动回滚
		if err := tx.Create(user).Error; err != nil { // 先写用户记录
			return err
		}
		return tx.Create(credential).Error // 再写密码凭据
	})
}

// GetCredentialByUsername 按用户名查询密码凭据。
// 输入：ctx，上下文；db，gorm 实例；username，原始大小写的登录名。
// 输出：返回匹配的凭据；未找到或出错时返回 gorm 错误。
// 功能：登录验证密码时读取哈希及所属用户。
// 例如存在该用户名时返回对应凭据，不存在时返回 ErrRecordNotFound。
func GetCredentialByUsername(ctx context.Context, db *gorm.DB, username string) (*model.PasswordCredential, error) {
	var credential model.PasswordCredential // 查询结果容器
	if err := db.WithContext(ctx).Where("username = ?", username).First(&credential).Error; err != nil { // 按用户名查询凭据
		return nil, err
	}
	return &credential, nil
}

// GetUserByID 按用户标识查询用户。
// 输入：ctx，上下文；db，gorm 实例；id，用户 UUID。
// 输出：返回匹配用户；未找到或出错时返回 gorm 错误。
// 功能：登录、刷新或资源权限检查时读取账号状态与版本。
// 例如存在该用户时返回记录，不存在时返回 ErrRecordNotFound。
func GetUserByID(ctx context.Context, db *gorm.DB, id string) (*model.User, error) {
	var user model.User // 查询结果容器
	if err := db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil { // 按主键查询用户
		return nil, err
	}
	return &user, nil
}
