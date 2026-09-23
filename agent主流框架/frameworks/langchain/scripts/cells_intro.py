"""
脚本功能：
定义 LangChain 学习 Notebook 的入门部分内容：标题、框架总览图、核心概念表，
以及用电商客服问答场景把核心概念串起来的关系图与可运行骨架链。

启动命令：
由 scripts/build_langchain_notebook.py 导入，不单独运行。
"""


def build_intro_cells() -> list[tuple]:
    """
    输入：无。
    输出：入门部分的 cell 描述列表，元素为 ("md"|"code", source[, tags])。
    功能：组装标题、总览图、核心概念与客服问答骨架链的内容。
    """
    return [
        (
            "md",
            "# LangChain 核心用法（带看懂版）\n\n"
            "> 目标：带你看懂 LangChain 的核心代码。全书用同一条真实场景贯穿：**电商客服订单问答**。\n"
            "> 先看总览图，再用客服问答链串起核心概念，看懂两种写法，最后逐个看懂核心能力。\n"
            "> 基于 langchain 1.3.15 / langchain-core 1.5.6（2026-08-18）验证；无 Key 示例用 fake 模型模拟。",
        ),
        (
            "md",
            "## 框架总览：一张图看懂 LangChain\n\n"
            "LangChain 是一套**组件库 + 管道语言**：提示词、模型、解析器都是 Runnable 组件，"
            "用 LCEL 管道符 `|` 串成一条链。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[用户输入] --> B[ChatPromptTemplate 提示词模板]\n"
            "    B -->|LCEL 管道 | C[ChatModel 模型]\n"
            "    C --> D[OutputParser 输出解析]\n"
            "    D --> E[最终回答]\n"
            "    C -.bind_tools 工具调用.-> F[Tool 工具]\n"
            "    B -.RunnableWithMessageHistory 会话记忆.-> G[会话历史]\n"
            "    C -.stream 流式.-> H[逐块输出]\n"
            "```\n\n"
            "先记住四个词：**Runnable（统一接口）、LCEL（管道）、ChatPromptTemplate（模板）、ChatModel（模型）**。",
        ),
        (
            "md",
            "## 核心概念（先读一遍，遇到再回来看）\n\n"
            "| 概念 | 作用 | 在哪看 |\n"
            "| --- | --- | --- |\n"
            "| Runnable | 所有组件的统一接口：`invoke` / `stream` / `batch` | 概念关系、能力 3 |\n"
            "| LCEL | 管道符 `|`，把 Runnable 串成链 | 概念关系、能力 1 |\n"
            "| ChatPromptTemplate | 提示词模板，拼装系统消息与用户输入 | 概念关系、能力 2 |\n"
            "| ChatModel | 模型统一接口，可切换厂商 | 概念关系、能力 3 |\n"
            "| OutputParser | 输出解析，把模型回复变成字符串或对象 | 概念关系、能力 2 |\n"
            "| Memory | 会话记忆，把历史塞回提示词 | 能力 4 |\n"
            "| stream | 流式输出，逐块返回 | 能力 5 |\n"
            "| bind_tools / create_agent | 工具调用与高层 Agent 构建 | 能力 6 |",
        ),
        (
            "md",
            "## 概念之间的关系：用客服问答链全部串起来\n\n"
            "把上一张表里的全部概念放进同一条**客服订单问答链**：\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[用户问题] --> B[ChatPromptTemplate 客服话术模板]\n"
            "    B -->|管道符| C[ChatModel 客服大脑]\n"
            "    C --> D[OutputParser 整理回答]\n"
            "    D --> E[回答用户]\n"
            "    C -.bind_tools 查询订单.-> F[订单查询工具]\n"
            "    B -.Memory 记住历史.-> G[会话历史]\n"
            "    C -.stream 逐字输出.-> H[前端打字机效果]\n"
            "```\n\n"
            "| 概念 | 在客服链里的位置 |\n"
            "| --- | --- |\n"
            "| Runnable | prompt、模型、parser 都是 Runnable，统一 `invoke` 调用 |\n"
            "| LCEL | `prompt | model | parser` 管道符把三者串成一条链 |\n"
            "| ChatPromptTemplate | 客服话术模板：系统人设 + 用户问题 |\n"
            "| ChatModel | 客服大脑，这里用 fake 模型模拟真实模型 |\n"
            "| OutputParser | 把模型的回复消息变成纯字符串 |\n"
            "| Memory | 记住用户上一句，多轮对话不断上下文（能力 4） |\n"
            "| stream | 回答逐字流到前端（能力 5） |\n"
            "| bind_tools / create_agent | 让模型能查订单、组成完整 Agent（能力 6） |\n\n"
            "先运行下面的**骨架链**（不需要 API Key，用 fake 模型模拟），建立最小直觉。",
        ),
        (
            "code",
            'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
            'from langchain_core.messages import AIMessage\n'
            'from langchain_core.output_parsers import StrOutputParser\n'
            'from langchain_core.prompts import ChatPromptTemplate\n\n\n'
            '# fake 模型：不调 API，固定返回这条消息，用于本地看懂管道机制\n'
            'fake = FakeMessagesListChatModel(responses=[AIMessage(content="订单已发货，预计明天送达")])\n\n'
            'prompt = ChatPromptTemplate.from_messages([\n'
            '    ("system", "你是电商客服，用一句话回答用户问题。"),\n'
            '    ("human", "{question}"),\n'
            '])\n\n'
            '# LCEL：管道符把模板、模型、解析器串成一条链\n'
            'chain = prompt | fake | StrOutputParser()\n\n'
            'result = chain.invoke({"question": "我的订单 A001 发货了吗？"})\n'
            'print(result)\n',
        ),
        (
            "md",
            "### 骨架链：按执行顺序走一遍\n\n"
            "| 顺序 | 执行的代码 | 发生了什么 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `chain.invoke({\"question\": \"...\"})` | 输入进入管道 |\n"
            "| 2 | `prompt` 把 system + 用户问题组装成消息 | 传给模型 |\n"
            "| 3 | fake 模型返回固定 `AIMessage` | 模拟真实模型的回复 |\n"
            "| 4 | `StrOutputParser` 提取消息的文本 | 得到最终字符串 |\n\n"
            "这条链只是**一条直线**。LangChain 的真正能力在后面的「核心能力逐个看」："
            "批处理、结构化解析、记忆、流式、工具调用。",
        ),
    ]
