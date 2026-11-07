package server

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// DependencyProbe 探测外部依赖连通性，任一依赖不可用时返回错误。
type DependencyProbe func(ctx context.Context) error

// defaultProbeTimeout 限制存活接口执行依赖检查的整体期限，避免探针阻塞请求。
const defaultProbeTimeout = 3 * time.Second

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
	return &HealthState{} // 无参数；返回初始未就绪的健康状态
}

// SetReady 将服务标记为就绪。
// 输入：无。
// 输出：原子更新状态，无返回值。
// 功能：表示服务已经完成初始化并开始接收请求。
// 例如服务开始运行时调用后，Ready 返回 true，/readyz 返回200。
func (state *HealthState) SetReady() {
	state.ready.Store(true) // 传入 true，原子地将就绪标记置为就绪
}

// SetNotReady 将服务标记为未就绪。
// 输入：无。
// 输出：原子更新状态，无返回值。
// 功能：把服务标记为暂时不能接收请求，供就绪接口通知外部流量管理组件。
// 例如退出时调用后，/readyz 返回503；停止监听由服务关闭流程负责。
func (state *HealthState) SetNotReady() {
	state.ready.Store(false) // 传入 false，原子地将就绪标记置为未就绪
}

// Ready 查询服务当前就绪状态。
// 输入：无。
// 输出：服务能够接收请求时返回 true。
// 功能：查询当前就绪标记，允许多个请求同时读取。
// 例如调用 SetReady 后返回 true，调用 SetNotReady 后返回 false。
func (state *HealthState) Ready() bool {
	return state.ready.Load() // 无参数；返回当前就绪标记
}

// healthResponse 是存活和就绪接口的响应体。
type healthResponse struct {
	Status string `json:"status"` // 对外状态文本，使用 ok 或 not_ready。
}

// liveHandler 保存存活接口可选执行的依赖探测函数。
type liveHandler struct {
	probe DependencyProbe // nil 表示不检查依赖，只报告进程存活。
}

// handle 返回进程存活状态，并在配置了依赖探测时校验依赖连通性。
// 输入：c，当前 Gin 请求上下文。
// 输出：进程与依赖均正常时写入200；依赖不可用或探测超时时写入503。
// 功能：让运行平台根据进程和关键依赖状态决定是否重启服务。
// 例如未配置探测时 /livez 恒返回200；MySQL 不可达时返回503并附带 unhealthy 状态。
func (handler liveHandler) handle(c *gin.Context) {
	if handler.probe != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), defaultProbeTimeout) // 传入请求上下文与探测期限；返回探测上下文、取消函数
		defer cancel()
		if err := handler.probe(ctx); err != nil { // 执行依赖探测；依赖不可用时进入该分支
			c.JSON(http.StatusServiceUnavailable, healthResponse{Status: "unhealthy"}) // 写入 503 依赖不可用响应
			return
		}
	}
	c.JSON(http.StatusOK, healthResponse{Status: "ok"}) // 传入状态码与响应体，写入存活响应
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
	if !handler.health.Ready() { // 查询当前就绪标记；未就绪时进入该分支
		c.JSON(http.StatusServiceUnavailable, healthResponse{Status: "not_ready"}) // 写入 503 未就绪响应
		return
	}
	c.JSON(http.StatusOK, healthResponse{Status: "ok"}) // 写入 200 就绪响应
}
