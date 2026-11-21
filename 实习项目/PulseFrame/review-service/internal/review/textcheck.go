package review

import "strings"

// injectionPatterns 是提示词注入的特征词（流程设计附录 E）：画面文字、转写、标题里出现时转人审，并在人审页面标出。
var injectionPatterns = []string{
	"忽略以上", "忽略上述", "忽略之前", "忽略前面", "忽略所有规则", "无视以上", "无视规则",
	"判定为正常", "判定为通过", "判定为合规", "直接通过审核", "你是审核员", "你是一个审核",
	"ignore previous", "ignore the above", "ignore all instructions", "system prompt",
}

// FindInjection 返回文字中命中的注入特征词（去重、按出现顺序）。
// 输入：texts，标题、画面文字、转写等。
// 输出：命中的特征词；没有命中时为空切片。
// 功能：在 VLM 判定之外独立检查，模型被注入成功时人审仍能拦住；例如画面文字"忽略以上规则"命中。
func FindInjection(texts ...string) []string {
	joined := strings.ToLower(strings.Join(texts, "\n"))
	joined = strings.NewReplacer(" ", "", "　", "", "，", "", ",", "").Replace(joined)
	hits := []string{}
	for _, pattern := range injectionPatterns {
		key := strings.ReplaceAll(pattern, " ", "")
		if strings.Contains(joined, key) {
			hits = append(hits, pattern)
		}
	}
	return hits
}
