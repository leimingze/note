# 产物：Agent 轨迹自动评测论文方向

日期：2026-08-24
类型：研究方向文档

## 路径

- `/Users/leimingze/notes/agent测评/论文/agent轨迹自动评测研究方向.md`
- `/Users/leimingze/notes/agent测评/论文/相关论文/README.md`
- `/Users/leimingze/notes/agent测评/论文/相关论文/`
- `/Users/leimingze/notes/agent测评/论文/实验/`
- `/Users/leimingze/notes/agent测评/论文/实验/实验系统设计文档.md`
- `/Users/leimingze/notes/agent测评/论文/实验/reports/data_audit.md`
- `/Users/leimingze/notes/agent测评/论文/实验/reports/normalization_report.md`
- `/Users/leimingze/notes/agent测评/论文/实验/reports/prompt_rendering_report.md`
- `/Users/leimingze/notes/agent测评/论文/实验/reports/token_audit.md`
- `/Users/leimingze/notes/agent测评/论文/实验/reports/zero_shot_full.md`
- `/Users/leimingze/notes/agent测评/论文/实验/核心贡献验证实验设计.md`
- `/Users/leimingze/notes/agent测评/论文/实验/reports/core_validation_execution.md`
- `/Users/leimingze/notes/agent测评/论文/实验/data/normalized/index.json`
- `/Users/leimingze/notes/agent测评/论文/实验/data/prompts/index.json`

## 用途

归档 Agent 评测论文的暂定题目、背景、问题、方法、实验、公开数据集和评价指标。

## 状态

经数据可用性和最新先前工作复审后已重写。当前候选主线为研究异构 Agent 评测监督的兼容性与负迁移，并在负迁移得到复现后开发兼容性感知的多源训练方法。题目和创新点尚未冻结，不要求自行建设大规模人工标注数据集。

已下载并校验 16 篇相关论文，其中 6 篇用于优先复现，10 篇用于确认创新边界。论文标题、用途、arXiv 来源和本地文件链接记录在论文索引中。

第一阶段数据审计已完成：五个数据源的实测数量全部匹配发布说明，下载版本、许可证边界、轨迹长度和标签分布已经固化；Plan-RewardBench 必须按 584 个源任务 UUID 做分组统计，不能把 1,171 行视为独立任务。

第二阶段统一数据转换已完成：4,212 条记录全部通过 Schema、标签语义和数据角色校验，四类监督保持独立；当前训练候选 1,609 条、固定评测 1,603 条，AgentProcessBench 1,000 条因许可证待定而排除。

实验系统设计文档已归档，覆盖已完成的数据基础设施、第三阶段 Prompt 与 Token 审计、零样本基线、训练顺序、资源边界和分阶段验收标准。

第三阶段已完成：4,212 条统一样本生成 5,383 条严格 JSON Prompt，模型输入与隐藏标签物理分离；Qwen3-4B 原生上下文覆盖率为 99.87%，7 条超窗样本保留清单并标记为该模型不可评测，不实施静默裁剪。

第四阶段零样本基线已完成：Qwen3-4B 在 MetaX C500 上以并发 16 运行 5,376 条可覆盖 Prompt，5,372 条严格解析成功、4 条格式失败且无截断；预测支持按批持久化和断点续跑，完整分数据集结果记录在 `reports/zero_shot_full.md`。

核心贡献验证已经进入执行阶段：固定分组划分、64K 训练覆盖审计、MetaX BF16 LoRA、assistant-only loss、尾部 logits 等价验证、四条件 10 步训练和 LoRA 推理评分链路均已完成；当前小规模结果仅用于验证实现，不支持判断负迁移，正式步数仍需开发集冻结。

## 关联记忆

- `memory/artifacts/2026-08-19-agent-eval-notes.md`
- `memory/PROJECT_MEMORY.md`
- `memory/research/2026-08-24-agent-eval-dataset-usability-review.md`

## 维护说明

研究方向正文已按数据可用性复审结果更新；下一步用两个开发集确定共同训练步数，完成四条件乘三个随机种子的正式训练与 2,938 条固定测试评测，再决定是否冻结兼容性感知方法。
