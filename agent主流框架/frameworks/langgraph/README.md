# LangGraph 使用方法

> 必学框架 1/3。本文只讲怎么用：安装、核心概念、四种写法和核心用法 Notebook。基于 langgraph 1.2.11（2026-08-18）与官方文档验证。

> Jupyter 学习版（带看懂式，可直接运行）：`demo/LangGraph使用方法.ipynb`，用电商订单处理场景贯穿全书：先画框架流程图，用订单流水线串起核心概念，再看懂两种写死方式（计算订单运费），最后逐个看懂六大核心能力（物流重试、三源并行、进度持久化、大额审批、流式展示、多 Agent 分派），用 `.venv` 内核（`langgraph-demo`）打开。

## 一、它是什么

LangGraph 是 LangChain 生态里的低层编排框架，专注状态化、可持久化、可长时运行的 Agent。核心思路是把 Agent 画成一张有向图：节点（node）是函数，边（edge）决定流转，状态（state）在节点之间传递。

可以只装 LangGraph 单独使用；搭配 LangChain 组件（模型、工具抽象）会更顺手，但不是必须。

## 二、安装

```bash
cd agent主流框架/frameworks/langgraph
python3 -m venv .venv
.venv/bin/pip install -U langgraph langgraph-prebuilt langchain-openai
.venv/bin/pip install nbformat nbclient ipykernel
.venv/bin/python -m ipykernel install --user --name langgraph-demo --display-name "Python 3 (langgraph)"
```

- `langgraph`：核心框架，当前版本 1.2.11
- `langgraph-prebuilt`：提供 `create_react_agent`、`ToolNode` 等预置组件
- `langchain-openai`：调用 OpenAI 模型时安装；只跑不依赖模型的图可以省略

## 三、核心概念（10 分钟版）

| 概念 | 作用 |
| --- | --- |
| State | 图的共享状态，用 `TypedDict` 定义字段；字段可以配置 Reducer 决定如何合并 |
| Node | 普通函数：入参是当前 state，返回值是 state 的部分更新 |
| Edge | 边，表示节点之间的流转 |
| Conditional Edge | 条件边，按函数返回值路由到不同节点 |
| StateGraph | 构建图的类，`compile()` 后得到可执行的图 |
| invoke / stream | 同步执行；`stream` 可以逐步拿到中间输出 |
| Checkpointer | 断点与持久化，human-in-the-loop 依赖它 |

最容易踩的坑是 State 合并：没有配置 Reducer 的字段默认「后写覆盖先写」；`MessagesState` 内置的 `messages` 字段自带追加逻辑。

## 四、四种用法

### 1. 手搓 StateGraph（最底层、最可控）

```python
from langgraph.graph import StateGraph, MessagesState, START, END

def mock_llm(state: MessagesState):
    return {"messages": [{"role": "ai", "content": "hello world"}]}

graph = StateGraph(MessagesState)
graph.add_node(mock_llm)          # 节点可以是函数名
graph.add_edge(START, "mock_llm")
graph.add_edge("mock_llm", END)
graph = graph.compile()

graph.invoke({"messages": [{"role": "user", "content": "hi!"}]})
```

完整可运行版见 `demo/LangGraph使用方法.ipynb` 的「用法一」cell：无 LLM，演示状态追加、条件路由和 `invoke`。

### 2. create_react_agent（预置 ReAct 循环，一行创建）

```python
from langchain_openai import ChatOpenAI
from langchain_core.tools import tool
from langgraph.prebuilt import create_react_agent

@tool
def add(a: int, b: int) -> int:
    """计算两个整数相加。"""
    return a + b

model = ChatOpenAI(model="gpt-4o-mini")
agent = create_react_agent(model, tools=[add])
response = agent.invoke({"messages": [{"role": "user", "content": "3 + 4 等于多少？"}]})
```

`create_react_agent` 内部已经实现了「模型思考 → 调用工具 → 把结果喂回模型 → 直到不再调用工具」的循环，适合快速得到一个标准 Agent。完整版见 `demo/react_agent.py`。

### 3. ToolNode + 工具

工具用 `@tool` 定义后，可以交给 `ToolNode` 放进图里：

```python
from langgraph.prebuilt import ToolNode

tool_node = ToolNode([add])
```

手搓图时需要自己写条件边：模型返回 `tool_calls` 就进 `tool_node`，否则结束。`create_react_agent` 把这套流程内置了。

### 4. 函数式 API（entrypoint / task）

不用声明节点和边，把普通函数直接变成可执行图，调用方式最接近直接函数调用：

```python
from langgraph.func import entrypoint, task

@task
def classify(number: int) -> str:
    return f"positive: {number}" if number >= 0 else f"negative: {number}"

@entrypoint()                      # 注意要带括号
def main(state: dict) -> dict:
    steps = ["classify", classify(state["number"]).result()]  # task 返回 Future
    return {"steps": steps}

main.invoke({"number": 5})
```

与手搓版对比：手搓版需要显式定义 State、节点函数、边和条件边；函数式版把流程写在一个函数里，框架自动生成图。两者输出完全一致，完整对照见 Notebook 的「三种写法怎么选」节。

两个易错点：`@entrypoint()` 必须带括号；`@task` 调用返回 Future，需要 `.result()` 取结果。

## 五、核心用法 Notebook

原三个 `.py` 脚本的内容已全部并入 Notebook 对应 cell，demo 目录只保留一个学习载体：

| 文件 | 内容 | 运行条件 |
| --- | --- | --- |
| `demo/LangGraph使用方法.ipynb` | 电商订单场景贯穿：订单流水线 + 两种写死方式 + 六大核心能力 | 选择 `langgraph-demo` 内核；含 Key 的 cell 需配置 |

```bash
# 用 VS Code 或任意 Jupyter 客户端打开 demo/LangGraph使用方法.ipynb
# 内核选择 langgraph-demo
```

## 六、常见问题

- **LangGraph 和 LangChain 什么关系？** LangGraph 只管编排（图、状态、持久化）；模型和工具的抽象来自 LangChain，也可以完全不用 LangChain。
- **网上教程版本太旧怎么办？** 0.1.x 教程大量存在，1.x 语法以官方 overview 为准，核心就是 `StateGraph`、`START`、`END`、`compile`、`invoke`。
- **state 被覆盖而不是追加？** 字段没配 Reducer。列表字段用 `Annotated[list, operator.add]`，消息用 `MessagesState`。
- **提示找不到工具？** 工具要么传给 `create_react_agent`，要么在自建模型上调用 `bind_tools`。
- **持久化报错？** 检查是否安装了 checkpointer（`langgraph` 内置 sqlite 版本，使用 `MemorySaver` 需安装 `langgraph-checkpoint`）。

## 七、下一步

- 官方文档：<https://docs.langchain.com/langgraph/>
- 进阶方向：Checkpointer 断点恢复、human-in-the-loop、streaming、跨会话长期记忆、LangGraph Platform 部署
