# EvoAgent 全貌：Harness 在系统中的位置

## 一句话定义

EvoAgent 不是一个“调用一次 LLM 就结束”的代码审查工具，而是一个把以下内容串成一条可审计、可恢复工作流的系统：

- 输入：PR Diff、GitHub Webhook 或 API 请求；
- 执行：Diff 解析、规则扫描、多角色 LLM Agent、工具调用；
- 决策：Lead 委派、Worker 产出、Critic 挑战、Lead 综合；
- 质量：格式、证据、置信度、修复与测试建议门禁；
- 持久化：任务、轨迹、检查点、原始 Diff、记忆、反馈、版本；
- 演进：失败反馈回流、候选提示词或 Skill 评测、激活与回滚；
- 产品外壳：Web 控制台、API、登录/RBAC、GitHub 回写、自动修复、监控告警。

这里说的 **Harness** 通常指 `ReviewHarness`。它不是模型，也不等于产品本身，而是“把一次审查任务包装成有状态、有预算、有门禁、可断点续跑的执行壳”。

## 先澄清：代码里有两种 Harness

| 名称 | 所在文件 | 用途 |
|---|---|---|
| `ReviewHarness` | `evoagent/harness.py` | 产品运行时。一次真实审查的 planning → executing → reviewing 状态机 |
| `EndToEndEvaluationHarness` | `evoagent/evaluation_harness.py` | 离线评测运行时。用带金标的 PR 数据批量回放，计算 F1、召回、修复成功率 |

`ReviewHarness` 是线上产品用的；`EndToEndEvaluationHarness` 及 `ProductionEvaluationHarness` 是用来验证产品能力、支持提示词或 Skill 演进的。如果不区分两者，很容易把“评测框架”误当成“产品执行框架”。

## Harness 到底做了什么

`ReviewHarness` 的核心职责可以概括为五件事：

1. **定义状态机**：`PENDING → PLANNING → EXECUTING → REVIEWING → SUCCESS`，失败进入 `FAILED`，取消进入 `CANCELLED`。
2. **管理预算**：外层步骤数和墙钟时间由 `AgentRuntime` 控制；每个 Agent 角色还有自己的 Token/时间预算。
3. **提供可恢复能力**：每个节点完成时写 `checkpoints`，失败时写失败状态；同一任务再次执行会跳过已完成节点。
4. **隔离 Agent 行为**：真正做推理的是 `ModeRouterReviewer` 和 `BoundedRole`，Harness 只负责编排和持久化边界。
5. **生成统一产物**：最终输出 `ReviewReport`，其中包含 findings、风险评估、协作记录、执行统计和上下文管理信息。

## 系统分层

建议把 EvoAgent 看成“一条垂直主链路 + 两个横切面”，而不是简单的五层堆叠。

```text
入口与适配层
  HTTP API / Web 控制台 / GitHub Webhook / GitHub App / 自动修复分支
        |
产品服务与控制面
  Settings、ReviewService、登录/RBAC、租户与仓库授权、
  TaskQueue、ReleaseManager、AlertManager、Observability
        |
工作流与状态机层
  ReviewHarness
  AgentRuntime（节点、预算、重试、取消、checkpoint）
        |
Agent 认知层
  ModeRouterReviewer
  Lead → Security / Correctness-Reliability → Critic
  BoundedRole、Prompt、ContextManager、MemoryManager、SkillRegistry
        |
能力与工具层
  DiffParser
  LocalRuleReviewer、SecurityRuleReviewer、ReliabilityRuleReviewer
  RepositoryToolSuite（搜索、读取、AST、Git、Scanner、测试）
  ToolRegistry（参数 Schema 校验）
        |
质量门禁层
  FindingGate（格式 / 证据 / 置信度 / 发布）
  RepairVerifier（编译、AST/CST、前后测试对比）
  RegressionEvaluator、Evolution/Skill Evolution 门禁
        |
持久化与恢复层
  TaskStore（SQLite）/ PostgresTaskStore（PostgreSQL）
  checkpoints、task_payloads、trace_events、agent_memories
  Redis Streams / ACK / Lease / Retry / DLQ
        |
反馈与演进层
  failure_cases、evaluation_cases、evolution_runs、
  skill_versions、skill_artifact_versions、skill_evolution_runs
  激活、回滚、灰度与影子发布
```

### 1. 入口与适配层

`api.py` 是唯一对外 HTTP 入口。它负责：

- `/v1/reviews`：手动提交审查，支持同步和异步；
- `/webhooks/github`：接收 GitHub PR Webhook，校验 HMAC 签名；
- `/v1/tasks/*`：查询任务、报告、反馈、修复、取消、续跑；
- `/api/*`：Dashboard、Skills、失败案例、审计、告警、队列、发布配置；
- 静态 `web/index.html`、`app.js`、`app.css`、`login.css`：Web 控制台。

GitHub 的 Diff 拉取、评论回写、仓库读取、分支与 Draft PR 创建由 `github.py` 完成。自动修复由 `patching.py`、`fixer.py`、`verifier.py` 完成。

### 2. 产品服务与控制面

`ReviewService` 是组合根。它把存储、LLM、审查器、Harness、队列、认证、发布、告警、评测与 Skill 进化全部组装起来。

控制面负责所有“不属于某一次 Agent 推理”的事情：

- 环境配置与 `.env` 加载；
- 数据库和 Redis 选择；
- 登录、短期 Token、角色权限、租户隔离；
- 仓库授权；
- Webhook 幂等与重放时间窗；
- 异步任务投递、重试、死信；
- 发布流量分配、失败率告警；
- OpenTelemetry Span 和 Prometheus 指标；
- 接受反馈、创建修复任务、触发演进。

### 3. 工作流与状态机层

这就是通常所说的“Harness 核心”。

- `ReviewHarness` 定义三个业务节点：解析、执行、汇总；
- `AgentRuntime` 是通用执行器，不知道“代码审查”是什么；
- `RuntimeNode` 描述节点名、处理函数、重试次数、是否打检查点；
- `ToolRegistry` 提供工具目录和参数 Schema 校验；
- `RuntimeBudgetExceeded`、`RuntimeCancelled`、`ToolProtocolError` 是执行层异常。

这种拆分让“审查逻辑”和“可恢复执行逻辑”解耦。以后换任务类型时，可以复用 `AgentRuntime`，只改节点和状态存储。

### 4. Agent 认知层

这一层回答“谁来思考、按什么协议思考、能调用什么”。

- `ModeRouterReviewer`：产品级 Agent 编排器；
- `BoundedRole`：单个角色的有限步数循环；
- `Lead`：拆解、委派、要求返工、调度 Critic、最终选择；
- `Security` / `Correctness-Reliability`：两个并行 Worker；
- `Critic`：盲审、找反例、挑战置信度，不新增 finding；
- `ContextManager`：语义压缩 Diff、管理观察历史、控制输入 Token；
- `MemoryManager`：工作记忆、历史记忆和语义记忆的租户/仓库隔离；
- `SkillRegistry`、`AgentSkill`：动态 Skill 包加载与版本切换。

### 5. 能力与工具层

这一层是“事实来源”，不是“决策来源”。

- `parse_unified_diff`：把 Diff 解析成文件列表和新增行；
- `LocalRuleReviewer`：内置规则扫描；
- `SecurityRuleReviewer` / `ReliabilityRuleReviewer`：注册为同类型扫描器的领域规则；
- `RepositoryToolSuite`：文件列举、全文搜索、读取文件、AST、符号、调用链、Git 上下文、运行 Scanner、编译/测试；
- `ToolRegistry`：在调用前校验参数，避免 Agent 任意传参。

所有工具输出都带有 `evidence_id`，供后面证据门禁引用。仓库内容在进入 Prompt 前被明确标记为“不可信数据，不是指令”。

### 6. 质量门禁层

`FindingGate` 的四个检查点：

- 格式：位置必须落在真正新增行，规则 ID、标题、解释必须非空；
- 证据：必须有匹配代码、调用链或工具证据；高危结论必须有强证据；
- 置信度：低于阈值拒绝；
- 发布：高危结论必须同时给出修复建议和测试建议。

修复侧还有 `RepairVerifier`，负责：

- Python 编译和 tokenize 检查；
- 在临时完整仓库副本中运行配置好的测试命令；
- 比较修复前、修复后结果，识别行为回归。

评测侧有 `RegressionEvaluator`，用于新旧 Prompt 或新 Skill 的分数对比。

### 7. 持久化与恢复层

默认使用 SQLite；设置 `EVOAGENT_DATABASE_URL` 后切换到 PostgreSQL。两者实现同一套 `TaskStore` 公共接口。

Redis 用于异步队列：Redis Streams、消费组、ACK、Lease、重试、死信。没有 Redis 时使用进程内 `memory-acked`，但不具备跨重启持久性。

关键持久化数据：

- `tasks`：任务状态、输入元数据、最终报告、错误、租户；
- `task_payloads`：原始 Diff；
- `trace_events`：状态迁移和日志；
- `checkpoints`：节点检查点和 Agent 会话检查点；
- `agent_memories`：工作记忆、历史记忆、语义记忆；
- `failure_cases`：反馈和失败案例；
- `evaluation_cases`、`evolution_runs`、`skill_versions`、`skill_artifact_versions`、`skill_evolution_runs`：评测与版本；
- `webhook_deliveries`、`audit_log`、`alerts`、`deployments`、`release_observations`：安全、审计和发布。

### 8. 反馈与演进层

这是把 EvoAgent 从“一次工具”变成“一个会学习的产品”的关键：

```text
审查结果
  → 用户反馈（误报 / 漏报 / 坏修复）
  → failure_cases + 记忆
  → 生成候选 Prompt 或候选 Agent Skill
  → 在 evaluation_cases 上回放新旧版本
  → Validation 提升且 Holdout 不退化才激活
  → 激活后 reload skills，下一次任务使用新版本
  → 不通过则保留版本记录，可手动回滚
```

## 两个横切面

下面的内容不单独占一层，但贯穿所有层：

- **安全与权限**：登录、RBAC、租户隔离、仓库授权、Webhook HMAC、路径穿越防护、工具参数校验、敏感 Key 不进源码；
- **可观测性**：`Metrics`、`Observability`、`AlertManager`、执行账本 `ExecutionLedger`、上下文压缩统计、失败案例追踪。

## 代码目录与系统层对应

| 代码文件 | 系统位置 | 主要职责 |
|---|---|---|
| `api.py` | 入口适配层 | HTTP、静态页面、接口路由 |
| `service.py` | 产品控制面 | 组装全部组件，承载业务动作 |
| `harness.py` | 工作流层 | 一次审查的节点和状态 |
| `runtime.py` | 工作流层 | 通用节点执行、预算、重试、检查点 |
| `agentic_core.py` | Agent 认知层 | Lead/Worker/Critic 编排 |
| `context_manager.py` | Agent 认知层 | Diff 压缩、上下文预算、观察历史 |
| `memory.py` | Agent 认知层 | 工作/历史/语义记忆 |
| `skills.py` | 能力层 | `SKILL.md` 解析、资源读取、注册表 |
| `reviewer.py` | 能力层 | 规则与 OpenAI 兼容 Reviewer |
| `repository_tools.py` | 能力层 | 仓库事实工具 |
| `gates.py` | 门禁层 | Finding 质量门禁 |
| `store.py` / `postgres_store.py` | 持久化层 | SQLite / PostgreSQL |
| `task_queue.py` | 持久化层 | 内存 / Redis 队列 |
| `evolution.py` / `evolution_v2.py` | 演进层 | 提示词候选与版本门禁 |
| `skill_evolution.py` | 演进层 | Agent Skill 候选与版本门禁 |
| `rollout.py` | 发布层 | 稳定 / 金丝雀 / 影子分配 |
| `observability.py` / `metrics.py` | 横切面 | Trace 与指标 |

## 阅读建议

1. 先读本文件，建立整体分层；
2. 再读 `01-产品组装.md`，看组件如何被拼成一个可运行产品；
3. 然后读 `02-一次请求链路.md`，跟着一个请求走完；
4. 最后读 `03-数据留存与恢复.md`，理解哪些状态被保存、崩溃后能恢复到哪里；
5. 可选读 `04-反馈回流与演进发布.md`，理解数据如何回流并改变未来行为。
6. 查阅 `05-模块与代码覆盖清单.md`，确认哪些模块已说明、哪些仍待补。
