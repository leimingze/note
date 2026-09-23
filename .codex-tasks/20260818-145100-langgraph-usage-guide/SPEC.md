# 任务说明

## 任务形态

- **形态**: single-full

## 目标

编写第一个必学框架 LangGraph 的使用方法文档与最小 demo：

- `agent主流框架/frameworks/langgraph/README.md`：使用方法讲解（安装、核心概念、三种用法、常见问题）
- `agent主流框架/frameworks/langgraph/demo/hello_graph.py`：不依赖 LLM 的可运行最小 demo
- `agent主流框架/frameworks/langgraph/demo/react_agent.py`：带工具调用的 ReAct Agent 示例（需要 API Key）

## 非目标

- 不写 LangGraph 平台部署、LangSmith 深度使用教程
- 不写记忆系统、human-in-the-loop 的完整教程，只在文档中给方向

## 约束

- 内容基于 2026-08-18 官方文档与 langgraph 1.2.11 实测验证
- 文档使用中文，代码与命令保留英文
- 未实际运行的示例必须明确标注所需条件
- demo 必须真实运行验证后再声称可用
- 文档尽量少于 300 行

## 环境

- **项目根目录**: `/Users/leimingze/notes`
- **语言/运行时**: Python 3.10.11（本机），venv 位于 `frameworks/langgraph/.venv`
- **包管理**: pip

## 风险评估

- [x] 外部依赖：PyPI 安装 langgraph 1.2.11，需要网络，已确认可访问
- [x] 破坏性变更：无，`frameworks/` 为新建目录
- [x] 大文件：无
- [x] 长时间运行测试：无

## 交付物

- `frameworks/langgraph/README.md`
- `frameworks/langgraph/demo/hello_graph.py`
- `frameworks/langgraph/demo/react_agent.py`

## 完成条件

- [ ] README 覆盖安装、核心概念、StateGraph 手写方式、create_react_agent 方式、常见问题
- [ ] hello_graph.py 真实运行通过
- [ ] react_agent.py 基于官方 API 编写并标注运行条件

## 最终验证命令

```bash
.venv/bin/python demo/hello_graph.py
```
