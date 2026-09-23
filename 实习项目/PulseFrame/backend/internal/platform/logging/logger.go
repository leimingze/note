package logging

import (
	"errors"
	"io"
	"log/slog"
	"strings"
)

var errNilOutput = errors.New("log output is required")

// New 创建 JSON 结构化日志器。
// 输入：level，debug、info、warn 或 error；output，日志写入目标，不能为 nil。
// 输出：配置完成的 slog.Logger；输入非法时返回错误。
// 功能：先校验输出目标，再清理并解析日志级别；随后创建按该级别过滤日志的 JSON Handler，并用它构造 Logger。
func New(level string, output io.Writer) (*slog.Logger, error) {
	if output == nil {
		return nil, errNilOutput
	}
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(strings.ToUpper(strings.TrimSpace(level)))); err != nil {
		return nil, err
	}
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: parsed})
	return slog.New(handler), nil
}
