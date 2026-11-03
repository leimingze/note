//go:build mysqlintegration

package test

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"pulseframe/config"
	"pulseframe/infra"
)

const (
	poolTestTimeout       = 2 * time.Second        // 查询及读超时测试的外层上下文期限。
	poolWaitTimeout       = 50 * time.Millisecond  // 等待连接、查询取消和驱动读超时测试使用的短期限。
	serverSleepSeconds    = 1                      // SQL SLEEP 的等待秒数，须长于被验证的短期限。
	connectionRecycleTime = 50 * time.Millisecond  // 回收测试中为连接设置的存活期限。
	connectionRecycleWait = 100 * time.Millisecond // 再次查询前的等待时间，确保超过连接存活期限。
)

// 输入：t，测试上下文，进程环境需配置独立测试数据库。
// 输出：完整 MySQL 配置；缺失配置时测试失败。
// 功能：让集成测试连接真实数据库，不跳过缺失依赖。
func integrationMySQLConfig(t *testing.T) config.MySQL {
	t.Helper()
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.MySQL
}

// 输入：t，测试上下文；cfg，测试数据库配置。
// 输出：可查询的连接池，并登记测试结束时的关闭操作。
// 功能：统一真实连接创建和错误检查。
func openMySQL(t *testing.T, cfg config.MySQL) *sql.DB {
	t.Helper()
	db, err := infra.NewMySQL(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// 即使后续断言提前结束测试，也关闭已建立的真实数据库连接。
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

// 输入：t，测试上下文。
// 输出：连接未复用、数据库选择错误或关闭后仍可查询时测试失败。
// 功能：验证启动检查、连接复用和关闭的可观察行为。
func TestMySQLReuseAndClose(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	db := openMySQL(t, cfg)
	ctx, cancel := context.WithTimeout(t.Context(), poolTestTimeout)
	defer cancel()
	var first, second uint64
	var database string
	if err := db.QueryRowContext(ctx, "SELECT CONNECTION_ID(), DATABASE()").Scan(&first, &database); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&second); err != nil {
		t.Fatal(err)
	}
	if first != second || database != cfg.Database {
		t.Fatal("connection was not reused or database selection changed")
	}
	if db.Stats().MaxOpenConnections != cfg.MaxOpenConns {
		t.Fatal("configured connection limit was not applied")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(ctx); err == nil || db.Stats().OpenConnections != 0 {
		t.Fatal("pool still usable after close")
	}
}

// 输入：t，测试上下文。
// 输出：满载时未遵守上限和取消期限，或归还连接后不能恢复时测试失败。
// 功能：验证有界连接池的等待取消与连接归还行为。
func TestMySQLPoolWaitCancellation(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	// 保持唯一连接被借用，让后续获取操作必然进入等待状态。
	cfg.MaxOpenConns, cfg.MaxIdleConns = 1, 1
	db := openMySQL(t, cfg)
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(t.Context(), poolWaitTimeout)
	defer cancel()
	_, err = db.Conn(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected pool wait deadline, got %v", err)
	}
	stats := db.Stats()
	if stats.InUse != 1 || stats.OpenConnections != 1 || stats.WaitCount == 0 {
		t.Fatalf("unexpected saturated pool statistics: %+v", stats)
	}
	// 归还连接后再次访问，确认等待超时只取消当前获取操作，连接池仍可使用。
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// 输入：t，测试上下文。
// 输出：禁用空闲连接后仍保留连接时测试失败。
// 功能：验证最大空闲连接数配置作用于真实连接。
func TestMySQLNoIdleConnections(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	cfg.MaxIdleConns = 0
	db := openMySQL(t, cfg)
	if stats := db.Stats(); stats.OpenConnections != 0 || stats.MaxIdleClosed == 0 {
		t.Fatalf("idle connections were retained: %+v", stats)
	}
}

// 输入：t，测试上下文。
// 输出：正在运行的查询未响应取消或连接池不能恢复时测试失败。
// 功能：验证请求上下文能够中止真实 SQL 查询并释放连接。
func TestMySQLQueryCancellation(t *testing.T) {
	// SQL 等待时间长于上下文期限，用真实慢查询验证取消能够传递到驱动。
	db := openMySQL(t, integrationMySQLConfig(t))
	ctx, cancel := context.WithTimeout(t.Context(), poolWaitTimeout)
	defer cancel()
	_, err := db.ExecContext(ctx, "SELECT SLEEP(?)", serverSleepSeconds)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("query did not respect cancellation: %v", err)
	}
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// 输入：t，测试上下文。
// 输出：读取超时配置未中止慢查询时测试失败。
// 功能：通过真实 SQL 等待验证驱动的读超时配置。
func TestMySQLReadTimeout(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	cfg.ReadTimeout = poolWaitTimeout
	db := openMySQL(t, cfg)
	ctx, cancel := context.WithTimeout(t.Context(), poolTestTimeout)
	defer cancel()
	started := time.Now()
	_, err := db.ExecContext(ctx, "SELECT SLEEP(?)", serverSleepSeconds)
	// 上下文仍有效且 SQL 尚未自然结束，才能确认失败来自驱动的读取超时。
	if err == nil || ctx.Err() != nil || time.Since(started) >= time.Duration(serverSleepSeconds)*time.Second {
		t.Fatalf("driver read timeout was not enforced: %v", err)
	}
}

// 输入：t，测试上下文。
// 输出：已取消上下文或不存在的数据库未导致初始化失败时测试失败。
// 功能：确认初始化不会返回尚未连通的连接池。
func TestMySQLStartupFailures(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	db, err := infra.NewMySQL(ctx, cfg)
	if db != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled startup returned pool=%v error=%v", db != nil, err)
	}
	// 保留有效连接配置，仅缩短启动检查期限以验证该期限能独立生效。
	shortDeadline := cfg
	shortDeadline.StartupTimeout = time.Nanosecond
	db, err = infra.NewMySQL(t.Context(), shortDeadline)
	if db != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startup deadline was not enforced: %v", err)
	}
	cfg.Database += "_missing"
	db, err = infra.NewMySQL(t.Context(), cfg)
	if db != nil || err == nil {
		t.Fatal("startup succeeded with a missing database")
	}
}

// 输入：t，测试上下文。
// 输出：连接拒绝未作为初始化错误返回时测试失败。
// 功能：使用已关闭的本机端口验证真实连接错误传播。
func TestMySQLConnectionRefused(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Address = listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := infra.NewMySQL(t.Context(), cfg)
	if db != nil || err == nil {
		t.Fatal("startup succeeded against a closed port")
	}
}

// 输入：t，测试上下文。
// 输出：超过存活期限的连接仍被复用时测试失败。
// 功能：通过真实连接编号变化验证连接回收参数已生效。
func TestMySQLConnectionLifetime(t *testing.T) {
	cfg := integrationMySQLConfig(t)
	cfg.ConnMaxLifetime = connectionRecycleTime
	db := openMySQL(t, cfg)
	var first, second uint64
	if err := db.QueryRowContext(t.Context(), "SELECT CONNECTION_ID()").Scan(&first); err != nil {
		t.Fatal(err)
	}
	// 等到连接超过存活期限后再次借用，通过连接编号变化检查回收行为。
	select {
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	case <-time.After(connectionRecycleWait):
	}
	if err := db.QueryRowContext(t.Context(), "SELECT CONNECTION_ID()").Scan(&second); err != nil {
		t.Fatal(err)
	}
	if first == second || db.Stats().MaxLifetimeClosed == 0 {
		t.Fatal("expired connection was not recycled")
	}
}
