package judge

import "testing"

// 输入：Go 测试上下文。
// 输出：有效票数或预算边界错误时报告失败。
// 功能：检查不同的格式错误出现顺序；例如首轮只缺一票且首次补采也失败时，仍能继续补采。
func TestFormatBudget(t *testing.T) {
	for _, tc := range []struct{ valid, attempted, next int }{
		{0, 0, 2}, {1, 2, 1}, {1, 3, 1}, {1, 4, 0}, {0, 2, 2}, {2, 2, 0},
	} {
		if got := nextRoundSize(tc.valid, tc.attempted); got != tc.next {
			t.Errorf("valid=%d attempted=%d next=%d want=%d", tc.valid, tc.attempted, got, tc.next)
		}
	}
}
