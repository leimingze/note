package judge

import "testing"

// 输入：包含有效和无效来源编号的初筛回答。
// 输出：错误放行、缺失字段或原文引用失效时报告失败。
// 功能：验证格式失败与无命中有明确区别，例如空对象无法放行。
func TestScreenValidation(t *testing.T) {
	refs := map[string]bool{"OCR1": true}
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"findings":[]}`, true}, {`{}`, false}, {`{"findings":null}`, false},
		{`{"findings":[{"category":"营销与交易","frame":1,"evidence":"出现站外收款指引","references":["OCR1"]}]}`, true},
		{`{"findings":[{"category":"营销与交易","frame":1,"evidence":"出现站外收款指引","references":["ASR1"]}]}`, false},
		{`{"findings":[{"category":"营销与交易","frame":2,"evidence":"超出范围","references":[]}]}`, false},
	} {
		_, err := ParseFindings(tc.raw, 1, refs)
		if (err == nil) != tc.valid {
			t.Errorf("%s: %v", tc.raw, err)
		}
	}
}
