package review

import (
	"encoding/json"
	"testing"
	"time"

	"pulseframe-review/internal/grading"
	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/policy"
)

func TestDecide(t *testing.T) {
	strict := policy.StrictPolicy(time.Now().Add(time.Hour))
	closeups := []judge.Observation{{Type: "body_part_closeup", Timestamp: 2}, {Type: "body_part_closeup", Timestamp: 6}}
	cases := []struct {
		name   string
		obs    Observation
		table  *policy.Policy
		action policy.Action
		level  grading.VulgarityLevel
	}{
		{"无观察项放行", Observation{}, policy.DefaultPolicy(), policy.ActionPass, grading.L0},
		{"特写两次平时表限流", Observation{Observations: closeups}, policy.DefaultPolicy(), policy.ActionLimit, grading.L2},
		{"同一观察结果严查表拦截", Observation{Observations: closeups}, strict, policy.ActionReject, grading.L2},
		{"暴力拦截", Observation{ViolenceDetected: true}, policy.DefaultPolicy(), policy.ActionReject, grading.L0},
		{"文字规则命中拦截", Observation{TextHits: TextHits{Contacts: [][]string{{"微信", "abc123456"}}, ShouldReject: true}},
			policy.DefaultPolicy(), policy.ActionReject, grading.L0},

		{"注入特征词转人审", Observation{Injection: []string{"忽略以上"}}, policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
		{"模型没把握转人审", Observation{NeedHumanReview: true, HumanReasons: []string{"暴力只有 2/5 次检出"}},
			policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
		{"没把握但文字命中仍拦截（取最严）", Observation{NeedHumanReview: true,
			TextHits: TextHits{RedlineWords: []string{"冰毒"}, ShouldReject: true}}, policy.DefaultPolicy(), policy.ActionReject, grading.L0},
		{"有具体危害未成年人行为", Observation{OtherLabels: []string{"危害未成年人身心健康"}}, policy.DefaultPolicy(), policy.ActionReject, grading.L0},
		{"过期严查表回到平时表由调用方处理，这里直接按传入表", Observation{OtherLabels: []string{"不实信息"}},
			policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict := Decide(tc.obs, tc.table)
			if verdict.Action != tc.action || verdict.Level != tc.level {
				t.Fatalf("got action=%s level=%s reasons=%v human=%v, want %s %s",
					verdict.Action, verdict.Level, verdict.Reasons, verdict.HumanReasons, tc.action, tc.level)
			}
			if verdict.Action == policy.ActionHuman && len(verdict.HumanReasons) == 0 {
				t.Fatal("human review must carry a reason")
			}
		})
	}
}

// 输入：两份回答、时间分批与平时规则表。
// 输出：验证同处置自动处理、分歧转审、缺票和跨批定级。
// 功能：例如两份回答分别报告暴力和违法，但都需拦截时直接拦截。
func TestPairedDisposition(t *testing.T) {
	pass := judge.Output{}
	violent := judge.Output{ViolenceDetected: true}
	illegal := judge.Output{OtherLabels: []string{"违法犯罪"}}
	closeup := func(ts float64) judge.Output {
		return judge.Output{Observations: []judge.Observation{{Type: "body_part_closeup", Timestamp: ts}}}
	}
	tests := []struct {
		name string
		pair pairedEvidence
		want policy.Action
	}{
		{"same action", pairedEvidence{Batches: [][]judge.Output{{violent, illegal}}}, policy.ActionReject},
		{"pass against reject", pairedEvidence{Batches: [][]judge.Output{{pass, violent}}}, policy.ActionHuman},
		{"different evidence same pass", pairedEvidence{Batches: [][]judge.Output{{pass, closeup(2)}}}, policy.ActionPass},
		{"across chronological batches", pairedEvidence{Batches: [][]judge.Output{{closeup(2), closeup(2)}, {closeup(7), closeup(7)}}}, policy.ActionLimit},
		{"missing main answer", pairedEvidence{Batches: [][]judge.Output{{pass}}}, policy.ActionHuman},
	}
	for _, tc := range tests {
		got := Decide(Observation{Paired: &tc.pair}, policy.DefaultPolicy())
		if got.Action != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got.Action, tc.want)
		}
	}
}

// 输入：含旧版附加信号的观察记录。
// 输出：检查相同内容的两票按普通规则处置。
// 功能：历史记录即使含年龄标记、高频插帧及单帧复核结果，也只使用主审内容。
func TestIgnoreLegacyInsertionAndAge(t *testing.T) {
	var obs Observation
	raw := `{"evasion_suspected":true,"has_suspected_minor":true,"insertion_findings":["复核检出暴力"],"paired":{"check":[{"violence_detected":true}],"batches":[[{"has_suspected_minor":true,"observations":[{"type":"body_part_closeup","timestamp":3}]},{"observations":[{"type":"body_part_closeup","timestamp":3}]}]]}}`
	if err := json.Unmarshal([]byte(raw), &obs); err != nil {
		t.Fatal(err)
	}
	if v := Decide(obs, policy.DefaultPolicy()); v.Action != policy.ActionPass || v.Level != grading.L1 {
		t.Fatalf("unexpected verdict: %+v", v)
	}
}
