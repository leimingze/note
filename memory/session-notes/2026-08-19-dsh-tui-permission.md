# dsh 配置备忘（2026-08-19）

## 会话结论

- dsh 主目录：`~/.dsh`（凭据 `~/.dsh/.credentials.yaml`，provider 配置 `~/.dsh/settings.yaml`）。
- API key：存在 `~/.dsh/.credentials.yaml`（`DEEPSEEK_API_KEY` / `QQCODE_API_KEY`），settings.yaml 里 `apiKeyEnv` 指向环境变量名；当前 provider 为 `qqcode`（baseURL `https://subapi.nexrelay.xyz/v1`）。

## dsh-tui 权限机制（重要）

- **TUI 里切换权限的指令是 `/permission <preset>`**（由 `dsh-permission-presets` 插件注册，走 commands 服务）：
  - `/permission`（不带参数）→ 显示当前 preset 与可用列表
  - `/permission danger-full-access` → 最高权限（sandbox = danger-full-access + approval = never）
  - 可用 preset（dsh-base 层 permission 行）：`read-only` / `workspace-write` / `danger-full-access`
- `/permissions`（TUI 本地命令）只读显示权限策略状态，不切换。
- 另有 **Shift+Tab** 循环会话模式（默认三档 default→plan→full，full = 最高权限 + 不弹审批），以及启动环境变量 `DSH_PERMISSION_MODE=danger-full-access`（sandbox 与 approval 均覆盖）。
- `/permission danger-full-access` 在 Web 端同样有效。
- **注意**：dsh CLI 的 dump/启动会向 profile 目录写合成 cordis.yml；在沙箱会话里跑需要完整文件系统权限。

## 历史（已回滚，勿再执行）

- 曾临时把 `~/.dsh/profiles/dsh-tui/cordis.patch.yml` 覆盖为 sandbox danger-full-access + approval never，用户明确表示不要默认最高权限，**已回滚为空数组**。默认权限保持 workspace-write。
