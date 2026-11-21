package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WorkerClient 调用 Python worker 的挑帧和文字提取接口。worker 常驻、模型只加载一次。
type WorkerClient struct {
	baseURL string
	client  *http.Client
}

func NewWorkerClient(baseURL string) *WorkerClient {
	return &WorkerClient{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 5 * time.Minute}}
}

// FrameInfo 是挑中的一帧（字段与 worker/framesel/select_frames.py 的输出一致）。
type FrameInfo struct {
	Index     int     `json:"index"`
	Timestamp float64 `json:"timestamp"`
	FrameType string  `json:"frame_type"`
	Score     float64 `json:"score"`
	FramePath string  `json:"frame_path"`
	CropPath  string  `json:"crop_path,omitempty"`
}

// FrameResult 是挑帧结果：按时间顺序分好的批次和规避标记。
type FrameResult struct {
	Batches          [][]FrameInfo  `json:"batches"`
	EvasionSuspected bool           `json:"evasion_suspected"`
	Counts           map[string]int `json:"counts"`
	FPS              float64        `json:"fps"`
	Duration         float64        `json:"duration"`
	TotalFrames      int            `json:"total_frames"`
	ElapsedMS        int64          `json:"elapsed_ms"`
}

// TextHits 是文字规则（红线词、联系方式）的命中结果。
type TextHits struct {
	RedlineWords []string   `json:"redline_words"`
	Contacts     [][]string `json:"contacts"`
	ShouldReject bool       `json:"should_reject"`
}

// TranscriptLine 保存一条原文的时间位置。
type TranscriptLine struct {
	Start     float64 `json:"start"`         // 口播起点，单位为秒。
	End       float64 `json:"end,omitempty"` // 口播终点，单位为秒。
	Timestamp float64 `json:"timestamp"`     // 帧在视频中的位置，单位为秒。
	Text      string  `json:"text"`          // ASR 或 OCR 原文。
}

// TextResult 包含全片文字与带时间的逐条记录。
type TextResult struct {
	Audio       bool             `json:"audio"`
	ASRSegments []TranscriptLine `json:"asr_segments"`
	ASRText     string           `json:"asr_text"`
	OCR         []TranscriptLine `json:"ocr"`
	OCRText     string           `json:"ocr_text"`
	TextHits    TextHits         `json:"text_hits"`
	TimingsMS   map[string]int64 `json:"timings_ms"`
	ElapsedMS   int64            `json:"elapsed_ms"`
}

// SelectFrames 调用挑帧接口，选中的帧写到 outputDir。
func (c *WorkerClient) SelectFrames(ctx context.Context, videoPath, outputDir string) (*FrameResult, error) {
	var result FrameResult
	err := c.call(ctx, "/framesel", map[string]any{"video_path": videoPath, "output_dir": outputDir, "max_per_batch": 16}, &result)
	return &result, err
}

// MediaText 调用文字提取接口：整条音轨转写 + 挑中帧的 OCR + 标题与文字的规则检查。
func (c *WorkerClient) MediaText(ctx context.Context, videoPath, workDir, title string, frames []FrameInfo) (*TextResult, error) {
	items := make([]map[string]any, 0, len(frames))
	for _, frame := range frames {
		items = append(items, map[string]any{"path": frame.FramePath, "timestamp": frame.Timestamp})
	}
	var result TextResult
	err := c.call(ctx, "/media_text", map[string]any{"video_path": videoPath, "work_dir": workDir, "title": title, "frames": items}, &result)
	return &result, err
}

func (c *WorkerClient) call(ctx context.Context, path string, payload, out any) error {
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("worker %s: %w", path, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("worker %s: read body: %w", path, err)
	}
	if response.StatusCode != http.StatusOK {
		return &WorkerError{Path: path, Status: response.StatusCode, Body: truncate(string(data), 300)}
	}
	return json.Unmarshal(data, out)
}

// WorkerError 是 worker 返回的非 200 错误；400 表示输入本身有问题（如视频无法解码），重试也不会成功。
type WorkerError struct {
	Path   string
	Status int
	Body   string
}

func (e *WorkerError) Error() string {
	return fmt.Sprintf("worker %s: status %d: %s", e.Path, e.Status, e.Body)
}
