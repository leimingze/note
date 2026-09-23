# LangChain 使用方法

> 学习框架 2/4（当前学习中）。本文是速查入口，详细学习载体是 Jupyter Notebook。基于 langchain 1.3.15 / langchain-core 1.5.6（2026-08-18）验证。

> Jupyter 学习版（带看懂式，可直接运行）：`demo/LangChain使用方法.ipynb`，用电商客服问答场景贯穿全书，内核选择 `langchain-demo`。

## 一、它是什么

LangChain 是一套**组件库 + 管道语言**：提示词、模型、解析器都是 Runnable 组件，用 LCEL 管道符 `|` 串成一条链。官方 2026 年主推 `create_agent`（Agent = Model + Harness），其 agent 构建在 LangGraph 之上。

## 二、安装

```bash
cd agent主流框架/frameworks/langchain
python3 -m venv .venv
.venv/bin/pip install -U langchain langchain-core langchain-openai nbformat nbclient ipykernel
.venv/bin/python -m ipykernel install --user --name langchain-demo --display-name "Python 3 (langchain)"
```

## 三、核心概念速查

| 概念 | 作用 |
| --- | --- |
| Runnable | 所有组件的统一接口：invoke / stream / batch |
| LCEL | 管道符 `|`，把 Runnable 串成链 |
| ChatPromptTemplate | 提示词模板，拼装消息 |
| ChatModel | 模型统一接口，可切换厂商 |
| OutputParser | 输出解析：字符串或结构化对象 |
| Memory | 会话记忆（RunnableWithMessageHistory，已弃用） |
| create_agent | 高层 Agent 构建器 |

## 四、两种写法

- **LCEL 手搓链**：`prompt | model | parser`，透明可控，适合固定流水线
- **create_agent**：模型 + 工具 + 人设一次打包，适合标准工具调用 Agent

## 五、核心能力

| 能力 | 场景 | 关键 API |
| --- | --- | --- |
| 组合与批处理 | 多条提问一次处理 | `\|` / `batch` |
| 结构化解析 | 订单信息解析成对象 | `PydanticOutputParser` |
| 统一接口 | 换模型不改代码 | `invoke` / `stream` / `batch` |
| 会话记忆 | 多轮对话记住上下文 | `RunnableWithMessageHistory`（已弃用） |
| 流式输出 | 前端打字机效果 | `stream` / `RunnableGenerator` |
| 工具与 Agent | 客服自主查订单 | `bind_tools` / `create_agent` |

## 六、常见问题

- **LangChain 和 LangGraph 什么关系？** LangChain 是组件库 + 管道；LangGraph 是图编排运行时；LangChain 的 agent 构建在 LangGraph 之上。
- **为什么无 Key 也能跑？** Notebook 用 `FakeMessagesListChatModel` 模拟模型，机制与真实模型一致。
- **记忆组件还能用吗？** 能用但已标记 deprecated，生产推荐 LangGraph 持久化。

## 七、下一步

- 官方文档：<https://docs.langchain.com/oss/python/langchain/overview>
- 进阶方向：RAG、LangSmith 可观测性、迁移到 LangGraph 的持久化
