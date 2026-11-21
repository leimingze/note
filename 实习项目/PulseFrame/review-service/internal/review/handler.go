package review

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"pulseframe-review/internal/platform/response"
)

type Handler struct {
	service    *Service
	serviceKey string
}

func NewHandler(service *Service, serviceKey string) *Handler {
	return &Handler{service: service, serviceKey: serviceKey}
}

// authenticateService 验证服务间凭证（常量时间比较）。
func (h *Handler) authenticateService(r *http.Request) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")), []byte(h.serviceKey)) == 1
}

// HandleSubmit 处理 POST /internal/review/tasks：按 task_id 去重，重复投递返回 200。
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateService(r) {
		response.Error401(w)
		return
	}
	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error400(w, "INVALID_REQUEST", "Invalid JSON")
		return
	}
	if req.TaskID == 0 || req.VideoID == 0 || req.MediaKey == "" || (req.Kind != "publish" && req.Kind != "rereview") {
		response.Error400(w, "INVALID_REQUEST", "task_id, video_id, media_key and kind (publish|rereview) are required")
		return
	}
	resp, err := h.service.SubmitTask(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to submit task", "error", err)
		response.Error500(w, "Failed to create task")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// HandleQuery 处理 GET /internal/review/tasks/{task_id}：返回结论或"审核中"。
func (h *Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateService(r) {
		response.Error401(w)
		return
	}
	taskID, err := strconv.ParseUint(r.PathValue("task_id"), 10, 64)
	if err != nil {
		response.Error400(w, "INVALID_REQUEST", "task_id must be an integer")
		return
	}
	resp, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// HandleDetail 处理 GET /internal/review/tasks/{task_id}/detail：人审页面用，返回内部状态、转人审原因和完整机审记录
// （选中的帧及时间戳、转写、画面文字、观察项、等级、规则表版本），审核员据此回看原视频。
func (h *Handler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateService(r) {
		response.Error401(w)
		return
	}
	taskID, err := strconv.ParseUint(r.PathValue("task_id"), 10, 64)
	if err != nil {
		response.Error400(w, "INVALID_REQUEST", "task_id must be an integer")
		return
	}
	task, err := h.service.repo.GetTask(r.Context(), taskID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var detail any
	if task.ResultJSON != "" {
		_ = json.Unmarshal([]byte(task.ResultJSON), &detail)
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"task_id": task.TaskID, "video_id": task.VideoID, "status": task.Status, "result": task.Result,
		"reason": task.Reason, "human_reason": task.HumanReason, "level": task.Level,
		"policy_version": task.PolicyVersion, "attempts": task.Attempts, "last_error": task.LastError,
		"callback_state": task.CallbackState, "callback_attempts": task.CallbackAttempts,
		"callback_error": task.CallbackError, "decided_at": task.DecidedAt, "callback_at": task.CallbackAt,
		"created_at": task.CreatedAt, "detail": detail,
	})
}

// HandleDecision 处理 POST /internal/review/tasks/{task_id}/decision：人审结论，只接受正在等人审的任务。
func (h *Handler) HandleDecision(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateService(r) {
		response.Error401(w)
		return
	}
	taskID, err := strconv.ParseUint(r.PathValue("task_id"), 10, 64)
	if err != nil {
		response.Error400(w, "INVALID_REQUEST", "task_id must be an integer")
		return
	}
	var req HumanDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error400(w, "INVALID_REQUEST", "Invalid JSON")
		return
	}
	resp, err := h.service.HumanDecision(r.Context(), taskID, req)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// HandlePolicy 处理 POST /internal/review/policy：整版切换规则表，body {"table": "default"|"strict", "expire_at": RFC3339}。
func (h *Handler) HandlePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateService(r) {
		response.Error401(w)
		return
	}
	var req struct {
		Table    string    `json:"table"`
		ExpireAt time.Time `json:"expire_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error400(w, "INVALID_REQUEST", "Invalid JSON")
		return
	}
	active, err := h.service.SwitchPolicy(req.Table, req.ExpireAt)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"version": active.Version, "expire_at": active.ExpireAt})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrTaskNotFound):
		response.Error404(w, "Task not found")
	case errors.Is(err, ErrTaskNotInHuman):
		response.Error409(w, "NOT_IN_HUMAN_REVIEW", err.Error())
	case errors.Is(err, ErrInvalidRequest):
		response.Error400(w, "INVALID_REQUEST", err.Error())
	default:
		slog.ErrorContext(r.Context(), "request failed", "error", err)
		response.Error500(w, "Internal error")
	}
}

// RegisterRoutes 注册审核模块的路由。
func RegisterRoutes(mux *http.ServeMux, handler *Handler) {
	mux.HandleFunc("POST /internal/review/tasks", handler.HandleSubmit)
	mux.HandleFunc("GET /internal/review/tasks/{task_id}", handler.HandleQuery)
	mux.HandleFunc("GET /internal/review/tasks/{task_id}/detail", handler.HandleDetail)
	mux.HandleFunc("POST /internal/review/tasks/{task_id}/decision", handler.HandleDecision)
	mux.HandleFunc("POST /internal/review/policy", handler.HandlePolicy)
}
