package judge

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// ScreenPrompt 描述初筛需要识别的四类线索。
//
//go:embed screen_prompt.txt
var ScreenPrompt string

// categories 限定初筛可报告的类别。
var categories = []string{"色情与低俗", "暴力与危险行为", "言论与攻击", "营销与交易"}

// formatAttempts 是含首次在内的格式校验次数。
const formatAttempts = 2

// Finding 保存模型报告的位置和引用的原文编号。
type Finding struct {
	Category   string   `json:"category"`   // 四类线索之一。
	Frame      int      `json:"frame"`      // 本批从一开始的帧号。
	Evidence   string   `json:"evidence"`   // 可观察到的画面或台词。
	References []string `json:"references"` // 本批提供的 ASR/OCR 记录编号。
}

// 输入：模型原文、本批帧数和允许引用的原文编号。
// 输出：经过校验的线索，字段缺失、编号无来源时返回错误。
// 功能：只接受可定位的证据，例如第二批不能引用仅在第一批出现的文字。
func ParseFindings(raw string, frames int, refs map[string]bool) ([]Finding, error) {
	var response struct {
		Findings []Finding `json:"findings"`
	}
	decoder := json.NewDecoder(strings.NewReader(stripCodeFence(strings.TrimSpace(raw))))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF || response.Findings == nil {
		return nil, fmt.Errorf("expected one object with findings array")
	}
	for _, f := range response.Findings {
		if !slices.Contains(categories, f.Category) || f.Frame < 1 || f.Frame > frames || strings.TrimSpace(f.Evidence) == "" || f.References == nil {
			return nil, fmt.Errorf("invalid finding: %v", f)
		}
		for _, id := range f.References {
			if !refs[id] {
				return nil, fmt.Errorf("unknown transcript reference %q", id)
			}
		}
	}
	return response.Findings, nil
}

// StageRequest 包含一次模型审核所需的材料和回答校验方式。
type StageRequest[T any] struct {
	Sample Sample                  // 此次请求的图片和文字。
	Prompt string                  // 该阶段的系统提示词。
	Parse  func(string) (T, error) // 校验回答并返回结构化结果。
}

// StageResult 记录单个阶段的用量和最终格式错误。
type StageResult[T any] struct {
	Value       T      `json:"value"`                  // 有效回答。
	Usage       Usage  `json:"usage"`                  // 含格式重试的合计用量。
	FormatError string `json:"format_error,omitempty"` // 格式重试耗尽的原因。
}

// 输入：上下文、模型客户端和本阶段请求。
// 输出：有效回答或格式错误记录，连接故障返回 error 供队列重试。
// 功能：取得一份有效回答即返回，例如第一次回答合格时仅调用一次。
func RunStage[T any](ctx context.Context, client Inference, req StageRequest[T]) (StageResult[T], error) {
	var result StageResult[T]
	if client == nil {
		return result, fmt.Errorf("model client is not configured")
	}
	for range formatAttempts {
		raw, usage, err := client.Complete(ctx, req.Sample, req.Prompt) // 将本阶段材料提交给指定模型。
		if err != nil {
			return result, err
		}
		result.Usage.TokensIn += usage.TokensIn
		result.Usage.TokensOut += usage.TokensOut
		result.Usage.LatencyMS += usage.LatencyMS
		value, err := req.Parse(raw) // 校验完整回答，合格后结束本阶段。
		if err == nil {
			result.Value = value
			result.FormatError = ""
			return result, nil
		}
		result.FormatError = err.Error()
	}
	return result, nil
}
