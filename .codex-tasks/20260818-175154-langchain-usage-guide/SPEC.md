# 任务说明

## 任务形态

- **形态**: single-full

## 目标

编写第二个学习框架 LangChain 的带看懂式 Jupyter Notebook：

- `agent主流框架/frameworks/langchain/demo/LangChain使用方法.ipynb`
- `agent主流框架/frameworks/langchain/README.md`：速查文档
- 生成与验证脚本放在 `frameworks/langchain/scripts/`

## 非目标

- 不写 LangSmith 深度使用教程
- 不写 RAG 完整教程，只覆盖核心用法与核心能力

## 约束

- 基于 langchain 1.3.15（2026-08-18）与官方文档验证
- 带看懂式：不要求读者手写，代码配按执行顺序拆解
- 流程图提前用 markdown Mermaid 画好，不用代码运行时生成
- 无 Key 的 cell 全部真实运行验证；需要 Key 的 cell 标注 requires-api-key
- 内核固定为 langchain-demo，指向 frameworks/langchain/.venv

## 环境

- **项目根目录**: `/Users/leimingze/notes`
- **语言/运行时**: Python 3.10.11，venv 位于 `frameworks/langchain/.venv`

## 交付物

- `frameworks/langchain/demo/LangChain使用方法.ipynb`
- `frameworks/langchain/README.md`
- `frameworks/langchain/scripts/`

## 完成条件

- [ ] Notebook 覆盖总览图、核心概念、概念关系、用法拆解、核心能力逐个看
- [ ] 无 Key cell 全部执行通过
- [ ] README 与记忆已更新

## 最终验证命令

```bash
.venv/bin/python scripts/build_langchain_notebook.py && .venv/bin/python scripts/verify_langchain_notebook.py
```
