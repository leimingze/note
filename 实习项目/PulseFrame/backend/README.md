# PulseFrame Backend

当前目录包含 PulseFrame 核心 API 的工程基础。现阶段只提供配置、结构化日志、统一错误、请求中间件、健康检查和优雅退出，不包含业务接口或外部存储。

## 启动

```bash
go run ./cmd/pulseframe-api
```

默认监听 `:8080`。常用配置示例：

```bash
APP_ENV=local \
HTTP_ADDR=127.0.0.1:8080 \
LOG_LEVEL=debug \
go run ./cmd/pulseframe-api
```

支持的环境变量：

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `APP_ENV` | `local` | 运行环境：`local`、`test`、`staging`、`production` |
| `HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `LOG_LEVEL` | `info` | 日志级别：`debug`、`info`、`warn`、`error` |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | 请求头读取超时 |
| `HTTP_READ_TIMEOUT` | `15s` | 请求读取超时 |
| `HTTP_WRITE_TIMEOUT` | `15s` | 响应写入超时 |
| `HTTP_IDLE_TIMEOUT` | `1m` | 空闲连接超时 |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | 优雅退出期限 |

## 验证

```bash
curl -i http://127.0.0.1:8080/livez
curl -i http://127.0.0.1:8080/readyz
```

运行测试和静态检查：

```bash
go test -timeout=60s ./...
go vet ./...
```

## 架构说明

- [启动流程：`run` 函数逐段说明](docs/run-lifecycle.md)
