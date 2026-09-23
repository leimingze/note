# OfferLens Interview Data

让 AI Agent 基于真实面经准备面试，而不是只根据简历猜题。

`offerlens-interview-data` 会按公司、岗位方向、关键词和时间范围查询完整面经原文，适合用来总结高频八股、项目追问、算法题，以及开展有真实数据参考的模拟面试。简历始终由本地 Agent 处理，不会上传到 OfferLens 服务。

## 两个 Skill 配合使用

| Skill | 提供的信息 | 仓库 |
| --- | --- | --- |
| `offerlens-jobs` | 13 家公司的官方校园招聘岗位、可获得时的完整 JD 和官方投递链接 | [mnlz/offerlens-jobs](https://github.com/mnlz/offerlens-jobs) |
| `offerlens-interview-data` | 按公司、岗位和时间筛选的真实面经完整原文 | [mnlz/offerlens-interview-data](https://github.com/mnlz/offerlens-interview-data) |

Agent 可以在本地把真实 JD、用户简历和真实面经组合起来，完成岗位匹配、题目总结和模拟面试。没有安装 `offerlens-jobs` 时，也可以直接粘贴 JD。

## 能做什么

- 总结“字节最近 AI Agent 岗位高频八股”。
- 统计“美团后端最近常见的算法题”。
- 结合本地简历和腾讯真实 JD，预测项目深挖方向。
- 参考真实面经开展完整或单模块模拟面试。
- 按 90、180、365 天自适应扩大样本窗口，并说明样本量。

## 快速安装

安装到当前机器检测到的全部 Agent：

```bash
npx skills add mnlz/offerlens-interview-data --all
```

只安装到 Codex：

```bash
npx skills add mnlz/offerlens-interview-data --agent codex --yes
```

Codex、Claude Code、Cursor、OpenCode、OpenClaw、Gemini CLI 和 GitHub Copilot 的指定安装命令见 [install.md](install.md)。

## 免费额度与完整访问

- 未配置 API Key 时自动使用免费访问：每个 IP 最多 10 次成功请求，每次最多返回 10 篇面经。
- 空结果和失败请求不扣次数。
- 完整访问通过环境变量 `OFFERLENS_API_KEY` 配置，单次最多返回 100 篇并支持分页。
- API 不接收简历，也不返回内部来源 ID 或原始来源链接。

## 命令行示例

```bash
python3 scripts/query_interviews.py \
  --company 字节跳动 \
  --keyword "AI Agent" \
  --adaptive-recent \
  --format markdown
```

完整参数见 [references/api.md](references/api.md)，推荐工作方式见 [references/workflows.md](references/workflows.md)。

## License

[MIT](LICENSE)
