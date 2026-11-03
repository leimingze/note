# run 函数启动流程

本文按 cmd/pulseframe-api/main.go 中 run 的执行顺序说明 API 基座如何启动和退出。

## 总览

1. 加载并校验基础配置及 MySQL 配置。
2. 建立信号上下文，创建日志器并设置 Gin 模式。
3. 创建共享 MySQL 连接池并完成启动检查。
4. 创建健康状态、基础路由和 HTTP Server。
5. 收到退出信号后停止接收请求，在关闭期限内等待现有请求完成。
6. 关闭 MySQL 连接池，返回运行及资源关闭错误。

## 函数签名

~~~go
func run(parent context.Context, lookup config.LookupEnv) (runErr error)
~~~

- parent：服务生命周期的父上下文。
- lookup：环境变量读取函数，生产入口传入 os.LookupEnv。
- 返回值：正常退出时为 nil；配置、初始化或运行失败时返回带阶段信息的错误。

## 启动步骤

### 1. 加载配置

config.Load 读取运行环境、日志级别、HTTP 和 MySQL 配置。数据库账号、密码和数据库名称须显式配置；密码保留原值。非法配置会在创建连接池及监听器之前返回错误。

### 2. 建立生命周期上下文

signal.NotifyContext 监听父上下文、SIGINT 和 SIGTERM。收到终止信号后，同一上下文通知 HTTP Server 进入优雅退出流程。

### 3. 创建日志器并设置 Gin

logging.New 创建写入标准输出的 JSON 日志器。ginMode 把项目环境映射为 Gin 的 debug、test 或 release 模式。

### 4. 创建外部依赖

NewDependencies 调用 infra.NewMySQL，通过驱动的配置对象创建 sql.DB，并设置连接数、空闲数量和回收期限。PingContext 在 MYSQL_STARTUP_TIMEOUT 内检查真实连接，受启动上下文取消控制；失败时关闭已创建的连接池并返回错误。

创建成功后，run 立即登记 Dependencies.Close。此后无论路由构造、HTTP 监听失败还是正常退出，都会关闭连接池。当前只有 MySQL 依赖，每个 API 进程创建一个连接池，后续 DAO 使用传入的共享对象。

### 5. 创建基础路由

server.NewRouter 注册请求标识、访问日志、panic 恢复、/livez、/readyz、404 和 405 错误响应。

当前基座没有业务路由；就绪状态表示 MySQL 启动检查完成且 HTTP 已开始运行，不持续探测数据库状态。

### 6. 运行和退出

server.New 校验 HTTP 参数并创建 Server。Server.Run 监听端口、发布就绪状态，并持续服务到上下文取消或监听失败。

收到退出信号后，Server 先撤销就绪状态，再调用标准库 Shutdown。关闭期限由 HTTP_SHUTDOWN_TIMEOUT 控制。

正常退出时，HTTP 请求处理结束后 run 才返回并关闭 MySQL 连接池。若 HTTP 关闭超时则返回错误，连接池关闭仍会执行；不能将这一分支视为请求均已完成。Dependencies.Close 的错误通过 errors.Join 合并到返回值中。

## 错误传播

run 使用 fmt.Errorf 保留底层错误，并补充 load config、create logger、create dependencies、create router、create http server 或 run http server 等阶段名称。初始化失败不会继续启动 HTTP，不记录完整配置对象或密码。
