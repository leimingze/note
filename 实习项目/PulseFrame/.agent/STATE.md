# 任务状态索引

## 当前任务

TASK-2026-11-02-AUTH-DESIGN，文档修改已完成。

## 当前方案

登录与注册设计采用用户名密码及 QQ 登录，短期 JWT 配合 Redis 刷新会话。Access Token 最长15分钟，会话绝对期限7天，Refresh Token 强制原子轮换；用户索引关联稳定 sessionId。MySQL 保存账号状态和 authVersion，登录与刷新检查状态及版本。封禁或全部退出通过账号状态与版本阻止旧会话续期，Redis 数据按原 TTL 到期；刷新发现账号禁用或版本失效时撤销对应会话。业务请求只验签 JWT，退出和封禁后已有 Access Token 可继续使用至到期。依据：笔记/登录与注册.md。

代码当前仅保留项目基座。后端提供配置校验、结构化日志、请求标识、访问日志、异常恢复、统一错误响应、健康探针、HTTP 超时及优雅退出；不注册业务路由，不连接 MySQL 或 Redis。controller、service、dao 保留目录占位。前端保留现有基础工程。

依据：工程基座提交 2393c31、backend/cmd/pulseframe-api/main.go、backend/server/router.go、backend/go.mod。注册处理、数据库接入、建表脚本及对应测试已移除。

## 有效文件

- 笔记/登录与注册.md：当前认证设计，保留“思考、暂定方案”及客户端、服务端步骤格式。
- 笔记/登录与注册-数据结构.md：三张 MySQL 表的字段与约束、Redis 会话及令牌映射、用户索引、QQ state 和频率计数。
- backend/cmd/pulseframe-api/main.go：启动入口。
- backend/config/、backend/logging/、backend/middleware/、backend/response/、backend/server/：基础能力。
- backend/controller/.gitkeep、backend/service/.gitkeep、backend/dao/.gitkeep：业务目录占位。
- backend/test/：基座测试，其中 scaffold_test.go 验证路由集合与原注册路径的 404 响应。
- backend/README.md、backend/docs/run-lifecycle.md：启动、配置与生命周期说明。
- backend/go.mod、backend/go.sum：依赖清单。
- frontend/：前端基础工程。

## 验证情况

本次文档：已核对封禁、全部退出、刷新版本校验及 Redis 绝对期限的关系。MySQL 仅设计 users、password_credentials、external_identities 三张表。未执行接口或数据库测试，文档中的流程与表结构尚未编写为业务代码或建表脚本。

基座既有验证（2026-10-15）：

- backend 目录执行 go test -timeout=60s ./...：通过。
- backend 目录执行 go vet ./...：通过。
- TestScaffoldRoutes 使用真实 Gin 路由及随机请求标识生成器，确认仅有 GET /livez 和 GET /readyz，POST /api/v1/auth/register 返回统一 404。
- 生产入口、依赖及后端说明恢复为 2393c31 中的基座内容。

## 未解决问题

密码哈希参数和请求频率额度须结合部署环境确定；QQ 应用配置、业务接口及并发行为尚未通过运行验证。

## 下一步

等待业务代码编写要求。

## 归档位置

原注册代码保留在 Git 提交 52812e0；基座版本为 2393c31。过程目录 .agent/work/20261015-pulseframe-base/ 已归档并保留原位置，受 .gitignore 保护。

本次正式文档为笔记/登录与注册.md、笔记/登录与注册-数据结构.md；编辑补丁位于 /Users/leimingze/notes/开源书籍/.agent/work/20261102-auth-design/，已归档并受该目录的 Git 忽略规则保护。
