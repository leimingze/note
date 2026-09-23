# 调研：Agent Judge SOTA 基线与可优化空间

日期：2026-08-25

## 摘要

当前应从自建 Qwen3-4B 弱基线转向公开专用 Agent Judge：OS-Shepherd-9B 是已找到的最强可获得开放端到端 CUA 轨迹 Judge；WebJudge-7B 是关键截图筛选器而非独立端到端 Judge，不能把其流水线结果当成单模型结果。

最值得预验证的优化点是降低 Judge 对 Agent 完成声明的依赖，同时保持对动作和截图证据的敏感性；该问题可通过公开训练标签和自动反事实变体研究，不需要新增人工标注。

## 核验结论

- `OS-Copilot/OS-Shepherd-9B@6f40205c2fdd2e7db2ac0a5d6cb11edd2683f470` 非 gated、Apache-2.0，模型文件约 18.8GB，基座为 Qwen3.5-9B。
- `OS-Copilot/OS-Shepherd-100K@7729502905f40d1bbefb64ef04f7bebb01709845` 当前 API 显示非 gated，SFT/RL 文本与图片分片公开；图片总量超过 100GB。
- `OS-Copilot/OSReward@60dbd2896b0b04b0bdad4262f50705eca62cf955` 公开 1,019 条 Full 和 284 条 Hard 人类金标轨迹，Full Parquet 约 15GB。
- OS-Shepherd-9B 在 OSReward Full 的 Accuracy/Balanced Accuracy 为 `86.1/86.3`，Hard 为 `60.2/61.9`；Qwen3.5-9B 基座对应为 `76.7/79.4` 和 `39.4/55.9`。
- `osunlp/WebJudge-7B@bb550f8e301855933db5490eb8128217b3aca5dc` 非 gated、Apache-2.0，约 16.6GB；模型只完成截图总结和相关性评分，最终关键点提取与成败判断仍由 o4-mini 或 GPT-4o 执行。
- AgentForesight 的 AFTraj-2K 和推理代码公开，但 7B 检查点明确写为论文接收后发布，不能作为当前可执行基线。
- AgentV-RL 面向数学解答验证而非 Agent 环境轨迹；VAGEN 需要主动访问 GUI 环境，均不适合作为当前离线轨迹主基线。

## 创新边界

`From Confident Closing to Silent Failure` 已证明 TF-IDF/XGBoost 可超过 LLM Judge，并已做 detector 与 Judge 的加权融合；因此轻量分类器或简单 ensemble 不能单独作为创新。

BabelJudge 已使用九类自动轨迹扰动审计 Judge；Judge Reliability Harness 已覆盖格式、改写和标签翻转；Perceptual Judgment Bias 已用反事实响应和 Reward Modeling 缓解文本压过视觉证据的问题。因此候选方法必须聚焦 Agent 轨迹中的“完成声明与环境证据因果解耦”，并在真实人类金标 OSReward-Hard 上证明收益，而不能只报告自建扰动集准确率。

## 建议方法

先对 OS-Shepherd-9B 做声明删除、中性改写和反向声明的成对诊断；若存在明显 verdict 翻转，再使用 OS-Shepherd-100K 自动构造成对数据，执行声明反事实 SFT 与 verdict 一致性约束。

训练和评测以原轨迹为分组单位；成功标准要求提高 Hard failure recall 和 Balanced Accuracy，同时 Full success recall 下降不超过 1 个百分点，并在独立域保持同方向收益。

## 工程风险

服务器原环境的 `transformers 4.57.6` 不包含 `transformers.models.qwen3_5`，`vLLM 0.15.0` 注册表也不含 Qwen3.5；必须建立隔离环境验证新版本与 MetaX 插件，不能直接升级正在运行实验的环境。

Qwen2.5-VL 已在现有 Transformers 中可识别，WebJudge 可先用于验证多模态输入和模型下载链路；它不能替代 OS-Shepherd 的科学基线。

## 来源

- https://arxiv.org/abs/2607.28609
- https://github.com/OS-Copilot/OSReward
- https://huggingface.co/OS-Copilot/OS-Shepherd-9B
- https://huggingface.co/datasets/OS-Copilot/OS-Shepherd-100K
- https://huggingface.co/datasets/OS-Copilot/OSReward
- https://arxiv.org/abs/2504.01382
- https://github.com/OSU-NLP-Group/Online-Mind2Web
- https://huggingface.co/osunlp/WebJudge-7B
- https://arxiv.org/abs/2606.09863
- https://arxiv.org/abs/2606.22329
- https://arxiv.org/abs/2606.02578
- https://arxiv.org/abs/2603.05399
- https://github.com/ZBox1005/AgentForesight
