# 安装 OfferLens Interview Data

需要 Node.js 和 `npx`。安装命令会从 GitHub 下载 Skill，并写入对应 Agent 的标准 Skill 目录。

## 安装到全部 Agent

自动安装到当前机器检测到的全部受支持 Agent：

```bash
npx skills add mnlz/offerlens-interview-data --all
```

## 只安装到指定 Agent

只安装到 Codex：

```bash
npx skills add mnlz/offerlens-interview-data --agent codex --yes
```

其他常用 Agent：

| Agent | 安装命令 |
| --- | --- |
| Claude Code | `npx skills add mnlz/offerlens-interview-data --agent claude-code --yes` |
| Cursor | `npx skills add mnlz/offerlens-interview-data --agent cursor --yes` |
| OpenCode | `npx skills add mnlz/offerlens-interview-data --agent opencode --yes` |
| OpenClaw | `npx skills add mnlz/offerlens-interview-data --agent openclaw --yes` |
| Gemini CLI | `npx skills add mnlz/offerlens-interview-data --agent gemini-cli --yes` |
| GitHub Copilot | `npx skills add mnlz/offerlens-interview-data --agent github-copilot --yes` |

也可以一次指定多个 Agent：

```bash
npx skills add mnlz/offerlens-interview-data \
  --agent codex claude-code cursor \
  --yes
```

安装完成后，如果 Agent 没有自动重新加载 Skill，请重启对应 Agent。

## 配合官方岗位 Skill

`offerlens-jobs` 用于查询 13 家公司的当前校园招聘岗位和官方 JD：

```bash
npx skills add mnlz/offerlens-jobs --agent codex --yes
```

该 Skill 还需要在安装目录创建 Python 虚拟环境并安装依赖，具体以 [offerlens-jobs README](https://github.com/mnlz/offerlens-jobs#安装) 为准。没有安装它时，可以直接向 Agent 提供岗位 JD。

## 免费使用

无需配置。客户端会自动使用免费 Key `offerlens`：

- 同一 IP 最多 10 次成功且非空的请求。
- 每次最多返回 10 篇完整面经。
- 空结果和失败请求不消耗额度。

## 配置完整访问 Key

购买后，将收到的 Key 放入环境变量。不要把 Key 写进提示词、代码仓库或命令参数。

macOS / Linux（Zsh 或 Bash）：

```bash
export OFFERLENS_API_KEY="你的 API Key"
```

Fish：

```fish
set -Ux OFFERLENS_API_KEY "你的 API Key"
```

PowerShell：

```powershell
$env:OFFERLENS_API_KEY = "你的 API Key"
```

完整访问不受 10 次免费额度限制，支持分页，单次最多返回 100 篇。

## 验证安装

进入安装后的 Skill 目录执行：

```bash
python3 scripts/query_interviews.py \
  --company 字节跳动 \
  --days 90 \
  --limit 1
```

成功响应会包含 `data`、`total`、`access` 和 `trial_remaining`。请求中不会包含简历内容。
