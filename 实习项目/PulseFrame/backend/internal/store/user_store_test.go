//go:build mysqlintegration

package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"pulseframe/internal/model"
	"pulseframe/internal/platform/config"
	"pulseframe/internal/platform/infra"
)

// openTestDB 连接真实 MySQL 并完成迁移。
// 输入：t，测试上下文。
// 输出：可用于测试的 gorm 实例，登记测试结束时的关闭操作；连接或迁移失败时结束测试。
// 功能：统一真实连接准备与清理。
// 例如进程配置了有效 MYSQL_* 变量时可获得 migrations数据库实例。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg, err := config.Load(lookupEnv) // 读取完整配置
	if err != nil {
		t.Fatal(err)
	}
	db, err := infra.NewMySQL(t.Context(), cfg.MySQL) // 连接真实数据库
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil { // 同步表结构
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB() // 取出底层连接池用于关闭
		if err != nil {
			t.Error(err)
		} else if err := sqlDB.Close(); err != nil { // 测试结束时关闭连接池
			t.Error(err)
		}
	})
	return db
}

// lookupEnv 读取进程环境变量，作为集成测试的配置来源。
// 输入：key，环境变量名。
// 输出：变量值及是否存在标记。
// 功能：让测试使用真实环境变量配置。
func lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key) // 读取进程环境变量
}

// newID 生成用于测试的随机用户标识。
// 输入：t，测试上下文。
// 输出：32位十六进制字符串表示的用户标识。
// 功能：为每次测试生成唯一 UUIDv4 形式标识。
// 例如调用返回形如 7baf01e9c2d3465089abcdef0123456789 的标识。
func newID(t *testing.T) string {
	t.Helper()
	content := make([]byte, 16) // 分配16字节随机数据
	if _, err := rand.Read(content); err != nil { // 读取安全随机字节
		t.Fatal(err)
	}
	return hex.EncodeToString(content) // 编码为十六进制字符串
}

// TestUserStoreCreateAndQuery 验证创建、按用户名与按标识查询。
// 输入：Go 测试框架提供的测试上下文。
// 输出：创建或查询结果与预期不符时使测试失败。
// 功能：验证事务创建及两种查询路径。
// 例如创建用户后按用户名和标识均能取回同一条记录。
func TestUserStoreCreateAndQuery(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	userID := newID(t)
	user := &model.User{ID: userID, Status: "active", AuthVersion: 1}
	credential := &model.PasswordCredential{UserID: userID, Username: "demo_user", PasswordHash: "phc-hash"}
	if err := CreateUserWithCredential(ctx, db, user, credential); err != nil { // 事务创建用户与凭据
		t.Fatal(err)
	}
	got, err := GetCredentialByUsername(ctx, db, "demo_user") // 按用户名查询凭据
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != userID || got.Username != "demo_user" { // 核对凭据字段
		t.Fatalf("unexpected credential: %+v", got)
	}
	gotUser, err := GetUserByID(ctx, db, userID) // 按标识查询用户
	if err != nil {
		t.Fatal(err)
	}
	if gotUser.ID != userID || gotUser.Status != "active" { // 核对用户字段
		t.Fatalf("unexpected user: %+v", gotUser)
	}
}

// TestUserStoreUniqueConflictRollsBack 验证并发冲突回滚用户记录。
// 输入：Go 测试框架提供的测试上下文。
// 输出：重复用户名未报错或用户未回滚时使测试失败。
// 功能：确保凭据唯一约束冲突时整笔事务回滚。
// 例如用户名重复时创建失败，且首次用户之后不残留被回滚的用户。
func TestUserStoreUniqueConflictRollsBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	firstID := newID(t)
	first := &model.User{ID: firstID, Status: "active", AuthVersion: 1}
	firstCredential := &model.PasswordCredential{UserID: firstID, Username: "conflict_user", PasswordHash: "hash-1"}
	if err := CreateUserWithCredential(ctx, db, first, firstCredential); err != nil { // 先创建首个用户
		t.Fatal(err)
	}
	secondID := newID(t)
	second := &model.User{ID: secondID, Status: "active", AuthVersion: 1}
	secondCredential := &model.PasswordCredential{UserID: secondID, Username: "conflict_user", PasswordHash: "hash-2"}
	err := CreateUserWithCredential(ctx, db, second, secondCredential) // 创建冲突用户
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 { // 确认是唯一键冲突
		t.Fatalf("unexpected conflict error: %v", err)
	}
	var count int64
	if err := db.WithContext(ctx).Model(&model.User{}).Where("id = ?", secondID).Count(&count).Error; err != nil { // 统计冲突用户
		t.Fatal(err)
	}
	if count != 0 { // 冲突失败后不残留半成品用户
		t.Fatalf("conflict transaction left user: count=%d", count)
	}
}
