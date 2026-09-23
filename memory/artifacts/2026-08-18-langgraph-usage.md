# 产物：LangGraph 使用方法文档与最小 demo

日期：2026-08-18
类型：学习笔记 / 示例代码

## 路径

- `agent主流框架/frameworks/langgraph/README.md`：使用方法速查文档
- `agent主流框架/frameworks/langgraph/demo/LangGraph使用方法.ipynb`：带看懂式 Jupyter 学习版（电商订单场景贯穿：订单流水线 + 两种写死方式 + 六大核心能力），36 个 cell 已验证（跳过需 API Key 的 cell）
- `agent主流框架/frameworks/langgraph/scripts/`：notebook 生成与验证脚本
- `agent主流框架/frameworks/langgraph/.venv/`：本地运行环境（langgraph 1.2.11）

原 `demo/hello_graph.py`、`demo/direct_call.py`、`demo/react_agent.py` 三个脚本已并入 Notebook 对应 cell，demo 目录只保留一个 Notebook。

## 用途

作为必学框架 LangGraph 的入口笔记，后续学完可继续补记忆、human-in-the-loop 等进阶内容。

## 状态

已编写并验证；react_agent.py 待用户配置 API Key 后实测。

## 维护说明

框架版本升级后需要重新验证 demo；README 中标注的版本号与验证日期同步更新。
