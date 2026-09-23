# `run` 函数启动流程

本文按 `cmd/pulseframe-api/main.go` 中 `run` 函数的执行顺序，说明 API 进程如何读取配置、组装 HTTP 服务并响应退出信号。

## 总览

`run` 是启动装配入口：它把外部配置转换成服务需要的对象，再把这些对象连接起来。它本身不定义业务接口，也不直接实现 HTTP 监听和关闭细节。

```text
环境变量查询函数
        |
        v
加载并校验配置 -> 创建日志器 -> 设置 Gin 模式
                                     |
                                     v
创建健康状态 -> 创建 Gin 路由 -> 创建 HTTP Server
                                     |
                                     v
                         监听退出信号并运行服务
                                     |
                      收到退出信号后优雅关闭
```

## 函数签名与调用方

```go
func run(parent context.Context, lookup config.LookupEnv) error
```

- `parent`：父生命周期上下文。父上下文取消时，服务也会进入关闭流程。
- `lookup`：环境变量查询函数。生产入口传入 `os.LookupEnv`；测试可以传入可控实现。
- 返回值：正常关闭时返回 `nil`；配置、依赖构造或服务运行失败时返回带阶段信息的错误。

`main` 调用 `run(context.Background(), os.LookupEnv)`。若 `run` 返回错误，`main` 记录错误日志并以非零状态退出。

## 1. 加载配置

```go
cfg, err := config.Load(lookup)
```

配置层从 `lookup` 读取环境变量，解析并校验运行环境、日志级别、HTTP 监听地址和各项超时。未提供的变量使用默认值；非法值会立即返回错误，`run` 将其包装为 `load config` 阶段错误并停止启动。

此处得到的 `cfg` 是后续初始化的统一配置来源。例如，`cfg.Environment` 决定 Gin 模式，`cfg.LogLevel` 决定日志过滤级别，`cfg.HTTP` 提供监听地址和 HTTP 超时。

## 2. 创建结构化日志器

```go
logger, err := logging.New(cfg.LogLevel, os.Stdout)
```

日志器使用配置中的级别，并将 JSON 结构化日志写到标准输出，便于本地查看或由部署平台采集。日志器创建失败会包装为 `create logger` 错误并中止启动。

创建后，`logger` 会注入路由中间件，并用于记录服务启动、停止和请求信息。

## 3. 设置 Gin 运行模式

```go
gin.SetMode(ginMode(cfg.Environment))
```

`ginMode` 将已校验的应用环境映射到 Gin 模式：`test` 对应测试模式，`staging` 和 `production` 对应发布模式，其余当前合法环境（`local`）对应调试模式。`gin.SetMode` 设置 Gin 的进程级模式，影响框架的调试行为；它不读取或校验环境变量。

## 4. 创建健康状态

```go
health := httpserver.NewHealthState()
```

新建的状态初始为“未就绪”。同一个 `health` 对象会交给路由和 HTTP Server：路由用它响应 `/readyz`，Server 在开始接收请求时将其标记为就绪，在退出流程开始时将其标记为未就绪。

`/livez` 只表示进程能响应探针；`/readyz` 表示实例当前是否适合接收流量。

## 5. 创建路由和中间件

```go
router, err := httpserver.NewRouter(httpserver.RouterDependencies{
    Logger:      logger,
    IDGenerator: httpmiddleware.CryptoIDGenerator{},
    Health:      health,
})
```

`run` 把日志器、请求 ID 生成器和共享健康状态交给路由层。路由层据此创建 Gin Engine，配置受信代理策略，注册全局中间件和基础探针，并统一处理未找到路由及不支持的方法。

当前全局中间件按注册顺序为请求 ID、访问日志、panic 恢复。路由构造失败会包装为 `create router` 错误并中止启动。此时服务还没有开始监听端口。

## 6. 创建 HTTP Server

```go
server, err := httpserver.New(
    httpserver.OptionsFromConfig(cfg, router, health),
)
```

`OptionsFromConfig` 将配置中的地址和超时，加上上一步创建的路由及健康状态，组装成 HTTP Server 参数。`httpserver.New` 校验必要依赖和关闭超时，再创建标准库 `http.Server`。

此阶段只构造服务对象，尚未绑定端口。构造失败会包装为 `create http server` 错误并中止启动。

## 7. 订阅退出信号

```go
ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
defer stop()
```

上下文会在父上下文取消、收到中断信号（通常是 Ctrl+C）或收到 `SIGTERM` 时取消。`server.Run(ctx)` 监听这个上下文；`defer stop()` 确保 `run` 返回时撤销信号通知并释放相关资源。

## 8. 记录启动信息并运行

```go
logger.Info("pulseframe api starting", ...)
if err := server.Run(ctx); err != nil {
    return fmt.Errorf("run http server: %w", err)
}
```

启动日志包含监听地址和运行环境。`server.Run` 负责绑定 TCP 地址并进入服务生命周期；绑定失败或服务异常会作为 `run http server` 错误返回。

服务开始接收请求后，健康状态变为就绪。上下文取消时，Server 先将状态改为未就绪，再在配置的关闭期限内停止接收新请求并等待正在处理的请求结束。正常关闭不会被当作服务错误。

## 9. 记录停止并正常返回

```go
logger.Info("pulseframe api stopped")
return nil
```

只有 `server.Run` 正常结束后才会记录停止日志并返回 `nil`。若前面任一初始化步骤或服务运行失败，函数会提前返回对应错误，不会执行这段正常停止日志。

## 错误传播路径

`run` 在每个初始化边界添加阶段名称，例如 `load config`、`create router`。错误一路返回到 `main`，由进程入口统一记录并以非零状态退出。这样日志能指出失败发生在启动的哪个阶段，而底层错误仍通过 `%w` 保留，可供调用方继续检查。
