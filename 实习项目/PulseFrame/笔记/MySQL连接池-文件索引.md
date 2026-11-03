# MySQL 连接池文件索引

所属：[服务可用性与高并发治理](服务可用性与高并发治理.md)。

当前处于设计讨论阶段，以下为文件修改计划，尚未编写连接池代码。路径均相对于 PulseFrame 项目目录。

| MySQL 相关文件 | 变更类型 | 计划内容 |
| --- | --- | --- |
| `backend/config/mysql.go` | 新增 | 定义并校验 MySQL 地址、账号、密码、数据库、连接超时、启动检查超时，以及最大连接数、最大空闲连接数、连接存活和空闲期限；密码按原值读取 |
| `backend/config/config.go` | 修改 | 在现有类型化配置框架中接入 MySQL 配置，不改变基础配置管理方式 |
| `backend/infra/mysql.go` | 新增 | 使用成熟 MySQL 驱动创建 `sql.DB`，设置连接池参数，通过带超时的 `PingContext` 检查连接；初始化失败时释放已创建资源 |
| `backend/cmd/pulseframe-api/dependencies.go` | 新增 | 在启动装配处创建并持有一个 MySQL 客户端，管理初始化失败后的清理及正常退出时的关闭，不使用包级全局客户端 |
| `backend/cmd/pulseframe-api/main.go` | 修改 | 加载配置后创建依赖，再启动 HTTP 服务；HTTP 请求处理结束后关闭数据库连接池 |
| `backend/go.mod`、`backend/go.sum` | 更新依赖 | 引入 `github.com/go-sql-driver/mysql`，连接池由标准库 `database/sql` 管理 |
| `backend/test/mysql_config_test.go` | 新增 | 验证必填项、连接池参数、时长边界及密码原值保留 |
| `backend/test/config_test.go` | 修改 | 调整现有配置测试，覆盖 MySQL 配置接入后的启动校验，并保留基础配置验证 |
| `backend/test/mysql_test.go` | 新增 | 使用真实 MySQL 验证连接、连接池参数、连接失败、超时及关闭行为，测试设置60秒超时 |
| `backend/cmd/pulseframe-api/dependencies_test.go` | 新增 | 验证启动装配和资源释放顺序；访问数据库的场景使用真实 MySQL，测试设置60秒超时 |
| `backend/README.md` | 修改 | 补充 MySQL 环境变量、启动前提和验证命令 |
| `backend/docs/run-lifecycle.md` | 修改 | 补充 MySQL 客户端创建、启动检查、错误传播和退出关闭顺序 |
