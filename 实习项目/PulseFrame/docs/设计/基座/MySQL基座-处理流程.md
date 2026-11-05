# MySQL 连接池基座处理流程

所属：[服务可用性与高并发治理](服务可用性与高并发治理.md)。

本文复盘 MySQL 基座从启动装配、配置校验、连接池创建、运行期复用到退出关闭的完整链路，用于后续代码复盘。图内节点标注相对路径，路径均相对 PulseFrame 项目根。

## 完整链路

```mermaid
flowchart TD
    subgraph S1["启动阶段"]
        direction TB
        A["main.go: main()<br/>准备 context.Background 和 os.LookupEnv<br/>调用 app.Run；失败时记录错误并退出"] --> B["internal/app/run.go: Run(parent, lookup)<br/>组织配置、信号、依赖、路由和 HTTP 服务的完整生命周期"]
        B --> C["config.Load(lookup)<br/>读取 APP_ENV、LOG_LEVEL、HTTP、MySQL、Redis 配置<br/>完成文字转换、默认值填充和总配置校验"]
        C -- "返回错误" --> C1["配置错误出口<br/>没有创建外部连接，Run 返回错误，main 记录并退出"]
        C --> D["logging.New + signal.NotifyContext<br/>创建 JSON 结构化日志器<br/>建立可响应 SIGINT/SIGTERM 的服务上下文"]
        D --> E["internal/app: NewDependencies(ctx, cfg)<br/>只创建当前进程共享的一套 MySQL 和 Redis 客户端<br/>成功后由 Dependencies 持有关闭责任"]
        E --> F["infra.NewMySQL(ctx, cfg.MySQL)<br/>再次校验配置；映射 driverConfig；创建 Connector 和 sql.DB<br/>设置连接数量、生命周期和读写期限，再用 PingContext 检查连通性"]
        F -- "配置或 Ping 失败" --> F1["MySQL 初始化失败<br/>关闭已创建的 sql.DB，不把未就绪连接池交给上层"]
        F --> G["infra.NewRedis(ctx, cfg.Redis)<br/>把配置映射到 go-redis Options<br/>设置连接池、读写期限和重试次数，再用带期限的 Ping 检查"]
        G -- "配置或 Ping 失败" --> G1["Redis 初始化失败<br/>关闭已创建的 MySQL 连接池，并返回组合错误"]
        G --> H["server.NewRouter + server.New<br/>创建请求 ID、访问日志、异常恢复和健康探针<br/>把路由与 HTTP 超时配置装配成未监听的 Server"]
        H --> I["httpServer.Run(ctx)<br/>绑定 HTTP 监听地址并处理请求<br/>监听成功后发布进程就绪状态，取消上下文后开始优雅关闭"]
    end

    subgraph S2["运行阶段"]
        direction TB
        I --> R1["请求中间件链<br/>Request ID 先建立关联标识，Access Log 记录结果，Recovery 捕获 panic<br/>业务 Handler 从依赖对象借用 MySQL 或 Redis 客户端"]
        R1 --> R2["sql.DB / redis.Client<br/>客户端按配置复用连接；没有可用连接时按池规则等待<br/>调用方使用带期限的 Context 执行查询、事务或命令"]
        R2 --> R3["连接池内部管理<br/>MySQL 受 MaxOpenConns 和 MaxIdleConns 约束<br/>Redis 受 PoolSize、MaxActiveConns、PoolTimeout 约束"]
        R3 --> R4["请求结束后的资源释放<br/>查询结果、事务和显式获取的连接由使用方关闭<br/>客户端按最大存活时间和空闲时间回收连接"]
    end

    subgraph S3["退出阶段"]
        direction TB
        R4 --> X1["ctx 取消或收到退出信号<br/>HTTP Server 停止接收新请求，并等待处理中请求完成"]
        X1 --> X2["Dependencies.Close()<br/>closeOnce.Do 确保重复或并发调用只执行一次<br/>关闭 MySQL 连接池和 Redis 客户端"]
        X2 --> X3["关闭结果汇总<br/>closeDependency 为失败标注依赖名称<br/>errors.Join 合并多个关闭错误并返回 Run"]
        X3 --> X4["main.go 结束进程<br/>记录最终错误；正常退出返回0，运行失败返回非零状态"]
    end

    subgraph S4["配置要点"]
        direction TB
        P0["连接身份与目标<br/>MYSQL_ADDR 默认 127.0.0.1:3306<br/>MYSQL_USER、MYSQL_DATABASE 必填；MYSQL_PASSWORD 必须显式设置"]
        P1["连接安全模式<br/>MYSQL_TLS_MODE 默认 false<br/>只接受 true 或 false；true 使用 TLS 校验证书"]
        P2["连接数量<br/>MYSQL_MAX_OPEN_CONNS 默认20：使用中和空闲连接总上限<br/>MYSQL_MAX_IDLE_CONNS 默认5：归还后最多保留的空闲连接"]
        P3["网络与启动期限<br/>Connect 3s、Read 5s、Write 5s<br/>Startup Ping 5s；SQL 整体期限由调用方 Context 控制"]
        P4["连接回收期限<br/>ConnMaxLifetime 默认30分钟<br/>ConnMaxIdleTime 默认5分钟；只回收符合条件的连接"]
        P5["配置关系校验<br/>地址必须是非空 host:port 且端口合法<br/>MaxOpenConns > 0；MaxIdleConns 在0到MaxOpenConns之间；所有时长 > 0"]
    end

    P0 -. "提供连接信息" .-> C
    P1 -. "提供安全模式" .-> C
    P2 -. "约束连接池容量" .-> R3
    P3 -. "约束建连、读写和启动检查" .-> F
    P4 -. "约束连接回收" .-> R3
    P5 -. "校验失败即停止" .-> F
```

图中每个节点先写真实代码位置或方法名称，再用自然语言说明它做什么。一个函数内部的连续操作合并在同一节点中，只有会改变流程方向的失败出口单独列出。

## 运行期的注意点

- 并发安全:共享连接池由 Go 标准库管理,业务并发借用同一连接池。
- 数量关系:`MaxOpenConns` 是进程级硬上限,同时容纳使用中与空闲连接;等待上限释放的连接排队,排队时间不计入 `ConnectTimeout`。
- 回收规则:`ConnMaxLifetime` 到期回收长期连接,`ConnMaxIdleTime` 到期清理长期空闲连接,不会强行中断正在执行的操作。
- 超时分工:驱动的 `ReadTimeout`/`WriteTimeout` 只限制单次网络读写,SQL 整体期限由调用方的 Context 控制,两者职责不同。
- 使用方负责释放查询结果、事务与显式获取的连接。
