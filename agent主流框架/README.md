# Agent 主流框架学习指南

> 本目录以「学什么、怎么学、学哪些」为主线。学习目标是：建立对主流 Agent 框架的完整认知，每个框架都有一份带逐块拆解的核心用法示例，能看懂框架是怎么工作的。

## 一、学什么

### 学习目标

1. 知道市面上有哪些主流 Agent 框架，能说出各自的定位
2. 对每个必学框架，能讲清特点、优缺点和局限性
3. 掌握一套稳定的选型方法，能根据场景做出选择
4. 每个框架都能看懂一份带逐块拆解的核心用法示例，能讲清框架的核心机制

### 必学内容

| 内容 | 说明 |
| --- | --- |
| 市面上有什么框架 | 当前主流的代码框架、平台、协议，各自属于哪一类 |
| 框架特点 | 核心抽象、设计哲学、典型使用方式 |
| 优缺点 | 强在哪里、弱在哪里、适合什么场景 |
| 局限性 | 生产短板、生态绑定、学习成本、维护状态 |
| 如何选型 | 选型维度、决策方法、checklist |
| 核心用法示例 | 每个框架一个可运行示例 + 逐块代码拆解（AI 写代码，重点是看懂） |

## 二、怎么学

### 每个框架固定四步

1. **特点**：读官方文档首页和一篇权威评测，回答「核心抽象是什么、它想解决什么问题」
2. **优缺点**：记录至少各 3 条，必须来自官方资料或实际体验
3. **局限性**：记录生产短板、生态绑定、维护状态
4. **核心用法示例**：运行示例代码，对照拆解表读懂核心机制；代码由 AI 写，但机制必须自己懂

每完成一个框架，产出两样东西：

- `frameworks/<框架名>/README.md`：四步学习笔记
- `frameworks/<框架名>/demo/<框架名>使用方法.ipynb`：带逐块拆解的核心用法 Notebook

### 学习顺序

| 阶段 | 内容 | 时间 |
| --- | --- | --- |
| 0 | 全景速览：先读本文「学哪些」和框架快照，建立整体地图 | 半天 |
| 1 | 逐个必学框架：LangGraph（已完成）→ LangChain（已完成）→ OpenAI Agents SDK（当前学习）→ CrewAI，每个先跑核心用法 Notebook 再沉淀笔记（Deep Agents 已了解，不深入） | 每个 1–2 天 |
| 2 | 协议与平台：MCP / A2A，Dify / Coze | 2–3 天 |
| 3 | 选型总结：汇总各框架对比，产出选型 checklist | 1 天 |
| 4 | 实时更新：按「实时性维护」核验生态变化 | 每 1–3 个月 |

顺序可按兴趣调整，但每个框架的四步流程不变。

## 三、学哪些

### 必学代码框架

| 框架 | 一句话定位 | 为什么必学 |
| --- | --- | --- |
| LangGraph | 状态图编排，生产级 Agent | 工程能力最强、生态最全 |
| LangChain | 组件库 + LCEL 管道 | 先理解组件与管道，再学图编排更顺 |
| OpenAI Agents SDK | 轻量 Agent 循环 | 最小闭环、官方维护 |
| CrewAI | 角色化多 Agent | 上手快、多 Agent 心智模型直观 |

### 按需了解（定位和取舍即可）

- **Deep Agents**：LangGraph 生态的长任务/深度 Agent Harness，已了解（本质是 LangGraph 预装工具包，不深入）
- **Google ADK**：多 Agent 分层编排，GCP 原生，协议完整
- **Microsoft Agent Framework**：AutoGen 与 Semantic Kernel 的统一后继，微软栈场景
- **Pydantic AI**：类型安全、结构化输出，适合生产服务
- **LlamaIndex**：文档密集型流水线，深度 RAG 场景

### 暂不单独学

- **AgentScope**：除非专门做国产多 Agent 研究，优先级最低

### 必学平台

- **Dify**：开源、可私有化，学习 RAG 与工作流产品化
- **Coze**：云托管低代码，学习平台形态的上手体验和边界

### 必学协议

- **MCP**：Agent 连接工具与数据的统一接口
- **A2A**：Agent 之间通信的标准，与 MCP 互补

### 框架实时快照（2026-08-18）

星标与最近提交来自 GitHub API，当日抓取，可能随时变化；此表覆盖主流生态全景，学习清单以必学与按需为准。

| 框架 | GitHub 星标 | 最近提交 | 仓库 |
| --- | --- | --- | --- |
| LangGraph | 39.9k | 2026-08-16 | [langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) |
| CrewAI | 57.2k | 2026-08-18 | [crewAIInc/crewAI](https://github.com/crewAIInc/crewAI) |
| OpenAI Agents SDK | 28.7k | 2026-08-17 | [openai/openai-agents-python](https://github.com/openai/openai-agents-python) |
| Google ADK | 21.2k | 2026-08-18 | [google/adk-python](https://github.com/google/adk-python) |
| Claude Agent SDK | 7.9k | 2026-08-17 | [anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python) |
| Microsoft Agent Framework | 12.9k | 2026-08-18 | [microsoft/agent-framework](https://github.com/microsoft/agent-framework) |
| AutoGen（维护模式） | 60.5k | 2026-04-15 | [microsoft/autogen](https://github.com/microsoft/autogen) |
| AG2 | 4.9k | 2026-08-17 | [ag2ai/ag2](https://github.com/ag2ai/ag2) |
| Pydantic AI | 19.4k | 2026-08-18 | [pydantic/pydantic-ai](https://github.com/pydantic/pydantic-ai) |
| smolagents | 28.8k | 2026-07-21 | [huggingface/smolagents](https://github.com/huggingface/smolagents) |
| Mastra | 27.3k | 2026-08-18 | [mastra-ai/mastra](https://github.com/mastra-ai/mastra) |
| LlamaIndex | 51.7k | 2026-08-17 | [run-llama/llama_index](https://github.com/run-llama/llama_index) |
| Dify | 152.8k | 2026-08-18 | [langgenius/dify](https://github.com/langgenius/dify) |
| MCP 规范 | 9.0k | 2026-08-17 | [modelcontextprotocol/modelcontextprotocol](https://github.com/modelcontextprotocol/modelcontextprotocol) |

## 四、核心用法示例计划

必学框架的 demo 都按「最小可用」标准设计：只验证该框架最核心的能力，不堆功能；按需框架学到时再补 demo。

| 框架 | 核心用法示例 | 看懂要点 |
| --- | --- | --- |
| LangGraph | 电商订单场景贯穿：订单流水线 + 两种写死方式 + 六大核心能力示例（已完成） | 能讲清 State、Node、Edge、条件边、Reducer，以及循环、并行、持久化、人工介入、流式、多 Agent |
| LangChain | 电商客服问答场景贯穿：LCEL 骨架链 + 两种写法 + 六大核心能力示例（已完成） | 能讲清 Runnable、LCEL、PromptTemplate、OutputParser，以及批处理、结构化解析、统一接口、记忆、流式、工具 |
| Deep Agents | 调研写报告场景贯穿：最小闭环 + 子 Agent 委派 + 内置文件工具（已了解，不深入） | 能讲清 create_deep_agent、内置工具、SubAgent/task 委派、StateBackend 与 LangGraph 关系 |
| OpenAI Agents SDK | 客服路由场景贯穿：单 Agent + 工具 + handoff + Agent as tool + context（已完成） | 能讲清 Agent、Runner、function_tool、handoff 接管、as_tool 委托、RunContextWrapper |
| CrewAI | 3 角色团队拆解示例（规划） | 能讲清角色、任务、协作方式 |
| Dify | 工作流 + 知识库问答拆解示例（规划） | 能讲清应用编排与 RAG 流程 |
| Coze | 客服 Agent 拆解示例（规划） | 能讲清低代码平台的边界 |

## 五、如何选型

### 选型维度

- 团队技术栈：Python、TypeScript 还是 .NET
- 部署方式：私有化、自托管还是云托管
- 生产要求：持久化、断点恢复、可观测性、评估
- 生态绑定：模型厂商、云厂商、办公生态
- 维护状态：是否活跃、是否进入维护模式

### 三问决策法

1. 要不要写代码？不需要 → Coze / Dify；需要 → 继续
2. 要生产级复杂编排，还是快速原型？复杂编排 → LangGraph 或 Microsoft Agent Framework；快速原型 → CrewAI 或 OpenAI Agents SDK
3. 是否绑定某家生态？GCP → Google ADK；微软 → Microsoft Agent Framework；不绑定 → LangGraph / OpenAI Agents SDK

### 选型 checklist

- [ ] 核心抽象是否匹配你的场景
- [ ] 持久化、恢复、人在回路是否开箱即用
- [ ] 可观测性与评估是否成熟
- [ ] 维护状态是否活跃，是否有明确后继
- [ ] 是否支持 MCP
- [ ] 私有化、成本、数据合规是否可接受

## 六、目录规划

```text
agent主流框架/
├── README.md          # 本文件：学什么、怎么学、学哪些
├── frameworks/
│   ├── langgraph/              # 必学：四步笔记 + 核心用法 Notebook
│   ├── langchain/              # 必学（已完成）：四步笔记 + 核心用法 Notebook
│   ├── deep-agents/            # 已了解（不深入，LangGraph 生态预装工具包）
│   ├── openai-agents-sdk/      # 当前学习：四步笔记 + 核心用法 Notebook
│   ├── crewai/                 # 必学
│   ├── google-adk/             # 按需
│   ├── microsoft-agent-framework/  # 按需
│   ├── pydantic-ai/            # 按需
│   └── llama-index/            # 按需
├── platforms/
│   ├── dify/                   # 必学
│   └── coze/                   # 必学
├── protocols/
│   ├── mcp/
│   └── a2a/
└── 选型总结.md
```

## 七、实时性维护

- **数据快照**：2026-08-18（Asia/Shanghai）
- **来源**：GitHub API（当日抓取）、官方文档、2026 年 5–8 月公开评测
- **核验频率**：每 1–3 个月一次

核验清单：

1. 核对「框架实时快照」表中的星标与最近提交
2. 核对官方公告：AutoGen / Microsoft Agent Framework 状态、MCP 与 A2A 版本
3. 每季度扫一眼生态综述，确认是否有新框架进入主流
4. 更新快照日期，并在「修改记录」追加一行

修改记录（最新在前）：

| 日期 | 修改内容 |
| --- | --- |
| 2026-08-18 | 开始学习 OpenAI Agents SDK：新增目录、README、Python 3.12 venv 与带看懂式 Notebook；单 Agent、工具、handoff、Agent as tool、context 的无 Key cell 全部验证通过 |
| 2026-08-18 | 根据用户决定将 Deep Agents 从「暂不单独学」改为「当前学习」，新增 deep-agents 目录、README 与带看懂式 Notebook（3 个无 Key 示例验证通过）；LangChain 标记已完成 |
| 2026-08-18 | 按对齐意见重写：以「学什么、怎么学、学哪些」为主线，补充每个框架的最小 demo 计划 |
| 2026-08-18 | 必学清单精简为 LangGraph、OpenAI Agents SDK、CrewAI、Dify、Coze；Google ADK、Microsoft Agent Framework、Pydantic AI、LlamaIndex 降为按需；LangChain、Deep Agents、AgentScope 暂不单独学 |
| 2026-08-18 | LangGraph 文档补充函数式 API（entrypoint/task）直接调用版 demo 与手搓版对比 |
| 2026-08-18 | LangGraph 增加 Jupyter 学习版 Notebook，讲解与代码穿插，含四种用法与对比 |
| 2026-08-18 | LangGraph demo 目录改为单一 Jupyter Notebook，原三个 .py 脚本已并入对应 cell |
| 2026-08-18 | LangGraph Notebook 改为练习式：顶部先画框架流程图，核心用法通过 3 个练习带做（2 个无 Key 必做 + 1 个需 Key 可选），已验证 |
| 2026-08-18 | LangGraph Notebook 核心概念后补充「概念之间的关系」：用客服工单处理例子把 State/Node/Edge/条件边/Reducer 串起来，含可运行代码 |
| 2026-08-18 | LangGraph Notebook 改为「带看懂式」：去掉手写练习，三个例子直接给完整代码并配逐块拆解表 |
| 2026-08-18 | README 与生成提示词同步改为「带看懂式」：学习载体从手写最小 demo 改为可运行示例 + 代码拆解 |
| 2026-08-18 | LangGraph Notebook 代码拆解改为按执行顺序讲解，并带入具体输入（工单/数字例子）逐步追踪 state 变化 |
| 2026-08-18 | LangGraph Notebook 新增「核心能力逐个看」：循环、并行（Send/Command）、Checkpointer、interrupt 人工介入、stream、多 Agent 编排，36 个 cell 全部验证通过 |
| 2026-08-18 | 通用生成提示词同步更新：必学内容加入「核心能力逐个看」，Notebook 要求顶部流程图 + 概念关系真实例子 + 按执行顺序拆解 |
| 2026-08-18 | LangGraph Notebook 全量更新例子：统一改为电商订单场景（订单流水线、计算运费、物流重试、三源并行、进度持久化、大额审批、多 Agent 分派），36 个 cell 验证通过 |
| 2026-08-18 | LangGraph Notebook 概念关系更新：用订单系统全景图 + 关系表把核心概念表全部 9 个概念串起来，骨架流水线作为可运行入口 |
| 2026-08-18 | 新增 Notebook生成提示词.md：通用 ipynb 生成模板（流程图 → 概念关系全景 → 用法拆解 → 核心能力逐个看），与 README 提示词互相链接 |
| 2026-08-18 | LangGraph Notebook 六大核心能力代码全部补充自动生成的真实流程图输出（draw_mermaid），36 个 cell 验证通过 |
| 2026-08-18 | LangGraph Notebook 流程图输出改为 PNG 图片显示（draw_mermaid_png + IPython.display.Image），不再打印 mermaid 文本；提示词模板同步 |
| 2026-08-18 | LangGraph Notebook 流程图改为提前手绘 markdown Mermaid（例子与六大能力各配一张清晰流程图），代码不再运行时生成图；提示词模板同步 |
| 2026-08-18 | 新增 LangChain 学习目录：LangChain 1.3.15 带看懂式 Notebook（客服问答场景 + 6 大核心能力，32 个无 Key cell 验证通过），提升为必学框架 |

## 八、数据来源

- GitHub API 仓库信息（抓取日期 2026-08-18）
- [LangChain：The best AI agent frameworks in 2026](https://www.langchain.com/resources/ai-agent-frameworks)（2026-06-05）
- [腾讯云社区：6 大 AI Agent 平台横评](https://cloud.tencent.com.cn/developer/article/2674338)（2026-05-26）
- A2A / MCP 相关公开报道（2026-03 至 2026-08）
