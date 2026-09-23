# 产物：EvoAgent 项目拆解文档

日期：2026-08-27
类型：设计文档 / 项目索引

## 路径

- `实习项目/EvoAgent/docs/00-全貌-harness分层.md`
- `实习项目/EvoAgent/docs/01-产品组装.md`
- `实习项目/EvoAgent/docs/02-一次请求链路.md`
- `实习项目/EvoAgent/docs/03-数据留存与恢复.md`
- `实习项目/EvoAgent/docs/04-反馈回流与演进发布.md`
- `实习项目/EvoAgent/docs/05-模块与代码覆盖清单.md`
- `实习项目/EvoAgent/README.md` 已新增文档入口

## 用途

按“全貌与 Harness 分层、产品组装、一次请求链路、数据留存与恢复、反馈回流与演进发布”五个视角拆解 EvoAgent，供后续阅读代码、解释架构、交接或扩展时使用。

## 分类结论

用户提出的四个视角成立，但建议补上第五个视角：

1. 全貌与 Harness 分层：回答系统是什么、有哪几层；
2. 产品组装：回答组件如何拼成可运行产品；
3. 一次请求：回答一次输入经历哪些节点；
4. 数据留存与恢复：回答系统留下什么、如何恢复；
5. 反馈回流与演进发布：回答留下的数据如何生成新版本、通过门禁并影响未来行为。

后续复核发现，五篇架构文档没有覆盖全部代码：`evaluation_benchmark.py`、`evaluation_v2.py`、四个 `scripts/*.py`、17 个测试文件和 9 个 Agent Skill 的具体目录均未纳入；已新增 `05-模块与代码覆盖清单.md` 记录覆盖缺口和后续补文档建议。

2026-08-27 在 Python 3.10.11 下运行 63 项测试：62 项通过、1 项失败（`test_safe_fixer_changes_only_supported_rules`）。失败原因是测试期待双引号，而 `SafeFixer` 经 `ast.unparse` 输出单引号；`SafeFixer` 在产品服务中未实际使用，README 声明 Python 3.11。

`ReviewHarness` 与 `EvaluationHarness` 是两个不同概念：前者是产品运行时，后者是离线评测运行时。

## 关键事实与边界

- 产品默认用 `ModeRouterReviewer`，包含 Lead、Security、Correctness/Reliability、Critic；
- `ReviewHarness` 使用 `AgentRuntime`、checkpoint、预算、重试和取消；
- SQLite 与 PostgreSQL 走同一套 `TaskStore` 接口；异步队列在 Redis 和内存模式之间切换；
- `agentic-lead-session` 支持 Lead 会话按阶段恢复；
- `observe_shadow()` 和 `record_shadow_observation()` 已实现，但 `ReviewService` 当前未接入完整 shadow 发布主链路；
- Agent Skill 当前 `sandboxed=False`，没有独立进程沙箱和签名校验；
- `agent_messages` 表存在，但当前代码没有主动写入调用。

## 关联记忆

- `memory/PROJECT_MEMORY.md`
- `memory/MEMORY_PROTOCOL.md`
