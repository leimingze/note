package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestObserveSQLOperation 验证数据库操作统计正确累加并区分错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：次数或错误分类不符合预期时使测试失败。
// 功能：确保成功操作只计数，失败操作计入对应错误分类。
// 例如 create 成功一次，计数为1；再发生超时错误，错误计数累计 dependency_timeout。
func TestObserveSQLOperation(t *testing.T) {
	registry := New() // 创建独立指标注册表
	observeSQLOperation(registry, "create", 10*time.Millisecond, nil) // 统计一次成功的 create 操作
	if got := testutil.ToFloat64(registry.DependencyOperations.WithLabelValues("mysql", "create", "ok")); got != 1 {
		t.Fatalf("create operations = %v, want 1", got)
	}
	observeSQLOperation(registry, "create", 20*time.Millisecond, context.DeadlineExceeded) // 统计一次超时的 create 操作
	if got := testutil.ToFloat64(registry.DependencyOperations.WithLabelValues("mysql", "create", "error")); got != 1 {
		t.Fatalf("create error operations = %v, want 1", got)
	}
	if got := testutil.ToFloat64(registry.DependencyErrors.WithLabelValues("mysql", "create", ErrorDependencyTimeout)); got != 1 {
		t.Fatalf("dependency timeout errors = %v, want 1", got)
	}
	// 非超时错误应归入内部错误分类。
	observeSQLOperation(registry, "query", 5*time.Millisecond, errors.New("sql failure")) // 统计一次普通失败的查询操作
	if got := testutil.ToFloat64(registry.DependencyOperations.WithLabelValues("mysql", "query", "error")); got != 1 {
		t.Fatalf("query error operations = %v, want 1", got)
	}
	if got := testutil.ToFloat64(registry.DependencyErrors.WithLabelValues("mysql", "query", ErrorInternal)); got != 1 {
		t.Fatalf("internal errors = %v, want 1", got)
	}
}

// TestClassifySQLError 验证数据库错误分类映射。
// 输入：Go 测试框架提供的测试上下文。
// 输出：分类结果不符合预期时使测试失败。
// 功能：把 deadline 超时归为依赖超时，其余归为内部错误。
// 例如传入 context.DeadlineExceeded 返回 dependency_timeout。
func TestClassifySQLError(t *testing.T) {
	if got := classifySQLError(context.DeadlineExceeded); got != ErrorDependencyTimeout {
		t.Fatalf("classify deadline = %q, want %q", got, ErrorDependencyTimeout)
	}
	if got := classifySQLError(errors.New("boom")); got != ErrorInternal {
		t.Fatalf("classify generic = %q, want %q", got, ErrorInternal)
	}
}
