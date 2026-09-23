"""
脚本功能：
定义 LangGraph 学习 Notebook 的写法对比部分内容：例子一（手搓 StateGraph）、
例子二（函数式 API）实现同一任务「计算订单运费」，例子三（create_react_agent）
实现库存查询，并附选型与常见问题。流程图全部用 markdown Mermaid 提前画好。

启动命令：
由 scripts/build_langgraph_notebook.py 导入，不单独运行。
"""


def build_example_cells() -> list[tuple]:
    """
    输入：无。
    输出：写法对比部分的 cell 描述列表，元素为 ("md"|"code", source[, tags])。
    功能：组装两种写死方式、预置 Agent 与选型总结的内容。
    """
    return [
        (
            "md",
            "## 先看懂两种写死方式\n\n"
            "LangGraph 不会自动生成业务路径，流程必须由人定义。例子一和例子二就是两种写死方式：\n"
            "例子一用图 API 写死（节点、边、条件边），例子二用函数体写死（语句顺序、if 分支）。\n"
            "两个例子做同一个任务：**计算订单运费**——金额 ≥ 99 元免运费，否则收 10 元。",
        ),
        (
            "md",
            "### 例子一：手搓 StateGraph（最底层、控制力最强）\n\n"
            "先看这张图，再看下面的代码：\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[START] --> B[classify 记录入口]\n"
            "    B --> C{route_by_amount<br/>金额 >= 99 ？}\n"
            "    C -->|是| D[handle_free 免运费]\n"
            "    C -->|否| E[handle_paid 收 10 元]\n"
            "    D --> F[END]\n"
            "    E --> F\n"
            "```\n\n"
            "代码里的每个节点、边、条件边都能在这张图上找到。",
        ),
        (
            "code",
            'from typing import Annotated, TypedDict\n'
            'import operator\n\n'
            'from langgraph.graph import StateGraph, START, END\n\n\n'
            'class FeeState(TypedDict):\n'
            '    order_amount: float\n'
            '    shipping_fee: float\n'
            '    steps: Annotated[list[str], operator.add]\n\n\n'
            'def classify(state: FeeState) -> dict:\n'
            '    """记录入口节点。"""\n'
            '    return {"steps": ["classify"]}\n\n\n'
            'def handle_free(state: FeeState) -> dict:\n'
            '    """免运费分支。"""\n'
            '    return {"shipping_fee": 0.0, "steps": ["免运费"]}\n\n\n'
            'def handle_paid(state: FeeState) -> dict:\n'
            '    """收运费分支。"""\n'
            '    return {"shipping_fee": 10.0, "steps": ["运费 10 元"]}\n\n\n'
            'def route_by_amount(state: FeeState) -> str:\n'
            '    """金额 >= 99 免运费，否则收运费。"""\n'
            '    return "free" if state["order_amount"] >= 99 else "paid"\n\n\n'
            'def build_graph():\n'
            '    """构建并编译计算运费的状态图。"""\n'
            '    graph = StateGraph(FeeState)\n'
            '    graph.add_node("classify", classify)\n'
            '    graph.add_node("free", handle_free)\n'
            '    graph.add_node("paid", handle_paid)\n'
            '    graph.add_edge(START, "classify")\n'
            '    graph.add_conditional_edges(\n'
            '        "classify",\n'
            '        route_by_amount,\n'
            '        {"free": "free", "paid": "paid"},\n'
            '    )\n'
            '    graph.add_edge("free", END)\n'
            '    graph.add_edge("paid", END)\n'
            '    return graph.compile()\n\n\n'
            'compiled = build_graph()\n'
            'for amount in (129.0, 59.0):\n'
            '    result = compiled.invoke({"order_amount": amount, "shipping_fee": 0.0, "steps": []})\n'
            '    print(f"amount={amount} -> fee={result[\'shipping_fee\']}, steps={result[\'steps\']}")\n',
        ),
        (
            "md",
            "#### 例子一：按执行顺序拆解\n\n"
            "用 `order_amount=129.0` 走一遍上面的图：\n\n"
            "| 顺序 | 执行的代码 | 此时 state 变成 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `compiled.invoke({\"order_amount\": 129.0, \"shipping_fee\": 0.0, \"steps\": []})` | `shipping_fee: 0.0, steps: []` |\n"
            "| 2 | 进入 `classify` | `steps: [\"classify\"]` |\n"
            "| 3 | `route_by_amount`：`129 >= 99` → `\"free\"` | 不变 |\n"
            "| 4 | 进入 `handle_free`：`shipping_fee = 0.0` | `shipping_fee: 0.0, steps: [\"classify\", \"免运费\"]` |\n"
            "| 5 | 到达 END，返回 | 打印 `amount=129.0 -> fee=0.0` |\n\n"
            "换成 `order_amount=59.0`：第 3 步返回 `\"paid\"`，第 4 步 `shipping_fee = 10.0`。\n"
            "关键认知：**代码里先定义谁不重要，执行顺序由边决定**。",
        ),
        (
            "md",
            "### 例子二：函数式 API（代码量少一半）\n\n"
            "同一个任务。`@entrypoint()` 把函数包装成图（`main` 类型就是 `Pregel`），"
            "`@task` 标记子任务；函数体的行顺序就是执行顺序：\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[main.invoke 订单金额] --> B[组装 steps]\n"
            "    B --> C[calc_fee 任务启动]\n"
            "    C --> D[.result 等待结果]\n"
            "    D --> E[return 输出运费]\n"
            "```",
        ),
        (
            "code",
            'from langgraph.func import entrypoint, task\n\n\n'
            '@task\n'
            'def calc_fee(order_amount: float) -> float:\n'
            '    """金额 >= 99 免运费，否则 10 元。"""\n'
            '    return 0.0 if order_amount >= 99 else 10.0\n\n\n'
            '@entrypoint()\n'
            'def main(state: dict) -> dict:\n'
            '    """函数式 API 版：计算订单运费。"""\n'
            '    steps = ["classify", "免运费" if state["order_amount"] >= 99 else "运费 10 元"]\n'
            '    fee = calc_fee(state["order_amount"]).result()\n'
            '    return {"shipping_fee": fee, "steps": steps}\n\n\n'
            'for amount in (129.0, 59.0):\n'
            '    result = main.invoke({"order_amount": amount})\n'
            '    print(f"amount={amount} -> fee={result[\'shipping_fee\']}, steps={result[\'steps\']}")\n',
        ),
        (
            "md",
            "#### 例子二：按执行顺序拆解\n\n"
            "用 `order_amount=129.0`：\n\n"
            "| 顺序 | 执行的代码 | 结果 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `main.invoke({\"order_amount\": 129.0})` | 开始执行入口函数 |\n"
            "| 2 | 组装 `steps`：`129 >= 99` → `\"免运费\"` | `steps: [\"classify\", \"免运费\"]` |\n"
            "| 3 | `calc_fee(129.0)`：启动 `@task` 任务 | 返回 Future |\n"
            "| 4 | `.result()`：等待完成，得到 `0.0` | `fee = 0.0` |\n"
            "| 5 | `return` | `invoke` 返回 |\n\n"
            "区别一句话：例子一用**边**决定分支，例子二用**函数里的 if** 决定；逻辑顺序都由人写死，"
            "框架只负责调度与包装。",
        ),
        (
            "md",
            "### 例子三：create_react_agent（预置 ReAct 循环）\n\n"
            "需要 `OPENAI_API_KEY`，没 Key 先读懂调用链。它内置了「模型 → 工具 → 条件边 → 模型」的循环模板，"
            "**骨架写死，具体调哪个工具由模型动态决定**。这里让 Agent 帮你查库存。",
        ),
        (
            "code",
            'from langchain_openai import ChatOpenAI\n'
            'from langchain_core.tools import tool\n'
            'from langgraph.prebuilt import create_react_agent\n\n\n'
            '@tool\n'
            'def check_stock(item: str) -> str:\n'
            '    """查询商品是否有货。"""\n'
            '    return f"{item} 有货"\n\n\n'
            'model = ChatOpenAI(model="gpt-4o-mini")\n'
            'agent = create_react_agent(model, tools=[check_stock])\n'
            'response = agent.invoke({"messages": [{"role": "user", "content": "键盘有货吗？"}]})\n'
            'print(response["messages"][-1].content)\n',
            ("requires-api-key",),
        ),
        (
            "md",
            "#### 例子三：按执行顺序拆解（ReAct 循环）\n\n"
            "| 顺序 | 执行内容 | 说明 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `agent.invoke(用户消息)` | 把「键盘有货吗？」发给模型 |\n"
            "| 2 | 模型返回 tool_call：调用 `check_stock(\"键盘\")` | 模型决定需要工具 |\n"
            "| 3 | 框架执行工具，返回「键盘 有货」 | 结果喂回模型 |\n"
            "| 4 | 模型返回最终回答 | 循环结束 |\n\n"
            "它就是例子一的图加了一层循环：`模型 → 工具 → 条件边 → 模型`，直到模型不再要工具。",
        ),
        (
            "md",
            "## 三种写法怎么选\n\n"
            "| 写法 | 代码量 | 控制力 | 适用场景 |\n"
            "| --- | --- | --- | --- |\n"
            "| 手搓 StateGraph | 多 | 最强 | 复杂图、精确控制路由与状态 |\n"
            "| entrypoint / task | 少 | 中 | 线性或简单分支流程 |\n"
            "| create_react_agent | 最少 | 低 | 标准工具调用 Agent，快速交付 |\n\n"
            "AI 会帮你写代码，但选哪种写法、什么时候需要精确控制路由，需要你能看懂代码才能判断。",
        ),
    ]
