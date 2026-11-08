package metrics

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// sqlStartKey 用于在 gorm 会话上下文中保存回调开始时的时间。
type sqlStartKey struct{}

// sqlMetricsPlugin 统计 gorm 数据库操作并把结果发布到依赖指标。
type sqlMetricsPlugin struct {
	registry *Registry // 指标写入目标。
}

// Name 返回 gorm 插件名称。
// 输入：无。
// 输出：插件名称。
// 功能：标识该 gorm 插件。
// 例如 gorm 注册插件时使用该名称去重。
func (sqlMetricsPlugin) Name() string {
	return "metrics:sql"
}

// Initialize 为各类数据库操作注册统计回调。
// 输入：db，gorm 实例，用于注册各操作类型回调。
// 输出：注册成功返回 nil。
// 功能：在增删改查与原始 SQL 操作前后记录开始时间和统计结果。
// 例如执行一次 Create 时，统计"操作次数与耗时，出错时累加错误计数。
func (p sqlMetricsPlugin) Initialize(db *gorm.DB) error {
	// 每种操作在主回调之前记录开始时间，之后统一统计结果。
	db.Callback().Create().Before("gorm:create").Register("metrics:before_create", beforeSQLCallback) // 注册创建前回调，记录开始时间
	db.Callback().Create().After("gorm:create").Register("metrics:after_create", p.afterSQLCallback("create")) // 注册创建后回调，统计结果
	db.Callback().Update().Before("gorm:update").Register("metrics:before_update", beforeSQLCallback) // 注册更新前回调，记录开始时间
	db.Callback().Update().After("gorm:update").Register("metrics:after_update", p.afterSQLCallback("update")) // 注册更新后回调，统计结果
	db.Callback().Delete().Before("gorm:delete").Register("metrics:before_delete", beforeSQLCallback) // 注册删除前回调，记录开始时间
	db.Callback().Delete().After("gorm:delete").Register("metrics:after_delete", p.afterSQLCallback("delete")) // 注册删除后回调，统计结果
	db.Callback().Query().Before("gorm:query").Register("metrics:before_query", beforeSQLCallback) // 注册查询前回调，记录开始时间
	db.Callback().Query().After("gorm:query").Register("metrics:after_query", p.afterSQLCallback("query")) // 注册查询后回调，统计结果
	db.Callback().Raw().Before("gorm:raw").Register("metrics:before_raw", beforeSQLCallback) // 注册原始 SQL 前回调，记录开始时间
	db.Callback().Raw().After("gorm:raw").Register("metrics:after_raw", p.afterSQLCallback("raw")) // 注册原始 SQL 后回调，统计结果
	return nil
}

// AttachDBCallbacks 把 SQL 观测插件挂到 gorm 实例上。
// 输入：db，gorm 实例；registry，指标注册表。
// 输出：修改 gorm 实例，注册统计回调，无返回值。
// 功能：让依赖操作指标开始累积 gorm 各操作的次数、耗时与错误。
// 例如调用后执行任意查询，query 操作计数和耗时指标开始更新。
func AttachDBCallbacks(db *gorm.DB, registry *Registry) {
	db.Use(&sqlMetricsPlugin{registry: registry}) // 传入指标注册表；注册插件并挂载回调
}

// beforeSQLCallback 记录一次数据库操作开始的时间。
// 输入：tx，当前 gorm 会话。
// 输出：在会话上下文中写入开始时间，无返回值。
// 功能：为同一操作的后置回调提供耗时计算起点。
// 例如查询开始时写入当前时间，对应后置回调据此计算耗时。
func beforeSQLCallback(tx *gorm.DB) {
	tx.Statement.Context = context.WithValue(tx.Statement.Context, sqlStartKey{}, time.Now()) // 写入开始时间
}

// afterSQLCallback 统计一次数据库操作的次数、耗时与错误。
// 输入：operation，操作类型标签。
// 输出：更新依赖指标，无返回值。
// 功能：完成一次操作后累加计数与耗时，出错时追加错误分类。
// 例如 create 出错时，create 计数与耗时增加，错误计数也累加。
func (p sqlMetricsPlugin) afterSQLCallback(operation string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		start, ok := tx.Statement.Context.Value(sqlStartKey{}).(time.Time) // 读取操作开始时间
		if !ok {
			return
		}
		observeSQLOperation(p.registry, operation, time.Since(start), tx.Error) // 统计操作次数、耗时与错误
	}
}

// observeSQLOperation 把一次数据库操作结果发布到依赖指标。
// 输入：registry，指标注册表；operation，操作类型标签；duration，操作耗时；err，操作错误（可为 nil）。
// 输出：更新依赖操作指标，无返回值。
// 功能：累加操作次数与耗时，出错时按分类累加错误。
// 例如 create 成功时次数与耗时增加，失败且超时时另累积 dependency_timeout 错误。
func observeSQLOperation(registry *Registry, operation string, duration time.Duration, err error) {
	result := "ok" // 默认结果标签
	if err != nil {
		result = "error" // 出错时标记结果标签
	}
	registry.DependencyOperations.WithLabelValues("mysql", operation, result).Inc()                                 // 按依赖、操作与结果累加计数
	registry.DependencyOperationDuration.WithLabelValues("mysql", operation, result).Observe(duration.Seconds()) // 记录操作耗时分布
	if err != nil {
		registry.DependencyErrors.WithLabelValues("mysql", operation, classifySQLError(err)).Inc() // 按依赖、操作与错误类型累加错误
	}
}

// classifySQLError 把数据库操作错误归入依赖错误分类。
// 输入：err，数据库操作返回的错误。
// 输出：对应的错误分类字符串。
// 功能：区分依赖超时与一般内部错误。
// 例如错误包含 deadline exceeded 时返回 dependency_timeout，其余返回 internal_error。
func classifySQLError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorDependencyTimeout
	}
	return ErrorInternal
}
