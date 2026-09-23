# Deep Agents 使用方法

> 学习框架（LangGraph 生态纵深，当前学习）。本文只讲怎么用：是什么、和 LangChain/LangGraph 的关系、安装、核心概念、可运行示例。基于 `deepagents 0.7.7`（2026-08-18）验证，要求 Python 3.11+。

> Jupyter 学习版（带看懂式，可直接运行）：`demo/DeepAgents使用方法.ipynb`，用「调研并写报告」场景贯穿：先看懂 `create_deep_agent` 最小闭环，再看懂子 Agent 委派，最后看内置文件工具与 LangGraph 关系。内核选择 `deepagents-demo`（Python 3.12）。

## 一、它是什么

Deep Agents 是 LangChain 生态里的**开箱即用 Agent Harness**：不是独立框架，而是构建在 LangGraph + LangChain `create_agent` 之上的一层“深度 Agent 默认配置”。

它把长任务 Agent 常用的能力直接打包好：

- 内置文件系统工具：`ls` / `read_file` / `write_file` / `edit_file` / `glob` / `grep`
- 命令执行：`execute`（由 backend 决定是否真正执行）
- 子 Agent 委派：`task` 工具，让主 Agent 把复杂任务拆给隔离上下文的子 Agent
- 上下文管理：长对话自动摘要、工具输出落盘
- 持久化记忆：`memory` / `store`
- 权限与人工审批：`permissions` / `interrupt_on`
- Skills：按需加载可复用行为
- 可插拔 Backend：内存 StateBackend、本地 FilesystemBackend、沙箱等

## 二、和 LangGraph / LangChain 的关系

| 层 | 定位 |
| --- | --- |
| LangGraph | 图运行时：State、Node、Edge、Checkpointer、流式 |
| LangChain `create_agent` | 轻量 Agent Harness：模型 + 工具 + 中间件 |
| Deep Agents | 更“重”的 Harness：在 `create_agent` 上预装文件系统、子 Agent、上下文管理、Skills、权限等 |

一句话：**Deep Agents 不是用来替代 LangGraph 的，而是 LangGraph 之上一个更完整的 Agent 模板**。你也可以把任意 LangGraph `CompiledStateGraph` 作为 `CompiledSubAgent` 塞进 Deep Agents。

## 三、安装

```bash
cd agent主流框架/frameworks/deep-agents
python3.12 -m venv .venv
.venv/bin/pip install -U deepagents langchain-openai nbformat nbclient ipykernel
.venv/bin/python -m ipykernel install --user --name deepagents-demo --display-name "Python 3 (deepagents)"
```

注意：

- `deepagents` 要求 **Python >= 3.11**，不能用原来的 3.10 venv
- 默认模型依赖 `langchain-anthropic`，但示例用 Fake Model 不需要 Key
- 真实使用按需安装模型包：OpenAI 用 `langchain-openai`，Anthropic 用 `langchain-anthropic`

## 四、核心概念速查

| 概念 | 作用 |
| --- | --- |
| `create_deep_agent` | 主入口：一行创建完整 Deep Agent |
| 内置工具 | 默认带文件系统、`execute`、`task` 三类能力 |
| `task` / `SubAgent` | 子 Agent 委派：主 Agent 把子任务交给隔离上下文的子 Agent |
| `StateBackend` | 默认后端：文件存在 LangGraph state 里，随线程 checkpoint 持久化 |
| `FilesystemBackend` | 本地磁盘后端：直接读写真实目录 |
| `permissions` | 文件权限规则：allow / deny / interrupt |
| `interrupt_on` | 人工审批：在指定工具调用前暂停 |
| `middleware` | 中间件栈：可替换/扩展 Deep Agents 默认行为 |
| `HarnessProfile` | 按模型微调 prompt、工具可见性、中间件 |
| `DeepAgentState` | 基于 LangGraph `AgentState`，messages 用 DeltaChannel 减少 checkpoint 体积 |

## 五、最小示例

```python
from deepagents import create_deep_agent
from langchain_core.tools import tool
from langchain_openai import ChatOpenAI

@tool
def get_weather(city: str) -> str:
    """查询城市天气。"""
    return f"{city}: sunny 25C"

model = ChatOpenAI(model="gpt-5.5")  # 或任意支持 tool calling 的模型
agent = create_deep_agent(
    model=model,
    tools=[get_weather],
    system_prompt="你是深度研究助手。",
)

result = agent.invoke({"messages": [{"role": "user", "content": "北京天气怎么样？"}]})
print(result["messages"][-1].content)
```

Notebook 里用 Fake Model 复现同一机制，不需要 API Key。

> 这个最小示例只展示 ReAct 闭环，这部分 LangGraph / LangChain `create_agent` 也能做；Deep Agents 的增量在“默认装配”，不在循环本身。真正的差异看下面：自带文件工具、`task` 子 Agent、上下文管理、权限、Skills、HarnessProfile 等。

## 六、核心能力示例计划

| 能力 | 示例 | 看懂要点 |
| --- | --- | --- |
| 最小闭环 | 自定义工具 + 两轮模型调用 | `create_deep_agent` 返回的就是 LangGraph `CompiledStateGraph` |
| 子 Agent 委派 | `task` 工具调用 `researcher` | `SubAgent` 是 TypedDict，主 Agent 通过 `task` 工具发起 |
| 内置文件工具 | `write_file` + `read_file` | `StateBackend` 把文件存在 state 里，可用 `files` 预置 |
| LangGraph 关系 | 返回对象类型 / 可配 checkpointer | Deep Agents 不是新运行时，只是 LangGraph 图 |

## 七、常见问题

- **Deep Agents 和 LangChain 什么关系？** LangChain 是组件库 + `create_agent`；Deep Agents 是基于 `create_agent` 的预装 Harness。
- **和 LangGraph 什么关系？** LangGraph 是底层图运行时；Deep Agents 返回的正是 `CompiledStateGraph`，所以天然支持流式、断点、持久化。
- **为什么示例不用 Key？** Notebook 用 `FakeMessagesListChatModel` 子类模拟工具调用，机制与真实模型一致。
- **能用本地模型吗？** 可以，任何支持 tool calling 的 LangChain Chat Model 都能传。
- **生产注意什么？** 官方明确“trust the LLM”模型：能力边界要靠工具/沙箱/权限控制，不能指望模型自我约束。

## 八、下一步

- 官方文档：<https://docs.langchain.com/oss/python/deepagents/overview>
- 官方仓库：<https://github.com/langchain-ai/deepagents>
- 进阶方向：`FilesystemBackend`、`permissions` + `interrupt_on` 人工审批、`memory` / `store` 长期记忆、`Skills`、把自定义 LangGraph 图作为 `CompiledSubAgent` 接入
