# 任务状态索引

## 当前任务

TASK-2026-11-02-AUTH-DESIGN，文档修改已完成。

## 当前方案

可用性与高并发治理要求见笔记/服务可用性与高并发治理.md：当前明确超时、连接池、重试与幂等、频率及并发限制、资源隔离、有限排队和故障响应；熔断、高可用、自适应限流、缓存及数据分片按目标与监测证据选择。

登录名区分大小写。注册、登录保留原始 username，MySQL 使用 ascii_bin 唯一索引；abc 与 ABC 可分别注册。账号限流 key 直接使用原始 username。IP 限流 key 使用可信来源的 normalizedIp，通过 Go net/netip 解析、Unmap、String 规范化；同一 IPv6 地址的不同写法共用计数。

登录与注册.md 的每个暂定流程已标注相关 MySQL 表、Redis key 及读写操作；登录通过 password_credentials.user_id 关联 users.id，QQ 身份通过 external_identities.user_id 关联 users.id。

登录按账号与 IP 两个维度限流，注册按 IP 限流。Redis ZSET 记录滑动窗口内已允许进入处理的请求，Lua 原子检查并计数；超额返回429，Redis 不可用返回503，窗口和额度通过配置管理。规则见笔记/登录与注册.md 的2.7。

QQ 授权上下文仅保留 browserBindingDigest、targetUserId、targetAuthVersion。登录只写浏览器绑定摘要，绑定账号时写入全部三个字段。重新认证成功后立即创建上下文并设置5分钟 TTL；应用和跳转地址使用服务端固定配置。绑定事务检查账号状态与版本，失败时直接拒绝。

Redis 刷新会话仅保存 userId、authVersion、currentTokenDigest、absoluteExpiresAt 四个字段，分别用于确定用户、校验账号版本、原子轮换令牌和保持固定到期时间。

refresh:{tokenDigest} 使用 Redis String 直接保存 sessionId。轮换时原子删除旧映射、创建新映射并更新会话摘要；旧令牌查不到映射时拒绝刷新，不连带撤销当前会话。退出使用当前令牌，原子删除映射和会话。

登录与注册设计采用用户名密码及 QQ 登录，短期 JWT 配合 Redis 刷新会话。Access Token 最长15分钟，会话绝对期限7天，Refresh Token 强制原子轮换，sessionId 保持不变。MySQL 保存账号状态和 authVersion，登录与刷新检查状态及版本。封禁或全部退出通过账号状态与版本阻止旧会话续期，Redis 数据按原 TTL 到期；刷新发现账号禁用或版本失效时撤销对应会话。业务请求只验签 JWT，退出和封禁后已有 Access Token 可继续使用至到期。依据：笔记/登录与注册.md。

代码当前仅保留项目基座。后端提供配置校验、结构化日志、请求标识、访问日志、异常恢复、统一错误响应、健康探针、HTTP 超时及优雅退出；不注册业务路由，不连接 MySQL 或 Redis。controller、service、dao 保留目录占位。前端保留现有基础工程。

依据：工程基座提交 2393c31、backend/cmd/pulseframe-api/main.go、backend/server/router.go、backend/go.mod。注册处理、数据库接入、建表脚本及对应测试已移除。

## 有效文件

- 笔记/服务可用性与高并发治理.md：认证模块的可用性治理、引入条件、指标和验证场景。
- 笔记/登录与注册.md：当前认证设计，保留“思考、暂定方案”及客户端、服务端步骤格式。
- 笔记/登录与注册-数据结构.md：三张 MySQL 表的字段与约束、Redis 会话及令牌映射、QQ state 和频率计数。
- backend/cmd/pulseframe-api/main.go：启动入口。
- backend/config/、backend/logging/、backend/middleware/、backend/response/、backend/server/：基础能力。
- backend/controller/.gitkeep、backend/service/.gitkeep、backend/dao/.gitkeep：业务目录占位。
- backend/test/：基座测试，其中 scaffold_test.go 验证路由集合与原注册路径的 404 响应。
- backend/README.md、backend/docs/run-lifecycle.md：启动、配置与生命周期说明。
- backend/go.mod、backend/go.sum：依赖清单。
- frontend/：前端基础工程。

## 验证情况

本次文档：已核对治理要求与现有认证方案的关系，包括三类限流 key、刷新结果不确定时重新登录、依赖故障返回503，以及认证会话不能从用户表重建。未执行接口、数据库或故障切换测试，治理措施为设计要求。

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

本次正式文档为笔记/登录与注册.md、笔记/登录与注册-数据结构.md、笔记/服务可用性与高并发治理.md；编辑补丁位于 /Users/leimingze/notes/开源书籍/.agent/work/20261102-auth-design/，已归档并受该目录的 Git 忽略规则保护。
