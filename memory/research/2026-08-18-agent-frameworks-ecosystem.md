# 调研：Agent 主流框架生态（2026-08）

日期：2026-08-18

## 摘要

2026 年 8 月主流 Agent 生态按三类划分：代码框架、平台、协议。代码框架主线为 LangGraph、CrewAI、OpenAI Agents SDK、Google ADK、Claude Agent SDK、Microsoft Agent Framework；AutoGen 已进入维护模式，微软后继为 Microsoft Agent Framework（1.0 GA，2026-04）。协议层为 MCP（Agent 连接工具/数据）与 A2A（Agent 间通信，v1.0 于 2026-03 发布，治理移交 Linux Foundation Agentic AI Foundation，2026-06 出现 MCP + A2A 融合草案）。

## 结论

- 学习主线：先学 Agent 核心机制，再深挖一个主力框架，最后补协议与平台
- 每个框架固定四步：特点 → 优缺点 → 局限性 → 最小 demo
- 选型三问：是否写代码、生产还是原型、是否绑定生态
- 实时数据以 GitHub API 与官方公告为准，建议每 1–3 个月核验一次

## 来源

- GitHub API 仓库信息（抓取日期 2026-08-18）
- LangChain 官方框架对比：https://www.langchain.com/resources/ai-agent-frameworks （2026-06-05）
- 腾讯云社区平台横评：https://cloud.tencent.com.cn/developer/article/2674338 （2026-05-26）
- A2A / MCP 相关公开报道（2026-03 至 2026-08）

## 风险

- 框架星标、版本与维护状态变化快，引用时必须带快照日期
- 部分公开报道的下载量与星标数据口径不一，仅作参考
