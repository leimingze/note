# 调研：LangGraph 使用方法与当前 API

日期：2026-08-18

## 摘要

LangGraph 当前稳定版为 1.2.11（PyPI 2026-08-18）。核心 API 稳定：`StateGraph`、`MessagesState`、`START`、`END`、`compile`、`invoke`；预置 ReAct Agent 使用 `langgraph.prebuilt.create_react_agent`，工具节点使用 `ToolNode`。LangGraph 可以独立使用，模型与工具抽象来自 LangChain 组件。1.x 不再暴露 `langgraph.__version__`，需用 `importlib.metadata` 查询版本。

## 结论

- 最小可运行 demo 可完全不用 LLM（StateGraph + 条件路由）
- `create_react_agent` 适合快速搭建标准 Agent，手搓 StateGraph 适合学习底层机制
- 本机无模型 API Key，带模型的示例只做语法与导入验证，不声称运行成功

## 来源

- 官方 overview：https://docs.langchain.com/langgraph/
- PyPI：https://pypi.org/pypi/langgraph/json
- 本地 venv 实测：langgraph 1.2.11 / langgraph-prebuilt / langchain-openai

## 风险

- 网上 0.1.x 教程大量存在且 API 过时，学习时以官方文档与 1.2.11 实测为准
