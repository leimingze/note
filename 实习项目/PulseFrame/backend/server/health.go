package server

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// HealthState 保存服务是否已经准备接收请求。
type HealthState struct {
	ready atomic.Bool // 可并发读写的就绪标记，初始值为 false。
}

// NewHealthState 创建初始未就绪的健康状态。
// 输入：无。
// 输出：可并发读写的健康状态。
// 功能：在服务启动和退出阶段向就绪探针暴露准确状态。
// 例如刚创建状态对象时，Ready 返回 false，/readyz 返回503。
func NewHealthState() *HealthState {
	return &HealthState{}
}

// SetReady 将服务标记为就绪。
// 输入：无。
// 输出：原子更新状态，无返回值。
// 功能：表示服务已经完成初始化并开始接收请求。
// 例如服务开始运行时调用后，Ready 返回 true，/readyz 返回200。
func (state *HealthState) SetReady() {
	state.ready.Store(true)
}

// SetNotReady 将服务标记为未就绪。
// 输入：无。
// 输出：原子更新状态，无返回值。
// 功能：把服务标记为暂时不能接收请求，供就绪接口通知外部流量管理组件。
// 例如退出时调用后，/readyz 返回503；停止监听由服务关闭流程负责。
func (state *HealthState) SetNotReady() {
	state.ready.Store(false)
}

// Ready 查询服务当前就绪状态。
// 输入：无。
// 输出：服务能够接收请求时返回 true。
// 功能：查询当前就绪标记，允许多个请求同时读取。
// 例如调用 SetReady 后返回 true，调用 SetNotReady 后返回 false。
func (state *HealthState) Ready() bool {
	return state.ready.Load()
}

// healthResponse 是存活和就绪接口的响应体。
type healthResponse struct {
	Status string `json:"status"` // 对外状态文本，使用 ok 或 not_ready。
}

// handleLive 返回进程存活状态。
// 输入：c，当前 Gin 请求上下文。
// 输出：写入 200 和固定存活状态。
// 功能：让运行平台判断进程是否需要被重启。
// 例如 GET /livez 返回200和 {"status":"ok"}，此接口不检查数据库。
func handleLive(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

// readyHandler 保存就绪接口依赖。
type readyHandler struct {
	health *HealthState // 与服务生命周期共享的就绪标记。
}

// handle 根据当前状态返回服务是否就绪。
// 输入：c，当前 Gin 请求上下文。
// 输出：就绪时返回 200，否则返回 503。
// 功能：让运行平台只把流量发送给完成初始化的实例。
// 例如就绪标记为 false 时返回503和 {"status":"not_ready"}，为 true 时返回200。
func (handler readyHandler) handle(c *gin.Context) {
	// 初始化未完成或服务正在退出时返回 503，让负载均衡停止分流。
	if !handler.health.Ready() {
		c.JSON(http.StatusServiceUnavailable, healthResponse{Status: "not_ready"})
		return
	}
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}
