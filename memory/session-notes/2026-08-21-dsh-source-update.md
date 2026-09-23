# 会话记录：dsh 与 dsh-TUI 源码更新

日期：2026-08-21

## 用户目标

更新 `/Users/leimingze/agent-project` 下以源码方式安装的 dsh 与 dsh-TUI。

## 结果

- `deepseek-harness` 从 `99f6f02fec` 快进到 `141eb6fef834`，版本从 `0.1.0-rc.7` 更新为 `0.1.0-rc.8`。
- `dsh-TUI` 从 `b5245b2` 快进到 `8c8e955ab1d0`，包版本从 `0.8.1` 更新为 `0.8.6`；该提交比 `v0.8.6` tag 多 37 个主分支提交。
- `dsh-TUI` 的递归子模块已同步到父仓库记录的提交。
- 两个仓库都执行了 `pnpm install --frozen-lockfile` 和 `pnpm build`，全部成功。
- TUI 构建包含的上游契约、插件生命周期、预设等全部 `verify:build` 门禁通过。

## 安装链接

- `~/.local/bin/dsh` → `/Users/leimingze/agent-project/deepseek-harness/apps/cli/lib/bin.js`
- `~/.local/bin/dsh-tui` → `/Users/leimingze/agent-project/dsh-TUI/bin/dsh-tui.js`
- `~/.dsh/profiles/dsh-tui/node_modules/@deepseek-harness-tui/dsh-tui` → `/Users/leimingze/agent-project/dsh-TUI`

## 配置核对

- `~/.dsh/profiles/dsh-tui/cordis.patch.yml` 仍为空数组，默认权限没有改成最高权限。
- 更新时未修改 provider、凭据或用户运行配置。
- 2026-08-20 启动的一个既有 TUI 进程仍在运行，未被本次更新终止；需要重启该进程后才会加载新源码。

## 验证证据

- `dsh --version` 输出 `0.1.0-rc.8`。
- PTY smoke test 成功进入 `dsh-TUI v0.8.6` 主界面并正常退出。
- 两个仓库最终均与远端跟踪分支一致，工作区干净。
