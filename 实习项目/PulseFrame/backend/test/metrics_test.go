package test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"pulseframe/metrics"
	"pulseframe/middleware"
)

// fakePoolStats 提供固定的连接池统计，供采集器测试使用。
type fakePoolStats struct{}

// MySQLStats 返回固定 MySQL 统计。
func (fakePoolStats) MySQLStats() sql.DBStats {
	return sql.DBStats{OpenConnections: 3, InUse: 2, Idle: 1, WaitCount: 5, WaitDuration: 2 * time.Second}
}

// RedisPoolStats 返回固定 Redis 统计。
func (fakePoolStats) RedisPoolStats() *redis.PoolStats {
	return &redis.PoolStats{TotalConns: 4, IdleConns: 3, Timeouts: 1}
}

// metricGauge 从采集结果中取出指定指标名的 Gauge 值。
// 输入：registry，指标注册表；name，指标名称。
// 输出：Gauge 数值；未找到或不是 Gauge 时使测试失败。
// 功能：帮助测试断言连接池指标是否被真实发布。
func metricGauge(t *testing.T, registry *metrics.Registry, name string) float64 {
	t.Helper()
	gathered, err := registry.Prometheus().Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range gathered {
		if family.GetName() == name {
			return family.GetMetric()[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("metric not found: %s", name)
	return 0
}

// metricCounter 从采集结果中取出指定指标名的 Counter 值。
// 输入：registry，指标注册表；name，指标名称。
// 输出：Counter 数值；未找到或不是 Counter 时使测试失败。
// 功能：帮助测试断言请求计数指标是否累加。
func metricCounter(t *testing.T, registry *metrics.Registry, name string) float64 {
	t.Helper()
	gathered, err := registry.Prometheus().Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range gathered {
		if family.GetName() == name {
			return family.GetMetric()[0].GetCounter().GetValue()
		}
	}
	t.Fatalf("metric not found: %s", name)
	return 0
}

// TestRegistryPublishesMetrics 验证注册表能够采集到已注册指标。
// 输入：测试上下文。
// 输出：指标数量为零时测试失败。
// 功能：确认 Registry 至少暴露一个基座指标。
func TestRegistryPublishesMetrics(t *testing.T) {
	registry := metrics.New()
	gathered, err := registry.Prometheus().Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(gathered) == 0 {
		t.Fatal("expected at least one metric")
	}
}

// TestCollectorPublishesPoolStats 验证采集器把连接池统计写入指标。
// 输入：测试上下文。
// 输出：指标值与预期不符时测试失败。
// 功能：确认 MySQL 与 Redis 池指标随采集更新。
func TestCollectorPublishesPoolStats(t *testing.T) {
	registry := metrics.New()
	collector := metrics.NewCollector(registry, fakePoolStats{}, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go collector.Run(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()

	if open := metricGauge(t, registry, "pulseframe_mysql_open_connections"); open != 3 {
		t.Fatalf("unexpected mysql open connections: %v", open)
	}
	if total := metricGauge(t, registry, "pulseframe_redis_pool_total_connections"); total != 4 {
		t.Fatalf("unexpected redis total connections: %v", total)
	}
}

// TestMetricsMiddlewareRecordsRequest 验证请求指标中间件记录请求。
// 输入：测试上下文。
// 输出：请求计数指标未累加时测试失败。
// 功能：确认 HTTP 请求被指标中间件记录。
func TestMetricsMiddlewareRecordsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := metrics.New()
	router := gin.New()
	router.Use(middleware.NewMetrics(registry).Handle)
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/ok", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	// 找到指标计数器，Gather 返回的所有同名义 family 合并成一个 Counter。
	count := metricCounter(t, registry, "pulseframe_http_requests_total")
	if count < 1 {
		t.Fatalf("expected request counter to increment, got %v", count)
	}
}
