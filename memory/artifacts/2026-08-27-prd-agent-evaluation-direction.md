# SpecEval：PRD 驱动的测试 Agent 测评平台

日期：2026-08-27
状态：候选方向，待完成 MVP 设计

## 项目定位

SpecEval 是一个将 PRD 转换为可执行 Agent 评测任务，并对不同测试 Agent 进行统一运行、证据采集、评分和回归比较的平台。

平台第一阶段专门评测**测试 Agent**：给定同一份 PRD、设计稿、接口契约和被测环境，比较不同 Agent 在需求理解、测试设计、测试代码生成、真实缺陷发现和结果报告上的能力。

```text
PRD / 设计稿 / OpenAPI
  -> PRD Eval Compiler
  -> 公开任务 + 隐藏判分标准
  -> 运行多个候选测试 Agent
  -> 收集轨迹、测试产物与执行证据
  -> 确定性校验 + LLM Judge + 人工校准
  -> Agent 对比、回归与坏例分析
```

一句话价值：**把业务 PRD 变成可复现的测试 Agent 考场，而不是只让模型根据 PRD 再生成一份主观评分。**

## 核心问题

现有 PRD 测试项目已经能生成测试点和用例，但通常难以回答以下问题：

- 换一个模型、Prompt、RAG 或工作流后，测试 Agent 是否真的变好？
- 生成的测试能否加载、执行、发现真实缺陷，并用证据支撑成功声明？
- Agent 是否遗漏 PRD 的关键约束，或虚构了不存在的业务规则？
- Agent 重复运行是否稳定，差异来自模型、工具、知识库还是评测器偏差？

SpecEval 的目标是把这些问题转换为版本化、可运行、可统计的评测任务。

## 被测对象

MVP 只评测从 PRD 生成并执行 API 测试的 Agent。每个候选 Agent 接收相同输入：

- PRD 与已确认验收条件；
- OpenAPI 契约、被测服务地址和受控测试账号；
- 允许使用的工具集合；
- 固定时间、Token 和权限预算。

候选 Agent 输出统一为：

- 测试计划与结构化测试用例；
- 可执行 pytest 文件；
- 执行结果、失败归因与证据引用；
- 最终测试结论。

后续再增加 Web UI Agent、移动端 Agent、代码修复 Agent 等 Profile，避免第一版同时覆盖多个问题域。

## PRD Eval Compiler

PRD Eval Compiler 是项目最有辨识度的模块。它把自然语言 PRD 编译为版本化 `EvalSpec`，而不是直接把 PRD 交给 LLM Judge。

### PRD 中间表示

```text
Feature
├── AcceptanceCriterion
├── BusinessRule
├── StateTransition
├── InputConstraint
├── PermissionConstraint
├── SideEffect
├── FailureBehavior
└── Ambiguity
```

每个条目保留原文引用、来源位置、确认状态和稳定 ID。LLM 可以提出候选拆解，但验收条件与隐藏判分标准必须经过人工确认。

### EvalSpec

每个评测任务分为两部分：

| 部分 | 候选 Agent 是否可见 | 内容 |
|---|---:|---|
| `task` | 是 | PRD、公开上下文、环境入口、工具和资源预算 |
| `oracle` | 否 | 验收点、预置缺陷、数据库断言、允许行为、评分规则 |

隐藏 `oracle` 防止 Agent 直接针对答案生成测试。公开任务和隐藏标签物理分离，并分别保存摘要校验值。

## 评测方法

### 第一层：结构与契约

- 输出 Schema 是否有效；
- 测试文件能否被 pytest 收集；
- 工具调用参数和证据引用是否有效；
- 是否违反时间、Token 或权限预算。

### 第二层：真实执行

- 测试能否在正确版本服务上稳定通过；
- 能否发现预置故障或 mutation；
- 是否产生误报；
- 数据库、缓存和消息队列副作用是否被正确验证；
- Agent 声明与实际执行结果是否一致。

### 第三层：PRD 覆盖

- 已确认验收条件的覆盖率；
- 状态转换、边界、异常和权限规则覆盖率；
- 未被 PRD 支撑的虚构规则数量；
- 不可测试或有歧义条目是否被正确指出。

### 第四层：轨迹质量

- 无效工具调用率；
- 工具错误后的恢复率；
- 重复或无收益步骤比例；
- 关键结论的证据引用完整率；
- 多次运行的一致性和方差。

LLM Judge 只处理难以由规则确定的语义覆盖和归因问题，不能覆盖确定性执行结果。Judge 输出必须引用 PRD 片段、Agent 轨迹或执行证据。

## dsh 式插件架构

### 内核职责

内核只负责：

- 插件生命周期与依赖解析；
- 类型化 Service 注册表；
- 类型化事件总线；
- 追加式运行日志、恢复和回放；
- Profile/Bundle 配置组合；
- 权限、审批与沙箱；
- Artifact、Evidence 和版本追踪。

PRD 解析、Agent 运行、环境部署、评分和报告均由插件提供。

### Service Seams

| Service | 职责 | Provider 示例 |
|---|---|---|
| `ctx.sources` | 读取并冻结评测输入 | PRD、Figma、OpenAPI、Git |
| `ctx.specCompiler` | 生成 PRD IR 和 EvalSpec | 规则编译器、LLM 编译器 |
| `ctx.evalSets` | 管理评测集和分组 | JSONL、本地数据库、远程服务 |
| `ctx.agentRunners` | 运行候选 Agent | dsh、pi-agent、tRPC、HTTP/A2A |
| `ctx.environments` | 创建可复现被测环境 | Docker Compose、远程测试环境 |
| `ctx.mutators` | 注入可归因故障 | 代码 mutation、配置故障、数据故障 |
| `ctx.evidence` | 保存执行与状态证据 | 本地文件、SQLite、对象存储 |
| `ctx.evaluators` | 运行单项判分器 | 规则、pytest、Judge、轨迹评估 |
| `ctx.metrics` | 聚合指标和置信区间 | 分组统计、bootstrap、版本比较 |
| `ctx.reporters` | 输出报告 | Markdown、HTML、飞书、Jira |
| `ctx.optimizers` | 离线生成候选优化版本 | GEPA、TextGrad、MIPRO、AFlow |

每个 seam 都包含稳定接口、Provider 和 Consumer。插件只依赖契约，不直接访问其他插件的私有数据库或内部类。

### Profile

```yaml
profiles:
  api-test-agent-eval:
    bundles:
      - base-runtime
      - source-prd-openapi
      - compiler-api-evalspec
      - environment-compose
      - mutator-api
      - runner-agent-adapters
      - evaluator-deterministic
      - evaluator-agent-judge
      - reporter-comparison

  testcase-generation-eval:
    bundles:
      - base-runtime
      - source-prd
      - compiler-case-evalspec
      - runner-agent-adapters
      - evaluator-coverage
      - reporter-comparison
```

同一候选 Agent 可进入多个 Profile；同一 EvalSpec 也可交给不同 Agent Runner。这样才能真正比较模型、框架和工作流，而不是为每个 Agent 单独编写一套评测代码。

## 统一数据容器

沿用现有 Agent Judge ADR，只统一容器，不强行统一监督语义：

- `task`：公开任务和资源预算；
- `trajectory`：模型消息、工具调用、工具结果和状态变化；
- `evaluation`：各评测器的原始标签与指标；
- `evidence`：请求响应、日志、截图、数据库状态和测试报告；
- `source`：PRD、环境、Agent、模型、Prompt、插件和数据版本；
- `sample_id`：稳定样本标识。

同一 PRD 派生的多个任务、mutation 和重复运行共享 `source.group_id`。数据集划分和置信区间按 `group_id` 处理，不能把它们当成完全独立样本。

## 现有项目的复用方式

| 项目 | 在 SpecEval 中的角色 |
|---|---|
| dsh-harness | 插件宿主、Profile、Service seam、事件日志、权限与沙箱的架构母版 |
| pi-agent | 一个候选 Agent Runner，或轻量 Agent loop 与 TUI 事件适配器 |
| tRPC-Agent-Python | Python Agent Runner、GraphAgent、MCP/A2A 接入和评测组件 |
| AITC | PRD/设计稿解析器、case writer、专项测试和 Judge 的领域插件来源 |
| Auto_prd_test_agent | 多模态解析、RAG、Critic 和资产回流基线 |
| 火山引擎项目 | 简单 RAG 用例生成基线 |
| EvoAgentX | 离线 Optimizer Provider，不直接修改当前线上版本 |

主进程只选择一套插件生命周期。其他 Agent 框架通过 `AgentRunner` 协议适配，不把多套运行时内核混在一起。

## MVP

### 数据与环境

复用现有电商接口测试 Demo，准备一组小而可信的 PRD 变更任务：

- 订单创建参数与校验；
- 订单状态流转；
- 幂等与重复提交；
- 数据库写入一致性；
- Redis 缓存一致性；
- RabbitMQ 消息发送与失败恢复。

每个 PRD 任务配置正确版本和若干预置故障版本，隐藏故障 ID 和判分断言。

### 候选 Agent

第一版只接两个候选：

1. 直接 Prompt + PRD 的基线 Agent；
2. 带 RAG、工具执行和 Critic 的插件化 Agent。

两者使用相同模型、预算、环境和重复运行次数，避免把模型差异误认为架构收益。

### 必做能力

1. PRD 拆解与人工确认界面或配置文件；
2. 公开任务和隐藏 oracle 的物理分离；
3. dsh 式插件发现、Service 注册和 Profile 组合；
4. 至少两个 Agent Runner 适配器；
5. pytest 收集、执行和证据归档；
6. mutation 缺陷召回、误报和稳定性统计；
7. 单任务下钻与 Agent 版本对比报告；
8. 全流程可断点恢复和版本可追溯。

### 暂不实现

- 自动生成大规模 PRD 数据集；
- 在线自进化；
- 通用插件市场；
- 同时评测所有类型的 Agent；
- 只依靠 LLM Judge 的主观排行榜；
- 自动修复被测业务代码。

## 核心指标

| 指标 | 说明 |
|---|---|
| PRD 验收点召回率 | 已确认验收条件中被有效测试覆盖的比例 |
| 可执行率 | 测试可被 pytest 收集并实际开始运行的比例 |
| 缺陷召回率 | 预置故障或 mutation 被发现的比例 |
| 误报率 | 正确版本被错误判定失败的比例 |
| 证据一致率 | Agent 声明与真实执行证据一致的比例 |
| 幻觉率 | 测试或结论引用 PRD 中不存在规则的比例 |
| 恢复率 | 工具或执行失败后完成任务的比例 |
| 稳定性 | 同一任务多次运行的均值、方差和最差表现 |
| 成本 | Token、模型调用、执行时间和环境资源 |

主指标应优先使用真实执行和 mutation 结果；Judge 分数作为补充指标，并报告其与人工标注的一致性。

## 最小演示

用户导入一份“订单取消规则变更”PRD，确认验收点后，SpecEval 自动完成：

1. 生成公开任务和隐藏 oracle；
2. 在正确版本与三个故障版本上分别运行两个测试 Agent；
3. 收集 Agent 轨迹、pytest、HTTP、数据库和消息队列证据；
4. 输出每个 Agent 找到了哪些故障、漏掉了哪些验收条件、产生了哪些误报；
5. 对比基线 Agent 和插件化 Agent 的缺陷召回、稳定性、成本与证据一致率。

该演示同时证明 PRD 编译、插件互用、Agent 测评和真实测试价值。

## 关键约束

- LLM 生成的验收点不能直接作为最终真值，必须支持人工确认和修改。
- 公开任务与隐藏 oracle 必须物理分离，运行时禁止候选 Agent 访问隐藏内容。
- 确定性执行失败不能被 LLM Judge 覆盖为成功。
- 证据不足必须显式返回 `insufficient_evidence`。
- 超长轨迹不得静默截断；应标记不可评测并报告覆盖率。
- Prompt、模型、插件、环境、PRD、EvalSpec 和数据集均需固定版本。
- 同源任务、mutation 和重复运行按 `group_id` 分组统计。
- Optimizer 只能生成候选版本，必须通过固定回归集和人工门禁后才能发布。

## 参考材料

- `/Users/leimingze/agent-project/deepseek-harness/docs/architecture.zh.md`
- `/Users/leimingze/agent-project/pi-mono/packages/agent/README.md`
- `/Users/leimingze/agent-project/trpc-agent-python/README.md`
- `/Users/leimingze/notes/实习项目/测试提效agent/AITC-20260811.zip`
- `/Users/leimingze/notes/memory/decisions/ADR-0001-agent-judge-unified-data-schema.md`
- `/Users/leimingze/notes/memory/decisions/ADR-0002-agent-judge-context-overflow-policy.md`
- <https://github.com/zxLeva/ByteDance--Auto_prd_test_agent>
- <https://github.com/ANative-Lab/EvoAgentX/blob/main/README-zh.md>
