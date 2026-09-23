# ADR-0001：Agent Judge 统一数据只统一容器

日期：2026-08-24
状态：已采纳

## 背景

Counsel、Plan-RewardBench、AgentProcessBench、SkillTV-Bench 和 AJ-Bench DS 分别提供批评元判断、成对偏好、步骤标签和轨迹结果，强行统一成成功或失败会改变监督语义并引入错误训练目标。

## 决策

统一样本固定包含 `sample_id`、`task`、`trajectory`、`evaluation`、`evidence` 和 `source` 六个顶层模块，结构由 `agent测评/论文/实验/schema/unified_agent_judge_sample.schema.json` 约束。

`evaluation.supervision_type` 区分四类监督，原始标签空间保持独立；`source.group_id` 必须对应源轨迹或源任务，所有训练、开发和评测划分均按该字段分组。

数据角色由 `agent测评/论文/实验/data_usage.json` 约束：Plan-RewardBench 与 AJ-Bench DS 仅评测，AgentProcessBench 在许可证确认前不进入训练。

## 影响

标准化 JSONL 是保真中间层，不是最终 Prompt；后续需要为四类监督分别设计 Prompt 和损失，不能直接拼接标签。

数据量必须同时报告监督记录数和独立任务组数，避免同一任务的多个轨迹、批评或 hard-negative 被当成独立任务。

## 替代方案

未采用统一二分类标签，因为会丢失批评可信度、步骤位置和成对偏好信息；未采用只保存文件引用的方案，因为训练和复现实验需要稳定、可校验的自包含 JSONL。

## 证据

- `agent测评/论文/实验/reports/data_audit.md`
- `agent测评/论文/实验/reports/normalization_report.md`
- `agent测评/论文/实验/data/normalized/index.json`

## 后续约束

修改 Schema、标签语义、`group_id` 定义或数据角色时，必须更新适配器、自动化测试、使用清单和本 ADR；不得通过静默映射兼容旧输出。
