package judge

import "context"

// Frame 是送审的一帧：图片路径、在原视频中的时间和挑帧类型；局部变化帧另附变化区域的放大图。
type Frame struct {
	Path      string  // 帧图片路径
	Timestamp float64 // 在原视频中的时间（秒）；KuaiMod 截图没有时间时传负数
	Type      string  // brief_insertion / local_change / fade_overlay / scene_change / timed，可为空
	CropPath  string  // 局部变化的放大图，可为空
}

// Sample 表示一次审核判定使用的结构化输入（一个批次）。
type Sample struct {
	SampleID         string  // 回放样本编号；在线任务用 task_id 加批次号。
	Title            string  // 视频标题。
	OCR              string  // 画面文字，按时间逐行。
	ASR              string  // 语音转写，带起止时间逐行。
	SuspiciousSounds string  // 可疑声音描述，如 "0:05-0:08 疑似呻吟 (0.86)"；音频分类只做设计，当前为空。
	Frames           []Frame // 关键帧，每批最多 16 帧，按时间排序。
}

// Policy 表示一次判定使用的不可变策略快照。
type Policy struct {
	Version       string   // 识别版本号（模型 + 提示词 + 观察项定义）。
	AllowedLabels []string // 允许输出的其他违规标签。
	SystemPrompt  string   // 观察项定义与输出格式要求。

}

// Observation 一个低俗观察项
type Observation struct {
	Type      string  `json:"type"`      // 类型
	Timestamp float64 `json:"timestamp"` // 秒
}

// Output 表示单次模型调用通过严格校验的结果。
type Output struct {
	Observations      []Observation `json:"observations"`        // 低俗观察项列表
	ViolenceDetected  bool          `json:"violence_detected"`   // 暴力检出
	OtherLabels       []string      `json:"other_labels"`        // 其他命中的标签
	NeedHumanReview   bool          `json:"need_human_review"`   // 模型判定转人审
	Evidence          []string      `json:"evidence"`            // 证据
	IsSwimwearProduct bool          `json:"is_swimwear_product"` // 内衣泳装商品展示
	HasSuspectedMinor bool          `json:"has_suspected_minor"` // 疑似未成年人
}

// Usage 表示一次推理调用的资源消耗。
type Usage struct {
	TokensIn  int   // 输入 token 数。
	TokensOut int   // 输出 token 数。
	LatencyMS int64 // 调用耗时，单位毫秒。
}

// Inference 定义判定所需的模型调用接口。
type Inference interface {
	// Complete 接收样本与提示词，返回原始文本及 token 和延迟统计。
	Complete(ctx context.Context, sample Sample, prompt string) (string, Usage, error)
}

// Decision 表示两次采样汇总后的判定。
type Decision struct {
	SampleID          string        `json:"sample_id"`           // 判定对应样本编号。
	Version           string        `json:"version"`             // 识别版本。
	Observations      []Observation `json:"observations"`        // 汇总后的观察项列表。
	ViolenceDetected  bool          `json:"violence_detected"`   // 暴力检出。
	OtherLabels       []string      `json:"other_labels"`        // 其他命中的标签。
	NeedHumanReview   bool          `json:"need_human_review"`   // 是否需要人审。
	HumanReasons      []string      `json:"human_reasons"`       // 转人审原因（格式失败、模型判人审）。
	Evidence          []string      `json:"evidence"`            // 证据。
	IsSwimwearProduct bool          `json:"is_swimwear_product"` // 内衣泳装商品展示。
	HasSuspectedMinor bool          `json:"has_suspected_minor"` // 疑似未成年人。
	FormatFailures    int           `json:"format_failures"`     // 两次格式校验都失败的采样次数。
	TokensIn          int           `json:"tokens_in"`           // 全部采样输入 token 总数。
	TokensOut         int           `json:"tokens_out"`          // 全部采样输出 token 总数。
	LatencyMS         int64         `json:"latency_ms"`          // 两次采样并发执行的墙钟耗时。
	Samples           []Output      `json:"samples,omitempty"`   // 各次采样通过校验的原始输出，供阈值校准和追溯。
}
