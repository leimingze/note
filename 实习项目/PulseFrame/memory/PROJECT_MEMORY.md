# 项目记忆

最后更新：2026-10-05

## 项目目标

PulseFrame 是用于系统学习的 Go 视频社交平台项目，计划逐步覆盖视频发布与分发、互动、直播、内容安全 Agent 和云原生工程能力。每项技术选择需要由需求、数据和验证结果支持。

## 长期要求

- 开源项目只作为思路参考，不复用其代码。
- 功能实现前说明背景、需求和方案选择。
- 后端测试统一放在 backend/test/。
- 不提前创建没有实现内容的业务目录或运行单元。
- 注释说明业务意图和关键判断，不逐行翻译代码。
- 配置与凭据分离，源码和项目记忆不得保存凭据。

## 当前状态

当前只保留工程基座，账号注册、登录、会话、限流及相关网页功能已于 2026-10-05 移除。

后端现有能力：

- 环境变量读取、类型转换和启动校验。
- log/slog JSON 日志。
- 请求标识、访问日志和 panic 恢复。
- 统一错误响应。
- GET /livez 和 GET /readyz。
- HTTP 超时、就绪状态和优雅退出。
- 配置、日志、路由和真实端口生命周期测试。

网页端保留 Vue 3、TypeScript、Vite、项目基础页与组件测试。

## 当前目录和环境

- 后端：backend/
- 网页端：frontend/
- 设计文档：设计文档/
- 后端测试：backend/test/
- Go 模块：pulseframe
- Go 版本：1.26
- Gin 版本：v1.11.0

核心 API 默认监听 :8080。backend/.env.local 只保存基础运行参数，不再保存 MySQL、Redis 或账号配置。

## 数据清理

2026-10-05 已完成以下清理：

- 远端 MySQL 的 pulseframe.users 表已移除，复查确认该表不存在。
- Redis 的 pulseframe:auth:* 键数量为 0。
- MySQL 数据库本身、MySQL 账号、Redis 服务和 Redis ACL 账号继续保留，不影响其他用途。
- 本机 MySQL 与 Redis SSH 隧道当时未运行；清理通过远端容器内客户端执行。

## 架构方向

- 当前只运行核心 API，不依赖数据库、缓存或消息系统。
- 未来保留核心 API、事件 Worker、直播网关和内容安全 Agent 四类运行边界。
- Worker、直播网关和 Agent 仅在出现实际需求后创建。
- 新业务目录在实现阶段根据职责确定，不保留空 Controller、Service、Repository 或 Entity 目录。
- Gin 只用于 HTTP 接入和 Server 生命周期。

ADR-0002 记录此前业务代码采用横向分层的历史；账号业务移除后，该目录方案当前没有对应实现。未来新增业务时重新评估。

## 验证入口

~~~bash
cd backend
go test -timeout=60s ./...
go vet ./...

cd ../frontend
npm test -- --run
npm run typecheck
npm run build
~~~

本轮最终结果记录在 .agent/STATE.md。

## 后续问题

- 视频存储、上传协议、媒体处理、内容审核和 Feed 方案尚未设计。
- 是否接入 MySQL、Redis 或其他数据服务，需要在对应业务设计中重新选择。
- 事件 Worker、直播网关和内容安全 Agent 的创建时机仍由实际需求决定。

## 主要依据

- PulseFrame项目总纲.md
- 设计文档/01-总体架构与目录设计.md
- 设计文档/02-工程基础设计.md
- 设计文档/03 阅读代码.md
- backend/README.md
- backend/go.mod
