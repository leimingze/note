package review

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"pulseframe-review/internal"
	"pulseframe-review/internal/judge"
)

// funnelVersion 标识已完成两阶段审核的文件记录。
const funnelVersion = 3

// BatchScreen 保留批次、模型线索和可核对的文字来源。
type BatchScreen struct {
	Result  judge.StageResult[[]judge.Finding] `json:"result"`            // 本批初筛回答及用量。
	First   int                                `json:"first_frame"`       // 全片从零开始的首帧索引。
	Sources map[string]TranscriptLine          `json:"sources,omitempty"` // 引用编号对应的时间和原文。
}

// 输入：当前时间范围与原始文字。
// 输出：编号文字、对应原始记录及可引用编号。
// 功能：按半开区间选取材料，跨越两个批次的口播在两批均可见。
func batchText(text *TextResult, start, end float64) (judge.Sample, map[string]TranscriptLine, map[string]bool) {
	sources := map[string]TranscriptLine{}
	allowed := map[string]bool{}
	render := func(prefix string, rows []TranscriptLine) string {
		var lines []string
		for i, row := range rows {
			lo, hi := row.Start, row.End
			if prefix == "OCR" {
				lo, hi = row.Timestamp, row.Timestamp
			}
			if hi < start || lo >= end || row.Text == "" {
				continue
			}
			id := fmt.Sprintf("%s%d", prefix, i+1)
			sources[id], allowed[id] = row, true
			lines = append(lines, fmt.Sprintf("[%s %.2f–%.2fs] %s", id, lo, hi, row.Text))
		}
		return strings.Join(lines, "\n")
	}
	return judge.Sample{ASR: render("ASR", text.ASRSegments), OCR: render("OCR", text.OCR)}, sources, allowed
}

// 输入：任务及已完成的挑帧和文字记录。
// 输出：按批保存初筛证据，模型故障返回错误。
// 功能：按相邻批次的时间中点分配文字，例如结尾帧之后的口播归入最后一批。
func (s *Service) screenBatches(ctx context.Context, task *internal.Task, r *TaskResult) error {
	first, start := 0, 0.0
	for i, batch := range r.Frames.Batches {
		end := math.Inf(1)
		if i+1 < len(r.Frames.Batches) {
			end = (batch[len(batch)-1].Timestamp + r.Frames.Batches[i+1][0].Timestamp) / 2
		}
		sample, sources, allowed := batchText(r.Text, start, end)
		sample.Frames, sample.Title = toJudgeFrames(batch), task.Title
		req := judge.StageRequest[[]judge.Finding]{Sample: sample, Prompt: judge.ScreenPrompt, Parse: func(raw string) ([]judge.Finding, error) { return judge.ParseFindings(raw, len(batch), allowed) }}
		screened, err := judge.RunStage(ctx, s.engine.Inference, req) // 取得本批唯一有效回答。
		if err != nil {
			return fmt.Errorf("screen batch %d: %w", i+1, err)
		}
		r.Screens = append(r.Screens, BatchScreen{First: first, Result: screened, Sources: sources})
		if screened.FormatError != "" {
			r.Observation.InvalidAnswer = fmt.Sprintf("第 %d 批初筛格式无效：%s", i+1, screened.FormatError)
			return nil
		}
		first += len(batch)
		start = end
	}
	return nil
}

// 输入：全片原始帧和各批的可疑位置。
// 输出：按时间排序、去重的命中帧及左右相邻帧。
// 功能：跨批保留上下文，例如批末可疑帧也会带上下一批首帧。
func contextFrames(all []FrameInfo, screens []BatchScreen) []FrameInfo {
	chosen := make([]bool, len(all))
	for _, screen := range screens {
		for _, hit := range screen.Result.Value {
			center := screen.First + hit.Frame - 1
			for i := max(0, center-1); i <= min(len(all)-1, center+1); i++ {
				chosen[i] = true
			}
		}
	}
	var selected []FrameInfo
	for i, frame := range all {
		if chosen[i] {
			selected = append(selected, frame)
		}
	}
	return selected
}

// 输入：任务及完成预处理的视频。
// 输出：写入各批初筛、整片复核及可按规则表重算的观察记录。
// 功能：发布前合并所有命中，发布后抽中的视频直接用全片抽帧独立复核。
func (s *Service) runFunnel(ctx context.Context, task *internal.Task, r *TaskResult) error {
	var all []FrameInfo
	for _, batch := range r.Frames.Batches {
		if len(batch) == 0 {
			return fmt.Errorf("frame worker returned empty batch")
		}
		all = append(all, batch...)
	}
	if len(all) == 0 {
		return fmt.Errorf("frame worker returned no frames")
	}
	obs := &r.Observation
	*obs = Observation{FlowVersion: funnelVersion, Title: task.Title, TextHits: r.Text.TextHits, FrameCounts: r.Frames.Counts, Injection: FindInjection(task.Title, r.Text.OCRText, r.Text.ASRText)}
	frames := all
	if task.Kind != "rereview" {
		if err := s.screenBatches(ctx, task, r); err != nil {
			return err
		}
		if obs.InvalidAnswer != "" {
			return nil
		}
		frames = contextFrames(all, r.Screens)
		if len(frames) == 0 {
			return nil
		}
	}
	sample, _, _ := batchText(r.Text, 0, math.Inf(1))
	sample.Title, sample.Frames = task.Title, toJudgeFrames(frames)
	if len(r.Screens) > 0 {
		obs.Findings = screenHints(all, r.Screens)
		hints, err := json.Marshal(obs.Findings)
		if err != nil {
			return err
		}
		sample.ScreenEvidence = string(hints)
	}
	req := judge.StageRequest[judge.Output]{Sample: sample, Prompt: s.judging.SystemPrompt, Parse: func(raw string) (judge.Output, error) { return judge.ParseOutput(raw, s.judging.AllowedLabels) }}
	result, err := judge.RunStage(ctx, s.engine.Strong, req) // 每条视频仅执行一次整片复核。
	if err != nil {
		return fmt.Errorf("strong review: %w", err)
	}
	r.Strong = &result
	if result.FormatError != "" {
		obs.InvalidAnswer = "强模型回答格式无效：" + result.FormatError
		return nil
	}
	d := judge.FromOutput(result.Value)
	obs.Observations, obs.ViolenceDetected, obs.OtherLabels = d.Observations, d.ViolenceDetected, d.OtherLabels
	obs.NeedHumanReview, obs.HumanReasons, obs.Evidence = d.NeedHumanReview, d.HumanReasons, d.Evidence
	obs.IsSwimwearProduct = d.IsSwimwearProduct
	return nil
}

// LocatedFinding 关联全片时间与被引用的原始文字。
type LocatedFinding struct {
	Timestamp float64                   `json:"timestamp"` // 命中帧的全片时间，单位为秒。
	Category  string                    `json:"category"`  // 初筛风险大类。
	Evidence  string                    `json:"evidence"`  // 模型描述的画面或台词。
	Related   map[string]TranscriptLine `json:"related"`   // 按 ASR 或 OCR 编号定位的原话。
}

// 输入：原始抽帧及各批已校验的线索。
// 输出：以全片时间标识的线索和相应原话。
// 功能：将批内编号换为可跨批使用的时间，例如第二批首帧改用其视频秒数。
func screenHints(all []FrameInfo, screens []BatchScreen) []LocatedFinding {
	var hints []LocatedFinding
	for _, screen := range screens {
		for _, hit := range screen.Result.Value {
			refs := map[string]TranscriptLine{}
			for _, id := range hit.References {
				refs[id] = screen.Sources[id]
			}
			hints = append(hints, LocatedFinding{Timestamp: all[screen.First+hit.Frame-1].Timestamp, Category: hit.Category, Evidence: hit.Evidence, Related: refs})
		}
	}
	return hints
}
