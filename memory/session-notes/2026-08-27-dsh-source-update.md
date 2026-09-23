# 会话记录：dsh 与 dsh-TUI 源码更新

日期：2026-08-27

## 背景

用户要求更新 `/Users/leimingze/agent-project/deepseek-harness` 与 `/Users/leimingze/agent-project/dsh-TUI` 源码。

## dsh 结果

- 本地 `master` 从 `141eb6fef834` 快进到 `b150a551b8`，版本从 `0.1.0-rc.8` 更新为 `0.1.1-rc.2`。
- 执行 `pnpm install --frozen-lockfile` 成功，1215 个锁文件条目通过供应链策略校验。
- 执行 `pnpm build` 成功，Host、Client 和 Web 前端均完成构建。
- `~/.local/bin/dsh` 仍指向 `apps/cli/lib/bin.js`，`dsh --version` 输出 `0.1.1-rc.2`。
- 工作区干净，本地分支与 `origin/master` 一致。

## dsh-TUI 结果

- 本地 `main` 从 `8c8e955ab1d0` 快进到 `4c1d09f12596`，版本从 `0.8.6` 更新为 `0.9.3`。
- 递归子模块已同步，包含新增的 `dsh-auth`（`55e828e7`）以及 `dsh-ecosystem-spec`（`04d67981`）。
- 执行 `pnpm install --frozen-lockfile` 成功，376 个锁文件条目通过供应链策略校验。
- 执行 `pnpm build` 成功，全部 `verify:build` 门禁通过。
- `~/.local/bin/dsh-tui` 仍指向 `bin/dsh-tui.js`，`dsh-tui --version` 输出 `0.9.3`。
- 工作区干净，本地分支与 `origin/main` 一致。

## 注意事项

- 构建输出包含上游已知的 cyclic workspace dependencies 警告，以及 Linux landlock 包在当前 macOS 平台上不支持平台警告，均不影响本次构建结果。
- 本次未修改用户配置、凭据或权限设置。
