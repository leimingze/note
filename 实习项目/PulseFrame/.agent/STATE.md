# 任务状态索引

## REVIEW-POLICY 审核策略配置

- 规则文件为 `review-service/policies/moderation.json`，启动时由 `POLICY_FILE` 指定。已加载的策略可按名称启用，到期时使用配置中的默认表。
- 读取、启动切换及到期验证见 `review-service/internal/policy/catalog.go` 与 `review-service/internal/review/decide_test.go`。

## REVIEW-RETRY 机审故障排队

- 故障任务持续重试，默认间隔为 1、2、4 分钟，此后每 4 分钟再试。模型格式补采耗尽时转人审。
- 代码及验证见 `review-service/internal/review/worker.go`、`repository.go` 与 `retry_mysql_test.go`。本地 MySQL 验证六次故障、租约接手和结论写回。带 mysqlintegration 标签的全套 Go 测试与 go vet 通过。测试目录 `.agent/work/review-retry/` 已归档。

## REVIEW-FUNNEL 发布前分批初筛与发布后抽查

- 小 VLM 每批查四类线索并保留 ASR、OCR 的来源与时间。强 VLM 汇总各批命中及相邻帧复核一次，按活动规则表处置。
- 发布后由内容模块投递已发布候选，支持随机或曝光增长抽查。抽中后跳过机审缓存，强模型直接审核原始抽帧与全片文字。
- 设计见仓库根目录 `内容审核/流程设计.md` 附录 G。有效代码及配置为 `review-service/internal/review/funnel.go`、`published.go`、`internal/judge/funnel.go` 和 `policies/published_sampling.json`（均在审核服务内）。
- `go test -race -timeout 60s ./...` 与 `go vet ./...` 通过。双模型部署、真实视频效果和 KuaiRand 曝光数据回放待验证。

## M2-CPU100 挑帧配对评估（2026-11-21）

- 新版挑帧及复现脚本在 `review-service/worker/framesel/{select_frames,paired_eval}.py`。沿用 `rs-fulltest/m3/thresholds.json`，按固定种子从历史评估集抽 100 个原视频，默认启用每条最多一帧的补选。服务器输入在 `/root/autodl-tmp/rs-fulltest/m3`，运行目录为同级 `m2-paired-100`。
- 旧版 533/600，新版 548/600，等帧数覆盖对照 533/600。正常视频平均 8.81 → 9.17 帧。证据见 `review-service/reports/data/m2-paired-100/paired.jsonl.gz` 和仓库根目录 `内容审核/结论.md`。
- 本机与服务器各通过 10 项挑帧测试。原始 MP4 样本 `3x4rtskqqw2sz8i/fade_30` 导出 11 帧，第 80 帧命中可见区间 77–83。跨来源泛化与下游审核效果尚未评估。过程目录 `.agent/work/m2-cpu100/` 已归档。

## 内容与发布设计（2026-11-11）

任务：内容与发布模块数据结构与功能边界评审后的修订。设计文件为 docs/设计/业务模块设计/内容与发布/ 下的主文档、数据结构与实施路线。

本轮确定的约束：

- 草稿使用 `video.status=0`；提交、通过、驳回、下架分别为 1/2/3/4。video 用 7 条状态/时间 CHECK，review_task 用 5 条终态/租约 CHECK，草稿、待审核、已发布、已驳回、已下架不能任意组合。
- 媒体先落 `upload` 凭证再创建视频，数据库存对象键；原始字节哈希 `source_sha256` 用于去重与幂等，faststart/remux 后的最终哈希 `sha256` 用于对象键与完整性校验。
- 公开桶副本与数据库状态分离：`is_public` 表示业务期望，`public_object_present` 表示公开对象真实存在，`is_playable` 供列表与播放接口过滤。公开 key 按 `public/videos/{videoId}/` 隔离，复制与删除由对账任务收敛，接受分钟级窗口。
- 送审任务为 outbox：提交事务只插入 `review_task`，worker 用原子 UPDATE 加租约领取，外部调用在事务外；投递带稳定幂等键，租约覆盖整批最坏耗时。
- 撤回与删除把当前轮任务 1/2/3 置 7；回调允许任务 2/3/7，任务 2/3 按 `status=1 AND review_round=? AND deleted_at IS NULL` 条件更新视频，任务 7 只写任务结论。恢复待审核视频退回草稿。
- 封面凭证由 `video.cover_upload_id` 引用并加唯一索引；换独立封面凭证时旧凭证置 3 后回收，换自动抽帧封面时清空主媒体 upload.cover_key。
- 幂等键在 Redis 记录请求指纹与响应，multipart 指纹按语义字段与 `source_sha256` 拼接；字节配额按 Content-Length 预留。

验证：三份设计文档、docs/README.md 与本状态文件已完成互链和术语核对；`git diff --check` 通过。本轮只改设计文档，未编写或运行代码。

下一步：按实施路线 P0 继续实现对象存储接口（MinIO 与内存实现）、CDN 接口与媒体探测。

## 当前任务

TASK-2026-11-04-REDIS-POOL，10份后端测试文件统一保存在 backend/test；启动逻辑和依赖管理位于 backend/internal/app，main.go 保留入口。Redis 配置、客户端创建和依赖生命周期已实现，真实 MySQL 8.4 与 Redis 7 服务验证已通过。

## 当前方案

backend/config/config.go 定义总配置并组织加载；env.go 保存 LookupEnv、可选环境变量读取、整数与时长解析和共用端口范围；http.go、mysql.go 与 redis.go 各自负责模块配置。运行环境及日志级别校验保留在统一入口文件，公共解析函数不调用模块加载函数。配置接口、默认值及校验行为保持一致，说明见 backend/README.md。

docs/设计/ 保存6份项目方案与文件索引；docs/学习笔记/ 保存2份通用知识文档及 assets/ 中的4张配图，原有文档和图片内容保持不变。

可用性与高并发治理的职责、进度及连接池讨论范围见 docs/设计/服务可用性与高并发治理.md，该文档引用 MySQL 和 Redis 的独立文件索引。日志与指标方案见 docs/设计/日志与指标设计.md：保留现有 slog、请求 ID、访问日志和异常恢复，指标采用 github.com/prometheus/client_golang；`/livez` 只检查进程，`/readyz` 聚合 HTTP、MySQL 和 Redis 的后台探活状态，`error_code` 与 `error_type` 分别维护固定集合。MySQL 通过 go-sql-driver/mysql v1.10.1 和 database/sql 接入，Redis 通过 github.com/redis/go-redis/v9 v9.7.0 接入；config 读取配置，infra 创建并检查连接，internal/app/dependencies.go 持有和关闭客户端，internal/app/run.go 组织启动及退出，main.go 调用 app.Run。MySQL 单进程默认最多20个连接、5个空闲连接；Redis 默认基础连接10个、最大同时分配20个连接、最小空闲2个，均可配置。两者连接最长存活30分钟、空闲5分钟，启动检查失败直接返回错误。账号、密码、数据库为显式配置，密码保留原值。

MySQL 连接池等待使用操作 Context 的截止时间，网络读写超时单独配置；Redis 连接池等待使用 PoolTimeout 和操作 Context，客户端默认每条命令最多自动重试3次；写操作是否允许重试仍由业务语义决定。使用方负责释放查询结果、事务和显式获取的连接。外部 HTTP 客户端已建立文件索引、QQ 授权码登录流程和通用 HTTPS 客户端边界，代码尚未编写。部署连接预算及监测继续讨论。

登录名区分大小写。注册、登录保留原始 username，MySQL 使用 ascii_bin 唯一索引；abc 与 ABC 可分别注册。账号限流 key 直接使用原始 username。IP 限流 key 使用可信来源的 normalizedIp，通过 Go net/netip 解析、Unmap、String 规范化；同一 IPv6 地址的不同写法共用计数。

登录与注册.md 的每个暂定流程已标注相关 MySQL 表、Redis key 及读写操作；登录通过 password_credentials.user_id 关联 users.id，QQ 身份通过 external_identities.user_id 关联 users.id。

登录按账号与 IP 两个维度限流，注册按 IP 限流。Redis ZSET 记录滑动窗口内已允许进入处理的请求，Lua 原子检查并计数；超额返回429，Redis 不可用返回503，窗口和额度通过配置管理。规则见 docs/设计/登录与注册.md 的2.7。

QQ 授权上下文仅保留 browserBindingDigest、targetUserId、targetAuthVersion。登录只写浏览器绑定摘要，绑定账号时写入全部三个字段。重新认证成功后立即创建上下文并设置5分钟 TTL；应用和跳转地址使用服务端固定配置。绑定事务检查账号状态与版本，失败时直接拒绝。

Redis 刷新会话仅保存 userId、authVersion、currentTokenDigest、absoluteExpiresAt 四个字段，分别用于确定用户、校验账号版本、原子轮换令牌和保持固定到期时间。

refresh:{tokenDigest} 使用 Redis String 直接保存 sessionId。轮换时原子删除旧映射、创建新映射并更新会话摘要；旧令牌查不到映射时拒绝刷新，不连带撤销当前会话。退出使用当前令牌，原子删除映射和会话。

登录与注册设计采用用户名密码及 QQ 登录，短期 JWT 配合 Redis 刷新会话。Access Token 最长15分钟，会话绝对期限7天，Refresh Token 强制原子轮换，sessionId 保持不变。MySQL 保存账号状态和 authVersion，登录与刷新检查状态及版本。封禁或全部退出通过账号状态与版本阻止旧会话续期，Redis 数据按原 TTL 到期；刷新发现账号禁用或版本失效时撤销对应会话。业务请求只验签 JWT，退出和封禁后已有 Access Token 可继续使用至到期。依据：docs/设计/登录与注册.md。

代码当前为项目基座及 MySQL、Redis 连接池。后端提供配置校验、结构化日志、请求标识、访问日志、异常恢复、统一错误响应、健康探针、HTTP 超时及优雅退出；启动时检查 MySQL 和 Redis，不注册业务路由。controller、service、dao 保留目录占位。前端保留现有基础工程。

依据：工程基座提交 2393c31、backend/cmd/pulseframe-api/main.go、backend/server/router.go、backend/go.mod。注册处理、数据库接入、建表脚本及对应测试已移除。

## 有效文件

- docs/README.md：项目设计与学习笔记的统一入口。
- docs/设计/业务模块设计/内容与发布/内容与发布.md：上传、草稿、审核、下架、删除与播放地址的主方案。
- docs/设计/业务模块设计/内容与发布/内容与发布-数据结构.md：upload、video、video_stat、review_task 四张表及 Redis key-value。
- docs/设计/业务模块设计/内容与发布/内容与发布-字段设计说明.md：video 与 review_task 各字段及取值的设计理由。
- docs/设计/业务模块设计/内容与发布/内容与发布-实施路线.md：P0–P5 分期交付与阶段验收。
- docs/设计/服务可用性与高并发治理.md：职责与进度表，引用 MySQL 和 Redis 文件索引。
- docs/设计/日志与指标设计.md：结构化日志字段、敏感信息规则、Prometheus 指标和后续实现职责。
- docs/设计/MySQL连接池-文件索引.md：仅列 MySQL 连接池配置、创建、启动管理及依赖清单相关文件与职责；Agent 执行时的测试和文档维护要求不列入表格。
- docs/设计/Redis连接池-文件索引.md：Redis 连接池配置、创建、启动管理及依赖清单相关文件与职责。
- docs/设计/外部HTTP客户端-文件索引.md：外部 HTTP 客户端的文件职责、通用边界和后续实现计划。
- docs/设计/基座/MySQL基座-处理流程.md：按函数职责聚合的 MySQL 连接池启动、运行和退出流程图。
- docs/设计/登录与注册.md：当前认证设计，保留“思考、暂定方案”及客户端、服务端步骤格式。
- docs/设计/登录与注册-数据结构.md：三张 MySQL 表的字段与约束、Redis 会话及令牌映射、QQ state 和频率计数。
- docs/学习笔记/注册与发现.md、docs/学习笔记/通用高并发架构设计/高并发架构设计要点.md：通用知识笔记；配图保存在 docs/学习笔记/assets/。
- backend/cmd/pulseframe-api/main.go：启动入口。
- backend/internal/app/run.go：可由入口和后端测试调用的应用启动与退出流程。
- backend/config/config.go、backend/config/env.go、backend/config/http.go、backend/config/mysql.go、backend/config/redis.go：统一加载、公共环境解析及各模块配置。
- backend/config/mysql.go、backend/config/redis.go、backend/infra/mysql.go、backend/infra/redis.go、backend/internal/app/dependencies.go：MySQL/Redis 配置、客户端创建及生命周期管理。
- backend/test/mysql_config_test.go、backend/test/mysql_test.go、backend/test/redis_config_test.go、backend/test/redis_test.go、backend/test/dependencies_test.go：配置校验及真实数据库、Redis 集成测试。
- backend/config/、backend/logging/、backend/middleware/、backend/response/、backend/server/：基础能力。
- backend/controller/.gitkeep、backend/service/.gitkeep、backend/dao/.gitkeep：业务目录占位。
- backend/test/：基座测试，其中 scaffold_test.go 验证路由集合与原注册路径的 404 响应。
- backend/internal/model/video.go、backend/internal/model/review_task.go：内容发布模块 video 与 review_task 两表的 gorm 模型。
- backend/internal/migration/migrate.go：迁移登记 video 与 review_task 两表，并使用 InnoDB 与 utf8mb4。
- backend/test/migration_video_test.go：验证建表、索引、唯一约束、字符集与迁移可重放。
- backend/internal/platform/storage/storage.go、memory.go、s3.go：对象存储接口、内存实现与 minio-go（S3 兼容）实现。
- backend/internal/platform/storage/storage_test.go、s3_integration_test.go：内存实现一致性测试与真实 S3 集成测试（s3integration 标签）。
- backend/internal/platform/cdn/cdn.go、cdn_test.go：CDN 接口本地实现与测试。
- backend/README.md：启动、配置与生命周期说明。
- backend/go.mod、backend/go.sum：依赖清单。
- frontend/：前端基础工程。

## 验证情况

内容与发布存储接口（2026-11-12）：对象存储接口与内存、S3 双实现落地；内存实现一致性测试、CDN 测试、go build、go vet（含 s3integration 标签）通过。依赖新增 github.com/minio/minio-go/v7 v7.4.1。SeaweedFS 4.42 已在服务器 120.48.147.164 以 Docker Compose 部署，S3 端点映射宿主端口 30057（master 9333、volume 8080）。

内容与发布建表（2026-11-11）：本地 MySQL 8.0.44（数据目录 .agent/work/mysql-pool，端口 13316）运行 TestMigrationVideoTables 通过，重复执行迁移结果稳定；既有 TestMySQL* 连接池测试仍通过。审查 build 与 go vet 通过，改动文件已 gofmt。

测试目录整理（2026-11-03）：go test -timeout=60s ./...、go test -race -tags=mysqlintegration -timeout=60s ./...、go vet -tags=mysqlintegration ./... 均通过；测试使用本机13316端口的独立 MySQL 8.0.44 和 pulseframe_pool_test 数据库，测试实例已关闭。核对8份后端测试文件均位于 backend/test，相关索引无旧路径，修改文件的 gofmt 和 git diff --check 通过。既有测试场景保留，入口通过 app.Run 调用相同启动流程。

Redis 连接池实现（2026-11-04）：config/redis.go 读取并校验 Redis 地址、认证、逻辑数据库、连接池容量、等待期限、读写期限、启动检查期限和连接回收期限；infra/redis.go 使用 go-redis/v9 创建客户端并执行带超时的 PING；internal/app/dependencies.go 在 Redis 初始化失败时关闭已创建的 MySQL 连接池，并在退出时只关闭一次两个客户端。新增 redis_config_test.go 和 redis_test.go；普通测试、带标签编译检查及真实 Redis 服务读写均通过。

项目注释更新（2026-11-03）：后端 go test -timeout=60s ./...、go vet -tags=mysqlintegration ./...，前端 npm test 和 npm run build 均通过；gofmt 和本次文件的 git diff --check 通过。标准库扫描器确认23份 Go 文件修改前后的代码词法单元一致，所有具名函数文档包含输入、输出、功能和示例；前端6份文件差异均为注释。未重新运行 MySQL 集成测试。验证脚本：/Users/leimingze/notes/开源书籍/.agent/work/20261102-auth-design/verify_go_comments.go，以修改前 Git HEAD 和工作区文件为输入。

调用注释更新（2026-11-03）：四个配置文件的内部注释按处理顺序说明环境读取、默认值选择、类型转换和配置校验，并使用具体输入示例帮助理解；仅修改注释。go test -timeout=60s ./...、gofmt 格式检查及本次文件的 git diff --check 通过。

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

P0 建表与对象存储、CDN 接口代码已完成（内存测试通过）。下一步：在服务器 SeaweedFS S3 端点（120.48.147.164:30057）上运行 s3integration 集成测试，并实现媒体探测（ffprobe）及样本验证。

## 归档位置

原注册代码保留在 Git 提交 52812e0；基座版本为 2393c31。过程目录 .agent/work/20261015-pulseframe-base/ 已归档并保留原位置，受 .gitignore 保护。

正式文件见上方有效文件索引；MySQL 测试实例数据与日志保留在 .agent/work/mysql-pool/，编辑补丁保留在 /Users/leimingze/notes/开源书籍/.agent/work/20261102-auth-design/，均作为已归档过程目录保留，受 Git 忽略规则保护。
