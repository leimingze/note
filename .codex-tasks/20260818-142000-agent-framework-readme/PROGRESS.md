# 进度日志

---

## 会话开始

- **日期**: 2026-08-18 14:20 CST
- **任务名**: `20260818-142000-agent-framework-readme`
- **任务目录**: `.codex-tasks/20260818-142000-agent-framework-readme/`
- **说明**: 见 `SPEC.md`
- **计划**: 见 `TODO.csv`（4 个里程碑）
- **环境**: markdown / 公开网络资料 / GitHub API

---

## 上下文恢复块

- **当前里程碑**: #2 — 撰写 README 初稿
- **当前状态**: IN_PROGRESS
- **最近完成**: #1 — 调研 2026 年主流框架生态与实时数据
- **当前工件**: `.codex-tasks/20260818-142000-agent-framework-readme/TODO.csv`
- **关键上下文**: GitHub API 与 2026 年 5-8 月公开评测已收集；核心结论为 LangGraph/CrewAI/OpenAI Agents SDK/Google ADK/Claude Agent SDK/Microsoft Agent Framework 为代码框架主线，Dify/Coze 为平台主线，MCP/A2A 为协议主线
- **已知问题**: 暂无
- **下一动作**: 写 README 初稿后向用户发起对齐

---

## 里程碑 1：调研 2026 年主流框架生态与实时数据

- **状态**: DONE
- **完成时间**: 14:20 CST
- **完成内容**:
  - 通过 GitHub API 抓取 14 个框架/平台仓库的星标与最近提交（2026-08-18）
  - 阅读 LangChain 官方框架对比（2026-06-05）与腾讯云社区平台横评（2026-05-26）
  - 确认 AutoGen 维护模式、Microsoft Agent Framework 1.0 GA、A2A v1.0 与 AAIF 治理、MCP+A2A 融合草案等生态动态
- **验证**: GitHub API 请求成功返回；open_page 获取两篇综述正文
- **变更文件**:
  - 本文件 — 记录调研里程碑
- **下一步**: 里程碑 2 — 撰写 README 初稿

---

## 对齐反馈 1：README 主线调整

- **时间**: 2026-08-18 14:35 CST
- **用户意见**: README 主要写学什么、怎么学、学哪些；必学内容为框架清单、特点、优缺点、局限性、选型、每个框架的最小 demo
- **处理**: 整体重写 README，改为「学什么 / 怎么学 / 学哪些」三段主线；补充四步学习法、三问决策法、选型 checklist 与最小 demo 计划表
- **变更文件**:
  - `agent主流框架/README.md` — 按新主线完整重写
- **下一步**: 再次向用户发起对齐
