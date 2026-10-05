# PulseFrame Backend

当前目录是 PulseFrame 核心 API 的工程基座。它提供配置校验、结构化日志、请求标识、访问日志、异常恢复、统一错误响应、健康检查、HTTP 超时和优雅退出，不依赖 MySQL 或 Redis。

## 启动

在 backend/ 目录执行：

~~~bash
go run ./cmd/pulseframe-api
~~~

默认监听 :8080。本地需要固定地址时可以加载 .env.local：

~~~bash
set -a
source .env.local
set +a
go run ./cmd/pulseframe-api
~~~

支持的环境变量：

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| APP_ENV | local | 运行环境：local、test、staging、production |
| HTTP_ADDR | :8080 | HTTP 监听地址 |
| LOG_LEVEL | info | 日志级别：debug、info、warn、error |
| HTTP_READ_HEADER_TIMEOUT | 5s | 请求头读取超时 |
| HTTP_READ_TIMEOUT | 15s | 请求读取超时 |
| HTTP_WRITE_TIMEOUT | 15s | 响应写入超时 |
| HTTP_IDLE_TIMEOUT | 1m | 空闲连接超时 |
| HTTP_SHUTDOWN_TIMEOUT | 10s | 优雅退出期限 |

## 接口

- GET /livez：进程可以响应时返回 200。
- GET /readyz：服务完成启动后返回 200。
- 未知路径和不支持的方法使用统一 JSON 错误结构。

本地检查：

~~~bash
curl -i http://127.0.0.1:8080/livez
curl -i http://127.0.0.1:8080/readyz
~~~

## 验证

~~~bash
go test -timeout=60s ./...
go vet ./...
~~~

启动调用说明见 docs/run-lifecycle.md。
