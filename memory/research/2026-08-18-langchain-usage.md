# 调研：LangChain 使用方法与当前 API

日期：2026-08-18

## 摘要

LangChain 当前稳定版 1.3.15（langchain-core 1.5.6）。核心是 Runnable 统一接口 + LCEL 管道（`|`），组件包括 ChatPromptTemplate、ChatModel、OutputParser。官方 2026 年主推 `create_agent`（Agent = Model + Harness），其 agent 构建在 LangGraph 之上。`RunnableWithMessageHistory` 已标记 deprecated，生产推荐 LangGraph 持久化。无 API Key 时可用 `langchain_core.language_models.fake_chat_models.FakeMessagesListChatModel` 模拟模型（真实机制一致）。

## 结论

- 无 Key 示例全部可用 fake 模型验证：invoke / stream / batch / PydanticOutputParser / RunnableGenerator 均实测通过
- fake 模型不支持 `with_structured_output`，结构化解析改用 `PydanticOutputParser`
- `langchain-community` 已 sunset，不要依赖

## 来源

- 官方 overview：https://docs.langchain.com/oss/python/langchain/overview
- PyPI：https://pypi.org/pypi/langchain/json
- 本地 venv 实测：langchain 1.3.15 / langchain-core 1.5.6

## 风险

- 记忆组件已弃用，教学时需标注，避免用户用到生产
