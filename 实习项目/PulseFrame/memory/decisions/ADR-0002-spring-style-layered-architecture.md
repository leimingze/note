# ADR-0002：核心 API 采用 Spring Boot 风格横向分层

日期：2026-09-29
状态：当前无对应业务实现

2026-10-05，账号业务代码已按用户要求移除，只保留工程基座。本记录继续保存此前目录选择的原因；未来新增业务时需要结合当时需求重新评估，不能直接视为当前实现要求。

## 背景

账号模块首版使用 `modules/account/httpapi/application/domain/persistence/secure` 组织代码。该结构强调业务模块和适配器边界，但目录层级、术语及两处中间件位置增加了当前项目的阅读成本。项目主要用于系统学习，用户明确希望按熟悉的 Spring Boot 调用链定位代码。

## 决策

核心 API 的 `backend/internal/` 改为横向技术分层：

- `controller`：HTTP 路由、请求解析和响应输出。
- `service`：注册、登录等业务逻辑与编排。
- `repository`：数据访问接口及 MySQL、Redis 实现。
- `entity`：业务实体、状态、校验和业务错误。
- `security`：密码哈希接口和实现。
- `middleware`：全局及认证 HTTP 中间件。
- `config/server/response/storage/logging`：工程运行基础能力。

依赖方向为 `Controller -> Service -> Repository/Security -> Entity`。Controller 不直接访问 Repository；所有 Gin 中间件统一放在 `middleware`；启动入口负责创建具体实现并完成依赖注入。

所有 Go 测试统一放在 `backend/test/`，使用独立的 `test` 包从公开 API 验证生产代码。测试不与 `internal/` 各层文件混放，也不为访问私有实现而扩大生产代码可见性。

## 影响

- 注册和登录代码可以沿 `controller -> service -> repository` 直接阅读。
- `modules` 与 `platform` 两层目录被移除，中间件不再分散在平台层和账号模块内。
- 横向分层不再通过目录强制隔离账号、视频和 Feed 等业务；后续必须通过类型命名、Service 边界和代码审查控制跨业务依赖。
- Gin 仍只用于 Controller、Middleware 和 Server，Service、Repository 与 Entity 不依赖 `gin.Context`。
- 生产目录只保留运行时代码，测试可按文件名直接定位到对应层，并能检验公开层边界是否足够清晰。
- 事件 Worker、直播网关和内容安全 Agent 的独立部署边界继续有效，不因核心 API 的目录变化而提前创建。

## 替代方案

- 保留原有模块化 Clean Architecture：边界更强，但当前学习和导航成本高于收益，因此不采用。
- 在每个业务模块内部使用 `controller/service/repository`：仍有额外模块层级，未满足用户对 Spring Boot 式平铺目录的要求，因此不采用。
- 把全部代码放在单个 package：目录最少，但无法维持 HTTP、业务和数据访问边界，因此不采用。

## 证据

- 用户于 2026-09-29 明确要求重新设计为更接近 Spring Boot 的目录架构。
- `backend/internal/` 已迁移为横向分层，测试已集中迁移至 `backend/test/`。
- `设计文档/01-总体架构与目录设计.md`、`03 阅读代码.md` 和用户认证实现文档已同步。

## 后续约束

- 新增业务代码沿用相同技术层，不提前创建空目录。
- Controller 不直接访问 Repository，跨业务写操作必须经过对应 Service。
- Repository 只负责数据访问和存储错误转换，不承载注册、登录等业务流程。
- 新增 Go 测试统一放入 `backend/test/`，默认使用 `package test` 和生产代码的公开 API。
- 若业务数量增长后横向分层导致明显耦合，再用测试和依赖证据重新评估按业务模块组织。
