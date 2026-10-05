package logging

import (
	"errors"
	"io"
	"log/slog"
	"strings"
)

// errNilOutput 表示日志器没有可写入的输出目标。
var errNilOutput = errors.New("log output is required")

// New 创建 JSON 结构化日志器。
// 输入：level，debug、info、warn 或 error；output，日志写入目标，不能为 nil。
// 输出：配置完成的 slog.Logger；输入非法时返回错误。
// 功能：先校验输出目标，再清理并解析日志级别；随后创建按该级别过滤日志的 JSON Handler，并用它构造 Logger。
func New(level string, output io.Writer) (*slog.Logger, error) {
	// 没有输出目标时日志会静默丢失，因此构造阶段直接拒绝。
	if output == nil {
		return nil, errNilOutput
	}
	// 交给 slog 解析级别，避免项目维护第二套字符串映射。
	var parsed slog.Level
	// 未知级别属于部署配置错误，不能悄悄回落到默认级别。
	if err := parsed.UnmarshalText([]byte(strings.ToUpper(strings.TrimSpace(level)))); err != nil {
		return nil, err
	}
	// 统一使用 JSON，方便日志采集系统按字段检索。
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: parsed})
	return slog.New(handler), nil
}
