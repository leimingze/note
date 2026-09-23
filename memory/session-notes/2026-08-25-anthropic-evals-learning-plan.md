# 会话记录：Anthropic Agent 评测文章加入学习计划

日期：2026-08-25

## 背景

用户要求把 Anthropic 工程博客《Demystifying evals for AI agents》加入 `agent测评/README.md` 学习计划。

## 用户目标

把文章纳入既有九阶段学习路线，不新建模块、不改 TODO 任务。

## 结果

- 文章已下载存档：`agent测评/原始资料/anthropic-agent-evals/anthropic-agent-evals.html`，含完整正文与结论章节。
- README 更新：阶段 2 表格行、学习重点和指定资料均加入该文章；阶段 3 指定资料补充“Anthropic 实践中的回归方法”；资料索引新增一行。
- 归属理由：文章核心是 task/trial/grader/outcome 结构与三类评分器、能力与回归评测划分、pass@k/pass^k、20-50 条任务起步和八步落地路线，与阶段 2“离线评估与统计判断”的产出（20-50 条 Golden Set、两类评分器校准）直接对应；能力评测毕业为回归套件的部分补充到阶段 3。

## 文章核心可沉淀方法

- 评测结构：task、trial、grader、transcript、outcome、evaluation harness、agent harness、evaluation suite。
- 三类评分器：code-based（快、客观、可复现）、model-based（灵活但需人工校准）、human（黄金标准但贵、慢）；组合方式为 weighted、binary、hybrid。
- capability evals 从低通过率起步，regression evals 应接近 100% 通过；高分能力评测可“毕业”为回归套件。
- 非确定性用 pass@k（至少一次成功）与 pass^k（全部 k 次成功）区分，按产品需求选择。
- 落地八步：尽早开始；从手工测试、缺陷库和客服队列收集 20-50 条真实失败任务；写无歧义任务并配参考解；平衡正反用例；使用稳定且每次隔离的评测环境；慎重设计评分器（不过度约束调用顺序、支持部分得分、LLM 评委留“Unknown”出口）；读 transcript 验证评分器公平性；监控能力评测饱和。
- 自动评测不是唯一手段，需与线上监控、A/B 测试、用户反馈、人工 transcript 审查和系统化人工研究组合（瑞士奶酪模型）。

## 后续动作

- 编写《Agent 评测中的随机性与版本判断指南》（TODO 9）时，把 pass@k/pass^k、能力/回归转化和“读 transcript 验证评分器”作为素材。

## 需要同步到长期记忆的内容

- 学习计划新增该文章及其本地路径（已写入 PROJECT_MEMORY 当前资源与环境）。
