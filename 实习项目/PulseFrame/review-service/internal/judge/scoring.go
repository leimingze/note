package judge

import "fmt"

// SampleCount 是主审及可疑帧复核各批需要的有效回答数。
const SampleCount = 2

// 输入：一次有效采样的回答。
// 输出：保留该次观察、标签、证据及主动转审请求的判定。
// 功能：将一次回答单独交给规则表，例如暴力证据只影响报告它的那一票。
func FromOutput(out Output) Decision {
	d := Decision{Observations: out.Observations, ViolenceDetected: out.ViolenceDetected,
		OtherLabels: out.OtherLabels, IsSwimwearProduct: out.IsSwimwearProduct, HasSuspectedMinor: out.HasSuspectedMinor,
		NeedHumanReview: out.NeedHumanReview, Evidence: out.Evidence}
	if out.NeedHumanReview {
		d.HumanReasons = []string{"模型要求复核"}
	}
	return d
}

// 输入：同批两份有效回答、识别策略及缺票数。
// 输出：两份原始回答，首票观察供展示；缺票时返回错误。
// 功能：完整保存两次判断，处置时分别查当前规则表，例如两种标签都导致拦截时可直接拦截。
func ScoreOutputs(outputs []Output, policy Policy, failures int) (Decision, error) {
	if len(outputs) != SampleCount || failures != 0 {
		return Decision{}, fmt.Errorf("expected two valid samples, got %d valid and %d missing", len(outputs), failures)
	}
	decision := FromOutput(outputs[0])
	decision.Samples, decision.Version = outputs, policy.Version
	return decision, nil
}
