# PulseFrame Backend

当前目录是 PulseFrame 核心 API 的工程基座。它提供配置校验、MySQL 连接池、结构化日志、请求标识、访问日志、异常恢复、统一错误响应、健康检查、HTTP 超时和优雅退出。启动需要 MySQL 连通，Redis 尚未接入。

## 目录

config、infra、logging、middleware、response 和 server 分别负责配置、外部客户端创建、日志、HTTP 中间件、错误响应和服务生命周期。cmd/pulseframe-api/main.go 是程序入口，调用 internal/app.Run；internal/app/run.go 组织启动与退出，internal/app/dependencies.go 持有并关闭进程共享的 MySQL 连接池。所有后端测试统一保存在 test/，通过可导入的应用包验证启动和依赖管理。controller、service 和 dao 已建立为业务职责目录。

config 包按职责组织：

| 文件 | 职责 |
| --- | --- |
| `config/config.go` | 总配置 Config、统一加载入口 Load，以及应用环境和日志级别校验 |
| `config/env.go` | LookupEnv、可选环境变量读取、整数与时长解析，以及共用端口范围 |
| `config/http.go` | HTTP 配置类型、默认值、加载和监听参数校验 |
| `config/mysql.go` | MySQL 配置类型、默认值、加载和连接池参数校验 |

Load 组织各模块加载；HTTP 和 MySQL 模块使用 env.go 中的公共解析函数。所有文件属于同一个 config 包，包外通过 config.Load 获取总配置。

## 启动

先创建数据库并配置 MYSQL_USER、MYSQL_PASSWORD、MYSQL_DATABASE，应用不会自动建库或建表。在 backend/ 目录执行：

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
| MYSQL_ADDR | 127.0.0.1:3306 | MySQL 的 host:port，IPv6 使用 [地址]:端口 |
| MYSQL_USER | 必填 | 数据库账号，按原值读取 |
| MYSQL_PASSWORD | 必须显式设置 | 密码按原值读取；允许显式空字符串，是否允许连接由数据库认证决定 |
| MYSQL_DATABASE | 必填 | 已存在且账号有权访问的数据库 |
| MYSQL_TLS_MODE | false | false 使用普通连接；true 启用 TLS 并验证服务器证书及主机名 |
| MYSQL_MAX_OPEN_CONNS | 20 | 每个进程的最大连接数，必须大于零 |
| MYSQL_MAX_IDLE_CONNS | 5 | 最大空闲连接数，允许零，不得超过最大连接数 |
| MYSQL_CONNECT_TIMEOUT | 3s | 建立网络连接的超时 |
| MYSQL_READ_TIMEOUT | 5s | 单次网络读取超时 |
| MYSQL_WRITE_TIMEOUT | 5s | 单次网络写入超时 |
| MYSQL_STARTUP_TIMEOUT | 5s | 启动 PingContext 的整体期限，父上下文可以提前取消 |
| MYSQL_CONN_MAX_LIFETIME | 30m | 连接最长存活时间，到期连接由连接池回收 |
| MYSQL_CONN_MAX_IDLE_TIME | 5m | 连接最长空闲时间，由连接池清理空闲连接 |

所有 MySQL 时长必须为正数，支持 Go 时长格式，如 250ms、3s、5m。可选参数未设置或为空时使用默认值。多个后端实例各自创建连接池，部署时按实例数乘以单实例上限计算连接预算，并为管理和其他程序预留连接。

sql.DB 本身管理连接池，DAO 借用共享对象，不自行创建或关闭。查询、执行和事务应使用带截止时间的 Context；连接池满载时，等待连接的期限也由该 Context 控制，网络读写超时不限制排队时间。查询结果、事务和显式获取的连接由使用方及时释放。

## 接口

- GET /livez：进程可以响应时返回 200。
- GET /readyz：MySQL 启动检查成功且 HTTP 服务开始运行后返回 200；该探针不持续查询数据库。
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

真实 MySQL 集成测试需要配置一个独立测试数据库，沿用上表的 MYSQL_* 环境变量；缺失配置或无法连接时直接失败。测试会建立连接、执行只读查询与 SLEEP，不创建业务表。

~~~bash
go test -race -tags=mysqlintegration -timeout=60s ./...
go vet -tags=mysqlintegration ./...
~~~

启动调用顺序为 cmd/pulseframe-api/main.go → internal/app.Run → NewDependencies → infra.NewMySQL；数据库检查成功后启动 HTTP 服务，服务退出时统一关闭依赖。
