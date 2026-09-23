# 进度日志

---

## 会话开始

- **日期**: 2026-08-18 17:52 CST
- **任务名**: `20260818-175154-langchain-usage-guide`
- **任务目录**: `.codex-tasks/20260818-175154-langchain-usage-guide/`
- **说明**: 见 `SPEC.md`
- **计划**: 见 `TODO.csv`（5 个里程碑）
- **环境**: Python 3.10.11 / venv / pip

---

## 上下文恢复块

- **当前里程碑**: #5 — 更新 README 与记忆
- **当前状态**: DONE
- **最近完成**: #1 — 调研 LangChain 当前版本与 API
- **当前工件**: `.codex-tasks/20260818-175154-langchain-usage-guide/TODO.csv`
- **关键上下文**: langchain 1.3.15；核心为 Runnable / LCEL（`|`）/ ChatPromptTemplate / ChatModel / OutputParser / create_agent；官方 agent 构建在 LangGraph 之上；RunnableWithMessageHistory 已标记 deprecated
- **已知问题**: 无 Key；示例需用 fake 模型或标注 requires-api-key
- **下一动作**: 任务完成，等待用户开始下一个框架

---

## 里程碑 2–5：安装、内容、验证与文档

- **状态**: DONE
- **完成时间**: 18:10 CST
- **完成内容**:
  - venv 安装 langchain 1.3.15 / langchain-core 1.5.6，注册 langchain-demo 内核
  - 编写客服问答场景的带看懂式 Notebook（总览图 + 核心概念 + 概念关系 + 两种写法 + 六大核心能力）
  - 无 Key 示例全部用 FakeMessagesListChatModel 验证通过（32 个 cell）
  - 更新顶层 README（LangChain 提升为必学）、frameworks/langchain/README.md、memory research/artifacts
- **验证**: `.venv/bin/python scripts/verify_langchain_notebook.py` → 32 cells executed, skipped requires-api-key cells
- **变更文件**:
  - `frameworks/langchain/demo/LangChain使用方法.ipynb`
  - `frameworks/langchain/README.md`
  - `frameworks/langchain/scripts/`（cells_intro / cells_examples / cells_capabilities / build / verify）
  - `agent主流框架/README.md`
  - `memory/research/2026-08-18-langchain-usage.md`、`memory/artifacts/2026-08-18-langchain-usage.md`
- **下一步**: 等待用户确认后继续 OpenAI Agents SDK
