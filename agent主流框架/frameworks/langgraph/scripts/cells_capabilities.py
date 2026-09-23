"""
脚本功能：
定义 LangGraph 学习 Notebook 的「核心能力逐个看」部分内容：物流查询重试（循环）、
订单/库存/物流并行查询（Send）、订单处理进度持久化（Checkpointer）、大额订单人工审批
（interrupt）、流式展示（stream）、订单/客服多 Agent 编排。每个能力先用 markdown
Mermaid 提前画好流程图，再给可运行代码。

启动命令：
由 scripts/build_langgraph_notebook.py 导入，不单独运行。
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
            "订单流水线和例子一、二都是「一条直线加一个分支」。下面这六个能力才是 LangGraph "
            "区别于普通管道（LangChain Chain / 纯 Python）的地方，全部沿用订单场景。",
        ),
        (
            "md",
            "### 能力 1：循环（物流查询重试）\n\n"
            "条件边指回自己，节点就会反复执行，直到路由函数说停。\n"
            "真实场景：物流接口不稳定，失败就重试，直到查询成功。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[START] --> B[query 查询物流]\n"
            "    B --> C{should_retry<br/>attempts < 3 ？}\n"
            "    C -->|是| B\n"
            "    C -->|否| D[END]\n"
            "```",
        ),
        (
            "code",
            'from typing import Annotated, TypedDict\n'
            'import operator\n\n'
            'from langgraph.graph import StateGraph, START, END\n\n\n'
            'class RetryState(TypedDict):\n'
            '    attempts: int\n'
            '    logs: Annotated[list[str], operator.add]\n\n\n'
            'def query_logistics(state: RetryState) -> dict:\n'
            '    """模拟物流查询：前 2 次失败，第 3 次成功。"""\n'
            '    attempts = state["attempts"] + 1\n'
            '    if attempts >= 3:\n'
            '        return {"attempts": attempts, "logs": [f"第 {attempts} 次：查询成功"]}\n'
            '    return {"attempts": attempts, "logs": [f"第 {attempts} 次：查询失败，准备重试"]}\n\n\n'
            'def should_retry(state: RetryState) -> str:\n'
            '    """还没成功就继续重试，否则结束。"""\n'
            '    return "query" if state["attempts"] < 3 else "end"\n\n\n'
            'graph = StateGraph(RetryState)\n'
            'graph.add_node("query", query_logistics)\n'
            'graph.add_edge(START, "query")\n'
            'graph.add_conditional_edges("query", should_retry, {"query": "query", "end": END})\n'
            'retry_graph = graph.compile()\n\n'
            'result = retry_graph.invoke({"attempts": 0, "logs": []})\n'
            'print(result["logs"])\n',
        ),
        (
            "md",
            "循环的拆解：\n\n"
            "- `query_logistics` 每次把 `attempts + 1`，前 2 次标记失败，第 3 次标记成功\n"
            "- `should_retry` 返回 `\"query\"` 时，条件边把执行**指回 query 自己**；返回 `\"end\"` 时走 END\n"
            "- 输出能看到完整重试轨迹：失败 → 失败 → 成功\n"
            "- 这也是 ReAct 循环的底层：模型节点反复执行，直到不再调用工具",
        ),
        (
            "md",
            "### 能力 2：并行（订单 / 库存 / 物流同时查询）\n\n"
            "一个节点可以一次性派出多个子任务，LangGraph 并行执行它们，结果用 Reducer 收集。\n"
            "真实场景：下单前同时查订单、库存、物流三个数据源。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[START] --> B[dispatcher 派发]\n"
            "    B --> C1[worker 订单]\n"
            "    B --> C2[worker 库存]\n"
            "    B --> C3[worker 物流]\n"
            "    C1 --> D[results 汇总]\n"
            "    C2 --> D\n"
            "    C3 --> D\n"
            "    D --> E[END]\n"
            "```",
        ),
        (
            "code",
            'from typing import Annotated, TypedDict\n'
            'import operator\n\n'
            'from langgraph.graph import StateGraph, START, END\n'
            'from langgraph.types import Send, Command\n\n\n'
            'class CheckState(TypedDict):\n'
            '    results: Annotated[list[str], operator.add]\n\n\n'
            'def dispatcher(state: CheckState):\n'
            '    """同时派出订单、库存、物流三个查询。"""\n'
            '    tasks = [\n'
            '        Send("worker", {"source": "订单"}),\n'
            '        Send("worker", {"source": "库存"}),\n'
            '        Send("worker", {"source": "物流"}),\n'
            '    ]\n'
            '    return Command(goto=tasks)\n\n\n'
            'def worker(state: dict) -> dict:\n'
            '    """模拟一个数据源查询。"""\n'
            '    return {"results": [f"{state[\'source\']}查询完成"]}\n\n\n'
            'graph = StateGraph(CheckState)\n'
            'graph.add_node("dispatcher", dispatcher)\n'
            'graph.add_node("worker", worker)\n'
            'graph.add_edge(START, "dispatcher")\n'
            'graph.add_edge("worker", END)\n'
            'parallel_graph = graph.compile()\n\n'
            'print(parallel_graph.invoke({"results": []})["results"])\n',
        ),
        (
            "md",
            "并行的拆解：\n\n"
            "- `dispatcher` 返回 `Command(goto=[Send(...), ...])`，每个 `Send` 指定目标节点和该任务的输入\n"
            "- LangGraph 为每个 `Send` 启动一个并行的 `worker` 实例，互不阻塞\n"
            "- 注意不要同时加 `dispatcher → worker` 静态边，`Send` 本身就是动态边\n"
            "- `results` 用 `operator.add` 收集中间结果，最终一次性汇总",
        ),
        (
            "md",
            "### 能力 3：Checkpointer 订单进度持久化\n\n"
            "给图配一个 checkpointer 后，每一步执行进度都会落盘。同一个 `thread_id` 再次调用，"
            "state 从上一次结束的位置继续。真实场景：处理大量订单时崩溃，重启后从断点继续。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[START] --> B[process_one 处理订单]\n"
            "    B --> C[END]\n"
            "    B -.每次执行结果写入.-> D[Checkpointer 落盘]\n"
            "    D -.同 thread 再次 invoke 从断点继续.-> B\n"
            "```",
        ),
        (
            "code",
            'from typing import TypedDict\n\n'
            'from langgraph.checkpoint.memory import MemorySaver\n'
            'from langgraph.graph import StateGraph, START, END\n\n\n'
            'class ProgressState(TypedDict):\n'
            '    processed: int\n\n\n'
            'def process_one(state: ProgressState) -> dict:\n'
            '    """每次处理一个订单。"""\n'
            '    return {"processed": state["processed"] + 1}\n\n\n'
            'graph = StateGraph(ProgressState)\n'
            'graph.add_node("process_one", process_one)\n'
            'graph.add_edge(START, "process_one")\n'
            'graph.add_edge("process_one", END)\n'
            'checkpoint_graph = graph.compile(checkpointer=MemorySaver())\n\n'
            'config = {"configurable": {"thread_id": "order-batch-1"}}\n'
            'print("第一次：", checkpoint_graph.invoke({"processed": 0}, config))\n'
            'print("第二次：", checkpoint_graph.invoke({"processed": 0}, config))\n',
        ),
        (
            "md",
            "持久化的拆解：\n\n"
            "- 第一次 `invoke` 后，`processed=1` 被写进 checkpointer（按 `order-batch-1` 保存）\n"
            "- 第二次用同一个 `thread_id`，输入虽然是 `0`，但框架从 checkpoint 恢复，接着累加成 `2`\n"
            "- `MemorySaver` 是内存版；生产用 Sqlite/Postgres 版本，重启不丢",
        ),
        (
            "md",
            "### 能力 4：interrupt 大额订单人工审批（human-in-the-loop）\n\n"
            "流程在指定节点挂起，返回给外部（人）一个待办；外部决定后通过 `Command(resume=...)` "
            "从断点继续。真实场景：订单金额超过 1000 元必须人工审批，审批人可能几天后才点。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[START] --> B[request_approval 请求审批]\n"
            "    B -->|interrupt 挂起| C[人工审批]\n"
            "    C -->|resume 恢复| D[finalize 执行]\n"
            "    D --> E[END]\n"
            "```",
        ),
        (
            "code",
            'from typing import TypedDict\n\n'
            'from langgraph.checkpoint.memory import MemorySaver\n'
            'from langgraph.graph import StateGraph, START, END\n'
            'from langgraph.types import Command, interrupt\n\n\n'
            'class ApprovalState(TypedDict):\n'
            '    amount: int\n'
            '    approved: str\n\n\n'
            'def request_approval(state: ApprovalState) -> dict:\n'
            '    """挂起流程，等人工审批。"""\n'
            '    decision = interrupt({"question": f"订单金额 {state[\'amount\']} 元，是否批准？"})\n'
            '    return {"approved": decision}\n\n\n'
            'def finalize(state: ApprovalState) -> dict:\n'
            '    """审批通过后执行。"""\n'
            '    return {"approved": state["approved"] + "，已执行"}\n\n\n'
            'graph = StateGraph(ApprovalState)\n'
            'graph.add_node("request_approval", request_approval)\n'
            'graph.add_node("finalize", finalize)\n'
            'graph.add_edge(START, "request_approval")\n'
            'graph.add_edge("request_approval", "finalize")\n'
            'graph.add_edge("finalize", END)\n'
            'approval_graph = graph.compile(checkpointer=MemorySaver())\n\n'
            'config = {"configurable": {"thread_id": "order-approval-1"}}\n\n'
            'result = approval_graph.invoke({"amount": 1000, "approved": ""}, config)\n'
            'print("第一次调用（挂起）：", result)\n'
            'print("中断事件：", result.get("__interrupt__"))\n\n'
            'result = approval_graph.invoke(Command(resume="同意"), config)\n'
            'print("恢复后结果：", result["approved"])\n',
        ),
        (
            "md",
            "人工介入的拆解：\n\n"
            "- `interrupt(...)` 让流程**挂起**：第一次 `invoke` 返回的 state 里带 `__interrupt__`，流程没有结束\n"
            "- 外部（审批人）看完待办后，用 `Command(resume=\"同意\")` 把决定传回\n"
            "- 框架从挂起节点恢复，`decision` 拿到 `\"同意\"`，继续走 `finalize`\n"
            "- 这就是 LangChain 管道做不到的：**流程可以停几天，然后接着跑**",
        ),
        (
            "md",
            "### 能力 5：stream 流式展示处理过程\n\n"
            "`invoke` 只给你最终结果；`stream` 让你逐步看到每一步的输出。"
            "真实场景：前端逐条展示物流查询重试的进度。\n\n"
            "图还是能力 1 的循环图，区别只在调用方式：`invoke` 等全部跑完返回一个结果，"
            "`stream` 每跑一步就吐一个事件。",
        ),
        (
            "code",
            '# 复用能力 1 的 retry_graph，按步输出每次查询的事件\n'
            'for event in retry_graph.stream({"attempts": 0, "logs": []}):\n'
            '    print(event)\n',
        ),
        (
            "md",
            "流式的拆解：\n\n"
            "- 默认 `stream_mode=\"updates\"`，每个事件形如 `{\"query\": {\"attempts\": 1, \"logs\": [...]}}`\n"
            "- 可以看到节点名字、每步的 state 更新，方便前端展示和调试\n"
            "- 还有 `stream_mode=\"values\"`、`\"messages\"` 等模式，按需选择",
        ),
        (
            "md",
            "### 能力 6：多 Agent 编排（订单 Agent / 客服 Agent）\n\n"
            "多 Agent 不是玄学：把几个 Agent 当作图的节点，用一个 supervisor 节点分派任务。"
            "真实场景里，worker 内部可以是 `create_react_agent` 这种带模型的 Agent。\n\n"
            "```mermaid\n"
            "flowchart LR\n"
            "    A[START] --> B[supervisor 主 Agent]\n"
            "    B --> C{route 按任务分派}\n"
            "    C -->|订单| D[worker_a 订单 Agent]\n"
            "    C -->|其他| E[worker_b 客服 Agent]\n"
            "    D --> F[END]\n"
            "    E --> F\n"
            "```",
        ),
        (
            "code",
            'from typing import TypedDict\n\n'
            'from langgraph.graph import StateGraph, START, END\n\n\n'
            'class TeamState(TypedDict):\n'
            '    task: str\n'
            '    result: str\n\n\n'
            'def supervisor(state: TeamState) -> dict:\n'
            '    """主 Agent：记录任务并开始分派。"""\n'
            '    return {"result": f"supervisor 收到任务：{state[\'task\']}"}\n\n\n'
            'def worker_a(state: TeamState) -> dict:\n'
            '    """订单 Agent。"""\n'
            '    return {"result": f"订单 Agent 处理：{state[\'task\']}"}\n\n\n'
            'def worker_b(state: TeamState) -> dict:\n'
            '    """客服 Agent。"""\n'
            '    return {"result": f"客服 Agent 处理：{state[\'task\']}"}\n\n\n'
            'def route(state: TeamState) -> str:\n'
            '    """按任务内容分派。"""\n'
            '    return "worker_a" if state["task"].startswith("订单") else "worker_b"\n\n\n'
            'graph = StateGraph(TeamState)\n'
            'graph.add_node("supervisor", supervisor)\n'
            'graph.add_node("worker_a", worker_a)\n'
            'graph.add_node("worker_b", worker_b)\n'
            'graph.add_edge(START, "supervisor")\n'
            'graph.add_conditional_edges("supervisor", route, {"worker_a": "worker_a", "worker_b": "worker_b"})\n'
            'graph.add_edge("worker_a", END)\n'
            'graph.add_edge("worker_b", END)\n'
            'team_graph = graph.compile()\n\n'
            'print(team_graph.invoke({"task": "订单查询", "result": ""}))\n'
            'print(team_graph.invoke({"task": "退款咨询", "result": ""}))\n',
        ),
        (
            "md",
            "多 Agent 的拆解：\n\n"
            "- `supervisor` 是主 Agent（真实场景里它用模型决定派活）\n"
            "- 条件边按任务内容路由到 `worker_a`（订单 Agent）或 `worker_b`（客服 Agent）\n"
            "- 每个 worker 内部可以是独立 Agent；把「编排结构」和「Agent 内部逻辑」分开，"
            "就是多 Agent 系统的基本形态",
        ),
        (
            "md",
            "## 核心能力总览\n\n"
            "| 能力 | 订单场景 | 关键 API |\n"
            "| --- | --- | --- |\n"
            "| 循环 | 物流查询失败重试 | `add_conditional_edges` |\n"
            "| 并行 | 订单/库存/物流同时查询 | `Send` / `Command` |\n"
            "| 持久化 | 批量订单进度落盘恢复 | `compile(checkpointer=...)` |\n"
            "| 人工介入 | 大额订单审批挂起恢复 | `interrupt` / `Command(resume=...)` |\n"
            "| 流式 | 前端逐步展示处理进度 | `stream()` |\n"
            "| 多 Agent | supervisor 分派订单/客服 Agent | StateGraph + 条件边 |",
        ),
    ]
