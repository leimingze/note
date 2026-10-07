# PulseFrame Backend

当前目录提供核心 API 基础能力和用户注册接口。注册只向 MySQL 写入账号；Redis 尚未接入。

## 目录

Go 包直接位于 backend/ 下：config、logging、middleware、response 和 server 分别负责配置、日志、HTTP 中间件、错误响应和服务生命周期。controller、service 和 dao 分别处理注册请求、注册规则和数据库写入；cmd/pulseframe-api 是程序入口，test 保存后端测试。

## 启动

在 backend/ 目录执行：

~~~bash
go run ./cmd/pulseframe-api
~~~

启动前先在目标 MySQL 数据库执行 `migrations/001_create_users.sql`，并配置数据库连接。数据库及具备建表权限的账号由部署环境提供。默认监听 :8080；本地可加载项目根目录的私有凭据及后端配置：

~~~bash
set -a
source ../.env
source .env.local
set +a
go run ./cmd/pulseframe-api
~~~

`.env.local` 中还需要配置 `MYSQL_ADDR`、`MYSQL_USER` 和 `MYSQL_DATABASE`。远端直连时设置 `MYSQL_TLS=true` 并确保数据库证书可验证；若服务端未配置 TLS，则通过受控通道连接本地转发地址。

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
| MYSQL_ADDR | 必填 | MySQL 地址，格式为 host:port |
| MYSQL_USER | 必填 | MySQL 用户名 |
| MYSQL_DATABASE | 必填 | 已建表的数据库名称 |
| PULSEFRAME_MYSQL_PASSWORD | 必填 | MySQL 密码，不提交到版本库 |
| MYSQL_TLS | false | 远端直连时启用并验证 TLS 证书 |

## 接口

- GET /livez：进程可以响应时返回 200。
- GET /readyz：服务完成启动后返回 200。
- POST /api/v1/auth/register：JSON 请求体为 `{"username":"alice","password":"password"}`；成功返回 201（不自动登录），用户名重复返回 409，参数错误返回 400，数据库故障返回 500。用户名最多 24 个字符，密码最多 72 字节。用户名区分大小写。
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

真实数据库集成测试需要一个允许创建数据库的专用测试账号，并设置不指定数据库名称的 `PULSEFRAME_TEST_MYSQL_DSN`。测试只在自己创建的随机数据库中建表，结束时删除该数据库；未设置该变量时跳过数据库集成测试。

启动调用说明见 docs/run-lifecycle.md。
