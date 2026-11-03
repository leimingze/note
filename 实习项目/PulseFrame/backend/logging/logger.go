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
// 功能：检查日志级别和输出位置，再创建按级别过滤、以 JSON 格式输出的日志器。
// 例如传入 "info" 和标准输出时，Info 日志会写到终端，Debug 日志会被过滤。
func New(level string, output io.Writer) (*slog.Logger, error) {
	// 没有输出目标时日志会静默丢失，因此构造阶段直接拒绝。
	if output == nil {
		return nil, errNilOutput
	}
	// 交给 slog 解析级别，避免项目维护第二套字符串映射。
	var parsed slog.Level
	// 先去掉首尾空白并统一大小写，再交给标准库转换；无法识别时返回错误。
	if err := parsed.UnmarshalText([]byte(strings.ToUpper(strings.TrimSpace(level)))); err != nil {
		return nil, err
	}
	// 统一使用 JSON，方便日志采集系统按字段检索。
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: parsed})
	return slog.New(handler), nil
}
