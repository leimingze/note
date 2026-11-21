package review

import (
	"fmt"

	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/policy"
)

// mainReviewVersion 标识当前主审提示词和观察字段，供旧机审缓存重新识别。
const mainReviewVersion = 2

// pairedEvidence 保存重新按规则表计算所需的两份独立证据。
type pairedEvidence struct {
	Version int              `json:"version"` // 主审流程版本。
	Batches [][]judge.Output `json:"batches"` // 按视频时间保存各批有效回答。
}

// 输入：各批主审判定。
// 输出：可随观察记录保存的配对证据。
// 功能：保留每票的依据，例如切换严查表后重新比较两份处置。
func collectPair(batches []judge.Decision) *pairedEvidence {
	p := &pairedEvidence{Version: mainReviewVersion}
	for _, batch := range batches {
		p.Batches = append(p.Batches, batch.Samples)
	}
	return p
}

// 输入：两份处置、可定位的分歧说明。
// 输出：共同处置或带原因的人审结果。
// 功能：比较处置动作，例如两票分别报告暴力和违法但均需拦截时直接拦截。
func compareVerdicts(a, b Verdict, reason string) Verdict {
	if a.Action != b.Action {
		a.HumanReasons = append(a.HumanReasons, fmt.Sprintf("%s（%s / %s）", reason, a.Action, b.Action))
		a.Action = policy.ActionHuman
	}
	return a
}

// 输入：共享观察、当前规则表。
// 输出：整条视频的处置，并给出引起分歧的批次。
// 功能：逐批比较两票，再合并跨批观察定级，例如同一回答序列的两次特写可累计成 L2。
func (p *pairedEvidence) decide(shared Observation, table *policy.Policy) Verdict {
	var lanes [judge.SampleCount][]judge.Decision
	var conflicts []string
	for i, batch := range p.Batches {
		if len(batch) != judge.SampleCount {
			return incompletePair(fmt.Sprintf("主审第 %d 批未取得两份有效回答", i+1))
		}
		var local [judge.SampleCount]Verdict
		for j, out := range batch {
			d := judge.FromOutput(out)
			lanes[j] = append(lanes[j], d)
			local[j] = reviewVote(shared, d, table)
		}
		if local[0].Action != local[1].Action {
			conflicts = append(conflicts, fmt.Sprintf("第 %d 批处置分歧（%s / %s）", i+1, local[0].Action, local[1].Action))
		}
	}
	if len(p.Batches) == 0 {
		return incompletePair("缺少有效的主审回答")
	}
	a := reviewVote(shared, judge.MergeBatches(lanes[0]), table)
	b := reviewVote(shared, judge.MergeBatches(lanes[1]), table)
	v := compareVerdicts(a, b, "全片两票处置分歧")
	if len(conflicts) > 0 {
		v.Action = policy.ActionHuman
		v.HumanReasons = append(v.HumanReasons, conflicts...)
	}
	return v
}

// 输入：缺票位置。
// 输出：含原因的人审处置。
// 功能：记录补采用尽后的任务，例如主审只有一份回答。
func incompletePair(reason string) Verdict {
	return Verdict{Action: policy.ActionHuman, Reasons: []string{reason}, HumanReasons: []string{reason}}
}

// 输入：共享规则信号、一份主审回答和当前规则表。
// 输出：该回答对应的处置及依据。
// 功能：按内容证据评价每票，例如检出暴力时查当前表确定处置。
func reviewVote(shared Observation, d judge.Decision, table *policy.Policy) Verdict {
	shared.Observations, shared.OtherLabels, shared.ViolenceDetected = d.Observations, d.OtherLabels, d.ViolenceDetected
	shared.IsSwimwearProduct = d.IsSwimwearProduct
	shared.NeedHumanReview, shared.HumanReasons = d.NeedHumanReview, d.HumanReasons
	return singleVerdict(shared, table)
}
