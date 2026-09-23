"""
脚本功能：
定义 LangChain 学习 Notebook 的「核心能力逐个看」部分内容：LCEL 组合与批处理、
提示词模板与结构化解析、模型统一接口、会话记忆、流式输出、工具绑定与 create_agent。
每个能力先用 markdown Mermaid 提前画好流程图，再给可运行代码。

启动命令：
由 scripts/build_langchain_notebook.py 导入，不单独运行。
"""


def build_capability_cells() -> list[tuple]:
    """
    输入：无。
    输出：核心能力部分的 cell 描述列表，元素为 ("md"|"code", source[, tags])。
    功能：组装六个核心能力的流程图、讲解与可运行示例。
    """
    return [
        (
            "md",
            "## 核心能力逐个看\n\n"
            "骨架链是「一条直线」。下面六个能力才是 LangChain 常用但不一定一眼看懂的机制，"
            "全部沿用客服问答场景。",
        ),
        (
            "md",
            "### 能力 1：LCEL 组合与批处理\n\n"
            "`|` 可以任意组合 Runnable，`batch` 一次处理多条输入。"
            "真实场景：客服系统同时收到很多用户提问，批量交给链处理。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[用户问题 1] --> B[prompt]\n"
            "    A2[用户问题 2] --> B\n"
            "    B --> C[fake 模型]\n"
            "    C --> D[StrOutputParser]\n"
            "    D --> E[回答 1]\n"
            "    D --> F[回答 2]\n"
            "```",
        ),
        (
            "code",
            'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
            'from langchain_core.messages import AIMessage\n'
            'from langchain_core.output_parsers import StrOutputParser\n'
            'from langchain_core.prompts import ChatPromptTemplate\n\n\n'
            'fake = FakeMessagesListChatModel(responses=[\n'
            '    AIMessage(content="A001 已发货"),\n'
            '    AIMessage(content="退货需在 7 天内申请"),\n'
            '])\n\n'
            'prompt = ChatPromptTemplate.from_messages([("human", "{question}")])\n'
            'chain = prompt | fake | StrOutputParser()\n\n'
            'results = chain.batch([\n'
            '    {"question": "订单 A001 发货了吗？"},\n'
            '    {"question": "怎么退货？"},\n'
            '])\n'
            'for q, r in zip(["A001 发货了吗？", "怎么退货？"], results):\n'
            '    print(f"{q} -> {r}")\n',
        ),
        (
            "md",
            "批处理的拆解：\n\n"
            "- `chain.batch([...])` 一次喂两条输入，`|` 管道对每条输入分别走一遍\n"
            "- fake 模型按顺序消费 `responses` 列表，两条输入各拿到一条回复\n"
            "- 真实模型会并行请求，批量比循环调用更快",
        ),
        (
            "md",
            "### 能力 2：提示词模板 + 结构化解析\n\n"
            "输出解析器能把模型回复解析成带类型的对象，而不是裸字符串。"
            "真实场景：客服查询订单后，把结果解析成结构化的订单信息。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[查询订单] --> B[prompt]\n"
            "    B --> C[模型返回 JSON 文本]\n"
            "    C --> D[PydanticOutputParser 校验解析]\n"
            "    D --> E[OrderInfo 对象]\n"
            "```",
        ),
        (
            "code",
            'from pydantic import BaseModel, Field\n\n'
            'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
            'from langchain_core.messages import AIMessage\n'
            'from langchain_core.output_parsers import PydanticOutputParser\n'
            'from langchain_core.prompts import ChatPromptTemplate\n\n\n'
            'class OrderInfo(BaseModel):\n'
            '    """订单信息结构。"""\n'
            '    order_id: str = Field(description="订单号")\n'
            '    status: str = Field(description="订单状态")\n\n\n'
            '# fake 模型直接返回一段 JSON 文本\n'
            'fake = FakeMessagesListChatModel(\n'
            '    responses=[AIMessage(content=\'{"order_id": "A001", "status": "已发货"}\')]\n'
            ')\n\n'
            'prompt = ChatPromptTemplate.from_messages([("human", "{question}")])\n'
            'parser = PydanticOutputParser(pydantic_object=OrderInfo)\n'
            'chain = prompt | fake | parser\n\n'
            'obj = chain.invoke({"question": "查询订单 A001"})\n'
            'print(f"order_id={obj.order_id}, status={obj.status}")\n'
            'print(f"类型：{type(obj).__name__}")\n',
        ),
        (
            "md",
            "结构化解析的拆解：\n\n"
            "- `OrderInfo` 定义了输出结构：order_id + status\n"
            "- fake 模型返回 JSON 字符串，`PydanticOutputParser` 校验并解析\n"
            "- 结果不是字典而是 `OrderInfo` 对象，字段带类型，后续代码可以直接用 `obj.order_id`",
        ),
        (
            "md",
            "### 能力 3：模型统一接口\n\n"
            "所有 ChatModel 都提供同一个接口：`invoke` / `stream` / `batch`。"
            "真实场景：切换模型厂商（OpenAI → Anthropic → Gemini）时业务代码不用改。\n\n"
            "```mermaid\n"
            "flowchart TB\n"
            "    A[ChatModel 统一接口] --> B[invoke 单次调用]\n"
            "    A --> C[stream 流式输出]\n"
            "    A --> D[batch 批量调用]\n"
            "```",
        ),
        (
            "code",
            'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
            'from langchain_core.messages import AIMessage, HumanMessage\n\n\n'
            'model = FakeMessagesListChatModel(responses=[AIMessage(content="接口一致")])\n\n'
            'print("invoke:", model.invoke([HumanMessage(content="你好")]).content)\n'
            'print("stream:", [c.content for c in model.stream([HumanMessage(content="你好")])])\n'
            'print("batch:", [m.content for m in model.batch([[HumanMessage(content="你好")]])])\n\n'
            '# 真实切换模型：init_chat_model("anthropic:claude-sonnet-4-6")，需 API Key\n',
        ),
        (
            "md",
            "统一接口的拆解：\n\n"
            "- 同一个模型对象支持 `invoke`、`stream`、`batch` 三种调用，签名一致\n"
            "- 真实项目用 `init_chat_model(\"openai:gpt-5.5\")` 或 `\"anthropic:...\"` 切换厂商，"
            "下游代码不需要改\n"
            "- fake 模型的 stream 一次返回完整内容；真实模型会逐 token 返回",
        ),
        (
            "md",
            "### 能力 4：会话记忆\n\n"
            "默认链是无状态的，`RunnableWithMessageHistory` 把历史消息塞回提示词，实现多轮对话。"
            "真实场景：用户问完订单又追问物流，客服要记得上下文。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[用户输入] --> B[prompt]\n"
            "    G[会话历史] --> B\n"
            "    B --> C[模型]\n"
            "    C --> D[回答]\n"
            "    D --> G\n"
            "```",
        ),
        (
            "code",
            'from langchain_core.chat_history import InMemoryChatMessageHistory\n'
            'from langchain_core.language_models.fake_chat_models import FakeMessagesListChatModel\n'
            'from langchain_core.messages import AIMessage\n'
            'from langchain_core.output_parsers import StrOutputParser\n'
            'from langchain_core.prompts import ChatPromptTemplate\n'
            'from langchain_core.runnables.history import RunnableWithMessageHistory\n\n\n'
            'fake = FakeMessagesListChatModel(responses=[AIMessage(content="好的，我记住了")])\n\n'
            'prompt = ChatPromptTemplate.from_messages([\n'
            '    ("system", "你是客服。"),\n'
            '    ("placeholder", "{history}"),\n'
            '    ("human", "{input}"),\n'
            '])\n\n'
            'chain = prompt | fake | StrOutputParser()\n'
            'chain_with_history = RunnableWithMessageHistory(\n'
            '    chain,\n'
            '    get_session_history=lambda session_id: InMemoryChatMessageHistory(),\n'
            '    input_messages_key="input",\n'
            '    history_messages_key="history",\n'
            ')\n\n'
            'result = chain_with_history.invoke(\n'
            '    {"input": "你好"}, {"configurable": {"session_id": "customer-1"}}\n'
            ')\n'
            'print(result)\n',
        ),
        (
            "md",
            "会话记忆的拆解：\n\n"
            "- `get_session_history` 按 `session_id` 返回历史存储对象\n"
            "- 每次调用前，历史会注入 `{history}` 占位符，回答后新消息也会写回历史\n"
            "- **注意**：`RunnableWithMessageHistory` 已被官方标记 deprecated，"
            "生产环境推荐用 LangGraph 的持久化（thread + checkpointer）",
        ),
        (
            "md",
            "### 能力 5：流式输出\n\n"
            "`stream` 让结果逐块到达，而不是等全部算完。"
            "真实场景：前端打字机效果，用户不用干等。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[chain.stream] --> B[块1 订]\n"
            "    B --> C[块2 单]\n"
            "    C --> D[块3 已]\n"
            "    D --> E[块4 发]\n"
            "    E --> F[块5 货]\n"
            "```",
        ),
        (
            "code",
            'from langchain_core.runnables import RunnableGenerator\n\n\n'
            'def answer_stream(input):\n'
            '    """模拟模型逐字输出回答。"""\n'
            '    for ch in ["订", "单", "已", "发", "货"]:\n'
            '        yield ch\n\n\n'
            'chain = RunnableGenerator(answer_stream)\n\n'
            'for chunk in chain.stream({}):\n'
            '    print(chunk, end=" ")\n'
            'print()\n',
        ),
        (
            "md",
            "流式的拆解：\n\n"
            "- `RunnableGenerator` 把生成器函数变成 Runnable，`stream` 逐块拿到 yield 的值\n"
            "- 真实场景是 `model.stream(...)`：模型逐 token 输出，前端边收边显示\n"
            "- 流式和 invoke 是同一个接口的两种模式，链的写法不用变",
        ),
        (
            "md",
            "### 能力 6：工具绑定与 create_agent\n\n"
            "让模型调用工具需要 `bind_tools`；`create_agent` 把模型 + 工具 + 人设打包成完整 Agent。"
            "真实场景：客服 Agent 自己决定要不要查订单。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[用户问题] --> B[模型]\n"
            "    B -->|要查订单| C[订单工具]\n"
            "    C --> B\n"
            "    B -->|直接回答| D[最终回答]\n"
            "```",
        ),
        (
            "code",
            'from langchain.agents import create_agent\n'
            'from langchain_openai import ChatOpenAI\n'
            'from langchain_core.tools import tool\n\n\n'
            '@tool\n'
            'def query_order(order_id: str) -> str:\n'
            '    """查询订单状态。"""\n'
            '    return f"订单 {order_id} 已发货"\n\n\n'
            'model = ChatOpenAI(model="gpt-4o-mini").bind_tools([query_order])\n\n'
            '# 也可以直接用 create_agent 一步打包：模型 + 工具 + 人设\n'
            'agent = create_agent(\n'
            '    model=model,\n'
            '    tools=[query_order],\n'
            '    system_prompt="你是电商客服，需要时查询订单。",\n'
            ')\n\n'
            'response = agent.invoke({"messages": [{"role": "user", "content": "A001 到哪了？"}]})\n'
            'print(response["messages"][-1].content)\n',
            ("requires-api-key",),
        ),
        (
            "md",
            "工具调用的拆解：\n\n"
            "- `@tool` 把函数声明成工具；`bind_tools` 把工具绑定到模型\n"
            "- 模型返回 tool_call 时，框架执行工具并把结果喂回模型，形成 ReAct 循环\n"
            "- `create_agent` 是更高层封装：模型、工具、人设一次打包\n"
            "- 这个能力需要真实模型（API Key），代码结构与 LangGraph 的 create_react_agent 对应",
        ),
        (
            "md",
            "## 核心能力总览\n\n"
            "| 能力 | 客服场景 | 关键 API |\n"
            "| --- | --- | --- |\n"
            "| 组合与批处理 | 多条提问一次处理 | `|` / `batch` |\n"
            "| 结构化解析 | 订单信息解析成对象 | `PydanticOutputParser` |\n"
            "| 统一接口 | 换模型不改代码 | `invoke` / `stream` / `batch` |\n"
            "| 会话记忆 | 多轮对话记住上下文 | `RunnableWithMessageHistory`（已弃用） |\n"
            "| 流式输出 | 前端打字机效果 | `stream` / `RunnableGenerator` |\n"
            "| 工具与 Agent | 客服自主查订单 | `bind_tools` / `create_agent` |",
        ),
    ]
