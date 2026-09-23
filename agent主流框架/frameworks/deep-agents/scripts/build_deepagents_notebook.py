"""生成 Deep Agents 核心用法 Jupyter Notebook（带看懂版）。

启动命令：
    python3.12 scripts/build_deepagents_notebook.py
"""

from pathlib import Path

import nbformat
from nbformat.v4 import new_code_cell, new_markdown_cell, new_notebook

NOTEBOOK_PATH = Path(__file__).resolve().parent.parent / "demo" / "DeepAgents使用方法.ipynb"
KERNEL_NAME = "deepagents-demo"
KERNEL_DISPLAY_NAME = "Python 3 (deepagents)"


def md(src: str):
    return new_markdown_cell(src)


def code(src: str, tags=None):
    cell = new_code_cell(src)
    if tags:
        cell.metadata["tags"] = tags
    return cell


def build_cells():
    cells = []

    # ---------- 标题 ----------
    cells.append(md(
        "# Deep Agents 核心用法（带看懂版）\n\n"
        "> 目标：带你看懂 Deep Agents 的核心代码。它不是独立框架，而是 LangGraph + LangChain `create_agent` 之上的“深度 Agent Harness”。\n"
        "> 三个可运行示例全部使用 Fake Model，不需要 API Key。\n"
        "> 基于 `deepagents 0.7.7` / Python 3.12（2026-08-18）验证。"
    ))

    # ---------- 总览 ----------
    cells.append(md(
        "## 框架总览：一张图看懂 Deep Agents 的位置\n\n"
        "```mermaid\n"
        "flowchart LR\n"
        "    A[LangGraph 图运行时<br/>State/Node/Edge/Checkpointer] --> B[LangChain create_agent<br/>模型 + 工具 + 中间件]\n"
        "    B --> C[Deep Agents Harness<br/>文件系统/子Agent/上下文/权限/Skills]\n"
        "    C --> D[你的 Agent]\n"
        "```\n\n"
        "Deep Agents 返回的对象就是 `CompiledStateGraph`，所以 LangGraph 的流式、断点、持久化能力天然可用。"
    ))

    # ---------- 安装 ----------
    cells.append(md(
        "## 安装\n\n"
        "```bash\n"
        "cd agent主流框架/frameworks/deep-agents\n"
        "python3.12 -m venv .venv\n"
        ".venv/bin/pip install -U deepagents langchain-openai nbformat nbclient ipykernel\n"
        ".venv/bin/python -m ipykernel install --user --name deepagents-demo --display-name \"Python 3 (deepagents)\"\n"
        "```\n\n"
        "注意：`deepagents` 要求 **Python >= 3.11**，不能用原来的 3.10 venv。"
    ))

    # ---------- 概念表 ----------
    cells.append(md(
        "## 核心概念速查\n\n"
        "| 概念 | 作用 |\n"
        "| --- | --- |\n"
        "| `create_deep_agent` | 主入口，返回一个 LangGraph `CompiledStateGraph` |\n"
        "| 内置工具 | 默认带 `ls` / `read_file` / `write_file` / `edit_file` / `glob` / `grep` / `execute` / `task` |\n"
        "| `SubAgent` | 子 Agent 声明：`name` + `description` + `system_prompt`，可选 `tools` / `model` |\n"
        "| `task` 工具 | 主 Agent 用来把子任务委派给子 Agent |\n"
        "| `StateBackend` | 默认后端，文件存在 LangGraph state 里 |\n"
        "| `permissions` | 文件权限：allow / deny / interrupt |\n"
        "| `interrupt_on` | 在指定工具调用前暂停，人工审批 |\n"
        "| `middleware` | 中间件栈，可替换或扩展默认行为 |\n"
        "| `HarnessProfile` | 按模型微调 prompt / 工具 / 中间件 |"
    ))

    # ---------- 例子 1：最小闭环 ----------
    cells.append(md(
        "## 例子一：最小闭环（自定义工具 + 两轮模型调用）\n\n"
        "先看最核心的机制：`create_deep_agent` 把模型和工具打包成一个 Agent。模型第一次返回工具调用，框架执行工具，再把结果喂回模型，模型第二次返回最终回答。\n\n"
        "```mermaid\n"
        "flowchart LR\n"
        "    A[用户提问] --> B[模型 decide 调用 get_weather]\n"
        "    B --> C[框架执行 get_weather]\n"
        "    C --> D[模型 decide 最终回答]\n"
        "    D --> E[返回 messages]\n"
        "```"
    ))

    cells.append(code(
        'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
        'from langchain_core.messages import AIMessage\n'
        'from langchain_core.tools import tool\n'
        'from deepagents import create_deep_agent\n'
        '\n'
        '\n'
        'class ToolCallingFakeModel(FakeMessagesListChatModel):\n'
        '    """让 Fake Model 支持 bind_tools，从而模拟真实模型的工具调用。"""\n'
        '    def bind_tools(self, tools, **kwargs):\n'
        '        return self\n'
        '\n'
        '\n'
        '@tool\n'
        'def get_weather(city: str) -> str:\n'
        '    """查询城市天气。"""\n'
        '    return f"{city}: sunny 25C"\n'
        '\n'
        '\n'
        '# 第一次模型调用 -> 请求调用 get_weather；第二次 -> 给出最终回答\n'
        'model = ToolCallingFakeModel(responses=[\n'
        '    AIMessage(content="", tool_calls=[{\n'
        '        "name": "get_weather",\n'
        '        "args": {"city": "北京"},\n'
        '        "id": "call_1",\n'
        '        "type": "tool_call",\n'
        '    }]),\n'
        '    AIMessage(content="北京天气晴朗，25 度。"),\n'
        '])\n'
        '\n'
        'agent = create_deep_agent(model=model, tools=[get_weather])\n'
        'result = agent.invoke({"messages": [{"role": "user", "content": "北京天气怎么样？"}]})\n'
        '\n'
        'for i, m in enumerate(result["messages"]):\n'
        '    print(i, type(m).__name__, repr(getattr(m, "content", "")), getattr(m, "tool_calls", None))\n'
        'print("\\n最终回答:", result["messages"][-1].content)'
    ))

    cells.append(md(
        "#### 例子一：按执行顺序拆解\n\n"
        "| 顺序 | 发生什么 | 说明 |\n"
        "| --- | --- | --- |\n"
        "| 1 | `agent.invoke(用户消息)` | 把 `北京天气怎么样？` 发给模型 |\n"
        "| 2 | 模型返回 `tool_calls=[get_weather(北京)]` | 模型决定需要工具 |\n"
        "| 3 | 框架执行 `get_weather`，得到 `北京: sunny 25C` | 工具结果作为 ToolMessage 放回消息列表 |\n"
        "| 4 | 模型第二次返回最终回答 | 循环结束 |\n"
        "| 5 | `result[\"messages\"]` 包含完整轨迹 | 最后一条是最终回答 |\n\n"
        "关键认知：**`create_deep_agent` 返回的是 LangGraph 编译后的图**，所以这里看到的消息追加、工具执行、再回模型，就是 LangGraph 的 ReAct 循环。"
    ))

    cells.append(md(
        "#### 为什么例子一“看不出 Deep Agents 多了什么”？\n\n"
        "这是正常的。例子一只展示了 `create_deep_agent` 和普通 ReAct 一样能调用工具，这部分 LangGraph / LangChain `create_agent` 也能做。Deep Agents 的增量不在“循环”本身，而在它默认给你装好了这些：\n\n"
        "| 能力 | 如果用 LangGraph 自己搭 | Deep Agents 默认 |\n"
        "| --- | --- | --- |\n"
        "| 文件读写搜索 | 自己定义 `read_file` / `write_file` 等工具并加入图 | 自带 `ls` / `read_file` / `write_file` / `edit_file` / `glob` / `grep` |\n"
        "| 命令执行 | 自己接沙箱 / Shell | 自带 `execute`（由 backend 决定是否真正执行） |\n"
        "| 子 Agent | 自己设计子图、工具、上下文传递 | 自带 `task` 工具 + `SubAgent` 声明 |\n"
        "| 上下文管理 | 自己写摘要 / 裁剪中间件 | 默认集成 summarization、tool output 落盘 |\n"
        "| 权限 / 审批 | 自己写中间件或拦截 | 自带 `permissions` / `interrupt_on` |\n"
        "| 长期记忆 | 自己接 Store | 可选 `memory` / `store` |\n"
        "| 模型适配 | 自己调 prompt / 中间件 | 可按 `HarnessProfile` 适配模型 |\n\n"
        "所以看 Deep Agents 的价值，重点看例子二（子 Agent）和例子三（内置文件工具），以及这些“默认装配”。"
    ))

    # ---------- 例子 2：子 Agent 委派 ----------
    cells.append(md(
        "## 例子二：子 Agent 委派（task 工具）\n\n"
        "Deep Agents 的招牌能力是 `task` 工具：主 Agent 可以把一个复杂子任务交给隔离上下文的子 Agent。子 Agent 只把最终报告返回给主 Agent，中间过程不暴露给用户。\n\n"
        "```mermaid\n"
        "flowchart LR\n"
        "    A[用户提问] --> B[主 Agent 调用 task]\n"
        "    B --> C[子 Agent researcher 独立执行]\n"
        "    C --> D[返回简洁报告]\n"
        "    D --> E[主 Agent 汇总给用户]\n"
        "```"
    ))

    cells.append(code(
        'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
        'from langchain_core.messages import AIMessage\n'
        'from langchain_core.tools import tool\n'
        'from deepagents import create_deep_agent\n'
        '\n'
        '\n'
        'class ToolCallingFakeModel(FakeMessagesListChatModel):\n'
        '    def bind_tools(self, tools, **kwargs):\n'
        '        return self\n'
        '\n'
        '\n'
        '@tool\n'
        'def get_weather(city: str) -> str:\n'
        '    """查询城市天气。"""\n'
        '    return f"{city}: sunny 25C"\n'
        '\n'
        '\n'
        '# 执行顺序：主 Agent 调 task -> 子 Agent 返回报告 -> 主 Agent 汇总\n'
        'model = ToolCallingFakeModel(responses=[\n'
        '    AIMessage(content="", tool_calls=[{\n'
        '        "name": "task",\n'
        '        "args": {\n'
        '            "description": "请查询北京天气并返回简洁报告",\n'
        '            "subagent_type": "researcher",\n'
        '        },\n'
        '        "id": "call_task",\n'
        '        "type": "tool_call",\n'
        '    }]),\n'
        '    AIMessage(content="北京天气晴朗，25 度。"),\n'
        '    AIMessage(content="研究员报告：北京天气晴朗，25 度。"),\n'
        '])\n'
        '\n'
        'agent = create_deep_agent(\n'
        '    model=model,\n'
        '    tools=[get_weather],\n'
        '    subagents=[{\n'
        '        "name": "researcher",\n'
        '        "description": "查询天气并返回报告",\n'
        '        "system_prompt": "你是研究员，使用工具后返回简洁报告。",\n'
        '        "tools": [get_weather],\n'
        '    }],\n'
        ')\n'
        '\n'
        'result = agent.invoke({"messages": [{"role": "user", "content": "帮我调研北京天气"}]})\n'
        '\n'
        'for i, m in enumerate(result["messages"]):\n'
        '    print(i, type(m).__name__, repr(getattr(m, "content", "")), getattr(m, "tool_calls", None))\n'
        'print("\\n最终回答:", result["messages"][-1].content)'
    ))

    cells.append(md(
        "#### 例子二：按执行顺序拆解\n\n"
        "| 顺序 | 发生什么 | 说明 |\n"
        "| --- | --- | --- |\n"
        "| 1 | 主 Agent 收到 `帮我调研北京天气` | 模型决定调用 `task` |\n"
        "| 2 | `task(description=..., subagent_type=\"researcher\")` | 框架启动 `researcher` 子 Agent |\n"
        "| 3 | 子 Agent 独立执行，最终返回 `北京天气晴朗，25 度。` | 中间过程对用户不可见 |\n"
        "| 4 | 主 Agent 收到 ToolMessage 后再次调用模型 | 模型生成面向用户的汇总 |\n"
        "| 5 | `result[\"messages\"]` 只保留主 Agent 的消息轨迹 | 子 Agent 的内部消息不会混入 |\n\n"
        "关键认知：**`SubAgent` 就是普通 dict**，核心字段是 `name` / `description` / `system_prompt`；主 Agent 通过 `task` 工具按 `name` 选择子 Agent。"
    ))

    # ---------- 例子 3：内置文件工具 ----------
    cells.append(md(
        "## 例子三：内置文件工具（StateBackend）\n\n"
        "Deep Agents 默认带文件系统工具，而且默认 `StateBackend` 把文件存在 LangGraph state 里，不碰真实磁盘。下面用 `write_file` 写报告、`read_file` 读回来。\n\n"
        "```mermaid\n"
        "flowchart LR\n"
        "    A[用户要求写报告] --> B[模型调用 write_file]\n"
        "    B --> C[文件写入 state]\n"
        "    C --> D[模型调用 read_file]\n"
        "    D --> E[读回内容]\n"
        "    E --> F[模型汇总]\n"
        "```"
    ))

    cells.append(code(
        'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
        'from langchain_core.messages import AIMessage\n'
        'from deepagents import create_deep_agent\n'
        '\n'
        '\n'
        'class ToolCallingFakeModel(FakeMessagesListChatModel):\n'
        '    def bind_tools(self, tools, **kwargs):\n'
        '        return self\n'
        '\n'
        '\n'
        'model = ToolCallingFakeModel(responses=[\n'
        '    AIMessage(content="", tool_calls=[{\n'
        '        "name": "write_file",\n'
        '        "args": {"file_path": "/notes/report.md", "content": "# 报告\\n北京天气晴朗。"},\n'
        '        "id": "call_write",\n'
        '        "type": "tool_call",\n'
        '    }]),\n'
        '    AIMessage(content="", tool_calls=[{\n'
        '        "name": "read_file",\n'
        '        "args": {"file_path": "/notes/report.md"},\n'
        '        "id": "call_read",\n'
        '        "type": "tool_call",\n'
        '    }]),\n'
        '    AIMessage(content="已生成报告：北京天气晴朗。"),\n'
        '])\n'
        '\n'
        'agent = create_deep_agent(model=model)\n'
        'result = agent.invoke({"messages": [{"role": "user", "content": "写一份北京天气报告"}]})\n'
        '\n'
        'for i, m in enumerate(result["messages"]):\n'
        '    print(i, type(m).__name__, repr(getattr(m, "content", "")), getattr(m, "tool_calls", None))\n'
        'print("\\n最终回答:", result["messages"][-1].content)\n'
        'print("文件列表:", list((result.get("files") or {}).keys()))'
    ))

    cells.append(md(
        "#### 例子三：按执行顺序拆解\n\n"
        "| 顺序 | 发生什么 | 说明 |\n"
        "| --- | --- | --- |\n"
        "| 1 | 模型调用 `write_file(file_path=\"/notes/report.md\", ...)` | 写入 StateBackend |\n"
        "| 2 | ToolMessage 返回 `Updated file /notes/report.md` | 文件已进入 state |\n"
        "| 3 | 模型调用 `read_file(file_path=\"/notes/report.md\")` | 从 state 读回内容 |\n"
        "| 4 | ToolMessage 返回带行号内容 | 证明文件确实存在 |\n"
        "| 5 | 模型汇总最终回答 | 结束 |\n\n"
        "关键认知：**默认不碰真实磁盘**。`StateBackend` 让文件跟随 LangGraph 线程状态，天然支持 checkpoint；需要真实目录时换成 `FilesystemBackend`。"
    ))

    # ---------- 总结 ----------
    cells.append(md(
        "## 小结：Deep Agents 和 LangGraph 的关系\n\n"
        "- `create_deep_agent(...)` 返回的就是 `CompiledStateGraph`，所以 LangGraph 的 `invoke` / `stream` / checkpointer 全都能用。\n"
        "- Deep Agents 默认给你配好了文件系统、子 Agent、上下文管理、权限等“长任务 Agent 常用件”。\n"
        "- 如果只需要轻量 ReAct，用 LangChain `create_agent` 或 LangGraph `create_react_agent` 更简单；需要开箱即用的深度 Agent 时再上 Deep Agents。\n"
        "- 进阶可看：`FilesystemBackend`、`permissions` / `interrupt_on`、`memory` / `store`、`Skills`、`CompiledSubAgent` 接入自定义 LangGraph 图。"
    ))

    return cells


def main():
    notebook = new_notebook(
        cells=build_cells(),
        metadata={
            "kernelspec": {
                "display_name": KERNEL_DISPLAY_NAME,
                "language": "python",
                "name": KERNEL_NAME,
            },
            "language_info": {"name": "python", "version": "3.12"},
        },
    )
    nbformat.write(notebook, NOTEBOOK_PATH)
    print(f"notebook written: {NOTEBOOK_PATH}")


if __name__ == "__main__":
    main()
