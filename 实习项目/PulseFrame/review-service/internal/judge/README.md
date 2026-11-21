# VLM 推理客户端

本目录实现了用于内容审核的 VLM（Vision-Language Model）推理客户端，支持 OpenAI 兼容的多模态 API。

## 文件结构

```
internal/judge/
├── types.go              # 核心类型定义（Inference 接口、Sample、Policy 等）
├── client.go             # OpenAI 兼容的推理客户端实现
├── client_test.go        # 客户端测试
├── config.go             # 配置管理（环境变量加载）
├── config_test.go        # 配置测试
├── engine.go             # 判定引擎（两次采样与校验）
├── parser.go             # 输出解析与校验
├── parser_test.go        # 解析器测试
├── scoring.go            # 评分与汇总逻辑
├── scoring_test.go       # 评分测试
└── example_client_usage.go  # 使用示例
```

## 核心功能

### 1. OpenAI 兼容的推理客户端

`OpenAIClient` 实现了 `Inference` 接口，支持：

- ✅ OpenAI 兼容的 `/v1/chat/completions` API
- ✅ 多模态输入（文本 + 图片 base64 编码）
- ✅ 自动错误重试（指数退避）
- ✅ 超时控制
- ✅ Token 使用和延迟统计
- ✅ 4xx 客户端错误不重试，5xx 服务器错误自动重试

### 2. 配置管理

支持从环境变量加载配置：

| 环境变量 | 说明 | 是否必需 | 默认值 |
|---------|------|---------|--------|
| `VLM_BASE_URL` | VLM 服务根地址 | ✅ 必需 | 无 |
| `VLM_MODEL` | 模型名称 | ✅ 必需 | 无 |
| `VLM_API_KEY` | API Key | ❌ 可选 | 空 |
| `VLM_TIMEOUT` | 超时秒数 | ❌ 可选 | 60 |
| `VLM_MAX_RETRIES` | 最大重试次数 | ❌ 可选 | 3 |

### 3. 重试逻辑

- **网络错误**: 自动重试，指数退避（100ms, 200ms, 400ms, ...）
- **5xx 服务器错误**: 自动重试
- **4xx 客户端错误**: 不重试，直接返回错误
- **上下文取消**: 立即停止，不重试

## 使用示例

### 方式 1: 从环境变量加载配置

```go
import "pulseframe-review/internal/judge"

// 设置环境变量
// export VLM_BASE_URL=http://127.0.0.1:8000/v1
// export VLM_MODEL=Qwen/Qwen2-VL-7B-Instruct

config, err := judge.LoadConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

client := judge.NewClientFromConfig(config)
```

### 方式 2: 直接构造客户端

```go
client := judge.NewOpenAIClient(
    "http://127.0.0.1:8000/v1",
    "Qwen/Qwen2-VL-7B-Instruct",
    "", // API Key 可选
)
```

### 方式 3: 使用自定义配置

```go
config := judge.Config{
    BaseURL:    "http://127.0.0.1:8000/v1",
    Model:      "Qwen/Qwen2-VL-7B-Instruct",
    APIKey:     "your-api-key",
    Timeout:    30 * time.Second,
    MaxRetries: 5,
}

client := judge.NewClientFromConfig(config)
```

### 执行推理

```go
sample := judge.Sample{
    SampleID: "video-12345",
    Title:    "视频标题",
    OCR:      "OCR 文本",
    ASR:      "ASR 文本",
    Images:   []string{"/path/to/frame1.jpg", "/path/to/frame2.jpg"},
}

ctx := context.Background()
text, usage, err := client.Complete(ctx, sample, "系统提示词")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("结果: %s\n", text)
fmt.Printf("输入 tokens: %d, 输出 tokens: %d, 延迟: %d ms\n",
    usage.TokensIn, usage.TokensOut, usage.LatencyMS)
```

## 测试

运行所有测试：

```bash
go test ./internal/judge/ -v -cover
```

运行特定测试：

```bash
# 测试客户端
go test ./internal/judge/ -v -run TestOpenAIClient

# 测试配置
go test ./internal/judge/ -v -run TestLoadConfig

# 测试重试逻辑
go test ./internal/judge/ -v -run TestOpenAIClient_Complete_Retry
```

## API 格式

### 请求格式

```json
{
  "model": "Qwen/Qwen2-VL-7B-Instruct",
  "temperature": 1.0,
  "max_tokens": 512,
  "messages": [
    {
      "role": "system",
      "content": "系统提示词"
    },
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "标题：...\nOCR：...\nASR：..."
        },
        {
          "type": "image_url",
          "image_url": {
            "url": "data:image/jpeg;base64,..."
          }
        }
      ]
    }
  ]
}
```

### 响应格式

```json
{
  "choices": [
    {
      "message": {
        "content": "模型输出文本"
      }
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50
  }
}
```

## 错误处理

客户端会自动处理以下错误：

1. **网络错误**: 自动重试
2. **5xx 服务器错误**: 自动重试
3. **4xx 客户端错误**: 直接返回（不重试）
4. **超时**: 返回 context.DeadlineExceeded
5. **解析错误**: 自动重试

## 性能优化

- 使用连接池（`http.Client` 复用）
- 指数退避避免雪崩
- 支持上下文取消（立即停止）
- 图片 base64 编码仅执行一次

## 兼容性

支持以下 VLM 服务：

- ✅ vLLM (OpenAI 兼容模式)
- ✅ OpenAI API
- ✅ 其他 OpenAI 兼容服务

## 测试覆盖率

当前测试覆盖率：**68.9%**

覆盖的测试场景：
- ✅ 正常推理调用
- ✅ API Key 认证
- ✅ 服务器错误处理
- ✅ 空响应处理
- ✅ 超时处理
- ✅ 上下文取消
- ✅ 重试逻辑（成功重试、重试耗尽、4xx 不重试）
- ✅ 多模态内容构建
- ✅ 图片读取错误
- ✅ MIME 类型判定
- ✅ 配置加载（完整配置、默认值、缺失必需项、无效值）
- ✅ URL 尾部斜杠处理
