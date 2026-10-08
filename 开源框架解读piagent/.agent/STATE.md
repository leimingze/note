# 当前任务状态

- 任务编号：AGENT-COMPARISON-004
- 当前方案：每个对比维度只占一行，共 14 行；同一单元格内分行说明相关能力，保留明确的有无标记和启用条件。
- 有效文件：`总述.md`（全部 14 个包的用途和目录说明）；`piagent和claudecode的主要区别.md`（14 个维度及资料来源）。
- 证据位置：Pi 本地 `packages/coding-agent/README.md`、`docs/how-pi-works.md`、`docs/configuration.md`、`docs/settings.md`、`docs/mcp.md`、`docs/security.md`、`docs/session-format.md`、`src/core/system-prompt.ts`、`src/core/agent-session.ts`、`examples/extensions/todo.ts`、`packages/agent/src/agent-loop.ts`；Claude Code 官方工作机制、检查点、Agent SDK、TODO 工具、系统提示词及 CLI 参数文档，查阅日期为 2026-10-07。
- 已确认：Pi 本地版本为 1.0.4，已内置 MCP；默认会话导航不还原代码。Claude Code 的代码还原依赖检查点，不覆盖 Bash 修改及多数子代理修改；TODO 工具默认可用性取决于模型与版本。两者均可定制系统提示词，均使用模型与工具循环。
- 验证情况：2026-10-08 已核验比较文档共 28 行，表格覆盖 14 个维度且无重复，全部表格行均为三列；保留模型、版本、启用条件和代码还原范围。`总述.md` 沿用已核验版本，共 289 行、14 个包和 103 个目录条目。
- 未解决问题：无。
- 下一步：无，任务已完成。
- 归档位置：未创建过程文件。
