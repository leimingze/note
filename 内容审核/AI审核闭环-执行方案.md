# AI 内容审核闭环执行方案

本文是 `AI审核闭环-设计.md` 的落地计划：先做什么、产出放在哪、每一步怎么算完成。设计上的取舍和理由以设计文档为准，这里不重复。

总工期约 34–41 人天，分 7 个里程碑。M0–M4 不依赖 PulseFrame，可以完全离线完成；M5 才和 PulseFrame 集成。

里程碑顺序由数据流决定：

```text
M0 数据切分 + 两个基线（v0 API、v_a 开源基座）
M1 判定链路（结构化输出、5 次采样、缓存）
M2 判例库 + 版本 + 门禁 + 回滚
M3 冷启动集 → SFT → v_b1
M3b 外部数据清洗 → 混合 SFT v_b2 → 离线 DPO v_c1
M4 周期 1–3（反馈、判例迭代）→ 反馈 DPO → v_c2 → 周期 4 前瞻检查
M5 PulseFrame 集成
```

---

## 1. 开工前要定的事

| 事项 | 默认方案 | 备注 |
|---|---|---|
| 代码放哪 | PulseFrame 的 `backend/` 中新增 `internal/review` 模块和两个独立入口 `cmd/pulseframe-review`（在线服务）、`cmd/review-replay`（离线回放）；训练脚本放 `backend/tools/train/` | 复用 `platform/` 的配置、日志、MySQL；独立二进制即可独立部署。**待你确认** |
| 数据库 | 独立库 `pulseframe_review` | 满足"审核服务使用自己的数据库" |
| 推理 | 云 GPU 上的 vLLM，OpenAI 兼容接口；商用 API 只用于 v0 和教师模型 | 两者共用同一个客户端 |
| GPU | 默认租 24GB 显存实例，装不下升到 48/64GB | 训练和批量评估集中安排，跑完即释放 |
| 基座模型 | Qwen2.5-VL-3B-Instruct（多模态 VLM） | 选定后在整个实验期间不换 |
| 教师模型 | 商用 API 中较强的多模态模型，和基座不同源 | 先确认服务条款允许用其输出训练模型 |
| 训练框架 | ms-swift | 版本号固定并写入训练配置 |
| 预算上限 | API 设每日 token 上限；GPU 设单次租用时长上限 | 具体数值在 M0 实测后确定 |
| 数据与权重存放 | `backend/data/kuaimod/`、`backend/data/external/`、`backend/artifacts/adapters/`，都加入 `.gitignore` | KuaiMod 和由它训练出的权重受 CC-BY-NC-4.0 约束；外部数据按各自许可处理；都不入库、不发布 |
| 外部数据来源 | M3b 开始前逐个核实许可、获取方式和实际内容 | 需要申请的数据集提前提交申请，等待期间不阻塞 M0–M3 |

API Key 和云平台凭据只通过环境变量注入，不写入代码、配置文件或文档。

---

## 2. 目录布局

```text
backend/
  cmd/
    pulseframe-review/main.go    在线审核服务入口
    review-replay/main.go        离线回放命令行入口
  internal/review/
    model.go                     表模型与 Models()
    errors.go
    repository.go
    service.go                   在线流程：接收任务 → 判定 → 处置 → 回调
    handler.go / routes.go       /internal/review/tasks、查询接口、人审接口
    input/                       结构化输入；KuaiMod 与真实上传两种来源
    judge/                       prompt 组装、VLM 调用、输出校验、5 次采样打分、缓存
    policy/                      策略版本加载、active 指针、阈值标定、发布与回滚
    caselib/                     判例存取与检索
    replay/                      数据切分、周期运行、模拟反馈、门禁、报告
  tools/
    kuaimod_export.py            HF 数据集 → JSONL + 图片文件
    train/
      build_cot.py               教师模型生成 CoT 并过滤
      build_pairs.py             从 decision/feedback 导出 DPO 偏好对
      sft.yaml / dpo.yaml        ms-swift 训练配置
      serve.sh                   vLLM 启动脚本（基座 + 多 LoRA）
    external/                    外部数据清洗：格式转换、标签映射、去重、配比（M3b）
    media_sidecar/               抽帧、OCR、ASR 的 Python 服务（M3b 实现，M5 复用）
  policies/                      策略版本内容（prompt、阈值、判例集、model_ref），入库可 diff
    v0/  v_a/  ...
  data/kuaimod/  data/external/  不入库
  artifacts/adapters/            不入库；哈希记入 policy_version
  reports/                       回放与训练报告，入库
```

`internal/review` 沿用 backend README 规定的业务模块文件约定（model → errors → repository → service → handler → routes），子目录放模块私有工具。

Go 和 Python 之间只通过文件和数据库交换数据：Go 导出 JSONL，Python 读取 JSONL 后训练，产出 adapter 和清单文件，再由 Go 登记成版本。两边不互相调用代码。

---

## 3. 里程碑

### M0：数据与基线（5 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| 导出数据集 | `tools/kuaimod_export.py`；`data/kuaimod/samples.jsonl` 和 `images/` | 条数与数据集卡片一致；每条都有 tag，图片都能打开 |
| 核对字段与分布 | `reports/m0-data-profile.md` | 记录实际字段、各 tag 条数、违规与正常的比例、每条的平均帧数 |
| 选择延后类别 | 同上文件 | 2–3 个 tag 合并后，在留出集中至少有 30 条正例，在第 3–4 周期中至少有 30 条。达不到就把场景 1 标记为"定性展示"，写明原因 |
| 数据切分 | `review-replay split --seed 42`，输出 `data/kuaimod/split.json` | 留出集 300、冷启动集 300、周期 4×100；延后类别不进入冷启动集和第 1 周期；固定种子，重复运行结果一致 |
| 表结构 | `internal/review/model.go`：`policy_version`、`case_record`、`decision`、`feedback`、`review_task`、`replay_run`、`train_run` | 迁移在本地 MySQL 上通过集成测试 |
| 推理端 | `judge/` 中的客户端接口、真实实现和测试用假实现；`tools/train/serve.sh` | API 和 vLLM 各用 5 条样本打通一次调用，记录 token 数和延迟 |
| 基座选型 | 2 个候选基座在冷启动集的 50 条样本上做零样本对比 | 按格式失败率和二分类准确率选一个；**不使用留出集选型** |
| 成本实测 | `reports/m0-data-profile.md` 的成本一节 | API：单条 token 与费用；vLLM：留出集 300 条 × 5 次采样的耗时，以及 GPU 费用 |
| 两个基线 | `reports/baseline-holdout.md` | v0 和 v_a 在留出集上的召回、误杀率（Wilson 95% CI）、格式失败率、中间置信段占比 |

M0 的检查点：
- v_a 的格式失败率超过 5% 时，M3 先做一轮只修格式的 SFT，并在报告中单独列出。
- 留出集评估一次超过 1 小时 GPU 时间时，先排查 vLLM 的批处理配置，再考虑减少采样次数。

### M1：判定链路（3 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| 结构化输入 | `input/` 中的统一结构 | KuaiMod 样本能无损转换；真实上传的来源先留接口 |
| prompt | `policies/v_a/prompt.md` | 政策说明直接使用 KuaiMod 的 tag 定义；判例以文字形式注入 |
| 输出校验 | `judge/` 中的 JSON 解析与校验 | 格式不合法时重试一次，仍失败标记为转人审；`cited_cases` 引用了未注入的 id 也算格式错误；单元测试覆盖 |
| 5 次采样打分 | 违规分数取值 0、0.2 … 1 | 单元测试覆盖分数计算和理由与标签的一致性校验 |
| 判定缓存 | 按 `(sample_id, 判定相关哈希)` 缓存到 MySQL | 同版本第二次运行时推理调用数为 0 |

M0 中两个基线的评估要用到 M1 的判定代码。实际开发顺序是：先写 M1 的最小可用版本，再回头完成 M0 的基线。

### M2：判例库、版本、门禁与回滚（6 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| 判例存储 | `policies/vN/cases/*.yaml`，登记时导入 `case_record` | 判例集哈希由内容决定，内容不变则哈希不变 |
| 判例检索 | `caselib/`：按类别过滤后做文本向量相似度，取前 k 条（k ≤ 5） | 判例 ≤ 30 条时全量暴力检索；向量模型和版本记入策略版本 |
| 版本登记 | `review-replay register policies/vN` | 计算各部分哈希；检查相对父版本只改了一类内容，改了两类就登记失败 |
| 阈值标定 | `review-replay calibrate --version vN` | 按设计文档 A4 的固定程序在冷启动集上标定；同一输入两次运行结果一致 |
| 发布门禁 | `review-replay gate --candidate vN` | 实现设计文档 A9 的全部规则；过拟合规则只用于提示词、阈值、判例变更；输出通过或拒绝，并附理由和旧/新阈值下的两组指标 |
| 发布与回滚 | `review-replay release vN`、`review-replay rollback` | 切换 active 指针；回滚回到 `parent_version`；每次操作写审计记录 |

M2 的检查点：手工构造一个"阈值调得过激"的版本，门禁必须拒绝它；强制发布后执行回滚，active 要恢复到原版本，指标要和原版本一致（命中缓存）。这一步同时完成验收场景 2。

### M3：SFT（5–7 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| CoT 生成 | `tools/train/build_cot.py`；`data/train/sft_cot.jsonl` | 教师模型以真实标签为条件，生成符合输出格式的结果；输入中的判例集与 v_a 版本一致 |
| 过滤 | 同上，附丢弃统计 | 标签不一致、引用了未注入判例的直接丢弃；丢弃率写入 `train_run` |
| 人工抽查 | `reports/sft-cot-audit.md` | 随机抽 30 条，理由真正基于样本内容的合格率 ≥ 80%；不达标就改教师 prompt 后重新生成 |
| 显存试跑 | 记录峰值显存 | batch size 1 跑通一步；64GB 装不下就换 128GB；在 128GB 上仍装不下时，先排查 gradient checkpointing 和图片 token 数 |
| SFT 训练 | `artifacts/adapters/sft-xxx/`；训练日志 | 使用 LoRA + bf16；adapter、日志、数据清单、配置四者的哈希写入 `train_run` |
| 登记 v_b1 | `policies/v_b1/`，`change_type=model` | 自动执行阈值重新标定 |
| 门禁 | `reports/gate-v_b1.md` | 和 v_a 在留出集上配对比较；通过则发布，否则 active 保持 v_a |

M3 的检查点：v_b1 的中间置信段占比低于 2% 时，即使指标提升也要在报告中标注"置信度可能失效"，门禁按 A9 处理。

### M3b：外部数据（7–10 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| 来源核实 | `reports/external-sources.md` | 每个候选来源写明许可、获取方式、规模、类别、形态；不满足设计文档清洗第 1 步准入条件的直接排除 |
| 预处理 sidecar | `tools/media_sidecar/` | 外部视频转成和 KuaiMod 相同的结构化输入；处理速度和失败率写入报告 |
| 标签映射表 | `tools/external/label_map.yaml` | 每个来源标签映射为 KuaiMod tag、normal 候选或丢弃；拿不准的一律丢弃；映射到延后类别的全部丢弃 |
| normal 筛选 | 教师模型检查 normal 候选是否命中其他 KuaiMod 类别 | 命中的丢弃；丢弃比例写入清洗表 |
| 去重 | 帧感知哈希 + 文本相似度 | 外部内部去重；和 KuaiMod 1000 条去重，和留出集近似重复的全部丢弃；FakeSV 单独核查 |
| 质量过滤 | 同上清洗脚本 | 损坏、过短、无有效帧的样本丢弃 |
| CoT 与一致性检验 | 复用 M3 的 `build_cot.py` | 按来源和映射标签统计丢弃率，超过 30% 的来源或标签整体丢弃 |
| 配比与切分 | `data/external/manifest.json` | 外部总量 ≤ 冷启动集的 3 倍，单一来源 ≤ 50%；分成外部 SFT 池、外部 DPO 池、外部验证集，三者互不重叠；清洗表记录每一步前后的条数 |
| 混合 SFT | v_b2，训练配置与 v_b1 完全相同 | 门禁中和 v_b1 比较，读出外部数据的贡献 |
| 捷径学习检查 | `reports/shortcut-check.md` | 外部验证集准确率比留出集同类别高 15pp 以上，或留出集误杀率显著上升时，下调对应来源的配比后重训 |
| 离线 DPO | 当前 active 模型在外部 DPO 池上推理，错例组成偏好对，训练 v_c1 | 报告独立错例数和偏好对数；过门禁后在第 1 周期前发布 |

M3b 的检查点：
- 清洗后可用的外部样本少于 500 条时，跳过 v_b2，只保留离线 DPO；仍不足 200 条时整个 M3b 只保留清洗记录。
- API 预算按外部样本数重新估算：CoT 生成和 normal 筛选都要调用教师模型，开工前先用 50 条实测单价。

### M4：反馈周期与反馈 DPO（7–9 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| 周期运行 | `review-replay cycle --n K` | 当前 active 版本跑第 K 周期，判定结果写入 `decision` |
| 模拟反馈 | `replay/` 中的反馈生成器 | 默认参数：误杀 80% 申诉，漏判 30% 举报，正确通过 2% 恶意举报；固定种子可复现 |
| 模拟人审 | 只对有反馈的样本取 KuaiMod 真值，写入 `feedback` | 没有反馈的错误不暴露给开发者，也不进入训练数据 |
| 错例视图 | `review-replay errors --cycle K` | 只列出已暴露并经人审确认的错例 |
| 周期 1–3 | 每个周期至多产出一个提示词、阈值或判例版本，过门禁 | 延后类别从第 2 周期出现，新增判例后完成验收场景 1 |
| 偏好对导出 | `tools/train/build_pairs.py`；`data/train/dpo_pairs.jsonl` | rejected 取自 `decision` 表中当时 active 模型的原始错误输出；chosen 按 M3 的方法生成并过滤；报告独立错例数和偏好对数 |
| DPO 训练 | `artifacts/adapters/dpo-xxx/` | 在当前 active 模型上继续训练；参考模型为关闭 adapter 的基座 |
| 登记 v_c2 并过门禁 | `reports/gate-v_c2.md` | 和直接父版本比较；不显著也要如实记录 |
| 周期 4 | `reports/cycle-4.md` | v_c2（或者 v_c2 被拒时的 active 版本）在全新数据上运行，作为前瞻检查 |
| 过拟合演示 | 在某个周期针对已暴露错例写一批过窄的判例 | 门禁以"过拟合"为由拒绝，完成验收场景 3 |
| 汇总 | `reports/acceptance.md`、微调归因表 | 4 个验收场景各给出版本号、门禁输出和指标变化 |

周期中的人工操作只有一步：阅读错例，决定改哪一类内容，产出候选版本。其余步骤都由命令完成。

M4 的检查点：独立错例少于 15 条时，反馈 DPO 仍然要跑完整个流程，报告中注明"样本不足，结论仅供流程演示"。

### M5：PulseFrame 集成（4 人天）

| 任务 | 产出 | 完成标准 |
|---|---|---|
| 任务接收 | `POST /internal/review/tasks` | 以 `task_id` 去重，重复投递返回 200；落库后异步处理 |
| 媒体预处理 | 复用 M3b 的 `tools/media_sidecar/` | 图片分辨率与训练时一致；预处理版本记入决策 |
| 在线判定 | 判定 worker 读取 active 版本（带短 TTL 缓存） | 与离线回放调用同一套 `judge/` 代码 |
| 推理不可用 | GPU 实例没开时任务停在"审核中" | PulseFrame 通过查询接口对账时看到"审核中"；实例启动后任务继续处理 |
| 回调与重试 | 调用 `/internal/review/callback`，失败按退避重试 | 重复回调不产生副作用 |
| 查询 | `GET /internal/review/tasks/{task_id}` | 返回结论或"审核中" |
| 人审后台 | 最简页面或命令行 | 人审结论写入 `decision.final_label` 后回调 |
| 复审 | `kind=rereview`，`trigger=report` 时额外写一条 `feedback` | 能从反馈表查到由举报触发的复审 |
| 端到端演示 | 本地起 PulseFrame API、审核服务、sidecar，并开启 vLLM 实例 | 上传一条视频，走完投递、判定、回调、对账；记录在 `reports/e2e.md` |

M5 的检查点：替换内容与发布模块自带的模拟审核服务之前，先用它现有的重试与对账测试验证新服务。

---

## 4. 验收场景的操作步骤

**场景 1：新类型冒头**（M4）
1. 冷启动集和第 1 周期中没有延后类别。
2. 第 2 周期放入延后类别，报告中出现漏判，其中一部分被模拟举报暴露。
3. 根据暴露的错例新增 1–2 条判例，产出候选版本，过门禁后发布。
4. 预期结果：延后类别合并后的召回提升（留出集加上发布后的周期数据），且超出置信区间；误杀率没有显著变化。M0 判定样本不足时只做定性展示。

**场景 2：失败改动**（M2）
1. 基于当前 active 版本只把高阈值调低一档（0.2），产出候选版本。
2. 预期结果：门禁以"误杀率显著变差"为由拒绝。
3. 用 `release --force` 强制发布，再执行 `rollback`。
4. 预期结果：版本表中能看到这次发布与回滚，回滚后的指标与原版本一致。

**场景 3：过拟合**（M4）
1. 针对已暴露的具体错例，写一批针对性很强的判例，产出候选版本。
2. 预期结果：已用数据上的指标明显提升，留出集上不提升；门禁以"过拟合"为由拒绝。

**场景 4：微调归因**（M3、M3b、M4）
1. v0、v_a、v_b1、v_b2、v_c1、v_c2 各自在留出集上的指标，以及每一步门禁的结论。
2. 预期结果：每一步的贡献可以单独读出。"没有显著提升"同样是有效结论。

每个场景实际跑出的结果都要如实写入报告。预期没有出现时，记录实际现象和原因分析，不要调整数据去凑结果。

---

## 5. 测试策略

| 层次 | 内容 | 如何运行 |
|---|---|---|
| 单元测试 | 输出校验、违规分数、阈值标定、Wilson 区间、McNemar 检验、门禁规则、单类改动检查、反馈生成器（固定种子） | `go test ./internal/review/...`，不调用真实推理 |
| 集成测试 | 表迁移、缓存命中、发布与回滚、任务去重 | 使用 `mysqlintegration` 标签，沿用现有集成测试方式 |
| 训练数据测试 | CoT 过滤规则、偏好对导出（rejected 必须来自 active 模型） | Python 的 pytest，用小样本固定数据 |
| 契约测试 | 与 PulseFrame 的投递、回调、查询 | M5 中用内容与发布模块的测试驱动 |
| 真实调用 | 只在 `eval`、`cycle`、`gate` 命令和训练脚本中发生 | 受预算上限约束 |

统计函数（Wilson、McNemar）要和已知结果的手算样例做对照测试，避免门禁建立在错误的公式上。

---

## 6. 风险与应对

| 风险 | 现象 | 应对 |
|---|---|---|
| 数据集无法下载或字段与预期不符 | M0 导出失败或字段缺失 | 先核对数据集卡片和实际文件，按实际字段调整 `input/`；需要登录或同意条款时手动完成 |
| 延后类别样本不足 | M0 统计后不满足条件 | 场景 1 降级为定性展示，不调整切分去凑数量 |
| 外部数据申请被拒或迟迟没有回复 | M3b 开始时可用来源不足 | 只用公开可直接下载的来源；不足 500 条按 M3b 检查点处理 |
| 外部口径污染快手口径 | v_b2 误杀率显著上升 | 下调配比或去掉冲突来源；主线沿用 v_b1 |
| 多模态训练数据格式调试耗时 | M3 第一次训练迟迟跑不通 | 先用 10 条样本跑通完整流程，再扩到全量；超出预估 50% 时按最后一行处理 |
| 显存不足 | 128GB 上仍然 OOM | 排查训练配置；不改变输入形态 |
| 微调后置信度失效 | 中间置信段占比骤降 | 门禁按规则拒绝；在报告中分析原因 |
| 偏好对太少 | 独立错例少于 15 条 | DPO 只作为流程演示，如实报告 |
| GPU 费用失控 | 实例忘记释放 | 设置租用时长上限和自动关机；每次租用记录在报告中 |
| 留出集太小，测不出差异 | 多数门禁结论为"无显著变化" | 接受这一结论并写进报告；不要把其他数据并入留出集来扩大样本 |
| 进度超出 | 某个里程碑超出预估 50% 以上 | 优先级：M0–M3 > M4 > M3b > M5。M3b 可以缩减为只用 1 个来源；M5 缩减为只做接收、判定、回调，用 KuaiMod 样本伪装成上传视频 |

---

## 7. 交付清单

- [ ] `reports/m0-data-profile.md`：数据画像、切分方案、基座选型、成本实测
- [ ] `reports/baseline-holdout.md`：v0 与 v_a 基线
- [ ] `reports/sft-cot-audit.md`：CoT 抽查结果
- [ ] `reports/external-sources.md`、外部数据清洗表、`reports/shortcut-check.md`
- [ ] `reports/gate-*.md`：每次门禁的输出
- [ ] `reports/cycle-1.md` 到 `cycle-4.md`：周期报告
- [ ] `reports/acceptance.md`：四个验收场景和微调归因表
- [ ] `reports/e2e.md`：PulseFrame 端到端演示
- [ ] `policies/`：全部策略版本，包括被拒绝的版本
- [ ] `train_run` 表导出：每次训练的数据量、丢弃率、峰值显存、GPU 时长与费用
- [ ] README 写明：数据和权重仅限非商用；指标口径为快手标签；已知局限（见设计文档 R 节）
