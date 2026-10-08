# 任务状态索引

## 当前任务：TASK-2026-10-07-REGISTER-API

### 当前方案

已实现 `POST /api/v1/auth/register`：JSON 输入校验用户名不为空且不超过 24 个字符、密码不为空且不超过 72 字节；bcrypt 生成哈希，MySQL 单条参数化 `INSERT` 自动提交，唯一索引冲突返回 409。成功返回 201，不自动登录，不写 Redis。启动需配置 MySQL 地址、账号、数据库和私有密码；可选 `MYSQL_TLS=true` 用于验证远端直连证书。账号表由 `backend/migrations/001_create_users.sql` 创建。

### 有效文件

- backend/controller/registration.go
- backend/service/registration.go
- backend/dao/user.go
- backend/dao/mysql.go
- backend/config/mysql.go
- backend/migrations/001_create_users.sql
- backend/test/registration_test.go
- backend/README.md
- 设计文档/01-用户与认证模块设计.md

### 验证情况

本机 MySQL 8.0 的独立临时数据库集成测试已通过：成功写入 bcrypt 哈希和默认状态、区分大小写的用户名、重复用户名返回 409、非法请求不写入、数据库故障返回 500、8 个同名并发请求只有 1 个成功且只存入 1 条记录。`go test -timeout=60s ./...`、`go test -race -timeout=60s ./...` 和 `go vet ./...` 已通过。集成测试结束后删除自己创建的临时数据库，不修改现有业务库。

### 未解决问题

目标数据库尚未执行建表 SQL，远端 MySQL 的连接权限、TLS 证书及实际部署环境尚未验证；登录、鉴权和 Redis 接入尚未实现。运行服务前需配置 MySQL 连接环境变量。

### 下一步

部署时在目标数据库建表并使用应用账号配置连接；注册接口完成后按链路继续设计和实现登录。

### 归档位置

无。

## 既有任务：TASK-2026-10-06-AUTH-DESIGN

### 当前方案

用户与认证模块采用 MySQL 保存账号资料及 bcrypt 生成的密码哈希，Redis 保存随机 token 会话及账号状态缓存；鉴权时状态缓存未命中才查 MySQL。登录前校验输入，再按用户名查询 MySQL；注册校验输入、生成密码哈希并插入 MySQL，提交成功即注册成功，不预写 Redis 账号状态。注册链路已写入单独的 Mermaid 图，其余链路仍逐项讨论。用户名使用区分大小写的数据库比较规则，由唯一索引判断重复注册。账号状态缓存的基础有效期为 30 分钟，加 0 到 60 秒随机时间以分散到期，读取不续期；登录会话严格保留 7 天。禁用、恢复或调整权限先提交 MySQL，再同步 Redis，失败时记录异常并立即重试最多 3 次；旧状态可能在同步成功或缓存到期前继续被读取。恢复后未到期会话继续可用；当前设备退出删除对应会话。限流交由独立网关另行设计，MySQL 与 Redis 不保存限流计数。注册接口已实现，其他认证链路尚未实现。远端 Compose 位于 `120.48.147.164:/data/leimingze/docker-compose.yml`，当前 Redis 容器为 `redis:7-alpine`。本地连接凭据保存在项目根目录被 Git 忽略的 `.env`，变量名为 `PULSEFRAME_MYSQL_PASSWORD` 和 `PULSEFRAME_REDIS_PASSWORD`；记忆文件不记录取值，后端接入时从环境变量读取。

### 有效文件

- 设计文档/01-用户与认证模块设计.md
- .env（本地凭据文件，Git 忽略）
- PulseFrame项目总纲.md
- backend/README.md
- memory/MEMORY_PROTOCOL.md

### 验证情况

文档在技术方案中列出 MySQL `users` 的 5 个字段及 Redis 的两类会话与状态记录；仅保留注册链路的 Mermaid 图，图文均以 MySQL 插入并提交成功作为注册成功条件。登录按用户名查询 MySQL；Redis 状态未命中时查 MySQL。MySQL 先提交，Redis 写入失败时重试并返回同步异常，已命中的旧状态仍可能放行。根目录 `.env` 已确认被 Git 忽略，权限为 `600`，仅记录两个环境变量名。远端 Compose 路径、服务、容器健康状态和端口映射此前已只读核对；当前 Redis 容器使用 `redis:7-alpine`。注册接口的实现和验证见当前任务。

### 未解决问题

MySQL 与 Redis 的账号状态同步、持续失败后的修复及同一账号并发变更须在实现时验证；状态同步失败后，旧状态在缓存到期前仍可能放行。随机有效期无法避免 Redis 故障或批量丢失记录引发的集中查询。后端尚未接入 MySQL 和 Redis，也不会自动读取项目根目录的 `.env`。bcrypt 成本参数须根据运行环境确定。目标用户量与流量、参数配置及容量指标待实现阶段验证；首期按同源网页端与用户名密码登录设计。远端服务端口的外部可达性、网络访问控制、应用账号和 Redis 内存策略尚未核验。

### 下一步

先逐项确认注册链路，再按链路确认后续设计；之后接入 MySQL 与 Redis，完成接口、网页端和真实依赖集成测试。

### 归档位置

无。

## 既有任务记录

## 任务编号

TASK-2026-10-06-BUSINESS-DIRS。

## 当前方案

后端 Go 包按职责直接放在 backend/ 下；controller、service 和 dao 目录已创建，业务代码在功能实现时加入。

## 有效文件

- backend/config/
- backend/logging/
- backend/middleware/
- backend/response/
- backend/server/
- backend/controller/
- backend/service/
- backend/dao/
- backend/cmd/pulseframe-api/
- backend/test/
- backend/README.md
- memory/MEMORY_PROTOCOL.md（记忆管理规则）

## 验证情况

- controller、service、dao 目录及占位文件已确认存在，且未被 Git 忽略。
- `git diff --check`：通过。

## 未解决问题

无。

## 下一步

业务功能实现时向对应目录加入代码；项目记忆待重新编写。

## 归档位置

无。
