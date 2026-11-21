package review

import (
	"strings"
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
		{"疑似规避转人审", Observation{EvasionSuspected: true}, policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
		{"注入特征词转人审", Observation{Injection: []string{"忽略以上"}}, policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
		{"模型没把握转人审", Observation{NeedHumanReview: true, HumanReasons: []string{"暴力只有 2/5 次检出"}},
			policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
		{"没把握但文字命中仍拦截（取最严）", Observation{NeedHumanReview: true,
			TextHits: TextHits{RedlineWords: []string{"冰毒"}, ShouldReject: true}}, policy.DefaultPolicy(), policy.ActionReject, grading.L0},
		{"疑似未成年加特写拦截", Observation{Observations: closeups[:1], HasSuspectedMinor: true},
			policy.DefaultPolicy(), policy.ActionReject, grading.L3},
		{"过期严查表回到平时表由调用方处理，这里直接按传入表", Observation{OtherLabels: []string{"不实信息"}},
			policy.DefaultPolicy(), policy.ActionHuman, grading.L0},
		{"插入帧上的单条特写转人审（否则 L1 放行）", Observation{Observations: closeups[:1],
			InsertionFindings: []string{"body_part_closeup t=2.0s（短暂插入）"}}, policy.DefaultPolicy(), policy.ActionHuman, grading.L1},
		{"插帧发现不降低更严的处置", Observation{ViolenceDetected: true, InsertionFindings: []string{"复核认定暴力"}},
			policy.DefaultPolicy(), policy.ActionReject, grading.L0},
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
		{"missing check answer", pairedEvidence{Batches: [][]judge.Output{{pass, pass}}, Suspicious: []FrameInfo{{FrameType: "fade_overlay"}}, Check: []judge.Output{pass}}, policy.ActionHuman},
		{"one check detects violence", pairedEvidence{Batches: [][]judge.Output{{pass, pass}}, Suspicious: []FrameInfo{{FrameType: "brief_insertion"}}, Check: []judge.Output{pass, violent}}, policy.ActionHuman},
	}
	for _, tc := range tests {
		got := Decide(Observation{Paired: &tc.pair}, policy.DefaultPolicy())
		if got.Action != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got.Action, tc.want)
		}
	}
}

func TestDecideRecordsActionWithoutInsertion(t *testing.T) {
	obs := Observation{Observations: []judge.Observation{{Type: "body_part_closeup", Timestamp: 7.6}},
		InsertionFindings: []string{"body_part_closeup t=7.6s（短暂插入）"}}
	verdict := Decide(obs, policy.DefaultPolicy())
	if verdict.Action != policy.ActionHuman || verdict.ActionWithoutInsertion != policy.ActionPass {
		t.Fatalf("got action=%s without_insertion=%s", verdict.Action, verdict.ActionWithoutInsertion)
	}
	if len(verdict.HumanReasons) != 1 || verdict.HumanReasons[0] != "疑似插帧规避：body_part_closeup t=7.6s（短暂插入）" {
		t.Fatalf("unexpected human reasons: %v", verdict.HumanReasons)
	}
}

func TestInsertionFindings(t *testing.T) {
	suspicious := []FrameInfo{{Index: 227, Timestamp: 7.567, FrameType: "brief_insertion"},
		{Index: 449, Timestamp: 14.967, FrameType: "local_change"}}
	observations := []judge.Observation{{Type: "body_part_closeup", Timestamp: 7.6}, {Type: "body_part_closeup", Timestamp: 3.0}}
	violent := judge.Output{ViolenceDetected: true}
	gesture := judge.Output{Observations: []judge.Observation{{Type: "sexual_gesture", Timestamp: 15.0},
		{Type: "sexual_gesture", Timestamp: 7.6}}}
	contextOnly := judge.Output{OtherLabels: []string{"危险"}, HasSuspectedMinor: true, NeedHumanReview: true}
	cases := []struct {
		name  string
		check *judge.Decision
		want  []string
	}{
		{"只有整批判定：插入帧上的观察项计入，其他时间点不计", nil,
			[]string{"body_part_closeup t=7.6s（短暂插入）"}},
		{"复核按有效采样计票，同一采样同类只算一票", &judge.Decision{Samples: []judge.Output{violent, violent, violent, gesture, gesture}},
			[]string{"body_part_closeup t=7.6s（短暂插入）", "可疑帧复核 2/5 次检出 sexual_gesture", "可疑帧复核 3/5 次检出 暴力"}},
		{"只有 1 票不计", &judge.Decision{Samples: []judge.Output{violent, {}, {}, {}, {}}},
			[]string{"body_part_closeup t=7.6s（短暂插入）"}},
		{"标签、疑似未成年、模型自判人审不作为插帧证据", &judge.Decision{Samples: []judge.Output{contextOnly, contextOnly,
			contextOnly, contextOnly, contextOnly}}, []string{"body_part_closeup t=7.6s（短暂插入）"}},
		{"个别采样格式异常时用其余有效采样计票", &judge.Decision{FormatFailures: 1, Samples: []judge.Output{violent, violent, violent, violent}},
			[]string{"body_part_closeup t=7.6s（短暂插入）", "可疑帧复核 4/5 次检出 暴力"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := insertionFindings(observations, suspicious, tc.check)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSuspiciousFrames(t *testing.T) {
	frames := &FrameResult{Batches: [][]FrameInfo{
		{{Index: 1, FrameType: "brief_insertion"}, {Index: 2, FrameType: "timed"}, {Index: 3, FrameType: "fade_overlay"}},
		{{Index: 4, FrameType: "scene_change"}, {Index: 5, FrameType: "local_change"}},
	}}
	got := suspiciousFrames(frames)
	if len(got) != 3 || got[0].Index != 1 || got[1].Index != 3 || got[2].Index != 5 {
		t.Fatalf("unexpected suspicious frames: %+v", got)
	}
}
