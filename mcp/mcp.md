# mcp是什么
mcp是一种标准化接口协议，统一了ai模型与外部资源的链接方式，用于让llm能够访问外部的数据、工具和提示词。

# mcp和function calling 区别
Function Calling 解决的是"模型怎么表达我要调工具"，是模型层的能力；MCP 解决的是"工具怎么标准化接进来"，是基础设施层的协议。两者不是替代关系，而是上下游协作关系——MCP 的客户端内部，恰恰要靠 Function Calling 来让模型做决策。

没有 MCP 时：
假设有 N 个 AI 应用（Cursor、Claude Desktop、自研 Agent），M 个外部工具（GitHub、数据库、Slack、飞书）。
每个应用想接每个工具，都得单独写一套对接代码 —— 工作量是 N × M

有了 MCP 后：
工具方只需按 MCP 规范写一个 MCP Server（共 M 个）；
应用方只需实现一次 MCP Client（共 N 个）；
工作量变成 N + M。

# mcp实际链路
1. MCP Client 连接到各个 MCP Server
2. 拉取所有 MCP Server 暴露的工具列表（name + description + JSON Schema）
3. 把这些工具信息，转成模型厂商要求的 tools 格式，塞进请求
4. 【Function Calling 登场】模型看到工具列表，判断调哪个，输出结构化 Tool Call
5. MCP Client 截获这个调用，通过 MCP 协议转发给对应的 MCP Server
6. MCP Server 执行，返回结果
7. 结果回灌给模型，模型生成最终回答

# mcp本地server和远端server对比
主要是部署位置和通信协议的区别
部署位置：一个是本地部署，一个是云端或者远程服务器
通信协议：
- 本地：stdio（标准输入输出，在 C 语言中，stdio 通过 <stdio.h> 提供了一系列函数，比如 printf ）
- 远程：Streamable HTTP（Streamable HTTP 底层就是标准 HTTP，区别是 MCP 在 HTTP 上规定了一套标准。普通后端 HTTP API 的接口语义通常是业务自己定义的）

# mcp server tool 设计原则
1. 单一职责，粒度适中
2. schema清晰，输入输出稳定
3. 注意副作用和权限边界明确：MCP Tool 的权限边界不能只靠 prompt 约束，真正的权限控制必须落在 Server 端。模型只负责发起调用，Server 要根据用户身份、scope 和 Tool 风险等级决定是否执行

# mcp新技术&趋势
- tool search ：工具先注册，但是description不全部进入prompt；需要时再检索并加载相关工具
- code mode：传统是：LLM → tool A → LLM → tool B → LLM → tool C；code mode是：LLM 先生成一段流程/code -> runtime 连续执行多个 tool
- agent skills:MCP 更偏“我有哪些能力可以调用”；Skill 更偏“这类任务应该怎么做”。所以 Skill 是更高层的任务封装，底层依然可能调用 MCP Tool。如果某个任务已经有成熟 CLI、SDK、脚本，Agent 可以直接执行 Skill，不一定再包一层 MCP Server。

所以替代的不是mcp本身，而是出啊弄工具一步一步调用，全量暴露工具