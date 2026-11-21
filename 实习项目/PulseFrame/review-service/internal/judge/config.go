package judge

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 保存 VLM 推理客户端的配置参数。
type Config struct {
	BaseURL    string        // VLM 服务根地址，例如 http://127.0.0.1:8000/v1。
	Model      string        // 模型名称，例如 Qwen/Qwen2-VL-7B-Instruct。
	APIKey     string        // 可选 API Key，用于需要认证的服务。
	Timeout    time.Duration // 单次请求超时时间，默认 60 秒。
	MaxRetries int           // 网络错误最大重试次数，默认 3 次。
	MaxTokens  int           // 单次输出 token 上限，默认 1024。
	// DisableThinking 关闭 Qwen3.5 思考模式，默认关闭思考；VLM_DISABLE_THINKING=false 时保留思考。
	DisableThinking bool
}

// LoadConfigFromEnv 从环境变量加载配置。
// 输出：完整配置；必需参数缺失时返回错误。
// 功能：支持 VLM_BASE_URL、VLM_MODEL、VLM_API_KEY、VLM_TIMEOUT、VLM_MAX_RETRIES 环境变量。
func LoadConfigFromEnv() (Config, error) {
	baseURL := os.Getenv("VLM_BASE_URL")
	if baseURL == "" {
		return Config{}, fmt.Errorf("VLM_BASE_URL is required")
	}

	model := os.Getenv("VLM_MODEL")
	if model == "" {
		return Config{}, fmt.Errorf("VLM_MODEL is required")
	}

	config := Config{
		BaseURL:         baseURL,
		Model:           model,
		APIKey:          os.Getenv("VLM_API_KEY"), // 可选
		Timeout:         60 * time.Second,         // 默认 60 秒
		MaxRetries:      3,                        // 默认重试 3 次
		MaxTokens:       defaultMaxTokens,
		DisableThinking: os.Getenv("VLM_DISABLE_THINKING") != "false",
	}

	if tokensStr := os.Getenv("VLM_MAX_TOKENS"); tokensStr != "" {
		tokens, err := strconv.Atoi(tokensStr)
		if err != nil || tokens <= 0 {
			return Config{}, fmt.Errorf("invalid VLM_MAX_TOKENS: %q", tokensStr)
		}
		config.MaxTokens = tokens
	}

	// 解析可选的超时配置
	if timeoutStr := os.Getenv("VLM_TIMEOUT"); timeoutStr != "" {
		timeoutSec, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid VLM_TIMEOUT: %w", err)
		}
		if timeoutSec <= 0 {
			return Config{}, fmt.Errorf("VLM_TIMEOUT must be positive, got %d", timeoutSec)
		}
		config.Timeout = time.Duration(timeoutSec) * time.Second
	}

	// 解析可选的重试次数配置
	if retriesStr := os.Getenv("VLM_MAX_RETRIES"); retriesStr != "" {
		retries, err := strconv.Atoi(retriesStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid VLM_MAX_RETRIES: %w", err)
		}
		if retries < 0 {
			return Config{}, fmt.Errorf("VLM_MAX_RETRIES must be non-negative, got %d", retries)
		}
		config.MaxRetries = retries
	}

	return config, nil
}

// NewClientFromConfig 从配置创建推理客户端。
// 输入：config，完整的客户端配置。
// 输出：配置好超时和重试的 OpenAI 兼容客户端。
// 功能：统一从环境变量或显式配置构造客户端。
func NewClientFromConfig(config Config) *OpenAIClient {
	client := NewOpenAIClient(config.BaseURL, config.Model, config.APIKey)
	client.HTTPClient.Timeout = config.Timeout
	client.MaxRetries = config.MaxRetries
	client.MaxTokens = config.MaxTokens
	client.DisableThinking = config.DisableThinking
	return client
}
