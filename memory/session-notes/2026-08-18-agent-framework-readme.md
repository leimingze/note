# 会话记录：Agent 主流框架学习目录搭建

日期：2026-08-18

## 背景

用户新建 `agent主流框架/` 目录，要求先生成学习 README 并对齐，再逐步学习。

## 用户目标

- 学习主流 Agent 框架
- README 以「学什么、怎么学、学哪些」为主线
- 必学内容：框架清单、特点、优缺点、局限性、选型、每个框架的最小 demo
- README 需要实时性：快照日期、数据来源、核验方式

## 结果

- `agent主流框架/README.md` 定稿（189 行，含实时快照表与最小 demo 计划）
- `agent主流框架/README生成提示词.md` 已保存，供后续其他主题复用

## 后续动作

- 按 README 学习路径逐步建立 `frameworks/`、`platforms/`、`protocols/` 笔记与 demo
- 学到新内容时临时修改 README，不必提前固化

## 需要同步到长期记忆的内容

- 用户偏好：学习类目录 README 使用「学什么 / 怎么学 / 学哪些」结构
- 用户偏好：学习对象必须包含特点、优缺点、局限性、选型、最小 demo
- 用户偏好：README 要求实时数据快照与核验机制

## 后续修订（2026-08-18）

- 必学清单精简：LangGraph、OpenAI Agents SDK、CrewAI、Dify、Coze
- 按需：Google ADK、Microsoft Agent Framework、Pydantic AI、LlamaIndex
- Deep Agents 已了解（不深入，本质是 LangGraph 预装工具包）；AgentScope 仍暂不单独学；LangChain 已完成
- 当前学习：OpenAI Agents SDK；已建立 README 与无 Key Notebook（Agent / Runner / 工具 / handoff / as_tool / context）
