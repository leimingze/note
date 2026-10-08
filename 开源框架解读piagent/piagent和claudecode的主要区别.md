# Pi Agent 和 Claude Code 的主要区别

比较对象是 Pi 的 `packages/coding-agent` 编程应用与 Claude Code。Pi 依据本地 `1.0.4` 版本，Claude Code 依据 2026-10-07 查阅的官方文档；默认能力可能随版本、模型和配置变化。

**内置有**表示产品已提供该能力；**默认没有，需扩展**表示默认应用未提供，但可以增加。

| 对比维度 | Pi Agent | Claude Code |
| --- | --- | --- |
| 设计哲学 | **精简核心，用户组合扩展**；支持不同模型厂商。 | **集成工作流，产品提供功能**；围绕 Claude 模型优化。 |
| 默认工具 | **默认有**：`read`、`edit`、`write`、`bash`。<br>**内置但默认未启用**：`find`、`grep`、`ls`；默认可通过 `bash` 搜索。 | **默认有**：`Read`、`Edit`、`Write`、`Bash`、`Glob`、`Grep`。 |
| 系统提示词 | **替换、追加：有**，通过 `SYSTEM.md`、`APPEND_SYSTEM.md`。<br>**构建源码：开放**，可修改，也支持扩展动态调整。 | **替换、追加：有**，通过 `--system-prompt`、`--append-system-prompt`。<br>**核心构建源码：未开放**，可配置提示词内容。 |
| 权限系统 | **工具审批规则：默认没有，需扩展**，工具使用进程的系统权限。<br>**内置命令沙箱：没有**，需外部隔离。<br>**项目信任：有**，只控制资源加载。 | **工具审批规则：内置有**，支持审批模式和组织策略。<br>**命令沙箱：内置有，需启用**，不覆盖所有文件工具、MCP 和钩子。 |
| Plan（计划模式） | **默认没有，需扩展**。 | **内置有**：分析并提出计划，限制源代码修改。 |
| TODO（任务清单） | **默认没有，需扩展**；提供 `todo.ts` 示例。 | **内置有，默认启用取决于模型与版本**：`TaskCreate`、`TaskUpdate` 等，可显式启用。 |
| 子代理 | **默认没有，需扩展**；仓库其他包另有相关能力。 | **内置有**：`Agent` 工具和子代理配置，支持独立上下文与并行执行。 |
| MCP | **内置有**；支持工具搜索和 `codemode`。 | **内置有**；支持按需搜索、加载工具。 |
| 联网 | **专门的网页搜索、抓取工具：默认没有**，需扩展或 MCP；也可通过 `bash` 调用网络命令。 | **专门的网页搜索、抓取工具：内置有**，即 `WebSearch`、`WebFetch`；受接入平台、权限与网络条件影响。 |
| 增加功能的方式 | **扩展入口：有**，包括 TypeScript 扩展、技能、模板、MCP、SDK。<br>**核心源码：可修改**。 | **扩展入口：有**，包括技能、钩子、插件、MCP、自定义子代理、SDK。<br>**核心引擎：未开放**。 |
| 会话存储格式 | **JSONL**：`~/.pi/agent/sessions/`。<br>**分支：同一文件内保存树状结构**，使用 `id`、`parentId` 关联。 | **JSONL**：本地 CLI 位于 `~/.claude/projects/`。<br>**分支：复制历史到新会话**，使用新的会话 ID。 |
| 回退是否还原代码 | **对话回退：有**，通过 `/tree`、`/fork`。<br>**代码还原：默认没有**，需使用 Git 或配置检查点扩展。 | **对话回退：有**，通过 `/rewind`。<br>**代码还原：有，但范围有限**，依赖文件检查点，不覆盖 Bash 修改、多数子代理修改和外部系统操作等。 |
| 长期记忆 / 用户画像 | **跨会话规则：有**，支持全局及项目 `AGENTS.md`、`CLAUDE.md` 等。<br>**自动长期记忆：默认没有**，需自行配置或扩展。<br>**使用习惯：需用户定义**，写入规则文件，自动维护需另外配置。 | **跨会话规则：有**，支持 `CLAUDE.md`、`AGENTS.md`。<br>**自动长期记忆：内置有**。<br>**使用习惯：可自动记录**，通过规则文件或自动记忆保存。 |
| Agent loop（Agent 循环） | **内置有**：模型请求与工具执行循环，持续处理任务与消息。<br>**核心循环源码：开放**，可修改，也可通过事件、扩展和 SDK 调整。 | **内置有**：模型与工具循环，结合权限、上下文管理与子代理。<br>**核心循环源码：未开放**，通过 SDK 和钩子提供的入口控制。 |

资料来源：

1. Pi：[应用说明](/Users/leimingze/pi-agent/packages/coding-agent/README.md)、[工作机制](/Users/leimingze/pi-agent/packages/coding-agent/docs/how-pi-works.md)、[配置](/Users/leimingze/pi-agent/packages/coding-agent/docs/configuration.md)、[会话格式](/Users/leimingze/pi-agent/packages/coding-agent/docs/session-format.md)、[安全说明](/Users/leimingze/pi-agent/packages/coding-agent/docs/security.md)。
2. Pi 源码：[系统提示词](/Users/leimingze/pi-agent/packages/coding-agent/src/core/system-prompt.ts)、[Agent 循环](/Users/leimingze/pi-agent/packages/agent/src/agent-loop.ts)、[TODO 扩展示例](/Users/leimingze/pi-agent/packages/coding-agent/examples/extensions/todo.ts)。
3. Claude Code：[工作机制](https://code.claude.com/docs/en/how-claude-code-works)、[代码检查点与回退](https://code.claude.com/docs/en/checkpointing)、[TODO 工具及模型条件](https://code.claude.com/docs/en/agent-sdk/todo-tracking)、[Agent SDK](https://code.claude.com/docs/en/agent-sdk/overview)。
