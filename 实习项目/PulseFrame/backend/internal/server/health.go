package server

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// HealthState 保存服务是否已经准备接收请求。
type HealthState struct {
	ready atomic.Bool
}

// NewHealthState 创建初始未就绪的健康状态。
// 输入：无。
// 输出：可并发读写的健康状态。
// 功能：在服务启动和退出阶段向就绪探针暴露准确状态。
func NewHealthState() *HealthState {
	return &HealthState{}
}

// SetReady 将服务标记为就绪。
// 输入：无。
// 输出：原子更新状态，无返回值。
// 功能：表示服务已经完成初始化并开始接收请求。
func (state *HealthState) SetReady() {
	state.ready.Store(true)
}

// SetNotReady 将服务标记为未就绪。
// 输入：无。
// 输出：原子更新状态，无返回值。
// 功能：在启动前或关闭期间阻止流量继续进入服务。
func (state *HealthState) SetNotReady() {
	state.ready.Store(false)
}

// Ready 查询服务当前就绪状态。
// 输入：无。
// 输出：服务能够接收请求时返回 true。
// 功能：为就绪接口和生命周期测试提供并发安全的状态读取。
func (state *HealthState) Ready() bool {
	return state.ready.Load()
}

// healthResponse 是存活和就绪接口的响应体。
type healthResponse struct {
	Status string `json:"status"`
}

// handleLive 返回进程存活状态。
// 输入：c，当前 Gin 请求上下文。
// 输出：写入 200 和固定存活状态。
// 功能：让运行平台判断进程是否需要被重启。
func handleLive(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

// readyHandler 保存就绪接口依赖。
type readyHandler struct {
	health *HealthState
}

// handle 根据当前状态返回服务是否就绪。
// 输入：c，当前 Gin 请求上下文。
// 输出：就绪时返回 200，否则返回 503。
// 功能：让运行平台只把流量发送给完成初始化的实例。
func (handler readyHandler) handle(c *gin.Context) {
	// 初始化未完成或服务正在退出时返回 503，让负载均衡停止分流。
	if !handler.health.Ready() {
		c.JSON(http.StatusServiceUnavailable, healthResponse{Status: "not_ready"})
		return
	}
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}
