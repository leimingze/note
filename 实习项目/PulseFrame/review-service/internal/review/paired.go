package review

import (
	"fmt"

	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/policy"
)

// pairedEvidence 保存重新按规则表计算所需的两份独立证据。
type pairedEvidence struct {
	Batches    [][]judge.Output `json:"batches"`              // 按视频时间保存各批有效回答。
	Suspicious []FrameInfo      `json:"suspicious,omitempty"` // 可疑时刻及类型，用于关联主审证据。
	Check      []judge.Output   `json:"check,omitempty"`      // 单独复核的有效回答。
}

// 输入：各批判定、送入复核的帧及复核判定。
// 输出：可随观察记录保存的配对证据。
// 功能：保留每票的依据，例如切换严查表后重新比较两份处置。
func collectPair(batches []judge.Decision, suspicious []FrameInfo, check *judge.Decision) *pairedEvidence {
	p := &pairedEvidence{Suspicious: suspicious}
	for _, batch := range batches {
		p.Batches = append(p.Batches, batch.Samples)
	}
	if check != nil {
		p.Check = check.Samples
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
	if a.ActionWithoutInsertion != b.ActionWithoutInsertion {
		a.ActionWithoutInsertion = policy.ActionHuman
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
			local[j] = p.reviewVote(shared, d, table)
		}
		if local[0].Action != local[1].Action {
			conflicts = append(conflicts, fmt.Sprintf("第 %d 批处置分歧（%s / %s）", i+1, local[0].Action, local[1].Action))
		}
	}
	if len(p.Batches) == 0 || (len(p.Suspicious) > 0 && len(p.Check) != judge.SampleCount) {
		return incompletePair("缺少有效的主审或可疑帧复核回答")
	}
	a := p.reviewVote(shared, judge.MergeBatches(lanes[0]), table)
	b := p.reviewVote(shared, judge.MergeBatches(lanes[1]), table)
	v := compareVerdicts(a, b, "全片两票处置分歧")
	if len(conflicts) > 0 {
		v.Action = policy.ActionHuman
		v.HumanReasons = append(v.HumanReasons, conflicts...)
	}
	return v
}

// 输入：缺票位置。
// 输出：含原因的人审处置。
// 功能：记录补采用尽后的任务，例如可疑帧只有一份回答。
func incompletePair(reason string) Verdict {
	return Verdict{Action: policy.ActionHuman, ActionWithoutInsertion: policy.ActionHuman, Reasons: []string{reason}, HumanReasons: []string{reason}}
}

// 输入：共享信号、一份主审回答与规则表。
// 输出：同时考虑两份独立复核回答的处置。
// 功能：复核与主审独立采样，同一主审回答分别比较两份复核，不依赖返回顺序。
func (p *pairedEvidence) reviewVote(shared Observation, d judge.Decision, table *policy.Policy) Verdict {
	a := p.vote(shared, d, 0, table)
	if len(p.Check) != judge.SampleCount {
		return a
	}
	return compareVerdicts(a, p.vote(shared, d, 1, table), "可疑帧两票处置分歧")
}

// 输入：共享规则信号、单票观察、票序及规则表。
// 输出：该票对应的处置。
// 功能：将单票观察交给已有规则，例如仅比较可疑帧中的暴力和低俗证据。
func (p *pairedEvidence) vote(shared Observation, d judge.Decision, index int, table *policy.Policy) Verdict {
	shared.Observations, shared.OtherLabels, shared.ViolenceDetected = d.Observations, d.OtherLabels, d.ViolenceDetected
	shared.IsSwimwearProduct, shared.HasSuspectedMinor = d.IsSwimwearProduct, d.HasSuspectedMinor
	shared.NeedHumanReview, shared.HumanReasons = d.NeedHumanReview, d.HumanReasons
	shared.InsertionFindings = insertionFindings(d.Observations, p.Suspicious, nil)
	if index < len(p.Check) {
		check := p.Check[index]
		if check.ViolenceDetected {
			shared.InsertionFindings = append(shared.InsertionFindings, "复核检出暴力")
		}
		for _, o := range check.Observations {
			if lowVulgarTypes[o.Type] {
				shared.InsertionFindings = append(shared.InsertionFindings, fmt.Sprintf("%s t=%.1fs", o.Type, o.Timestamp))
			}
		}
	}
	return singleVerdict(shared, table)
}
