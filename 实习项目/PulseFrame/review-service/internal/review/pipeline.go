package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pulseframe-review/internal"
	"pulseframe-review/internal/grading"
	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/policy"
)

// Observation 是一个文件在某个识别版本下的观察结果，存在 file_record 里供复用。
// 只记"看到了什么"：等级由程序按分级规则算，处置由当前规则表算，换表不用重跑模型（流程设计第 4 节）。
type Observation struct {
	// Paired 保存各批两份回答和复核证据，换表后仍能逐票计算处置。
	Paired            *pairedEvidence     `json:"paired,omitempty"`
	Observations      []judge.Observation `json:"observations"`        // 低俗观察项（类型 + 时间点）
	ViolenceDetected  bool                `json:"violence_detected"`   // 暴力检出
	OtherLabels       []string            `json:"other_labels"`        // 其他命中的标签
	IsSwimwearProduct bool                `json:"is_swimwear_product"` // 内衣泳装商品展示
	HasSuspectedMinor bool                `json:"has_suspected_minor"` // 疑似未成年人
	NeedHumanReview   bool                `json:"need_human_review"`   // 报告需要复核或回答缺失
	HumanReasons      []string            `json:"human_reasons"`
	Evidence          []string            `json:"evidence"`
	TextHits          TextHits            `json:"text_hits"`          // 画面文字、转写、标题的红线词与联系方式
	Injection         []string            `json:"injection"`          // 命中的提示词注入特征词
	EvasionSuspected  bool                `json:"evasion_suspected"`  // 挑帧检出疑似规避
	InsertionFindings []string            `json:"insertion_findings"` // 落在疑似插帧上的发现（插帧规则，见 insertionFindings）
	FrameCounts       map[string]int      `json:"frame_counts"`       // 各类挑帧事件数
}

// TaskResult 是一次审核的完整记录，存在 task.result_json，供人审回看和实验统计。
type TaskResult struct {
	TaskID         uint64           `json:"task_id"`
	FileHash       string           `json:"file_hash"`
	Reused         bool             `json:"reused"`          // 复用了文件审核记录，没有重新跑模型
	HumanCorrected bool             `json:"human_corrected"` // 复用的是人工修正过的观察结果
	Duration       float64          `json:"duration"`
	Frames         *FrameResult     `json:"frames,omitempty"`
	Text           *TextResult      `json:"text,omitempty"`
	Batches        []judge.Decision `json:"batches,omitempty"`
	InsertionCheck *judge.Decision  `json:"insertion_check,omitempty"` // 可疑帧单独复核的判定
	Observation    Observation      `json:"observation"`
	Level          string           `json:"level"`
	PolicyVersion  string           `json:"policy_version"`
	Action         string           `json:"action"`
	// ActionWithoutInsertion 是不计插帧规则时的处置，用于审计和评估插帧规则的效果。
	ActionWithoutInsertion string           `json:"action_without_insertion,omitempty"`
	Reasons                []string         `json:"reasons"`
	HumanReasons           []string         `json:"human_reasons"`
	TimingsMS              map[string]int64 `json:"timings_ms"`
}

// errRejectMedia 表示媒体本身不合规（超时长、无法解码），直接拒绝，不重试。
type errRejectMedia struct{ reason string }

func (e errRejectMedia) Error() string { return e.reason }

// ProcessTask 完整处理一个审核任务：读原文件 → 校验 → 挑帧 → 转写与画面文字 → VLM 两次采样判定 →
// 定级 → 查规则表 → 写结论。基础设施故障返回 error，由 worker 按退避重试；媒体不合规直接拒绝。
func (s *Service) ProcessTask(ctx context.Context, task *internal.Task) error {
	started := time.Now()
	timings := map[string]int64{}
	mark := func(step string, since time.Time) { timings[step] = time.Since(since).Milliseconds() }
	result := TaskResult{TaskID: task.TaskID, TimingsMS: timings}

	mediaPath, err := s.mediaPath(task.MediaKey)
	if err != nil {
		return s.finishReject(ctx, task, &result, "媒体校验失败："+err.Error())
	}
	step := time.Now()
	result.FileHash, err = hashFile(mediaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.finishReject(ctx, task, &result, "媒体校验失败：原文件不存在")
		}
		return fmt.Errorf("hash media: %w", err)
	}
	result.Duration, err = s.probeMedia(ctx, mediaPath)
	if err != nil {
		var reject errRejectMedia
		if errors.As(err, &reject) {
			return s.finishReject(ctx, task, &result, "媒体校验失败："+reject.reason)
		}
		return err
	}
	mark("probe", step)

	record, err := s.repo.FindFileRecord(ctx, result.FileHash, s.judging.Version)
	if err != nil {
		return fmt.Errorf("find file record: %w", err)
	}
	if record != nil && record.Done && json.Unmarshal([]byte(record.ObservationJSON), &result.Observation) == nil && (record.HumanCorrected || result.Observation.Paired != nil) {
		// 同一文件、同一识别版本已审过：复用观察结果，只按当前规则表重新算处置。
		result.Reused, result.HumanCorrected = true, record.HumanCorrected
	} else if err := s.observe(ctx, task, mediaPath, &result); err != nil {
		var reject errRejectMedia
		if errors.As(err, &reject) {
			return s.finishReject(ctx, task, &result, "媒体校验失败："+reject.reason)
		}
		return err
	}
	return s.finish(ctx, task, &result, started)
}

// observe 跑一遍识别：挑帧、文字提取、规则检查、VLM 判定，结果写入 result.Observation 并存文件记录。
func (s *Service) observe(ctx context.Context, task *internal.Task, mediaPath string, result *TaskResult) error {
	workDir := filepath.Join(s.cfg.WorkDir, fmt.Sprintf("%d-a%d", task.TaskID, task.Attempts))
	step := time.Now()
	frames, err := s.worker.SelectFrames(ctx, mediaPath, filepath.Join(workDir, "frames"))
	if err != nil {
		var werr *WorkerError
		if errors.As(err, &werr) && werr.Status == 400 {
			return errRejectMedia{"无法解码"}
		}
		return fmt.Errorf("select frames: %w", err)
	}
	result.TimingsMS["framesel"] = time.Since(step).Milliseconds()
	result.Frames = frames

	var all []FrameInfo
	for _, batch := range frames.Batches {
		all = append(all, batch...)
	}
	step = time.Now()
	text, err := s.worker.MediaText(ctx, mediaPath, filepath.Join(workDir, "text"), task.Title, all)
	if err != nil {
		return fmt.Errorf("media text: %w", err)
	}
	result.TimingsMS["media_text"] = time.Since(step).Milliseconds()
	result.Text = text

	step = time.Now()
	// 可疑帧单独复核与整批判定并发：只送短暂插入、局部变化、渐变叠加帧，不带文字，
	// 避免一闪而过的画面被整批帧和大段文字稀释（插帧诊断实验：单独送审时认出率明显更高）。
	suspicious := suspiciousFrames(frames)
	var check judge.Decision
	var checkErr error
	var wg sync.WaitGroup
	defer wg.Wait()
	if len(suspicious) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			check, checkErr = s.engine.Judge(ctx, judge.Sample{SampleID: fmt.Sprintf("%d-insert", task.TaskID),
				Frames: toJudgeFrames(suspicious)}, s.judging)
		}()
	}
	decisions := make([]judge.Decision, 0, len(frames.Batches))
	for index, batch := range frames.Batches {
		sample := judge.Sample{
			SampleID: fmt.Sprintf("%d-b%d", task.TaskID, index), Title: task.Title,
			OCR: text.OCRText, ASR: text.ASRText, Frames: toJudgeFrames(batch),
		}
		decision, err := s.engine.Judge(ctx, sample, s.judging)
		if err != nil {
			return fmt.Errorf("judge batch %d: %w", index, err)
		}
		decisions = append(decisions, decision)
	}
	wg.Wait()
	if checkErr != nil {
		return fmt.Errorf("judge suspicious frames: %w", checkErr)
	}
	result.TimingsMS["vlm"] = time.Since(step).Milliseconds()
	result.Batches = decisions
	var checkResult *judge.Decision
	if len(suspicious) > 0 {
		checkResult = &check
		result.InsertionCheck = checkResult
	}

	merged := judge.MergeBatches(decisions)
	result.Observation = Observation{
		Paired:       collectPair(decisions, suspicious, checkResult),
		Observations: merged.Observations, ViolenceDetected: merged.ViolenceDetected, OtherLabels: merged.OtherLabels,
		IsSwimwearProduct: merged.IsSwimwearProduct, HasSuspectedMinor: merged.HasSuspectedMinor,
		NeedHumanReview: merged.NeedHumanReview, HumanReasons: merged.HumanReasons, Evidence: merged.Evidence,
		TextHits: text.TextHits, Injection: FindInjection(task.Title, text.OCRText, text.ASRText),
		EvasionSuspected: frames.EvasionSuspected, FrameCounts: frames.Counts,
		InsertionFindings: insertionFindings(merged.Observations, suspicious, checkResult),
	}
	data, _ := json.Marshal(result.Observation)
	if err := s.repo.SaveFileRecord(ctx, result.FileHash, s.judging.Version, string(data)); err != nil {
		return fmt.Errorf("save file record: %w", err)
	}
	return nil
}

// finish 按当前规则表算出处置并写结论：转人审时不回调，其余进入待回调。
func (s *Service) finish(ctx context.Context, task *internal.Task, result *TaskResult, started time.Time) error {
	active := s.activePolicy(time.Now().UTC())
	verdict := Decide(result.Observation, active)
	result.Level, result.PolicyVersion, result.Action = string(verdict.Level), active.Version, string(verdict.Action)
	result.ActionWithoutInsertion = string(verdict.ActionWithoutInsertion)
	result.Reasons, result.HumanReasons = verdict.Reasons, verdict.HumanReasons
	result.TimingsMS["total"] = time.Since(started).Milliseconds()
	data, _ := json.Marshal(result)

	outcome := Outcome{Level: string(verdict.Level), PolicyVersion: active.Version, FileHash: result.FileHash,
		ResultJSON: string(data), Reason: strings.Join(verdict.Reasons, "；")}
	if verdict.Action == policy.ActionHuman {
		outcome.Status, outcome.HumanReason = internal.TaskHumanReview, strings.Join(verdict.HumanReasons, "；")
	} else {
		outcome.Status, outcome.Result = internal.TaskCompleted, string(verdict.Action)
	}
	slog.InfoContext(ctx, "task decided", "task_id", task.TaskID, "action", verdict.Action, "level", verdict.Level,
		"reused", result.Reused, "total_ms", result.TimingsMS["total"])
	return s.repo.SaveOutcome(ctx, task.ID, outcome, time.Now().UTC())
}

// finishReject 媒体不合规时直接拒绝发布并回调原因（流程设计附录 F 第一行）。
func (s *Service) finishReject(ctx context.Context, task *internal.Task, result *TaskResult, reason string) error {
	result.Action, result.Reasons = string(policy.ActionReject), []string{reason}
	data, _ := json.Marshal(result)
	return s.repo.SaveOutcome(ctx, task.ID, Outcome{Status: internal.TaskCompleted, Result: string(policy.ActionReject),
		Reason: reason, FileHash: result.FileHash, ResultJSON: string(data)}, time.Now().UTC())
}

// Verdict 是按规则表算出的处置。
type Verdict struct {
	Action                 policy.Action
	ActionWithoutInsertion policy.Action // 不计插帧规则时的处置
	Level                  grading.VulgarityLevel
	Reasons                []string // 处置依据，回调时作为 reason
	HumanReasons           []string // 转人审原因
}

// Decide 汇总各路结果取最严的处置（流程设计 2.1、2.2）：
// 文字规则命中 → 拦截；疑似未成年人出现在疑似低俗片段 → 拦截并转人审；
// 注入特征词、疑似规避、模型没把握 → 人审；其余按规则表处置。纯函数，换表预演也用它。
func Decide(obs Observation, active *policy.Policy) Verdict {
	if obs.Paired != nil {
		return obs.Paired.decide(obs, active)
	}
	return singleVerdict(obs, active)
}

// 输入：单份观察与当前规则表。
// 输出：该回答对应的处置、等级和依据。
// 功能：按同一规则评价每份回答，例如识别出暴力时按表拦截。
func singleVerdict(obs Observation, active *policy.Policy) Verdict {
	gradingObs := make([]grading.Observation, 0, len(obs.Observations))
	for _, o := range obs.Observations {
		gradingObs = append(gradingObs, grading.Observation{Type: o.Type, Timestamp: o.Timestamp})
	}
	level := grading.ComputeLevel(gradingObs, grading.Flags{
		IsSwimwearProduct: obs.IsSwimwearProduct, HasSuspectedMinor: obs.HasSuspectedMinor})
	verdict := Verdict{Level: level, Action: active.Evaluate(level, obs.ViolenceDetected, obs.OtherLabels, false)}

	if level != grading.L0 {
		verdict.Reasons = append(verdict.Reasons, "低俗 "+string(level))
	}
	if obs.ViolenceDetected {
		verdict.Reasons = append(verdict.Reasons, "暴力")
	}
	verdict.Reasons = append(verdict.Reasons, obs.OtherLabels...)

	escalate := func(action policy.Action) {
		if severity[action] > severity[verdict.Action] {
			verdict.Action = action
		}
	}
	if obs.TextHits.ShouldReject {
		escalate(policy.ActionReject)
		verdict.Reasons = append(verdict.Reasons, describeTextHits(obs.TextHits))
	}
	if obs.HasSuspectedMinor && len(obs.Observations) > 0 {
		escalate(policy.ActionReject)
		verdict.Reasons = append(verdict.Reasons, "疑似未成年人出现在疑似低俗片段")
		verdict.HumanReasons = append(verdict.HumanReasons, "疑似未成年人出现在疑似低俗片段，已拦截，需人审复核")
	}
	var human []string
	if len(obs.Injection) > 0 {
		human = append(human, "检出提示词注入特征词："+strings.Join(obs.Injection, "、"))
	}
	if obs.EvasionSuspected {
		human = append(human, "挑帧检出疑似规避（短暂插入、局部变化、渐变叠加合计超过阈值）")
	}
	if obs.NeedHumanReview {
		human = append(human, obs.HumanReasons...)
	}
	if verdict.Action == policy.ActionHuman {
		human = append(human, "规则表要求人审")
	}
	if len(human) > 0 {
		escalate(policy.ActionHuman)
		verdict.HumanReasons = append(verdict.HumanReasons, human...)
	}
	// 插帧规则放在最后，单独记下不计它时的处置，便于评估和审计。
	verdict.ActionWithoutInsertion = verdict.Action
	if len(obs.InsertionFindings) > 0 {
		escalate(policy.ActionHuman)
		verdict.HumanReasons = append(verdict.HumanReasons, "疑似插帧规避："+strings.Join(obs.InsertionFindings, "；"))
	}
	if len(verdict.Reasons) == 0 {
		verdict.Reasons = []string{"未发现违规"}
	}
	verdict.Reasons = append(verdict.Reasons, "规则表 "+active.Version)
	return verdict
}

// severity 是处置的严重程度：放行 < 限流 < 人审 < 拦截。
var severity = map[policy.Action]int{policy.ActionPass: 0, policy.ActionLimit: 1, policy.ActionHuman: 2, policy.ActionReject: 3}

func describeTextHits(hits TextHits) string {
	parts := []string{}
	if len(hits.RedlineWords) > 0 {
		parts = append(parts, "红线词 "+strings.Join(hits.RedlineWords, "、"))
	}
	for _, contact := range hits.Contacts {
		parts = append(parts, strings.Join(contact, " "))
	}
	return "文字规则命中：" + strings.Join(parts, "，")
}

// suspiciousTypes 是挑帧标出的疑似插帧类型（流程设计附录 A）及其中文名。
var suspiciousTypes = map[string]string{"brief_insertion": "短暂插入", "local_change": "局部变化", "fade_overlay": "渐变叠加"}

// suspiciousFrames 按时间顺序取出疑似插帧，最多 16 帧，与一次请求的帧数上限一致。
func suspiciousFrames(frames *FrameResult) []FrameInfo {
	var out []FrameInfo
	for _, batch := range frames.Batches {
		for _, f := range batch {
			if _, ok := suspiciousTypes[f.FrameType]; ok && len(out) < 16 {
				out = append(out, f)
			}
		}
	}
	return out
}

// insertionMinVotes 是可疑帧复核计为插帧证据的最低票数：与附录 G 一致，2 票及以上就不能否定。
const insertionMinVotes = 2

// lowVulgarTypes 是低俗观察项类型，复核只看画面上的低俗和暴力。
var lowVulgarTypes = map[string]bool{"body_part_closeup": true, "swimwear_off_context": true, "sexual_gesture": true,
	"sexual_sound": true, "sexual_act": true, "private_part": true}

// insertionFindings 汇总落在疑似插帧上的发现（插帧规则）：
//   - 整批判定认定的低俗观察项，时间点与某个疑似插帧相差不超过 0.06 秒（模型按帧前标注的 t 填写，保留 1 位小数）；
//   - 可疑帧单独复核中，某类低俗观察项或暴力在有效采样里得到 2 票及以上。复核只看画面违规：
//     其他标签、疑似未成年、模型自判人审都依赖上下文，单看几帧判断不了，不作为插帧证据；
//     个别采样格式异常时用其余有效采样计票，不丢掉整次复核。
//
// 只插一帧擦边画面时只有一条观察项，按分级是 L1、平时表放行；违规内容出现在插入的帧上本身就是规避的证据，
// 所以有发现就转人审（不直接拦截：复核看不到上下文）。
func insertionFindings(observations []judge.Observation, suspicious []FrameInfo, check *judge.Decision) []string {
	findings := []string{}
	for _, o := range observations {
		for _, f := range suspicious {
			if math.Abs(o.Timestamp-f.Timestamp) <= 0.06 {
				findings = append(findings, fmt.Sprintf("%s t=%.1fs（%s）", o.Type, o.Timestamp, suspiciousTypes[f.FrameType]))
				break
			}
		}
	}
	if check == nil {
		return findings
	}
	votes := map[string]int{}
	for _, sample := range check.Samples {
		seen := map[string]bool{}
		for _, o := range sample.Observations {
			if lowVulgarTypes[o.Type] && !seen[o.Type] {
				seen[o.Type] = true
				votes[o.Type]++
			}
		}
		if sample.ViolenceDetected {
			votes["暴力"]++
		}
	}
	items := make([]string, 0, len(votes))
	for item := range votes {
		items = append(items, item)
	}
	sort.Strings(items)
	total := len(check.Samples) + check.FormatFailures
	for _, item := range items {
		if votes[item] >= insertionMinVotes {
			findings = append(findings, fmt.Sprintf("可疑帧复核 %d/%d 次检出 %s", votes[item], total, item))
		}
	}
	return findings
}

func toJudgeFrames(batch []FrameInfo) []judge.Frame {
	frames := make([]judge.Frame, 0, len(batch))
	for _, f := range batch {
		frames = append(frames, judge.Frame{Path: f.FramePath, Timestamp: f.Timestamp, Type: f.FrameType, CropPath: f.CropPath})
	}
	return frames
}

// mediaPath 把 media_key 解析成 MEDIA_ROOT 下的路径，拒绝 "../" 越界。
func (s *Service) mediaPath(mediaKey string) (string, error) {
	if mediaKey == "" {
		return "", errors.New("media_key 为空")
	}
	clean := filepath.Clean("/" + mediaKey)
	return filepath.Join(s.cfg.MediaRoot, clean), nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return ComputeFileHash(file)
}

// probeMedia 兜底校验：只有一路视频流、能读出时长、时长不超过上限（内容模块确认上传时已校验过一次）。
func (s *Service) probeMedia(ctx context.Context, path string) (float64, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries",
		"format=duration:stream=codec_type", "-of", "json", path).Output()
	if err != nil {
		return 0, errRejectMedia{"无法解析媒体文件"}
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, errRejectMedia{"无法解析媒体文件"}
	}
	videos := 0
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			videos++
		}
	}
	duration, _ := strconv.ParseFloat(probe.Format.Duration, 64)
	switch {
	case videos != 1:
		return duration, errRejectMedia{fmt.Sprintf("视频流数量为 %d", videos)}
	case duration <= 0:
		return duration, errRejectMedia{"读不出时长"}
	case duration > s.cfg.MaxVideoDuration:
		return duration, errRejectMedia{fmt.Sprintf("时长 %.1f 秒超过 %.0f 秒上限", duration, s.cfg.MaxVideoDuration)}
	}
	return duration, nil
}
