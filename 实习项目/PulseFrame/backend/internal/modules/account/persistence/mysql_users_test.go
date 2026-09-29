package persistence

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"pulseframe/internal/modules/account/domain"
)

// newMySQLMockRepository 创建不连接网络的 GORM 仓储测试。
// 输入：t，Go 测试上下文。
// 输出：账号仓储及 SQL mock；测试结束时验证所有 SQL 预期。
// 功能：检查 GORM 查询、唯一冲突映射和 SQL 参数化行为。
func newMySQLMockRepository(t *testing.T) (*UserRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		_ = sqlDB.Close()
	})
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing: true, Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("create GORM test handle: %v", err)
	}
	repository, err := NewUserRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	return repository, mock
}

// TestCreateUserUsesDatabaseUniqueConstraint 验证创建用户由 MySQL 分配编号。
// 输入：Go 测试框架提供的测试上下文。
// 输出：插入语句或领域用户不符合预期时使测试失败。
// 功能：保护用户创建使用参数化 SQL 和有效账号默认状态。
func TestCreateUserUsesDatabaseUniqueConstraint(t *testing.T) {
	repository, mock := newMySQLMockRepository(t)
	mock.ExpectBegin()
	expectation := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`username`,`password_hash`,`status`) VALUES (?,?,?)"))
	expectation.WithArgs("user_1", "argon-hash", domain.StatusActive).
		WillReturnResult(sqlmock.NewResult(17, 1))
	mock.ExpectCommit()
	user, err := repository.Create(context.Background(), "user_1", "argon-hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.ID != 17 || user.Username != "user_1" || user.Status != domain.StatusActive {
		t.Fatalf("unexpected created user: %+v", user)
	}
}

// TestCreateUserMapsDuplicateKey 验证唯一键冲突转换为稳定领域错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：MySQL 1062 未映射为 ErrUsernameTaken 时使测试失败。
// 功能：让并发同名注册由数据库裁决并向 HTTP 层返回冲突。
func TestCreateUserMapsDuplicateKey(t *testing.T) {
	repository, mock := newMySQLMockRepository(t)
	mock.ExpectBegin()
	expectation := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`username`,`password_hash`,`status`) VALUES (?,?,?)"))
	expectation.WithArgs("user_1", "argon-hash", domain.StatusActive).
		WillReturnError(&driver.MySQLError{Number: 1062, Message: "duplicate"})
	mock.ExpectRollback()
	if _, err := repository.Create(context.Background(), "user_1", "argon-hash"); err != domain.ErrUsernameTaken {
		t.Fatalf("unexpected duplicate error: %v", err)
	}
}

// TestCheckSchemaVerifiesMigratedUsersTable 验证启动检查会实际查询 users 表结构。
// 输入：Go 测试框架提供的测试上下文。
// 输出：schema 查询不符合预期或数据库错误未传播时使测试失败。
// 功能：保护服务启动时对显式迁移结果的检查。
func TestCheckSchemaVerifiesMigratedUsersTable(t *testing.T) {
	repository, mock := newMySQLMockRepository(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id`,`username`,`password_hash`,`status` FROM `users` LIMIT ?")).WithArgs(0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "status"}))
	if err := repository.CheckSchema(context.Background()); err != nil {
		t.Fatalf("check users schema: %v", err)
	}
}
