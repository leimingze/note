package judge

import (
	"context"
	"fmt"
)

// Engine 保存分批初筛与整片复核使用的两个模型。
type Engine struct {
	Strong    Inference // 用于整片复核的独立客户端。
	Inference Inference // OpenAI 兼容接口或测试用实现。
}

// 输入：上下文、离线样本及观察项提示词。
// 输出：一次独立复核的观察和资源用量，服务故障返回错误。
// 功能：让离线回放复用线上复核的格式校验，例如合格回答只调用一次。
func (e Engine) Judge(ctx context.Context, sample Sample, policy Policy) (Decision, error) {
	if e.Inference == nil {
		return Decision{}, fmt.Errorf("inference client is required")
	}
	req := StageRequest[Output]{Sample: sample, Prompt: policy.SystemPrompt, Parse: func(raw string) (Output, error) { return ParseOutput(raw, policy.AllowedLabels) }}
	r, err := RunStage(ctx, e.Inference, req)
	if err != nil {
		return Decision{}, err
	}
	d := FromOutput(r.Value)
	d.SampleID, d.Version = sample.SampleID, policy.Version
	d.TokensIn, d.TokensOut, d.LatencyMS = r.Usage.TokensIn, r.Usage.TokensOut, r.Usage.LatencyMS
	if r.FormatError != "" {
		d.NeedHumanReview = true
		d.FormatFailures = 1
		d.HumanReasons = []string{r.FormatError}
	} else {
		d.Samples = []Output{r.Value}
	}
	return d, nil
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
