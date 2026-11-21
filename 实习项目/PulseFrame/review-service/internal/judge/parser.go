package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ObservationTypes 是低俗观察项的全部类型，取自审核标签与分级标准。
var ObservationTypes = []string{
	"body_part_closeup", "swimwear_off_context", "sexual_gesture", "sexual_sound", "sexual_act", "private_part",
}

// ParseOutput 解析模型输出的 JSON，验证格式和字段。
// 输入：raw，模型原始输出；allowedLabels，允许的标签列表。
// 输出：通过校验的 Output；格式错误返回 error。
// 功能：严格校验 JSON、字段、标签白名单、观察项类型白名单；只容忍外层的 ```json 代码块包裹，
// 例如 Qwen 常把 JSON 放进代码块里输出，内容本身仍按严格格式检查。
func ParseOutput(raw string, allowedLabels []string) (Output, error) {
	raw = stripCodeFence(strings.TrimSpace(raw))
	if raw == "" {
		return Output{}, errors.New("empty output")
	}
	// 缺失或 null 的字段不能按 Go 零值计为“未发现风险”，这类回答须进入格式补采。
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return Output{}, fmt.Errorf("json decode failed: %w", err)
	}
	for _, name := range []string{"observations", "violence_detected", "other_labels", "need_human_review", "evidence", "is_swimwear_product"} {
		if value, exists := values[name]; !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Output{}, fmt.Errorf("required output field missing or null: %s", name)
		}
	}

	var output Output
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil {
		return Output{}, fmt.Errorf("json decode failed: %w", err)
	}
	if decoder.More() {
		return Output{}, errors.New("trailing content after JSON")
	}

	allowedTypes := map[string]bool{}
	for _, t := range ObservationTypes {
		allowedTypes[t] = true
	}
	for _, obs := range output.Observations {
		if !allowedTypes[obs.Type] {
			return Output{}, fmt.Errorf("invalid observation type: %s", obs.Type)
		}
		if obs.Timestamp < 0 {
			return Output{}, fmt.Errorf("invalid timestamp: %f", obs.Timestamp)
		}
	}
	for _, label := range output.OtherLabels {
		if err := validateLabel(label, allowedLabels); err != nil {
			return Output{}, err
		}
	}
	return output, nil
}

// stripCodeFence 去掉包住整段输出的 ``` 或 ```json 代码块标记。
func stripCodeFence(raw string) string {
	if !strings.HasPrefix(raw, "```") || !strings.HasSuffix(raw, "```") || len(raw) < 6 {
		return raw
	}
	body := strings.TrimSuffix(raw, "```")
	if newline := strings.Index(body, "\n"); newline >= 0 {
		body = body[newline+1:]
	} else {
		body = strings.TrimPrefix(body, "```")
	}
	return strings.TrimSpace(body)
}

// validateLabel 检查标签是否在白名单中。
func validateLabel(label string, allowedLabels []string) error {
	for _, allowed := range allowedLabels {
		if label == allowed {
			return nil
		}
	}
	return fmt.Errorf("label not in whitelist: %s", label)
}
