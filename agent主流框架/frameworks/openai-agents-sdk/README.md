# OpenAI Agents SDK 使用方法

> 必学框架 3/4。本文只讲怎么用：`Agent`、`Runner`、工具、handoff、Agent as tool、context 和选型。基于 `openai-agents 0.21.1`（2026-08-18）验证，要求 Python 3.10+。

> Jupyter 学习版（带看懂式，可直接运行）：`demo/OpenAIAgentsSDK使用方法.ipynb`。用客服路由场景贯穿：先看单 Agent 工具闭环，再看 handoff 接管和 `agent.as_tool()` 委托，最后看 context 如何进入工具。Notebook 的核心 cell 用 Scripted Model 验证，不需要 API Key。

## 一、它是什么

OpenAI Agents SDK 是 OpenAI 维护的轻量 Agent 工作流框架。它把 Agent 的核心循环封装成：

```text
Agent（配置） + Runner（执行循环） + Model（模型调用） + Tools / Handoffs（动作与路由）
```

`Runner` 会反复执行以下过程，直到产生最终输出：

1. 调用当前 Agent 的模型
2. 如果模型返回工具调用，执行工具并把结果放回输入
3. 如果模型返回 handoff，切换到目标 Agent
4. 如果得到最终输出，结束并返回 `RunResult`

它的重点不是让人手动画状态图，而是提供一个**简单的 Agent 循环 + 清晰的多 Agent 协作 API**。

## 二、和 LangGraph / LangChain 的关系

| 框架 | 核心抽象 | 控制方式 |
| --- | --- | --- |
| LangGraph | State / Node / Edge / Checkpointer | 人定义图和状态流转 |
| LangChain `create_agent` | Model + Tools + Agent Loop | 轻量工具调用循环 |
| OpenAI Agents SDK | Agent + Runner + Tools + Handoffs | SDK 管理循环，Agent 负责配置 |

最重要的区别是：

- LangGraph 的多 Agent 通常是图上的节点、边或子图
- OpenAI Agents SDK 的 handoff 是模型可调用的一类特殊工具；调用后**目标 Agent 接管对话**
- `agent.as_tool()` 则是把另一个 Agent 当普通工具调用；调用完成后**原 Agent 继续掌控对话**

## 三、安装

```bash
cd agent主流框架/frameworks/openai-agents-sdk
python3.12 -m venv .venv
.venv/bin/pip install -U openai-agents nbformat nbclient ipykernel
.venv/bin/python -m ipykernel install --sys-prefix --name openai-agents-sdk-demo --display-name "Python 3 (OpenAI Agents SDK)"
```

真实模型调用需要配置：

```bash
export OPENAI_API_KEY="你的 Key"
```

## 四、核心概念速查

| 概念 | 作用 |
| --- | --- |
| `Agent` | 配置名称、instructions、model、tools、handoffs、guardrails 等 |
| `Runner` | 驱动 Agent 循环，提供 `run` / `run_sync` / `run_streamed` |
| `RunResult` | 运行结果：`final_output`、`last_agent`、`new_items`、`raw_responses` |
| `function_tool` | 把 Python 函数转换为带 JSON Schema 的工具 |
| `handoff` | 把对话所有权转交给另一个 Agent |
| `as_tool` | 把 Agent 包装成工具，调用后原 Agent 继续回答 |
| `RunContextWrapper` | 把业务 context 注入工具、handoff、guardrail |
| `output_type` | 让 Agent 输出 Pydantic / dataclass 等结构化对象 |
| `input_guardrails` / `output_guardrails` | 在输入或最终输出处执行检查 |
| `Session` | 自动保存多次 Agent run 的对话历史 |
| `MCP` | 接入 MCP Server 提供的工具 |
| Tracing | 记录 Agent、工具、handoff 和模型调用链路 |

## 五、最小示例

```python
from agents import Agent, Runner, function_tool

@function_tool
def get_weather(city: str) -> str:
    """查询城市天气。"""
    return f"{city}: sunny 25C"

agent = Agent(
    name="Weather Assistant",
    instructions="使用天气工具回答用户问题。",
    tools=[get_weather],
)

result = Runner.run_sync(agent, "北京天气怎么样？")
print(result.final_output)
```

这段代码里：`Agent` 只负责声明能力，`Runner` 负责循环；工具调用是否发生、调用几次由模型决定。

## 六、三种核心协作方式

| 方式 | 代码入口 | 结果控制权 | 适用场景 |
| --- | --- | --- | --- |
| 单 Agent + 工具 | `tools=[function_tool]` | 当前 Agent | 查数据、执行动作 |
| Handoff | `handoffs=[specialist]` | 转给 specialist | 专业路由、客服分流 |
| Agent as tool | `specialist.as_tool(...)` | 保留给当前 Agent | 主 Agent 调研后统一汇总 |

### Handoff 和 Agent as tool 的区别

```python
# handoff：specialist 接管后续对话
triage = Agent(name="Triage", handoffs=[specialist])

# as_tool：specialist 只作为一个工具，主 Agent 继续掌控对话
research_tool = specialist.as_tool(
    tool_name="research",
    tool_description="研究用户问题并返回报告",
)
manager = Agent(name="Manager", tools=[research_tool])
```

## 七、优缺点与局限

### 优点

- Agent 循环 API 小，`Agent` / `Runner` / `function_tool` 很快能形成闭环
- handoff 与 Agent as tool 的语义清楚，适合多 Agent 路由
- 官方维护，内置 guardrails、sessions、MCP、tracing 和流式能力
- 支持自定义 `Model`，不把所有模型调用都锁死在 OpenAI API 上

### 缺点

- 复杂状态图、精确路由、长流程恢复不如 LangGraph 直观
- Agent 循环的内部状态由 SDK 管理，调度细节不如手搓图透明
- OpenAI Responses API、Hosted Tools、Tracing 等能力会带来生态绑定
- SDK 更新较快，版本升级时需要重新核对模型、handoff 和 session API

### 适用场景

- 单 Agent + 工具的快速生产原型
- 客服分流、专家 Agent 路由、主 Agent 调用多个研究 Agent
- 需要 guardrails、MCP、tracing，但不想自己搭完整图运行时

### 不适合优先使用的场景

- 需要显式状态图、复杂循环、并行 fan-out、断点恢复和人工审批流程
- 需要把每个节点的状态转换和路由规则完全掌握在业务代码中
- 已经深度使用 LangGraph，且项目主要难点是工作流编排而非 Agent 循环

## 八、常见问题

- **`Runner.run_sync` 和 `Runner.run` 怎么选？** 普通脚本用 `run_sync`；Jupyter、FastAPI 等已有事件循环的环境用 `await Runner.run(...)`。
- **handoff 是不是另一个工具？** 对模型来说它以工具 schema 暴露，但执行后是 Agent 接管，不是普通函数返回结果。
- **怎么保留会话历史？** 给 `Runner.run` 传 `session`，例如 `SQLiteSession("customer-1")`。
- **能不用 OpenAI 模型吗？** 可以通过 SDK 的 `Model` 接口或官方扩展接入其他模型，但 Hosted Tools 等 OpenAI 专属能力仍有绑定。
- **为什么 Notebook 不需要 Key？** 使用自定义 Scripted Model 返回固定的 Responses API 输出，验证 Runner、工具和 handoff 的机制。

## 九、下一步

- 官方文档：<https://openai.github.io/openai-agents-python/>
- 官方仓库：<https://github.com/openai/openai-agents-python>
- 进阶方向：guardrails、sessions、MCP、streaming、tracing、Agent as tool 与 sandbox agents
