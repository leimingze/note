# 调研：Agent Judge 数据集可用性与研究方案复审

日期：2026-08-24

## 摘要

原方案不能按“六个公开数据集统一后全部混合训练”直接实施：六个数据集只有 Counsel 和 AgentRewardBench 能直接提供与 Judge 训练相关的公开监督，Plan-RewardBench 官方定位为只用于评测，REFLECT 和 MobileJudgeBench 尚未找到公开数据入口，TRAJECT-Bench 则不提供现成的 Judge 人工标签。

同时，OSReward 已经发布跨平台计算机操作 Judge Benchmark、10 万条训练语料和 9B/35B Reward Model，并完成跨 Benchmark 域外分析，因此“多源训练轻量 Judge”和“低成本接近商业 Judge”不能再单独作为主要创新。

## 六个原候选数据集核验

| 数据集 | 当前可获取性 | 实际内容 | 许可证 | 建议用途 |
| --- | --- | --- | --- | --- |
| Counsel | 可直接下载 | 225 条轨迹、1,131 条人工元评测；标签为批评正确、位置对但原因差、错误标记 | MIT | 可作为批评验证辅助训练数据，但不是直接的轨迹成功标签 |
| AgentRewardBench | 可直接下载 | 1,302 条唯一 Web Agent 轨迹，公开 1,408 行人工标注，覆盖成功、副作用、最优性和循环 | 无标准许可证，只有自定义研究条款 | 可作为直接监督核心来源，但需处理 30 GB 多模态资源、许可和官方划分 |
| Plan-RewardBench | 可直接下载 | 1,171 对 chosen/reject 轨迹，7 个场景分区，约 18 MB | 代码 Apache-2.0，数据 CC BY 4.0 | 最适合作为偏好评测集；官方明确说明 evaluation-only，训练后不能再报告官方测试结果 |
| REFLECT | 暂不可用 | 论文报告 472 对受控扰动轨迹或报告 | 未发现数据许可证 | PDF、arXiv 页面、GitHub 和 Hugging Face 均未找到发布入口，当前不能纳入实验 |
| TRAJECT-Bench | 可直接下载 | 5,270 条参考工具轨迹，覆盖 10 个领域，约 24 MB Parquet | MIT | 适合工具选择和参数评测，但没有现成 Judge 人工标签，不能直接作为评分器监督数据 |
| MobileJudgeBench | 暂不可用 | 论文报告 931 条人工标注移动 Agent 轨迹，覆盖 6 个 Benchmark | 论文说明自建产物将发布 | 论文无项目链接，GitHub 与 Hugging Face 未检索到对应仓库，当前只能列为候选外部测试集 |

## 数据兼容性判断

这些数据不是同一种标签：Counsel 判断“批评是否可信”，AgentRewardBench 判断“轨迹是否成功及是否有副作用”，Plan-RewardBench 判断“两条轨迹谁更好”，TRAJECT-Bench 只给“参考工具路径”，因此不能强行压缩成一个正确或错误标签。

可行的统一方式是统一输入容器而不是统一标签语义，使用“任务、轨迹、评测问题、标签空间、证据、来源”六个字段，并以多任务方式保留分类、偏好和步骤标签。

## 新发现的可用资源

### AgentProcessBench

- 公开 1,000 条轨迹和 8,509 条人工步骤标签，覆盖 HotpotQA、GAIA、BFCL 和 tau2。
- 标签统一为正确、中性和错误，适合训练步骤级过程评分器。
- GitHub 仓库没有标准许可证，正式使用前需要作者确认研究使用与衍生数据权限。

### AJ-Bench

- 公开 155 个任务和 516 条标注轨迹，覆盖 Search、DS 和 GUI，仓库采用 MIT 许可证。
- DS 子集包含可直接读取的文本工具轨迹和成功或失败标签，适合文本 Judge 的域外测试。
- 完整 Agent-as-a-Judge 复现涉及搜索、数据库和 AWS GUI 环境，运行全部流程的资源成本较高。

### SkillTV-Bench

- 公开 681 个案例，其中 evolution 为 478 例、evaluation 为 203 例，两个分区在源任务级完全隔离。
- 数据覆盖 11 个领域并提供 pass/fail 标签、轨迹、技能和可检查工件，适合训练与域外评测。
- 自建数据为 CC BY 4.0，但上游轨迹含 AGPL-3.0 材料，使用和再分发时必须保留第三方许可边界。

## 最接近且会压缩创新空间的工作

### OSReward

- 发布 1,019 条 Full 和 284 条 Hard 多模态轨迹，并对 OSWorld、WindowsAgentArena、WebArena 和 AndroidWorld 做域外分析。
- 发布 OS-Shepherd-100K 和 9B/35B Reward Model，论文报告以商业 Judge 三十分之一至六十分之一的成本达到相近表现。
- OSReward 数据约 37 GB，OS-Shepherd-100K 约 118 GB，训练语料虽然已经公开，但资源需求并不轻量。
- 该工作已经覆盖跨平台数据、开放 Reward Model、域外评测和效率对比，原方案不能重复把这些内容作为主要贡献。

### AgentForesight

- 使用跨 Coding、Math 和 Agentic 领域的 AFTraj-2K 训练 7B 在线审计器，并在外部 Benchmark 上测试。
- 它主要解决在线首错预警而非完整轨迹离线评分，但已经证明“小模型加多域轨迹训练”不是空白。

### SkillTV-Evolve 与 RubricForge

- SkillTV-Evolve 使用开发池自动优化 JudgeSkill，在独立测试集上提升 Judge 准确率。
- RubricForge 使用少量带真实结果的轨迹自动演化评分 Rubric，重点降低把失败轨迹误判为成功的问题。
- 两者说明“不改权重、只优化评测规则”也是必须比较的强基线。

## 对原方案的重新评分

| 方案 | 创新性 | 可实现性 | 结论 |
| --- | ---: | ---: | --- |
| 六个数据集简单归一化并混合训练 | 4/10 | 5/10 | 不建议继续，标签不兼容、可用数据不足且与 OSReward 重叠 |
| 保留异构标签的普通多任务 LoRA | 5/10 | 7/10 | 可以实现，但主要是工程整合，论文贡献偏弱 |
| 兼容性感知的多源训练与负迁移抑制 | 6.5/10 | 7/10 | 当前最值得做的候选，但必须先用实验确认负迁移真实存在 |

## 修订后的候选问题

相比“把数据集混在一起”，更合理的问题是：不同 Agent 评测监督彼此差异很大，简单混训是否会产生负迁移，以及能否根据标签语义、任务领域和轨迹结构选择或加权训练来源，使轻量 Judge 在未见文本工具任务上更可靠。

## 修订后的候选方法

1. 使用统一输入容器保存不同标签空间，不把所有标签压缩成二分类。
2. 以来源标识和评测问题驱动多任务训练，建立简单混合、均衡采样和单源训练基线。
3. 设计兼容性感知的来源选择或损失加权方法，重点减少数据源之间的梯度冲突和负迁移。
4. 每轮完整留出一个 Benchmark，不允许目标 Benchmark 的样本进入训练、调参或校准。
5. 将全局校准、分来源校准和未见域校准作为分析问题，不再把普通温度缩放本身称为创新。

## 推荐的最小实验

1. 先使用 Counsel、SkillTV evolution 和 AgentProcessBench 构造文本可见的多任务训练集，但 AgentProcessBench 需先确认许可证。
2. 使用 AgentRewardBench 的官方测试集、SkillTV evaluation、AJ-Bench DS 和 Plan-RewardBench 做互斥的域外测试，任何进入训练的数据集不得同时作为该轮测试集。
3. 比较零样本、单源训练、简单混合、均衡混合和兼容性感知混合，观察简单混合是否确实造成某些目标域退化。
4. 只有在负迁移得到稳定复现且新方法显著改善至少两个未见 Benchmark 后，才冻结论文题目和创新点。

## 最终判断

原来的研究动机仍然成立，但原数据清单和创新表述需要收缩；当前不建议直接开始大规模训练，应该先做数据适配与单源或简单混合的最小实验，用结果决定是否转向“异构监督的负迁移抑制”。

这个修订方向仍不要求自行进行大规模人工标注，但需要处理许可证、长上下文、多模态裁剪或筛选、数据集污染和不同标签空间的评价协议。

## 第一阶段实测补充

已在 `agent测评/论文/实验/` 固定公开版本并完成最小数据下载与自动审计，机器可读结果和中文报告分别位于 `reports/data_audit.json` 与 `reports/data_audit.md`。

实测 Counsel 225 条轨迹和 1,131 条元评测、Plan-RewardBench 1,171 个轨迹对、AgentProcessBench 1,000 条轨迹和 8,509 个步骤标签、SkillTV-Bench 681 条轨迹、AJ-Bench DS 229 条轨迹，均与发布说明一致。

Plan-RewardBench 的 1,171 行只对应 584 个唯一任务 UUID，其中 375 个 UUID 包含多个 hard-negative 轨迹对，且发布文件含 1 条完全重复轨迹对；正式评测仍应保留官方全量口径，但抽样、置信区间和敏感性分析必须按 UUID 聚类，并额外报告去除完全重复行后的结果。

## 第二阶段标准化补充

五个数据集已经转换为 Unified Agent Judge Sample v1，共 4,212 条监督记录且没有空任务指令；其中 Counsel 对应 225 个轨迹组、Plan-RewardBench 对应 584 个任务组、AgentProcessBench 对应 200 个原任务、SkillTV-Bench 对应 50 个原任务、AJ-Bench DS 对应 42 个原任务。

当前可进入后续实验设计的训练候选为 Counsel 1,131 条和 SkillTV evolution 478 条，共 1,609 条；固定评测池为 SkillTV evaluation 203 条、Plan-RewardBench 1,171 条和 AJ-Bench DS 229 条，共 1,603 条；AgentProcessBench 1,000 条仅完成格式转换，许可证确认前不训练。

统一数据层保留四类监督标签，不是最终模型 Prompt；后续必须分别设计批评元分类、成对偏好、步骤分类和轨迹结果 Prompt，并按 `source.group_id` 建立数据划分。

## 主要来源

- [Counsel 数据集](https://huggingface.co/datasets/AtlaAI/counsel)
- [AgentRewardBench 数据集](https://huggingface.co/datasets/McGill-NLP/agent-reward-bench)
- [Plan-RewardBench 仓库](https://github.com/wyy-1112/Plan-RewardBench)
- [TRAJECT-Bench 仓库](https://github.com/PengfeiHePower/TRAJECT-Bench)
- [AgentProcessBench 仓库](https://github.com/RUCBM/AgentProcessBench)
- [AJ-Bench 仓库](https://github.com/aj-bench/AJ-Bench)
- [SkillTV-Bench 仓库](https://github.com/HanZhi306/SkillTV-Bench)
- [OSReward 项目](https://os-copilot.github.io/OSReward-Home/)
- [AgentForesight](https://arxiv.org/abs/2605.08715)
- [RubricForge](https://arxiv.org/abs/2608.13564)
