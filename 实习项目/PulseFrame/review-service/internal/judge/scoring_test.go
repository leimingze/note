package judge

import "testing"

// 输入：无。
// 输出：多个批次的观察项合并，任一批次人审则整体人审。
// 功能：验证超过 16 帧分批送审后按整条视频汇总。
func TestMergeBatches(t *testing.T) {
	merged := MergeBatches([]Decision{
		{Observations: []Observation{{Type: "body_part_closeup", Timestamp: 1}}, OtherLabels: []string{"炒作卖货"}},
		{Observations: []Observation{{Type: "body_part_closeup", Timestamp: 9}}, OtherLabels: []string{"炒作卖货"},
			NeedHumanReview: true, HumanReasons: []string{"x"}},
	})
	if len(merged.Observations) != 2 || len(merged.OtherLabels) != 1 || !merged.NeedHumanReview {
		t.Fatalf("unexpected merge: %+v", merged)
	}
}
