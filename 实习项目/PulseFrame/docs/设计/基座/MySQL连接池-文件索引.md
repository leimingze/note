# MySQL 连接池文件索引

所属：[服务可用性与高并发治理](服务可用性与高并发治理.md)。

MySQL 连接池代码已完成，并通过真实 MySQL 集成测试、竞态检查和静态检查。以下记录本次文件变更及职责，路径均相对于 PulseFrame 项目目录。

| 相关文件 | 变更类型 | 职责 |
| --- | --- | --- |
| `backend/config/mysql.go` | 新增 | 定义并校验 MySQL 地址、账号、密码、数据库、连接超时、启动检查超时，以及最大连接数、最大空闲连接数、连接存活和空闲期限；密码按原值读取 |
| `backend/config/config.go` | 修改 | 定义总配置 Config，通过 Load 组织各模块加载并汇总 MySQL 配置 |
| `backend/config/env.go` | 新增 | 提供 LookupEnv、环境变量读取、整数和时长解析，以及 HTTP 与 MySQL 共用的端口范围 |
| `backend/infra/mysql.go` | 新增 | 使用成熟 MySQL 驱动创建 `sql.DB`，设置连接池参数，通过带超时的 `PingContext` 检查连接；初始化失败时释放已创建资源 |
| `backend/internal/app/dependencies.go` | 新增 | 在启动装配处创建并持有一个 MySQL 客户端，管理初始化失败后的清理及正常退出时的关闭，不使用包级全局客户端 |
| `backend/internal/app/run.go` | 新增 | 加载配置后创建依赖，再启动 HTTP 服务；HTTP 请求处理结束后关闭数据库连接池 |
| `backend/cmd/pulseframe-api/main.go` | 修改 | 将进程环境传给 app.Run，启动或运行失败时记录错误并以非零状态退出 |
| `backend/go.mod`、`backend/go.sum` | 更新依赖 | 引入 `github.com/go-sql-driver/mysql`，连接池由标准库 `database/sql` 管理 |
