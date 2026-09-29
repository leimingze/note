# PulseFrame Backend

当前目录包含 PulseFrame 核心 API 工程基础和第一版用户认证模块。API 启动依赖 MySQL 与 Redis；配置缺失、连接失败或用户表迁移未执行时会拒绝启动，不使用内存模式兜底。

## 启动

先创建专用数据库和最小权限账号，并通过受控网络访问 MySQL/Redis。不要把 MySQL `root` 用作应用账号，也不要将凭据提交到仓库。应用启动不会自动创建或修改表；首次部署前由迁移账号执行：

```bash
mysql --host=127.0.0.1 --port=13306 --user=pulseframe_migrator --password --database=pulseframe < migrations/0001_create_users.sql
```

本机开发可通过 SSH 隧道访问远端服务：

```bash
ssh -N \
  -L 127.0.0.1:13306:127.0.0.1:30052 \
  -L 127.0.0.1:16379:127.0.0.1:30054 \
  root@120.48.147.164
```

以下示例假设数据库和 Redis 已通过本机隧道转发，密码从本地安全配置注入。PulseFrame 网页端在 `5173` 被占用时使用 `5174`，API 监听 `8081` 以匹配 Vite 代理：

本机 DSN 的 `tls=skip-verify` 仅用于经 SSH 主机密钥验证的隧道；部署到其他网络环境时应配置 CA 验证的 MySQL TLS。

```bash
export MYSQL_DSN='pulseframe_app:<password>@tcp(127.0.0.1:13306)/pulseframe?charset=utf8mb4&parseTime=true&loc=UTC&tls=skip-verify'
export REDIS_ADDR='127.0.0.1:16379'
export REDIS_USERNAME='pulseframe_api'
export REDIS_PASSWORD='<redis-password>'
export WEB_ORIGIN='http://127.0.0.1:5174'
export AUTH_COOKIE_SECURE='false'
export AUTH_HASH_CONCURRENCY='4'
export HTTP_ADDR='127.0.0.1:8081'
go run ./cmd/pulseframe-api
```

也可以将上述环境变量保存在权限为 `600` 且已被仓库忽略的 `backend/.env.local`，然后在 `backend/` 目录加载后启动：

```bash
set -a
source .env.local
set +a
go run ./cmd/pulseframe-api
```

默认监听 `:8080`；网页联调需显式设置 `HTTP_ADDR=127.0.0.1:8081`。除 HTTP 基础配置外，以下认证依赖配置为必填：

```bash
MYSQL_DSN='pulseframe_app:<password>@tcp(127.0.0.1:13306)/pulseframe?charset=utf8mb4&parseTime=true&loc=UTC&tls=skip-verify' \
REDIS_ADDR='127.0.0.1:16379' \
REDIS_USERNAME='pulseframe_api' \
REDIS_PASSWORD='<redis-password>' \
WEB_ORIGIN='http://127.0.0.1:5174' \
APP_ENV=local HTTP_ADDR=127.0.0.1:8081 LOG_LEVEL=debug \
go run ./cmd/pulseframe-api
```

支持的环境变量：

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `APP_ENV` | `local` | 运行环境：`local`、`test`、`staging`、`production` |
| `HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `LOG_LEVEL` | `info` | 日志级别：`debug`、`info`、`warn`、`error` |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | 请求头读取超时 |
| `HTTP_READ_TIMEOUT` | `15s` | 请求读取超时 |
| `HTTP_WRITE_TIMEOUT` | `15s` | 响应写入超时 |
| `HTTP_IDLE_TIMEOUT` | `1m` | 空闲连接超时 |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | 优雅退出期限 |
| `MYSQL_DSN` | 必填 | MySQL DSN；必须使用专用非 `root` 账号；不要打印或提交 |
| `REDIS_ADDR` | 必填 | Redis `host:port` |
| `REDIS_USERNAME` | 必填 | PulseFrame 专用 Redis ACL 用户名 |
| `REDIS_PASSWORD` | 必填 | Redis ACL 密码；密码原样保留 |
| `REDIS_DATABASE` | `0` | 非负逻辑数据库编号 |
| `WEB_ORIGIN` | 必填 | 精确网页来源，例如 `https://app.example.com`，不能带路径或尾斜杠 |
| `AUTH_COOKIE_SECURE` | local/test 为 `false`，staging/production 为 `true` | 生产环境不可设为 `false` |
| `AUTH_HASH_CONCURRENCY` | `4` | Argon2id 同时计算数，允许 `1` 至 `8`；满载时快速返回 503 |
| `AUTH_LOGIN_IP_LIMIT` | `30` | 单 IP 登录请求上限 |
| `AUTH_LOGIN_IDENTITY_IP_LIMIT` | `10` | 单 IP＋用户名登录请求上限 |
| `AUTH_REGISTER_IP_LIMIT` | `8` | 单 IP 注册请求上限 |
| `AUTH_LOGIN_LIMIT_WINDOW` | `15m` | 登录计数窗口 |
| `AUTH_REGISTER_LIMIT_WINDOW` | `1h` | 注册计数窗口 |

限流默认值是首轮实现基线，公网开放前需基于误拦截、滥用场景和压测结果复核。应用只信任实际 TCP 对端地址；启用反向代理后，必须先配置明确的可信代理范围，不能直接信任任意 `X-Forwarded-For`。
Argon2id 当前参数为 19 MiB、2 次迭代；并发默认 4、最多 8，容量满时拒绝新计算，不排队。上线前需在目标容器上测量哈希延迟、CPU 和峰值内存后再设定并发值。

## 验证

```bash
curl -i http://127.0.0.1:8081/livez
curl -i http://127.0.0.1:8081/readyz
```

认证接口：`POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`GET /api/v1/auth/me`、`GET /api/v1/auth/csrf`、`POST /api/v1/auth/logout`。浏览器请求需使用 `credentials: "include"`；`WEB_ORIGIN` 必须精确匹配网页来源。API 只为该来源返回凭据型 CORS，并仅允许 `Content-Type`、`X-CSRF-Token` 请求头。写请求需发送配置的 `Origin`；退出还需发送 `X-CSRF-Token`。登录令牌仅通过 HttpOnly Cookie 返回。网页与 API 应保持同站点，以符合 `SameSite=Lax` Cookie 策略。

运行测试和静态检查：

```bash
go test -timeout=60s ./...
go vet ./...
```

## 架构说明

- [启动流程：`run` 函数逐段说明](docs/run-lifecycle.md)
