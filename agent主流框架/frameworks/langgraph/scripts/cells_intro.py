"""
脚本功能：
定义 LangGraph 学习 Notebook 的入门部分内容：标题、框架总览图、核心概念表，
以及用电商订单处理系统把全部核心概念串起来的关系图与可运行的骨架流水线。

启动命令：
由 scripts/build_langgraph_notebook.py 导入，不单独运行。
"""


def build_intro_cells() -> list[tuple]:
    """
    输入：无。
    输出：入门部分的 cell 描述列表，元素为 ("md"|"code", source[, tags])。
    功能：组装标题、总览图、核心概念、概念关系全景与骨架流水线例子的内容。
    """
    return [
        (
            "md",
            "# LangGraph 核心用法（带看懂版）\n\n"
            "> 目标：带你看懂 LangGraph 的核心代码。全书用同一条真实场景贯穿：**电商订单处理**。\n"
            "> 先看一张总览图，再用订单系统串起全部核心概念，看懂两种写死方式，最后逐个看懂六大核心能力。\n"
            "> 基于 langgraph 1.2.11（2026-08-18）验证。",
        ),
        (
            "md",
            "## 框架总览：一张图看懂 LangGraph\n\n"
            "LangGraph 的核心是把流程画成一张有向图：**状态在节点之间流动，边决定流转方向，条件边决定分支**。\n"
            "图上的节点可以**循环**、可以**并行**，执行进度可以被 **Checkpointer 持久化**，\n"
            "流程可以 **interrupt 挂起等人审批**再恢复，还可以用 **stream 逐步输出**，多 Agent 则是把整个图当成一个节点。\n\n"
            "```mermaid\n"
            "flowchart TB\n"
            "    A[用户输入] -->|invoke| B[State（状态）]\n"
            "    B --> C[Node（节点）]\n"
            "    C --> D{条件边}\n"
            "    D -->|继续| E[下一个节点]\n"
            "    D -->|结束| Z[END（结束）]\n"
            "    E -.循环回到某节点.-> C\n"
            "    C -.并行 Send.-> F[并行节点组]\n"
            "    C -.checkpoint 落盘.-> G[Checkpointer（检查点）]\n"
            "    G -.崩溃后按 thread 恢复.-> C\n"
            "    C -.interrupt 挂起.-> H[人工审批]\n"
            "    H -.resume 恢复.-> C\n"
            "```",
        ),
        (
            "md",
            "## 核心概念（先读一遍，遇到再回来看）\n\n"
            "| 概念 | 作用 | 在哪看 |\n"
            "| --- | --- | --- |\n"
            "| State | 图的共享状态，用 `TypedDict` 定义 | 概念关系、例子一 |\n"
            "| Node | 普通函数，返回 state 的部分更新 | 概念关系、例子一 |\n"
            "| Edge / 条件边 | 节点流转与分支路由 | 概念关系、例子一 |\n"
            "| 循环 | 条件边指回自己，形成 Loop | 能力 1 |\n"
            "| Send | 一个节点扇出多个并行子节点 | 能力 2 |\n"
            "| Checkpointer | 执行进度持久化，按 thread 恢复 | 能力 3 |\n"
            "| interrupt | 流程挂起，等外部（人）输入再恢复 | 能力 4 |\n"
            "| stream | 逐步拿到每一步的输出 | 能力 5 |\n"
            "| 多 Agent 编排 | supervisor 分派任务给多个 worker | 能力 6 |",
        ),
        (
            "md",
            "## 概念之间的关系：用订单系统全部串起来\n\n"
            "下面把上一张表里的**全部概念**放进同一个订单处理系统。先看全景图：\n\n"
            "```mermaid\n"
            "flowchart TB\n"
            "    A[receive_order（接收订单）] --> B{route_by_stock（库存充足？）}\n"
            "    B -->|有货| C[deduct（扣库存）]\n"
            "    B -->|缺货| D[restock（通知补货）]\n"
            "    C --> E[complete（完成订单）]\n"
            "    D --> E\n"
            "    E --> F[END（结束）]\n"
            "    A -.并行 Send.-> P[订单/库存/物流 同时查询]\n"
            "    P -.查询结果.-> A\n"
            "    C -.物流失败循环重试.-> L[query（查询物流）]\n"
            "    L -.查询成功.-> C\n"
            "    C -.checkpoint 落盘.-> G[Checkpointer（检查点）]\n"
            "    C -.interrupt 挂起.-> H[人工审批（大额订单）]\n"
            "    H -.resume 恢复.-> C\n"
            "    E -.stream 逐步输出.-> S[前端展示]\n"
            "    S -.多 Agent 分派.-> M[supervisor（主 Agent）]\n"
            "```\n\n"
            "| 概念 | 在订单系统里的位置 |\n"
            "| --- | --- |\n"
            "| State | `OrderState`：order_id、item、in_stock、logs，订单数据全程跟着图走 |\n"
            "| Node | `receive_order`、`deduct`、`restock`、`complete` 等处理环节 |\n"
            "| Edge / 条件边 | `route_by_stock` 决定有货扣库存、缺货通知补货 |\n"
            "| 循环 | 物流查询失败自动重试，直到成功（能力 1） |\n"
            "| Send | 订单、库存、物流三个查询并行执行（能力 2） |\n"
            "| Checkpointer | 批量订单处理进度落盘，崩溃后按 thread 恢复（能力 3） |\n"
            "| interrupt | 大额订单挂起等人审批，resume 后继续（能力 4） |\n"
            "| stream | 前端逐步展示每一步处理过程（能力 5） |\n"
            "| 多 Agent | supervisor 把任务分派给订单 Agent / 客服 Agent（能力 6） |\n\n"
            "先运行下面的**骨架流水线**（只有 State、Node、Edge、条件边、Reducer），"
            "建立最小直觉；其余能力会在后面的「核心能力逐个看」用同一个订单场景逐个跑。",
        ),
        (
            "code",
            'from typing import Annotated, TypedDict\n'
            'import operator\n\n'
            'from langgraph.graph import StateGraph, START, END\n\n\n'
            'class OrderState(TypedDict):\n'
            '    order_id: str\n'
            '    item: str\n'
            '    in_stock: bool\n'
            '    logs: Annotated[list[str], operator.add]\n\n\n'
            'def receive_order(state: OrderState) -> dict:\n'
            '    """接收订单。"""\n'
            '    return {"logs": [f"收到订单：{state[\'order_id\']}（{state[\'item\']}）"]}\n\n\n'
            'def deduct(state: OrderState) -> dict:\n'
            '    """有货：扣减库存。"""\n'
            '    return {"logs": ["库存已扣减"]}\n\n\n'
            'def restock(state: OrderState) -> dict:\n'
            '    """缺货：通知补货。"""\n'
            '    return {"logs": ["库存不足，已通知补货"]}\n\n\n'
            'def complete(state: OrderState) -> dict:\n'
            '    """完成订单。"""\n'
            '    return {"logs": ["订单完成"]}\n\n\n'
            'def route_by_stock(state: OrderState) -> str:\n'
            '    """按库存是否充足路由。"""\n'
            '    return "deduct" if state["in_stock"] else "restock"\n\n\n'
            'graph = StateGraph(OrderState)\n'
            'graph.add_node("receive_order", receive_order)\n'
            'graph.add_node("deduct", deduct)\n'
            'graph.add_node("restock", restock)\n'
            'graph.add_node("complete", complete)\n'
            'graph.add_edge(START, "receive_order")\n'
            'graph.add_conditional_edges(\n'
            '    "receive_order", route_by_stock, {"deduct": "deduct", "restock": "restock"}\n'
            ')\n'
            'graph.add_edge("deduct", "complete")\n'
            'graph.add_edge("restock", "complete")\n'
            'graph.add_edge("complete", END)\n'
            'compiled = graph.compile()\n\n'
            'for order in (\n'
            '    {"order_id": "A001", "item": "键盘", "in_stock": True, "logs": []},\n'
            '    {"order_id": "A002", "item": "显示器", "in_stock": False, "logs": []},\n'
            '):\n'
            '    result = compiled.invoke(order)\n'
            '    print(result["logs"])\n',
        ),
        (
            "md",
            "### 骨架流水线：按执行顺序走一遍\n\n"
            "用订单 `{\"order_id\": \"A001\", \"item\": \"键盘\", \"in_stock\": True, \"logs\": []}` 追踪：\n\n"
            "| 顺序 | 执行的代码 | 此时 logs 变成 |\n"
            "| --- | --- | --- |\n"
            "| 1 | `compiled.invoke(订单)` | `[]` |\n"
            "| 2 | 进入 `receive_order` | `[\"收到订单：A001（键盘）\"]` |\n"
            "| 3 | `route_by_stock`：`in_stock=True` → `\"deduct\"` | 不变 |\n"
            "| 4 | 进入 `deduct` | `[\"收到订单：A001（键盘）\", \"库存已扣减\"]` |\n"
            "| 5 | 进入 `complete` | `[\"收到订单：A001（键盘）\", \"库存已扣减\", \"订单完成\"]` |\n"
            "| 6 | 到达 END，返回 | 打印三条记录 |\n\n"
            "订单 `A002`（`in_stock=False`）只有第 3、4 步不同：走 `restock`，输出\n"
            "`[\"收到订单：A002（显示器）\", \"库存不足，已通知补货\", \"订单完成\"]`。\n\n"
            "这张骨架图只演示了 State、Node、Edge、条件边、Reducer 五个概念；"
            "循环、并行、持久化、人工介入、流式、多 Agent 是加在骨架上的增强，"
            "后面「核心能力逐个看」会逐一跑给你看。",
        ),
    ]
