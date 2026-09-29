# `run` 函数启动流程

本文按 `cmd/pulseframe-api/main.go` 中 `run` 函数的执行顺序，说明 API 如何加载配置、验证认证依赖、组装路由并响应退出信号。

## 总览

`run` 是启动装配入口：它从环境配置构造依赖，并在 HTTP 服务开始接收请求前验证 MySQL、Redis 和用户表结构。

```text
加载并校验配置 -> 建立生命周期上下文 -> 创建日志器与 Gin 模式
                                      |
                                      v
                             连接并 Ping MySQL/Redis
                                      |
                                      v
                      检查 users 表 -> 组装认证服务和路由
                                      |
                                      v
                         创建 HTTP Server 并运行
                                      |
                         信号取消 -> 优雅关闭连接
```

## 函数签名与调用方

```go
func run(parent context.Context, lookup config.LookupEnv) error
```

- `parent`：父生命周期上下文；其取消会中止依赖初始化或触发服务关闭。
- `lookup`：环境变量查询函数；生产入口传入 `os.LookupEnv`，测试可注入确定来源。
- 返回值：服务正常关闭且依赖清理成功时为 `nil`；配置、连接、装配、运行或清理失败时返回错误。

`main` 调用 `run(context.Background(), os.LookupEnv)`。若 `run` 返回错误，`main` 记录阶段错误并以非零状态退出。

## 启动步骤

### 1. 加载配置

`config.Load` 解析运行环境、HTTP 地址和超时、MySQL DSN、Redis 连接设置、网页来源、Cookie 策略、限流策略及 Argon2id 并发上限。必需配置缺失或格式非法时立即停止启动。

### 2. 建立生命周期上下文

配置通过后，`signal.NotifyContext` 监听父上下文、`SIGINT` 和 `SIGTERM`。同一个上下文用于 MySQL/Redis Ping、用户表检查和 `server.Run`，因此启动中收到退出信号也会取消初始化。

### 3. 创建日志器和 Gin 模式

日志器以 JSON 格式写入标准输出。`ginMode` 将经过校验的运行环境映射为 Gin 的 debug、test 或 release 模式。

### 4. 连接认证依赖

`newRuntime` 创建 MySQL GORM 句柄和 Redis 客户端，并分别在 5 秒期限内 Ping。任一依赖不可用时拒绝启动；若 Redis 连接失败，会先关闭已打开的 MySQL 连接池。

MySQL 连接池限制为最多 25 个打开连接、5 个空闲连接，连接最长复用 3 分钟。GORM SQL 日志关闭，避免 DSN 或账号数据进入日志。

### 5. 检查结构并组装账号路由

`newBusinessRouter` 通过 `users` 仓储执行只读 schema 检查，最长等待 5 秒；服务不自动执行迁移。检查通过后创建 Redis 会话管理器、共享限流器、Argon2id 哈希器、账号应用服务和 HTTP Handler。

账号 HTTP Handler 注册网页认证路由、精确 `WEB_ORIGIN` 的凭据型 CORS、Origin 校验、Cookie 会话和 CSRF 中间件。Gin 路由同时关闭默认可信代理，来源 IP 默认取 TCP 对端地址。

### 6. 创建并运行 HTTP Server

HTTP Server 使用配置中的监听地址、读取/写入/空闲超时和优雅关闭期限。服务运行期间 `/readyz` 表示实例是否接收流量，`/livez` 只表示进程仍可响应。

上下文取消后，Server 先停止就绪状态、关闭监听并等待正在处理的请求；`server.Run` 返回后，`run` 关闭 Redis 客户端和 MySQL 连接池。关闭错误会与运行错误组合返回。

## 错误传播

每个初始化边界由 `run` 添加阶段名称，例如 `load config`、`connect authentication dependencies`、`create application router` 和 `run http server`。底层错误通过 `%w` 保留，便于定位缺失配置、网络故障、未执行迁移或服务运行失败。
