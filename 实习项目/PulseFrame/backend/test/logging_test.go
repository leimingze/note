package test

import (
	"bytes"
	"encoding/json"
	"testing"

	"pulseframe/internal/logging"
)

// TestNewWritesStructuredJSON 验证日志器输出可解析的 JSON。
// 输入：Go 测试框架提供的测试上下文。
// 输出：日志格式、级别或消息字段不符合预期时使测试失败。
// 功能：保证日志能够被后续采集系统按结构读取。
func TestNewWritesStructuredJSON(t *testing.T) {
	var output bytes.Buffer
	logger, err := logging.New("info", &output)
	// 合法级别和输出目标必须成功创建日志器。
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	logger.Info("service started", "component", "api")

	var record map[string]any
	// 输出必须是采集系统可以解析的 JSON 对象。
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	// 级别、消息和附加字段必须保留结构化语义。
	if record["level"] != "INFO" || record["msg"] != "service started" || record["component"] != "api" {
		t.Fatalf("unexpected log record: %+v", record)
	}
}

// TestNewRejectsInvalidLevel 验证非法日志级别会返回错误。
// 输入：Go 测试框架提供的测试上下文。
// 输出：非法级别未返回错误时使测试失败。
// 功能：避免日志配置错误被静默接受。
func TestNewRejectsInvalidLevel(t *testing.T) {
	// 未支持的日志级别不能静默回落到 info。
	if _, err := logging.New("verbose", &bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid log level error")
	}
}
