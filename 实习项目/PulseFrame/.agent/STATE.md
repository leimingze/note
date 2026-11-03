# 任务状态索引

## 当前任务

TASK-2026-11-03-CONFIG-ORGANIZATION，config 包已按统一入口、环境读取、HTTP 配置和 MySQL 配置组织，并通过验证。

## 当前方案

backend/config/config.go 定义总配置并组织加载；env.go 保存 LookupEnv、可选环境变量读取、整数与时长解析和共用端口范围；http.go 与 mysql.go 各自负责模块配置。运行环境及日志级别校验保留在统一入口文件，公共解析函数不调用模块加载函数。配置接口、默认值及校验行为保持一致，说明见 backend/README.md。

docs/设计/ 保存5份项目方案与文件索引；docs/学习笔记/ 保存2份通用知识文档及 assets/ 中的4张配图，原有文档和图片内容保持不变。

可用性与高并发治理的职责、进度及连接池讨论范围见 docs/设计/服务可用性与高并发治理.md，该文档引用 MySQL 和 Redis 的独立文件索引。MySQL 通过 go-sql-driver/mysql v1.10.1 和 database/sql 接入；config 读取配置，infra 创建并检查连接，cmd/pulseframe-api/dependencies.go 持有和关闭连接池，main.go 组织启动及退出。单进程默认最多20个连接、5个空闲连接，连接最长存活30分钟、空闲5分钟，均可配置。账号、密码、数据库为显式配置，密码保留原值；启动检查失败直接返回错误。

MySQL 连接池等待使用操作 Context 的截止时间，网络读写超时单独配置；使用方负责释放查询结果、事务和显式获取的连接。Redis 与外部 HTTP 客户端尚未编写；关闭 Redis 客户端自动重试为建议方案，尚未确认。部署连接预算及监测继续讨论。

登录名区分大小写。注册、登录保留原始 username，MySQL 使用 ascii_bin 唯一索引；abc 与 ABC 可分别注册。账号限流 key 直接使用原始 username。IP 限流 key 使用可信来源的 normalizedIp，通过 Go net/netip 解析、Unmap、String 规范化；同一 IPv6 地址的不同写法共用计数。

登录与注册.md 的每个暂定流程已标注相关 MySQL 表、Redis key 及读写操作；登录通过 password_credentials.user_id 关联 users.id，QQ 身份通过 external_identities.user_id 关联 users.id。

登录按账号与 IP 两个维度限流，注册按 IP 限流。Redis ZSET 记录滑动窗口内已允许进入处理的请求，Lua 原子检查并计数；超额返回429，Redis 不可用返回503，窗口和额度通过配置管理。规则见 docs/设计/登录与注册.md 的2.7。

QQ 授权上下文仅保留 browserBindingDigest、targetUserId、targetAuthVersion。登录只写浏览器绑定摘要，绑定账号时写入全部三个字段。重新认证成功后立即创建上下文并设置5分钟 TTL；应用和跳转地址使用服务端固定配置。绑定事务检查账号状态与版本，失败时直接拒绝。

Redis 刷新会话仅保存 userId、authVersion、currentTokenDigest、absoluteExpiresAt 四个字段，分别用于确定用户、校验账号版本、原子轮换令牌和保持固定到期时间。

refresh:{tokenDigest} 使用 Redis String 直接保存 sessionId。轮换时原子删除旧映射、创建新映射并更新会话摘要；旧令牌查不到映射时拒绝刷新，不连带撤销当前会话。退出使用当前令牌，原子删除映射和会话。

登录与注册设计采用用户名密码及 QQ 登录，短期 JWT 配合 Redis 刷新会话。Access Token 最长15分钟，会话绝对期限7天，Refresh Token 强制原子轮换，sessionId 保持不变。MySQL 保存账号状态和 authVersion，登录与刷新检查状态及版本。封禁或全部退出通过账号状态与版本阻止旧会话续期，Redis 数据按原 TTL 到期；刷新发现账号禁用或版本失效时撤销对应会话。业务请求只验签 JWT，退出和封禁后已有 Access Token 可继续使用至到期。依据：docs/设计/登录与注册.md。

代码当前为项目基座及 MySQL 连接池。后端提供配置校验、结构化日志、请求标识、访问日志、异常恢复、统一错误响应、健康探针、HTTP 超时及优雅退出；启动时连接 MySQL，不注册业务路由，不连接 Redis。controller、service、dao 保留目录占位。前端保留现有基础工程。

依据：工程基座提交 2393c31、backend/cmd/pulseframe-api/main.go、backend/server/router.go、backend/go.mod。注册处理、数据库接入、建表脚本及对应测试已移除。

## 有效文件

- docs/README.md：项目设计与学习笔记的统一入口。
- docs/设计/服务可用性与高并发治理.md：职责与进度表，引用 MySQL 和 Redis 文件索引。
- docs/设计/MySQL连接池-文件索引.md：仅列 MySQL 连接池配置、创建、启动管理及依赖清单相关文件与职责；Agent 执行时的测试和文档维护要求不列入表格。
- docs/设计/Redis连接池-文件索引.md：Redis 连接池配置、创建、启动管理及依赖清单相关文件与职责。
- docs/设计/登录与注册.md：当前认证设计，保留“思考、暂定方案”及客户端、服务端步骤格式。
- docs/设计/登录与注册-数据结构.md：三张 MySQL 表的字段与约束、Redis 会话及令牌映射、QQ state 和频率计数。
- docs/学习笔记/注册与发现.md、docs/学习笔记/通用高并发架构设计/高并发架构设计要点.md：通用知识笔记；配图保存在 docs/学习笔记/assets/。
- backend/cmd/pulseframe-api/main.go：启动入口。
- backend/config/config.go、backend/config/env.go、backend/config/http.go、backend/config/mysql.go：统一加载、公共环境解析及各模块配置。
- backend/config/mysql.go、backend/infra/mysql.go、backend/cmd/pulseframe-api/dependencies.go：MySQL 配置、连接创建及生命周期管理。
- backend/test/mysql_config_test.go、backend/test/mysql_test.go、backend/cmd/pulseframe-api/dependencies_test.go：配置校验及真实数据库集成测试。
- backend/config/、backend/logging/、backend/middleware/、backend/response/、backend/server/：基础能力。
- backend/controller/.gitkeep、backend/service/.gitkeep、backend/dao/.gitkeep：业务目录占位。
- backend/test/：基座测试，其中 scaffold_test.go 验证路由集合与原注册路径的 404 响应。
- backend/README.md、backend/docs/run-lifecycle.md：启动、配置与生命周期说明。
- backend/go.mod、backend/go.sum：依赖清单。
- frontend/：前端基础工程。

## 验证情况

配置组织调整（2026-11-03）：复用现有配置及后端测试，go test -race -timeout=60s ./...、go vet -tags=mysqlintegration ./... 通过；检查格式及文件职责。此次未重复运行真实 MySQL 集成测试。

目录整理（2026-11-03）：7份文档和4张图片的 Git 内容校验值与移动前一致；目录统一为 docs/，入口索引标题为“文档索引”；核对索引、文档互链、图片路径及旧路径引用，检查 git diff --check。未修改或运行后端代码。

注释更新（2026-11-03）：补充配置分支、批量时长赋值、连接创建与关闭、测试场景的中文说明，并逐项说明配置字段与包级常量的含义、单位和作用范围；仅修改注释及格式。go test -timeout=60s ./...、go vet -tags=mysqlintegration ./... 和 git diff --check 通过；未重复运行真实 MySQL 集成测试。

本次验证（2026-11-03）：独立 MySQL 8.0.44 实例监听本机13316，数据库 pulseframe_pool_test。backend 目录执行 go test -race -tags=mysqlintegration -timeout=60s ./... 及 go vet -tags=mysqlintegration ./... 均通过。覆盖必填配置、密码原值保留、连接复用、容量限制、等待及查询取消、读超时、启动取消与期限、连接回收、连接错误、依赖关闭及真实 HTTP 启动退出；未进行负载及故障切换测试。测试配置使用 MYSQL_* 环境变量，详见 backend/README.md。

基座既有验证（2026-10-15）：

- backend 目录执行 go test -timeout=60s ./...：通过。
- backend 目录执行 go vet ./...：通过。
- TestScaffoldRoutes 使用真实 Gin 路由及随机请求标识生成器，确认仅有 GET /livez 和 GET /readyz，POST /api/v1/auth/register 返回统一 404。
- 生产入口、依赖及后端说明恢复为 2393c31 中的基座内容。

## 未解决问题

密码哈希参数和请求频率额度须结合部署环境确定；QQ 应用配置、业务接口及并发行为尚未通过运行验证。

## 下一步

继续讨论 Redis 连接池配置、客户端创建、启动检查、重试与关闭行为，并逐项确定连接池讨论表中的其余内容。

## 归档位置

原注册代码保留在 Git 提交 52812e0；基座版本为 2393c31。过程目录 .agent/work/20261015-pulseframe-base/ 已归档并保留原位置，受 .gitignore 保护。

正式文件见上方有效文件索引；MySQL 测试实例数据与日志保留在 .agent/work/mysql-pool/，编辑补丁保留在 /Users/leimingze/notes/开源书籍/.agent/work/20261102-auth-design/，均作为已归档过程目录保留，受 Git 忽略规则保护。
