package review

import (
	"encoding/json"
	"testing"
	"time"

	"pulseframe-review/internal/grading"
	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/platform/config"
	"pulseframe-review/internal/policy"
)

func TestDecide(t *testing.T) {
	strict := testTable(t, "strict", time.Now().Add(time.Hour))
	closeups := []judge.Observation{{Type: "body_part_closeup", Timestamp: 2}, {Type: "body_part_closeup", Timestamp: 6}}
	cases := []struct {
		name   string
		obs    Observation
		table  *policy.Policy
		action policy.Action
		level  grading.VulgarityLevel
	}{
		{"无观察项放行", Observation{}, testTable(t, "default", time.Time{}), policy.ActionPass, grading.L0},
		{"特写两次平时表限流", Observation{Observations: closeups}, testTable(t, "default", time.Time{}), policy.ActionLimit, grading.L2},
		{"同一观察结果严查表拦截", Observation{Observations: closeups}, strict, policy.ActionReject, grading.L2},
		{"暴力拦截", Observation{ViolenceDetected: true}, testTable(t, "default", time.Time{}), policy.ActionReject, grading.L0},
		{"文字规则命中拦截", Observation{TextHits: TextHits{Contacts: [][]string{{"微信", "abc123456"}}, ShouldReject: true}},
			testTable(t, "default", time.Time{}), policy.ActionReject, grading.L0},

		{"注入特征词转人审", Observation{Injection: []string{"忽略以上"}}, testTable(t, "default", time.Time{}), policy.ActionHuman, grading.L0},
		{"模型没把握转人审", Observation{NeedHumanReview: true, HumanReasons: []string{"暴力只有 2/5 次检出"}},
			testTable(t, "default", time.Time{}), policy.ActionHuman, grading.L0},
		{"没把握但文字命中仍拦截（取最严）", Observation{NeedHumanReview: true,
			TextHits: TextHits{RedlineWords: []string{"冰毒"}, ShouldReject: true}}, testTable(t, "default", time.Time{}), policy.ActionReject, grading.L0},
		{"有具体危害未成年人行为", Observation{OtherLabels: []string{"危害未成年人身心健康"}}, testTable(t, "default", time.Time{}), policy.ActionReject, grading.L0},
		{"过期严查表回到平时表由调用方处理，这里直接按传入表", Observation{OtherLabels: []string{"不实信息"}},
			testTable(t, "default", time.Time{}), policy.ActionHuman, grading.L0},
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
		got := Decide(Observation{Paired: &tc.pair}, testTable(t, "default", time.Time{}))
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
	if v := Decide(obs, testTable(t, "default", time.Time{})); v.Action != policy.ActionPass || v.Level != grading.L1 {
		t.Fatalf("unexpected verdict: %+v", v)
	}
}

// 输入：测试上下文、已配置表名和到期时间。
// 输出：从随服务交付的 JSON 读取的规则表，配置错误时终止用例。
// 功能：使用正式规则检查判定，例如读取平时表并验证危险标签的处置。
func testTable(t *testing.T, name string, expiry time.Time) *policy.Policy {
	t.Helper()
	catalog, err := policy.LoadCatalog("../../policies/moderation.json")
	if err != nil {
		t.Fatal(err)
	}
	table, err := catalog.Select(name, expiry)
	if err != nil {
		t.Fatal(err)
	}
	return table
}

// 输入：正式配置文件与可控的到期时刻。
// 输出：验证启动、切换、非法切换和到期后的选择。
// 功能：启用临时策略后，在到期时刻恢复配置中的默认表。
func TestConfiguredPolicyLifecycle(t *testing.T) {
	cfg := config.Review{PolicyFile: "../../policies/moderation.json"}
	svc, err := NewService(nil, nil, nil, judge.Engine{}, judge.Policy{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	until := now.Add(time.Hour)
	if got := svc.activePolicy(now); got.Version != "default-v1" {
		t.Fatal(got.Version)
	}
	if _, err := svc.SwitchPolicy("strict", time.Time{}); err == nil {
		t.Fatal("missing expiry accepted")
	}
	if _, err := svc.SwitchPolicy("strict", until); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SwitchPolicy("unknown", until); err == nil {
		t.Fatal("unknown table accepted")
	}
	if got := svc.activePolicy(now); got.Version != "strict-v1" {
		t.Fatal(got.Version)
	}
	if got := svc.activePolicy(until); got.Version != "default-v1" {
		t.Fatal(got.Version)
	}
}
