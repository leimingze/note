package review

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"pulseframe-review/internal/platform/config"
	"pulseframe-review/internal/platform/response"
)

// PublishedCandidate 由内容模块为已发布视频的一次抽查机会分配稳定任务编号。
type PublishedCandidate struct {
	SubmitRequest        // 内容模块的复审任务信息，用于接收回调。
	WindowSeconds int64  `json:"window_seconds"`     // 上游两个相邻窗口各自的秒数。
	Previous      *int64 `json:"previous_exposures"` // 前一完整窗口的曝光次数。
	Current       *int64 `json:"current_exposures"`  // 后一完整窗口的曝光次数。
}

// 输入：抽查配置和已发布视频候选。
// 输出：是否入选，曝光统计缺失或无效时返回错误。
// 功能：在符合条件的候选中按比例选取，例如重复投递同一任务得到相同结果。
func selectedPublished(cfg config.Sampling, c PublishedCandidate) (bool, error) {
	if cfg.Mode == "exposure" {
		if c.WindowSeconds != cfg.WindowSeconds || c.Previous == nil || c.Current == nil || *c.Previous < 0 || *c.Current < 0 {
			return false, fmt.Errorf("supply both exposure counts from consecutive windows of %d seconds", cfg.WindowSeconds)
		}
		if *c.Current-*c.Previous < cfg.MinimumIncrease || float64(*c.Current) < float64(*c.Previous)*cfg.GrowthFactor {
			return false, nil
		}
	}
	var key [16]byte
	binary.BigEndian.PutUint64(key[:8], c.VideoID)
	binary.BigEndian.PutUint64(key[8:], c.TaskID)
	digest := sha256.Sum256(key[:]) // 相同视频和机会编号总是分到同一位置。
	fraction := float64(binary.BigEndian.Uint32(digest[:4])) / (float64(math.MaxUint32) + 1)
	return fraction*100 < *cfg.Percent, nil
}

// 输入：通过服务鉴权的已发布候选，有独立于发布任务的 task_id。
// 输出：入选状态，入选时持久化复审任务供强模型领取。
// 功能：从发布后的事件建立独立审核，例如同一已发布候选重投仍复用原任务。
func (h *Handler) HandlePublished(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateService(r) {
		response.Error401(w)
		return
	}
	var c PublishedCandidate
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil || c.TaskID == 0 || c.VideoID == 0 || c.MediaKey == "" {
		response.Error400(w, "INVALID_REQUEST", "valid task_id, video_id and media_key required")
		return
	}
	cfg := h.service.cfg.Sampling
	selected, err := selectedPublished(cfg, c)
	if err != nil {
		response.Error400(w, "INVALID_REQUEST", err.Error())
		return
	}
	if !selected {
		response.JSON(w, http.StatusOK, map[string]any{"selected": false})
		return
	}
	c.Kind, c.Trigger = "rereview", "sampling_"+cfg.Mode
	task, err := h.service.SubmitTask(r.Context(), c.SubmitRequest) // 通过任务唯一键去重，沿用内容模块回调。
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"selected": true, "task": task})
}
