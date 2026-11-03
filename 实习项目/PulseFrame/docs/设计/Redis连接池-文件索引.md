# Redis 连接池文件索引

所属：[服务可用性与高并发治理](服务可用性与高并发治理.md)。

当前处于设计讨论阶段，以下为文件修改计划，尚未编写连接池代码。路径均相对于 PulseFrame 项目目录。

| 相关文件 | 变更类型 | 职责 |
| --- | --- | --- |
| `backend/config/redis.go` | 新增 | 定义并校验 Redis 地址、认证信息、数据库编号、连接池参数及超时配置；密码按原值读取 |
| `backend/config/config.go` | 修改 | 在现有类型化配置框架中接入 Redis 配置 |
| `backend/infra/redis.go` | 新增 | 使用 go-redis 创建客户端并配置连接池，通过带超时的 PING 检查连接；初始化失败时释放已创建资源；自动重试行为待讨论确定 |
| `backend/cmd/pulseframe-api/dependencies.go` | 修改 | 调用 Redis 创建函数，持有共享客户端并统一管理关闭；Redis 初始化失败时，清理此前已创建的 MySQL 连接池 |
| `backend/cmd/pulseframe-api/main.go` | 修改 | 通过统一依赖入口获得 Redis 客户端，传给需要它的模块；HTTP 请求处理结束后统一关闭依赖 |
| `backend/go.mod`、`backend/go.sum` | 更新依赖 | 引入 `github.com/redis/go-redis/v9`，由客户端管理连接池 |
