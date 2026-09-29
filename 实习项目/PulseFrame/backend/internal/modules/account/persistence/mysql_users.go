package persistence

import (
	"context"
	"errors"
	"fmt"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"pulseframe/internal/modules/account/domain"
)

// UserRepository 使用 GORM 将账号数据映射到 MySQL。
type UserRepository struct {
	db *gorm.DB
}

// userRecord 映射 MySQL users 表，不直接暴露给应用或 HTTP 层。
type userRecord struct {
	ID           int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string `gorm:"column:username"`
	PasswordHash string `gorm:"column:password_hash"`
	Status       string `gorm:"column:status"`
}

// TableName 指定账号数据表名。
// 输入：无。
// 输出：MySQL 表名 users。
// 功能：避免 GORM 根据结构体名称推断不同表名。
func (userRecord) TableName() string {
	return "users"
}

// NewUserRepository 创建账号 MySQL 仓储。
// 输入：db，已经连接并通过 Ping 的 GORM 数据库句柄。
// 输出：可执行用户查询和创建的仓储；句柄为空时返回错误。
// 功能：将 MySQL 持久化实现限制在账号模块适配层。
func NewUserRepository(db *gorm.DB) (*UserRepository, error) {
	if db == nil {
		return nil, errors.New("MySQL database is required")
	}
	return &UserRepository{db: db}, nil
}

// CheckSchema 验证账号迁移已执行且必要字段存在。
// 输入：ctx，启动检查上下文。
// 输出：表可查询时返回 nil；缺表或结构不匹配时返回数据库错误。
// 功能：在服务进入就绪状态前发现未应用的显式 SQL 迁移。
func (repository *UserRepository) CheckSchema(ctx context.Context) error {
	var records []userRecord
	err := repository.db.WithContext(ctx).Select([]string{"id", "username", "password_hash", "status"}).Limit(0).Find(&records).Error
	if err != nil {
		return fmt.Errorf("verify users schema: %w", err)
	}
	return nil
}

// Create 插入一个初始有效的普通用户。
// 输入：ctx，请求上下文；username，已校验用户名；passwordHash，Argon2id 摘要。
// 输出：数据库分配 ID 的用户；用户名冲突映射为 domain.ErrUsernameTaken。
// 功能：依靠 MySQL 唯一索引保证并发注册安全。
func (repository *UserRepository) Create(ctx context.Context, username string, passwordHash string) (domain.User, error) {
	record := userRecord{Username: username, PasswordHash: passwordHash, Status: domain.StatusActive}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isDuplicateUsername(err) {
			return domain.User{}, domain.ErrUsernameTaken
		}
		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}
	return userFromRecord(record), nil
}

// FindByUsername 按唯一用户名读取账号。
// 输入：ctx，请求上下文；username，已校验的规范用户名。
// 输出：匹配用户；无记录时返回 domain.ErrUserNotFound。
// 功能：提供注册登录所需的凭据读取。
func (repository *UserRepository) FindByUsername(ctx context.Context, username string) (domain.User, error) {
	var record userRecord
	err := repository.db.WithContext(ctx).Where("username = ?", username).Take(&record).Error
	return mapUserLookup(record, err)
}

// FindByID 按服务端会话中的用户编号读取账号。
// 输入：ctx，请求上下文；userID，正数内部账号编号。
// 输出：匹配用户；无记录或编号非法时返回 domain.ErrUserNotFound。
// 功能：为当前用户接口读取权威账号状态。
func (repository *UserRepository) FindByID(ctx context.Context, userID int64) (domain.User, error) {
	if userID <= 0 {
		return domain.User{}, domain.ErrUserNotFound
	}
	var record userRecord
	err := repository.db.WithContext(ctx).Where("id = ?", userID).Take(&record).Error
	return mapUserLookup(record, err)
}

// mapUserLookup 将 GORM 查询结果映射成领域用户。
// 输入：record，数据库行；err，GORM 查询结果。
// 输出：领域用户或明确的未找到、存储错误。
// 功能：隔离 GORM 错误类型和持久化结构。
func mapUserLookup(record userRecord, err error) (domain.User, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("query user: %w", err)
	}
	return userFromRecord(record), nil
}

// userFromRecord 转换数据库记录为领域用户。
// 输入：record，users 表中的一行。
// 输出：不对外序列化的领域用户。
// 功能：集中持久化模型与业务模型之间的映射。
func userFromRecord(record userRecord) domain.User {
	return domain.User{
		ID: record.ID, Username: record.Username,
		PasswordHash: record.PasswordHash, Status: record.Status,
	}
}

// isDuplicateUsername 判断 MySQL 唯一键冲突。
// 输入：err，MySQL 驱动返回的写入错误。
// 输出：用户名唯一键冲突时返回 true。
// 功能：把数据库并发冲突转换成稳定领域错误。
func isDuplicateUsername(err error) bool {
	var mysqlError *driver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
