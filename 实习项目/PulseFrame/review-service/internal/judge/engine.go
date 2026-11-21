package judge

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Engine 执行两次采样、校验与汇总，不依赖具体模型服务。
type Engine struct {
	Inference Inference // OpenAI 兼容接口或测试用实现。
}

// maxFormatAttempts 是每批获取两张有效票的推理调用预算，包含首次采样。
const maxFormatAttempts = 2 * SampleCount

// 输入：已有有效票数和已经发出的采样次数。
// 输出：下一轮所需的并发调用数，零表示已经取得两票或用完预算。
// 功能：在同一批次内共享补采机会，例如已经调用两次且有一票时再取一票。
func nextRoundSize(valid, attempted int) int {
	return max(0, min(SampleCount-valid, maxFormatAttempts-attempted))
}

// Judge 对单个样本执行完整判定。
// 输入：ctx，调用上下文；sample，结构化输入；policy，不可变策略快照。
// 输出：返回两票汇总、用量或调用错误；补采用尽仍缺票时返回待人审结果。
// 功能：只补采无效的回答。例如首次四票有效时，保留这四票，继续获取第两票。
func (e Engine) Judge(ctx context.Context, sample Sample, policy Policy) (Decision, error) {
	if e.Inference == nil {
		return Decision{}, fmt.Errorf("inference client is required")
	}
	started := time.Now()
	outputs := make([]Output, 0, SampleCount)
	usage := Usage{}
	// 两次首采加两次共享补采，与此前每票最多调用两次的总上限一致。
	for attempted := 0; ; {
		count := nextRoundSize(len(outputs), attempted)
		if count == 0 {
			break
		}
		valid, cost, err := e.sampleRound(ctx, sample, policy, count)
		if err != nil {
			return Decision{}, err
		}
		outputs = append(outputs, valid...)
		usage.TokensIn += cost.TokensIn
		usage.TokensOut += cost.TokensOut
		attempted += count
	}
	usage.LatencyMS = time.Since(started).Milliseconds()
	failures := SampleCount - len(outputs)
	if len(outputs) < SampleCount {
		return Decision{SampleID: sample.SampleID, Version: policy.Version, NeedHumanReview: true,
			HumanReasons: []string{fmt.Sprintf("%d 次采样输出格式异常", failures)},
			Observations: []Observation{}, OtherLabels: []string{}, Evidence: []string{},
			FormatFailures: failures, TokensIn: usage.TokensIn, TokensOut: usage.TokensOut,
			LatencyMS: usage.LatencyMS, Samples: outputs}, nil
	}
	decision, err := ScoreOutputs(outputs, policy, failures)
	if err != nil {
		return Decision{}, err
	}
	decision.SampleID = sample.SampleID
	decision.TokensIn, decision.TokensOut, decision.LatencyMS = usage.TokensIn, usage.TokensOut, usage.LatencyMS
	return decision, nil
}

// 输入：调用上下文、视频样本、判定策略和本轮所需票数。
// 输出：有效回答及本轮总用量，推理调用失败时返回错误。
// 功能：并发获取缺少的票，例如缺两票就只发起两次推理。
func (e Engine) sampleRound(ctx context.Context, sample Sample, policy Policy, count int) ([]Output, Usage, error) {
	raw := make([]string, count)
	costs := make([]Usage, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Go(func() {
			raw[i], costs[i], errs[i] = e.Inference.Complete(ctx, sample, policy.SystemPrompt)
		})
	}
	wg.Wait()
	valid := make([]Output, 0, count)
	total := Usage{}
	for i, answer := range raw {
		if errs[i] != nil {
			return nil, total, errs[i]
		}
		total.TokensIn += costs[i].TokensIn
		total.TokensOut += costs[i].TokensOut
		output, err := ParseOutput(answer, policy.AllowedLabels)
		if err != nil {
			slog.WarnContext(ctx, "model output failed validation", "sample_id", sample.SampleID,
				"error", err, "raw", truncateRunes(answer, 300))
			continue
		}
		valid = append(valid, output)
	}
	return valid, total, nil
}

// MergeBatches 合并同一视频多个批次的判定：观察项和标签取并集，任一批次转人审则整体转人审。
// 输入：decisions，各批次的判定，批次之间的帧不重叠。
// 输出：整条视频的判定；token 和耗时累加。
// 功能：超过 16 帧分批送审后，按整条视频定级（流程设计附录 A，不丢帧）。
func MergeBatches(decisions []Decision) Decision {
	merged := Decision{Observations: []Observation{}, OtherLabels: []string{}, Evidence: []string{}, HumanReasons: []string{}}
	labels := map[string]bool{}
	for i, d := range decisions {
		if i == 0 {
			merged.SampleID, merged.Version = d.SampleID, d.Version
		}
		merged.Observations = append(merged.Observations, d.Observations...)
		for _, label := range d.OtherLabels {
			if !labels[label] {
				labels[label] = true
				merged.OtherLabels = append(merged.OtherLabels, label)
			}
		}
		merged.ViolenceDetected = merged.ViolenceDetected || d.ViolenceDetected
		merged.NeedHumanReview = merged.NeedHumanReview || d.NeedHumanReview
		merged.HumanReasons = append(merged.HumanReasons, d.HumanReasons...)
		merged.Evidence = append(merged.Evidence, d.Evidence...)
		merged.IsSwimwearProduct = merged.IsSwimwearProduct || d.IsSwimwearProduct
		merged.FormatFailures += d.FormatFailures
		merged.TokensIn += d.TokensIn
		merged.TokensOut += d.TokensOut
		merged.LatencyMS += d.LatencyMS
	}
	return merged
}

// truncateRunes 截断日志里的模型原始输出，按字符而不是字节截断，避免切坏中文。
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
