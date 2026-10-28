# 当前任务

- 任务编号：20261028-network-notes
- 当前方案：依据 `笔记整理/prompt.md` 的 Mappy 格式，为 `计算机网络` 中除传输层之外的五份源文各生成一份完整笔记，保存到 `计算机网络/obsidian`。源文件和现有传输层笔记保持原样。
- 当前进度：五篇均已生成、完成内容复核并选定为有效笔记。
- 有效文件：`计算机网络/obsidian/网络模型.md`、`计算机网络/obsidian/网络层.md`、`计算机网络/obsidian/应用层.md`、`计算机网络/obsidian/公钥与私钥.md`、`计算机网络/obsidian/数据链路层.md`。
- 空文件处理：源 `计算机网络/数据链路层.md` 为空，依据网络模型、网络层中的关联内容补齐基础知识；该篇所有原理字段均标记 `[补]`。
- 输入版本：提示词 SHA-256 为 `a77b40ece7ca178c9e17db3ca6d6b80ce6c5fd44e076860cb5cc23647cba1eea`；五份源文件 SHA-256 如下。
- 网络模型源文：`8f2bcd0c56648372600175eb36d1175a804cdd277de90a9054d6a48d6addfeb0`。
- 网络层源文：`6ef70443cd63840f54a4ef714fd4a6ff4260d5191ab13700ae057c4531d75d35`。
- 应用层源文：`afd6377a9f92ca9d4e932d48f145103284ce33511d518afe05991f007113d46f`。
- 数据链路层源文：`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`（空文件）。
- 公钥与私钥源文：`d884f1e7cda0d6231e57504ef788450222944bb9cdf7648801e8facb217ead4d`。
- 验证情况：五篇共 193 个知识节点、11 张图片，通过 MarkdownIt 语法树和 YAML 检查，每节点四字段齐全，正文与图片归属正确；独立内容复核完成，应用层 7 个原有 TODO 均已补齐。原图直接使用 `../assets/` 路径，未新建图片目录。
- 数量核对：网络模型 35 节点、3 图；网络层 57 节点、6 图；应用层 60 节点、2 图；数据链路层 24 节点；公钥与私钥 17 节点。
- 原文保护：源文件均与输入校验值一致；已有传输层笔记 SHA-256 保持 `af5dba821f3771dfdaa4c9691c2a54f5cb8463c8a3637819ecc2cc90e7395f58`。原有应用层、网络模型源文的未提交修改未纳入本次提交。
- 校验入口：`PYTHONPATH=.agent/work/20261028-network-notes/dependencies python3 .agent/work/20261028-network-notes/verify_notes.py`。
- 未解决问题：无本次生成范围内的待核项。
- 下一步：无。
- 归档位置：`.agent/work/20261028-network-notes/` 已归档并保留原位置，已由 Git 忽略；包含过程校验脚本及其依赖。

# 已完成任务：20261028-mappy-note-prompt

- 任务编号：20261028-mappy-note-prompt
- 当前方案：后续笔记按 Mappy 嵌套列表模式生成：YAML 属性 `mappy: true`、唯一 H2 主题标题、问题节点及四字段引用块正文，正文与图片均位于子节点之前。
- 有效文件：`笔记整理/prompt.md`，SHA-256 为 `a77b40ece7ca178c9e17db3ca6d6b80ce6c5fd44e076860cb5cc23647cba1eea`。
- 显示规则：导图展示问题标题、正文链接和图片，完整答案通过节点右键 `Edit body and links` 查看；图片根据输出位置直接引用原有文件。
- 验证情况：按已验证的 Mappy 0.4.11 解析规则复核格式约束与示例，确认正文归属、无序列表、缩进和有效链接要求一致；单份完整笔记与原图引用要求保留；`git diff --check` 通过。提示词共 198 行。
- 未解决问题：无。
- 下一步：后续整理笔记使用当前提示词。
- 归档位置：未新增过程文件。

# 已完成任务：20261028-mindmap-plugin

- 任务编号：20261028-mindmap-plugin
- 当前方案：在 Obsidian 1.14.5 中启用 Mappy 0.4.11，停用 Rich Mindmap 0.3.1；当前已打开传输层的 `mappy-map` 视图。
- 有效文件：`.obsidian/community-plugins.json`、`.obsidian/plugins/mappy/manifest.json`、`.obsidian/plugins/mappy/main.js`、`.obsidian/plugins/mappy/styles.css`、`计算机网络/obsidian/传输层.md`。插件文件及 Obsidian 配置沿用本机管理，未加入 Git。
- 笔记格式：添加 `mappy: true` frontmatter，主题标题使用 H2，知识树继续使用嵌套列表；其余正文及 `../assets/` 图片引用保持原样。
- 验证情况：官方发布安装包通过 `node --check`；Obsidian CLI 确认 Mappy 已启用、Rich Mindmap 未加载。当前视图解析为 `list`，有 104 个知识节点及 1 个主题节点，全部展开；15 个 `.mappy-node-attachments img` 的 `complete` 为 true，原图尺寸及显示尺寸均大于零，来源均为 `计算机网络/assets`。`dev:errors` 未捕获错误，`git diff --check` 通过。
- 使用方式：节点显示标题和图片，节点右键的 `Edit body and links` 可查看完整正文。打开视图没有改写正文。
- 环境恢复：验证时临时启用的 Obsidian CLI 已通过内置设置接口关闭，全局配置恢复原状。
- 未解决问题：无插件安装或图片加载问题。
- 下一步：无。
- 归档位置：`.agent/work/20261028-mindmap-plugin/` 已归档并保留原位置，目录为空；有效笔记及插件路径如上。

# 已完成任务：20261028-transport-single-note

- 任务编号：20261028-transport-single-note
- 当前方案：依据当前 `笔记整理/prompt.md`，将传输层内容组织为单份完整的 Markdown 追问笔记，使用在线课堂场景、嵌套列表及四字段引用块。
- 有效文件：`计算机网络/obsidian/传输层.md`。图片通过 `../assets/` 直接引用 `计算机网络/assets` 中的原有文件。
- 提示词规则：`笔记整理/prompt.md` 已规定根据输出笔记位置计算相对路径，直接引用原图，不复制或移动图片，不新建图片目录或符号链接；示例使用 `../assets/image16.png`。
- 当时输入版本：`计算机网络/传输层.md` 的 SHA-256 为 `4eec069c6a539fcdcd6fccc79bc3b3600b2440249377a5faa10849f14de36790`；`笔记整理/prompt.md` 的 SHA-256 为 `da175046816f35c56be0c57e1a4ba9c13c234f41b074e7375f6ede151ef2e5d3`。
- 验证情况：独立复核确认原文 83 项知识点、20 个 TODO 均已覆盖；MarkdownIt 解析核对 104 个知识节点，每节点具备场景、原理、记忆句、追问，无误缩进代码块或 HTML 残留；15 张图片均直接指向原有非空文件，`obsidian/assets` 链接已移除。SYN 携带数据的序号定义和 2MSL 寿命说明已校正。
- 未解决问题：目标 Linux 内核与配置未指定，队列压力、异常 SYN、TIME-WAIT 接纳新 SYN 和 tcp_tw_reuse 的环境条件在对应正文节点标记 `[待核]`。
- 下一步：无。
- 归档位置：本次未创建过程目录；正式交付物为上述单份笔记。

# 已完成任务：20261026-mysql-transactions

- 任务编号：20261026-mysql-transactions
- 当前方案：已按用户要求，使用文件编辑工具将 `mysql/mysql事务.md` 恢复为本次整理开始前的原文内容。
- 有效文件：恢复后的 `mysql/mysql事务.md`；原有内嵌图片 `mysql/assets/image6.png`、`mysql/assets/image7.png`、`mysql/assets/image8.png`、`mysql/assets/image9.png`。
- 恢复依据：会话首次读取的完整原文，以及 `.agent/work/20261026-mysql-transactions/knowledge-outline.md` 中保存的原文引文。
- 验证情况：独立复核确认正文、标题、原有拼写与四个内嵌图片引用一致；`git diff --check` 通过。当前 82 行、5873 字节，仅比原文件多一个末尾换行。
- 未解决问题：无正文恢复缺项。
- 下一步：无。
- 归档位置：`.agent/work/20261026-mysql-transactions/` 保留原位置；本次整理产物已停用，当前有效内容以恢复后的原文为准。

# 已完成任务：20261021-notes-git-sync

- 任务编号：20261021-notes-git-sync
- 当前方案：将 HTTP、HTTPS、公私钥笔记及 TCP 学习清单提交到 `main`，推送至 `origin/main`。
- 远程配置：`origin` 为 `git@github.com:leimingze/note.git`。
- 有效文件：`计算机网络/传输层.md`、`计算机网络/应用层.md`、`计算机网络/公钥与私钥.md`、`计算机网络/assets/image27.png`、`计算机网络/assets/image28.png`。
- 笔记提交：`e23090a`，提交信息为 `[update] 补充 HTTP 与 HTTPS 笔记、公私钥说明及 TCP 学习清单`，包含 5 个文件变更，新增 214 行、删除 1 行，含两张新增图片。
- 验证情况：`git diff --cached --check` 通过，两张新增引用图片存在且非空；`git push origin main` 成功，远程报告 `ba71d85..e23090a main -> main`。本次未修改程序代码，未运行程序测试。
- 未解决问题：无。
- 下一步：无。
- 归档位置：笔记同步任务已完成，未创建过程文件；本记录随仓库同步。
