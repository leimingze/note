"""
脚本功能：
定义 LangChain 学习 Notebook 的写法对比部分内容：用法一（LCEL 手搓链）、
用法二（create_agent 高层构建），以及选型总结。

启动命令：
由 scripts/build_langchain_notebook.py 导入，不单独运行。
"""


def build_example_cells() -> list[tuple]:
    """
    输入：无。
    输出：写法对比部分的 cell 描述列表，元素为 ("md"|"code", source[, tags])。
    功能：组装两种写法的代码、按执行顺序拆解与选型总结。
    """
    return [
        (
            "md",
            "## 先看懂两种写法\n\n"
            "同一个客服问答任务，LangChain 有两种主流写法：\n"
            "用法一用 LCEL 管道手搓链，用法二用官方高层 `create_agent`。",
        ),
        (
            "md",
            "### 用法一：LCEL 手搓链（最透明）\n\n"
            "先看这张管道图，再看代码：\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[invoke 用户问题] --> B[prompt 组装消息]\n"
            "    B --> C[fake 模型返回回复]\n"
            "    C --> D[StrOutputParser 提取文本]\n"
            "    D --> E[print 输出]\n"
            "```",
        ),
        (
            "code",
            'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
            'from langchain_core.messages import AIMessage\n'
            'from langchain_core.output_parsers import StrOutputParser\n'
            'from langchain_core.prompts import ChatPromptTemplate\n\n\n'
            'fake = FakeMessagesListChatModel(responses=[AIMessage(content="A001 已发货")])\n\n'
            'prompt = ChatPromptTemplate.from_messages([\n'
            '    ("system", "你是电商客服。"),\n'
            '    ("human", "{question}"),\n'
            '])\n\n'
            'chain = prompt | fake | StrOutputParser()\n\n'
            'result = chain.invoke({"question": "订单 A001 发货了吗？"})\n'
            'print(result)\n',
        ),
        (
            "md",
            "#### 用法一：按执行顺序拆解\n\n"
            "| 顺序 | 执行的代码 | 结果 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `chain.invoke({\"question\": \"...\"})` | 输入进入管道 |\n"
            "| 2 | `prompt` 填充模板，生成消息列表 | `[system 客服, human 问题]` |\n"
            "| 3 | fake 模型返回 `AIMessage(content=\"A001 已发货\")` | 模拟模型回复 |\n"
            "| 4 | `StrOutputParser` 提取 `content` | 输出 `A001 已发货` |\n\n"
            "管道里每个环节都是 Runnable，`|` 只是把它们按顺序接起来。",
        ),
        (
            "md",
            "### 用法二：create_agent（官方高层构建）\n\n"
            "需要 `OPENAI_API_KEY`，没 Key 先看结构。官方定位：**Agent = Model + Harness**，"
            "harness 是模型外围的一切（提示词、工具、中间件）：\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[Agent] --> B[Model 模型]\n"
            "    A --> C[Harness 外壳]\n"
            "    C --> D[system_prompt 人设]\n"
            "    C --> E[tools 工具]\n"
            "    C --> F[中间件 middleware]\n"
            "```",
        ),
        (
            "code",
            'from langchain.agents import create_agent\n\n\n'
            'def check_stock(item: str) -> str:\n'
            '    """查询商品是否有货。"""\n'
            '    return f"{item} 有货"\n\n\n'
            'agent = create_agent(\n'
            '    model="openai:gpt-5.5",\n'
            '    tools=[check_stock],\n'
            '    system_prompt="你是电商客服助手",\n'
            ')\n\n'
            'result = agent.invoke({"messages": [{"role": "user", "content": "键盘有货吗？"}]})\n'
            'print(result["messages"][-1].content)\n',
            ("requires-api-key",),
        ),
        (
            "md",
            "#### 用法二：按执行顺序拆解（ReAct 循环）\n\n"
            "| 顺序 | 执行内容 | 说明 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `agent.invoke(用户消息)` | 把问题发给模型 |\n"
            "| 2 | 模型返回 tool_call：调用 `check_stock(\"键盘\")` | 模型决定需要工具 |\n"
            "| 3 | 框架执行工具，返回「键盘 有货」 | 结果喂回模型 |\n"
            "| 4 | 模型返回最终回答 | 循环结束 |\n\n"
            "LangChain 的 agent 构建在 LangGraph 之上，所以它天然具备持久化、人工介入等能力。",
        ),
        (
            "md",
            "## 两种写法怎么选\n\n"
            "| 写法 | 代码量 | 控制力 | 适用场景 |\n"
            "| --- | --- | --- | --- |\n"
            "| LCEL 手搓链 | 少 | 中 | 固定流水线、RAG、结构化输出 |\n"
            "| create_agent | 最少 | 低 | 标准工具调用 Agent，快速交付 |\n"
            "| LangGraph | 多 | 最强 | 复杂图、循环、人工介入、多 Agent |\n\n"
            "AI 会帮你写代码，但选哪种写法、什么时候升级到 LangGraph，需要你能看懂管道和 Agent 的区别。",
        ),
    ]
