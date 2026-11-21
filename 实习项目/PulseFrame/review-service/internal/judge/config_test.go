package judge

import (
	"os"
	"testing"
	"time"
)

// TestLoadConfigFromEnv_Success 测试从环境变量加载完整配置。
func TestLoadConfigFromEnv_Success(t *testing.T) {
	// 设置环境变量
	os.Setenv("VLM_BASE_URL", "http://127.0.0.1:8000/v1")
	os.Setenv("VLM_MODEL", "Qwen/Qwen2-VL-7B-Instruct")
	os.Setenv("VLM_API_KEY", "test-key")
	os.Setenv("VLM_TIMEOUT", "30")
	os.Setenv("VLM_MAX_RETRIES", "5")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
		os.Unsetenv("VLM_API_KEY")
		os.Unsetenv("VLM_TIMEOUT")
		os.Unsetenv("VLM_MAX_RETRIES")
	}()

	config, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv failed: %v", err)
	}

	if config.BaseURL != "http://127.0.0.1:8000/v1" {
		t.Errorf("expected BaseURL 'http://127.0.0.1:8000/v1', got '%s'", config.BaseURL)
	}
	if config.Model != "Qwen/Qwen2-VL-7B-Instruct" {
		t.Errorf("expected Model 'Qwen/Qwen2-VL-7B-Instruct', got '%s'", config.Model)
	}
	if config.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got '%s'", config.APIKey)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", config.Timeout)
	}
	if config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", config.MaxRetries)
	}
}

// TestLoadConfigFromEnv_Defaults 测试默认值。
func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Setenv("VLM_MODEL", "test-model")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
	}()

	config, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv failed: %v", err)
	}

	if config.APIKey != "" {
		t.Errorf("expected empty APIKey, got '%s'", config.APIKey)
	}
	if config.Timeout != 60*time.Second {
		t.Errorf("expected default Timeout 60s, got %v", config.Timeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("expected default MaxRetries 3, got %d", config.MaxRetries)
	}
}

// TestLoadConfigFromEnv_MissingBaseURL 测试缺少必需的 BaseURL。
func TestLoadConfigFromEnv_MissingBaseURL(t *testing.T) {
	os.Unsetenv("VLM_BASE_URL")
	os.Setenv("VLM_MODEL", "test-model")
	defer os.Unsetenv("VLM_MODEL")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for missing VLM_BASE_URL")
	}
}

// TestLoadConfigFromEnv_MissingModel 测试缺少必需的 Model。
func TestLoadConfigFromEnv_MissingModel(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Unsetenv("VLM_MODEL")
	defer os.Unsetenv("VLM_BASE_URL")

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for missing VLM_MODEL")
	}
}

// TestLoadConfigFromEnv_InvalidTimeout 测试无效的超时值。
func TestLoadConfigFromEnv_InvalidTimeout(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Setenv("VLM_MODEL", "test-model")
	os.Setenv("VLM_TIMEOUT", "invalid")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
		os.Unsetenv("VLM_TIMEOUT")
	}()

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid VLM_TIMEOUT")
	}
}

// TestLoadConfigFromEnv_NegativeTimeout 测试负数超时值。
func TestLoadConfigFromEnv_NegativeTimeout(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Setenv("VLM_MODEL", "test-model")
	os.Setenv("VLM_TIMEOUT", "-10")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
		os.Unsetenv("VLM_TIMEOUT")
	}()

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for negative VLM_TIMEOUT")
	}
}

// TestLoadConfigFromEnv_InvalidMaxRetries 测试无效的重试次数。
func TestLoadConfigFromEnv_InvalidMaxRetries(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Setenv("VLM_MODEL", "test-model")
	os.Setenv("VLM_MAX_RETRIES", "invalid")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
		os.Unsetenv("VLM_MAX_RETRIES")
	}()

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid VLM_MAX_RETRIES")
	}
}

// TestLoadConfigFromEnv_NegativeMaxRetries 测试负数重试次数。
func TestLoadConfigFromEnv_NegativeMaxRetries(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Setenv("VLM_MODEL", "test-model")
	os.Setenv("VLM_MAX_RETRIES", "-1")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
		os.Unsetenv("VLM_MAX_RETRIES")
	}()

	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for negative VLM_MAX_RETRIES")
	}
}

// TestLoadConfigFromEnv_ZeroMaxRetries 测试零重试次数（有效值）。
func TestLoadConfigFromEnv_ZeroMaxRetries(t *testing.T) {
	os.Setenv("VLM_BASE_URL", "http://localhost:8000")
	os.Setenv("VLM_MODEL", "test-model")
	os.Setenv("VLM_MAX_RETRIES", "0")
	defer func() {
		os.Unsetenv("VLM_BASE_URL")
		os.Unsetenv("VLM_MODEL")
		os.Unsetenv("VLM_MAX_RETRIES")
	}()

	config, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv failed: %v", err)
	}

	if config.MaxRetries != 0 {
		t.Errorf("expected MaxRetries 0, got %d", config.MaxRetries)
	}
}

// TestNewClientFromConfig 测试从配置创建客户端。
func TestNewClientFromConfig(t *testing.T) {
	config := Config{
		BaseURL:    "http://127.0.0.1:8000/v1",
		Model:      "test-model",
		APIKey:     "test-key",
		Timeout:    30 * time.Second,
		MaxRetries: 5,
	}

	client := NewClientFromConfig(config)

	if client.BaseURL != config.BaseURL {
		t.Errorf("expected BaseURL '%s', got '%s'", config.BaseURL, client.BaseURL)
	}
	if client.Model != config.Model {
		t.Errorf("expected Model '%s', got '%s'", config.Model, client.Model)
	}
	if client.APIKey != config.APIKey {
		t.Errorf("expected APIKey '%s', got '%s'", config.APIKey, client.APIKey)
	}
	if client.HTTPClient.Timeout != config.Timeout {
		t.Errorf("expected Timeout %v, got %v", config.Timeout, client.HTTPClient.Timeout)
	}
	if client.MaxRetries != config.MaxRetries {
		t.Errorf("expected MaxRetries %d, got %d", config.MaxRetries, client.MaxRetries)
	}
}

// TestNewClientFromConfig_TrailingSlash 测试配置中的尾部斜杠处理。
func TestNewClientFromConfig_TrailingSlash(t *testing.T) {
	config := Config{
		BaseURL:    "http://127.0.0.1:8000/v1/",
		Model:      "test-model",
		Timeout:    60 * time.Second,
		MaxRetries: 3,
	}

	client := NewClientFromConfig(config)

	if client.BaseURL != "http://127.0.0.1:8000/v1" {
		t.Errorf("expected trailing slash removed, got '%s'", client.BaseURL)
	}
}
