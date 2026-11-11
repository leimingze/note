//go:build mysqlintegration

package test

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"pulseframe/internal/migration"
	"pulseframe/internal/platform/infra"
)

// indexExpectation 描述一张表上应存在的索引及其字段顺序。
type indexExpectation struct {
	table   string   // 索引所在表名。
	name    string   // 索引名。
	unique  bool     // 是否为唯一索引。
	columns []string // 按顺序期望的列名。
}

// 输入：t，测试上下文，进程环境需配置独立测试数据库。
// 输出：迁移成功、两张表均存在，且索引、唯一约束与字符集符合数据字典；否则测试失败。
// 功能：在真实 MySQL 上执行迁移，验证 video 与 review_task 两表创建正确、可重放。
// 例如迁移后查询 SHOW INDEX 能按顺序返回 idx_video_author_public 的四列。
func TestMigrationVideoTables(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	gdb, err := infra.NewMySQL(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := migration.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 重复执行迁移，验证迁移可重放且结果稳定。
	if err := migration.Migrate(gdb); err != nil {
		t.Fatalf("migrate second run: %v", err)
	}

	assertTableExists(t, gdb, cfg.Database, "video")
	assertTableExists(t, gdb, cfg.Database, "review_task")
	assertTableCharset(t, gdb, cfg.Database, "video")
	assertTableCharset(t, gdb, cfg.Database, "review_task")

	expectations := []indexExpectation{
		{table: "video", name: "idx_video_author_public", columns: []string{"author_id", "status", "published_at", "id"}},
		{table: "video", name: "idx_video_author_created", columns: []string{"author_id", "deleted_at", "created_at", "id"}},
		{table: "video", name: "idx_video_cleanup", columns: []string{"deleted_at"}},
		{table: "video", name: "idx_video_created", columns: []string{"status", "created_at"}},
		{table: "review_task", name: "uk_review_video_round", unique: true, columns: []string{"video_id", "review_round"}},
		{table: "review_task", name: "idx_review_due", columns: []string{"status", "next_attempt_at"}},
	}
	for _, exp := range expectations {
		assertIndex(t, gdb, cfg.Database, exp)
	}
}

// 输入：t，测试上下文；gdb，gorm 实例；schema，数据库名；table，表名。
// 输出：表存在时返回，否则失败结束测试。
// 功能：通过 information_schema 确认表已创建。
// 例如迁移后查 video 表计数应为 1。
func assertTableExists(t *testing.T, gdb *gorm.DB, schema, table string) {
	t.Helper()
	var count int
	err := gdb.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?", schema, table).Scan(&count).Error
	if err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}
	if count != 1 {
		t.Fatalf("table %q missing after migrate", table)
	}
}

// 输入：t，测试上下文；gdb，gorm 实例；schema，数据库名；table，表名。
// 输出：表字符集为 utf8mb4 时返回，否则失败结束测试。
// 功能：验证迁移设置的表默认字符集。
// 例如 video 表的排序规则应以 utf8mb4 开头。
func assertTableCharset(t *testing.T, gdb *gorm.DB, schema, table string) {
	t.Helper()
	var collation string
	err := gdb.Raw("SELECT table_collation FROM information_schema.tables WHERE table_schema = ? AND table_name = ?", schema, table).Scan(&collation).Error
	if err != nil {
		t.Fatalf("query collation of %s: %v", table, err)
	}
	if !strings.HasPrefix(collation, "utf8mb4") {
		t.Fatalf("table %s collation = %q, want utf8mb4 prefix", table, collation)
	}
}

// 输入：t，测试上下文；gdb，gorm 实例；schema，数据库名；exp，索引期望。
// 输出：索引存在、字段顺序与唯一性符合期望时返回，否则失败结束测试。
// 功能：通过 information_schema.statistics 核对索引定义。
// 例如唯一索引 uk_review_video_round 的非唯一标志应为 0。
func assertIndex(t *testing.T, gdb *gorm.DB, schema string, exp indexExpectation) {
	t.Helper()
	var rows []struct {
		ColumnName string
		NonUnique  int
	}
	err := gdb.Raw("SELECT column_name AS ColumnName, non_unique AS NonUnique FROM information_schema.statistics WHERE table_schema = ? AND table_name = ? AND index_name = ? ORDER BY seq_in_index", schema, exp.table, exp.name).Scan(&rows).Error
	if err != nil {
		t.Fatalf("query index %s.%s: %v", exp.table, exp.name, err)
	}
	if len(rows) != len(exp.columns) {
		t.Fatalf("index %s.%s has %d columns, want %d", exp.table, exp.name, len(rows), len(exp.columns))
	}
	for i, col := range exp.columns {
		if rows[i].ColumnName != col {
			t.Fatalf("index %s.%s col[%d]=%s, want %s", exp.table, exp.name, i, rows[i].ColumnName, col)
		}
	}
	if (rows[0].NonUnique == 0) != exp.unique {
		t.Fatalf("index %s.%s unique flag=%v, want %v", exp.table, exp.name, rows[0].NonUnique == 0, exp.unique)
	}
}
