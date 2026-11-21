package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"pulseframe-review/internal"
	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/platform/config"
	"pulseframe-review/internal/policy"
)

// ContentClient 封装对内容模块的调用，双向都携带服务间凭证（流程设计 2.5）。
type ContentClient struct {
	baseURL    string
	serviceKey string
	client     *http.Client
}

func NewContentClient(cfg config.ContentModule) *ContentClient {
	return &ContentClient{baseURL: cfg.BaseURL, serviceKey: cfg.ServiceKey, client: &http.Client{Timeout: cfg.Timeout}}
}

// post 发送 JSON 请求，非 200 视为失败（由调用方按退避重试）。
func (c *ContentClient) post(ctx context.Context, path string, payload any, accept ...int) error {
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Service-Key", c.serviceKey)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	for _, code := range accept {
		if response.StatusCode == code {
			return nil
		}
	}
	message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return fmt.Errorf("POST %s: status %d: %s", path, response.StatusCode, message)
}

// Callback 回调内容模块的审核结论：pass / limit / reject。
func (c *ContentClient) Callback(ctx context.Context, req CallbackRequest) error {
	return c.post(ctx, "/internal/review/callback", req)
}

// Offline 已发布视频改判下架。
func (c *ContentClient) Offline(ctx context.Context, videoID uint64, reason string) error {
	return c.post(ctx, fmt.Sprintf("/internal/videos/%d/offline", videoID), map[string]string{"reason": reason})
}

// Limit 限流或解除限流，seq 按视频单调递增，内容模块丢弃更旧的请求。
func (c *ContentClient) Limit(ctx context.Context, videoID uint64, limited bool, reason string, seq uint64) error {
	return c.post(ctx, fmt.Sprintf("/internal/videos/%d/limit", videoID),
		map[string]any{"limited": limited, "reason": reason, "seq": seq})
}

// Rereview 举报达到阈值时发起复审；409 表示已有复审在进行，视为成功。
func (c *ContentClient) Rereview(ctx context.Context, videoID uint64, trigger, detail string) error {
	return c.post(ctx, fmt.Sprintf("/internal/videos/%d/rereview", videoID),
		map[string]string{"trigger": trigger, "detail": detail}, http.StatusConflict)
}

// Service 封装审核业务：任务投递与查询、机审流程、人审结论和规则表切换。
type Service struct {
	repo    *Repository
	content *ContentClient
	worker  *WorkerClient
	engine  judge.Engine
	judging judge.Policy  // 识别版本对应的提示词与投票阈值
	cfg     config.Review // 运行参数

	mu      sync.RWMutex
	catalog *policy.Catalog // 启动时加载的所有规则表。
	base    *policy.Policy  // 临时规则到期后的默认表。
	current *policy.Policy  // 当前规则表；临时表到期后自动回到平时表
}

// 输入：审核依赖及运行配置，其中 PolicyFile 指向规则表文件。
// 输出：初始化后的服务或配置错误。
// 功能：启动时读取规则并选定活动策略，例如未指定表名时启用配置中的默认表。
func NewService(repo *Repository, content *ContentClient, worker *WorkerClient, engine judge.Engine,
	judging judge.Policy, cfg config.Review) (*Service, error) {
	catalog, err := policy.LoadCatalog(cfg.PolicyFile)
	if err != nil {
		return nil, err
	}
	base, err := catalog.Select(catalog.Default, time.Time{})
	if err != nil {
		return nil, err
	}
	current := base
	if cfg.PolicyTable != "" {
		current, err = catalog.Select(cfg.PolicyTable, cfg.PolicyExpireAt)
		if err != nil {
			return nil, err
		}
	}
	return &Service{repo: repo, content: content, worker: worker, engine: engine, judging: judging, cfg: cfg, catalog: catalog, base: base, current: current}, nil
}

// SubmitTask 处理任务投递，重复的 task_id 返回已有任务、不新建（流程设计 2.5）。
func (s *Service) SubmitTask(ctx context.Context, req SubmitRequest) (*TaskResponse, error) {
	task, created, err := s.repo.CreateTask(ctx, req, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "task submitted", "task_id", task.TaskID, "created", created, "video_id", task.VideoID)
	return toResponse(task), nil
}

// GetTask 查询任务：有结论返回结论，机审中、人审中都返回"审核中"，供内容模块对账。
func (s *Service) GetTask(ctx context.Context, taskID uint64) (*TaskResponse, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return toResponse(task), nil
}

func toResponse(task *internal.Task) *TaskResponse {
	if task.Status == internal.TaskCompleted {
		return &TaskResponse{TaskID: task.TaskID, Status: task.Result, Result: task.Result, Reason: task.Reason}
	}
	return &TaskResponse{TaskID: task.TaskID, Status: StatusInReview}
}

// HumanDecision 写入人审结论；带了修正后的观察结果时同时覆盖文件记录，之后换表、重新上传都以人工结果为准。
func (s *Service) HumanDecision(ctx context.Context, taskID uint64, req HumanDecisionRequest) (*TaskResponse, error) {
	switch req.Result {
	case "pass", "limit", "reject":
	default:
		return nil, fmt.Errorf("%w: result must be pass, limit or reject", ErrInvalidRequest)
	}
	reason := "人审：" + req.Reason
	task, err := s.repo.SaveHumanDecision(ctx, taskID, req.Result, reason, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if req.Observation != nil && task.FileHash != "" {
		corrected := *req.Observation
		corrected.Paired = nil // 人工修正已取代原始两票，即使客户端回传详情也以修正结果为准。
		data, _ := json.Marshal(corrected)
		if err := s.repo.CorrectFileRecord(ctx, task.FileHash, s.judging.Version, string(data), req.Reviewer); err != nil {
			slog.ErrorContext(ctx, "correct file record failed", "task_id", taskID, "error", err)
		}
	}
	slog.InfoContext(ctx, "human decision saved", "task_id", taskID, "result", req.Result, "reviewer", req.Reviewer)
	return toResponse(task), nil
}

// 输入：配置中的表名和可选到期时间。
// 输出：已启用的策略；名称或期限无效时返回错误。
// 功能：切换后续审核使用的表，例如临时表到期后再使用默认表。
func (s *Service) SwitchPolicy(table string, expireAt time.Time) (*policy.Policy, error) {
	next, err := s.catalog.Select(table, expireAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if next.IsExpired(time.Now()) {
		return nil, fmt.Errorf("%w: expire_at must be in the future", ErrInvalidRequest)
	}
	s.mu.Lock()
	s.current = next
	s.mu.Unlock()
	slog.Info("policy switched", "version", next.Version, "expire_at", next.ExpireAt)
	return next, nil
}

// activePolicy 返回当前生效的规则表：临时表过期时自动回到平时表。
func (s *Service) activePolicy(now time.Time) *policy.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current.IsExpired(now) {
		return s.base
	}
	return s.current
}

// StatusInReview 是机审中、人审中、故障重试中对外统一显示的状态。
const StatusInReview = "审核中"

var ErrInvalidRequest = errors.New("invalid request")

// SubmitRequest 是投递接口的请求，字段与内容模块的"审核对接"契约一致。
type SubmitRequest struct {
	TaskID      uint64 `json:"task_id"`
	Kind        string `json:"kind"`
	VideoID     uint64 `json:"video_id"`
	ReviewRound uint32 `json:"review_round"`
	MediaKey    string `json:"media_key"`
	CoverKey    string `json:"cover_key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Trigger     string `json:"trigger"`
	Detail      string `json:"detail"`
}

// TaskResponse 是投递和查询接口的响应：status 为 pass / limit / reject 或"审核中"。
type TaskResponse struct {
	TaskID uint64 `json:"task_id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// CallbackRequest 是回调内容模块的请求体。
type CallbackRequest struct {
	TaskID uint64 `json:"task_id"`
	Result string `json:"result"` // pass / limit / reject
	Reason string `json:"reason"`
}

// HumanDecisionRequest 是人审结论；observation 为修正后的观察结果，可省略。
type HumanDecisionRequest struct {
	Result      string       `json:"result"`
	Reason      string       `json:"reason"`
	Reviewer    string       `json:"reviewer"`
	Observation *Observation `json:"observation,omitempty"`
}
