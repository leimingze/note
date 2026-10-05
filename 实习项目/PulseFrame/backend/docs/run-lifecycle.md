# run 函数启动流程

本文按 cmd/pulseframe-api/main.go 中 run 的执行顺序说明 API 基座如何启动和退出。

## 总览

~~~text
加载并校验配置
  -> 建立信号上下文
  -> 创建日志器并设置 Gin 模式
  -> 创建健康状态和基础路由
  -> 创建并运行 HTTP Server
  -> 收到退出信号
  -> 停止接收请求并等待现有请求完成
~~~

## 函数签名

~~~go
func run(parent context.Context, lookup config.LookupEnv) error
~~~

- parent：服务生命周期的父上下文。
- lookup：环境变量读取函数，生产入口传入 os.LookupEnv。
- 返回值：正常退出时为 nil；配置、初始化或运行失败时返回带阶段信息的错误。

## 启动步骤

### 1. 加载配置

config.Load 读取运行环境、日志级别、监听地址和 HTTP 超时。非法值会在创建监听器之前返回错误。

### 2. 建立生命周期上下文

signal.NotifyContext 监听父上下文、SIGINT 和 SIGTERM。收到终止信号后，同一上下文通知 HTTP Server 进入优雅退出流程。

### 3. 创建日志器并设置 Gin

logging.New 创建写入标准输出的 JSON 日志器。ginMode 把项目环境映射为 Gin 的 debug、test 或 release 模式。

### 4. 创建基础路由

server.NewRouter 注册请求标识、访问日志、panic 恢复、/livez、/readyz、404 和 405 错误响应。

当前基座没有业务路由，也不创建外部存储连接。

### 5. 运行和退出

server.New 校验 HTTP 参数并创建 Server。Server.Run 监听端口、发布就绪状态，并持续服务到上下文取消或监听失败。

收到退出信号后，Server 先撤销就绪状态，再调用标准库 Shutdown。关闭期限由 HTTP_SHUTDOWN_TIMEOUT 控制。

## 错误传播

run 使用 fmt.Errorf 保留底层错误，并补充 load config、create logger、create router、create http server 或 run http server 等阶段名称。
