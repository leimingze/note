package judge

import (
	"testing"
)

// 输入：无。
// 输出：合法JSON通过校验，否则测试失败。
// 功能：验证正常输出可以进入评分。
func TestParseOutputAcceptsValidResult(t *testing.T) {
	raw := `{"observations":[{"type":"body_part_closeup","timestamp":1.5}],"violence_detected":false,"other_labels":[],"need_human_review":false,"evidence":["frame_2"],"is_swimwear_product":false}`
	output, err := ParseOutput(raw, []string{"违法犯罪", "色情低俗"})
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Observations) != 1 || output.Observations[0].Type != "body_part_closeup" {
		t.Fatalf("unexpected output: %+v", output)
	}
}

// 输入：无。
// 输出：未知字段和非法标签均被拒绝。
// 功能：验证模型不能输出策略外类别。
func TestParseOutputRejectsInvalidResults(t *testing.T) {
	cases := []string{
		`{"observations":[],"violence_detected":false,"other_labels":["未知标签"],"need_human_review":false,"evidence":[],"is_swimwear_product":false}`,
		`{"observations":[],"violence_detected":false,"other_labels":[],"need_human_review":false,"evidence":[],"is_swimwear_product":false,"extra_field":"should fail"}`,
	}
	for _, raw := range cases {
		if _, err := ParseOutput(raw, []string{"色情低俗"}); err == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
}

// 输入：无。
// 输出：包在 ```json 代码块里的合法 JSON 通过校验，代码块外有多余文字仍被拒绝。
// 功能：Qwen 常用代码块包裹 JSON，只容忍这一种外层格式。
func TestParseOutputCodeFence(t *testing.T) {
	body := `{"observations":[],"violence_detected":true,"other_labels":[],"need_human_review":false,"evidence":["第 2 帧"],"is_swimwear_product":false}`
	if output, err := ParseOutput("```json\n"+body+"\n```", nil); err != nil || !output.ViolenceDetected {
		t.Fatalf("fenced JSON should parse: %v %+v", err, output)
	}
	if _, err := ParseOutput("结论如下：\n"+body, nil); err == nil {
		t.Fatal("text before JSON should be rejected")
	}
}
