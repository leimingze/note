# 项目记忆

最后更新：2026-09-21

## 项目目标

记录项目是什么、服务谁、核心目标、短期目标和长期方向。

## 已确认的用户需求

记录用户已经明确确认的需求、权限规则、产品边界、业务规则和默认行为。

- Agent 测评学习指南默认面向没有软件测试经验的读者；术语必须在首次出现时用白话解释，自定义教学结构必须明确区别于行业标准术语。
- MySQL 八股学习目录按用户提供的大纲组织（基础、索引、事务、锁、MVCC、日志、内存、引擎、性能优化）；不在本地部署 MySQL，临时使用服务器 30052 实例。
- MySQL 练习必须先给出可作答的数据/业务背景、可用表字段、具体问题和完成标准；不能只给泛化练习名称或空作答格。
- 视频社交实习项目命名为 `PulseFrame`，从零设计和实现；GCFeed 只作为阅读与方案对照参考，不复用其代码。每个模块在实现前需按面试口吻说明背景、需求和方案选型，并尽量使用来源与许可明确的公开真实数据验证设计。
- 视频发布阶段只实现上传、发布及必要的基础处理，不建设剪辑、裁切、滤镜、特效和配乐等视频编辑能力。

## 当前策略

记录当前阶段怎么推进、优先做什么、暂时不做什么、采用哪些路线。

- 原“异构评测监督兼容性感知训练”路线的单种子预验证未发现可归因的稳定负迁移，正式三种子实验暂停；已完成的标签均衡、损失模式和曝光匹配实验只保留为诊断与消融。
- 2026-08-24 数据复审确认原六数据集不能直接统一混训，且简单多源训练与 OSReward 已有贡献重叠，因此不再把数据集归一化或普通混训本身作为论文创新。
- 2026-08-25 主实验路线进一步调整为“先复现公开专用 Agent Judge SOTA，再做可归因增量优化”：OS-Shepherd-9B 作为开放端到端主基线，WebJudge-7B 仅作关键截图筛选流程基线，Qwen3-4B 既有实验保留为诊断和消融。
- 候选优化为声明-证据反事实一致性训练：只在 OS-Shepherd 对 Agent 完成声明的自动反事实诊断通过门禁后实施，使用公开 OS-Shepherd-100K 标签，不新增人工标注，并以 OSReward-Hard failure recall、Balanced Accuracy 和独立域迁移作为核心判据。
- 测试提效实习项目的候选方向已收敛为“PRD 驱动的测试 Agent 测评平台”：将 PRD 编译为公开任务与隐藏判分标准，统一运行并比较不同测试 Agent；MVP 优先评测 API 测试 Agent，以真实执行、mutation 缺陷召回、误报和证据一致性为主指标。
- 实习项目 `EvoAgent` 已新增架构拆解与覆盖审计文档，位于 `实习项目/EvoAgent/docs/`，覆盖 Harness 分层、产品组装、一次请求链路、数据留存与恢复、反馈回流与演进发布，以及模块与代码覆盖清单；README 已加入文档入口。覆盖审计确认评测/离线实验模块、四个脚本、17 个测试文件和 9 个 Skill 目录尚未被架构文档完整覆盖。2026-08-27 在 Python 3.10.11 下运行 63 项测试：62 项通过、1 项失败（`test_safe_fixer_changes_only_supported_rules`，双引号/单引号断言差异；`SafeFixer` 当前产品未使用）。文档中记录的能力边界包括 shadow 发布未接入主链路、Agent Skill 未做进程沙箱、`agent_messages` 无主动写入。
- `PulseFrame` 按“总体架构与工程基础、用户认证、视频上传与审核、Feed 与播放、互动与推荐、弹幕、内容安全 Agent、直播、工程治理”的依赖顺序推进；技术组件必须在对应模块设计阶段根据需求和验证证据选择。

## 当前已实现

记录已经落地的功能、脚本、服务、配置、端口、访问地址和关键文件。

- `PulseFrame` 已在 `实习项目/PulseFrame/backend/` 落地第一阶段工程基础：Go 1.26 模块、Gin 1.11 核心 API、类型化环境配置、`log/slog` JSON 日志、请求标识、访问日志、panic 恢复、统一错误响应、存活/就绪探针和优雅退出。`go test -timeout=60s ./...`、`go test -race -timeout=60s ./...` 与 `go vet ./...` 均已通过；本地开发服务默认使用 `127.0.0.1:8080`。
- Agent 测评新增“接口测试 Demo”：本地订单服务 + Docker 真实 MySQL/Redis/RabbitMQ，
  本地目录 `agent测评/接口测试demo/`，服务器目录 `/data/leimingze/接口测试demo/`。
- Demo 端口：API 30051、MySQL 30052、Redis 30054、RabbitMQ 30055/管理台 30056；
  服务器 compose 位于 `/data/leimingze/docker-compose.yml`。
- 种子数据为贴近电商场景的虚构数据（用户 10001/10002，订单号 2026082300000001 起）。
- pytest 12 个用例已在服务器全量通过。
- 已生成度量与发布流程：`agent测评/接口测试demo/docs/05-测试度量与发布流程.md`，
  配套 `scripts/metrics.py`（执行/覆盖/缺陷/性能指标与六项发布门禁）和 `scripts/load_test.py`（压测）。
- 已新增 `agent测评/接口测试demo/docs/06-源码与测试学习指南.md`，按真实请求链路讲解源码，再从
  HTTP、数据副作用、系统一致性、故障恢复和发布门禁五层建立测试学习路径；现有 `load_test.py`
  尚未按 `--target-rps` 限速，固定 200 RPS 性能目标仍需补齐负载控制后验证。
- Agent Judge 论文第一阶段数据工程位于 `agent测评/论文/实验/`，已固定版本并下载 Counsel、Plan-RewardBench、AgentProcessBench、SkillTV-Bench 和 AJ-Bench DS 的最小审计数据；可复现下载器、审计器、测试与实测报告均已落盘。
- 2026-08-24 实测五个数据源的公开数量全部与官方一致；Plan-RewardBench 的 1,171 个轨迹对来自 584 个源任务 UUID，包含 1 条完全重复轨迹对，后续抽样、置信区间和误差分析必须按 UUID 分组。
- Agent Judge 保真统一数据层已生成 4,212 条样本：训练候选 1,609 条、固定评测 1,603 条、许可证待定排除 1,000 条；全部样本通过 JSON Schema、标签语义、角色与任务组泄漏校验，索引位于 `agent测评/论文/实验/data/normalized/index.json`。
- Agent Judge Prompt 阶段已完成：4,212 条源样本生成 5,383 条模型输入，输入与隐藏标签物理分离，Plan-RewardBench 每条样本生成 A/B 和 B/A 两种顺序；AgentProcessBench 输入已按官方固定提交排除 `ground_truth`；23 项自动化测试通过。
- Qwen3-4B Token 审计固定模型提交 `cdbee75f17c01a7cc42f958dc650907174af0554` 和 `transformers==4.57.6`：262,144 Token 窗口覆盖 5,376/5,383 条（99.87%），7 条超窗记录不裁剪并标记为该模型不可评测。
- Qwen3-4B 零样本全量基线已在 MetaX C500 以并发 16 完成 5,376 条 Prompt：严格解析成功 5,372 条、格式失败 4 条、无生成截断；推理脚本每批 fsync 并支持运行身份校验后的断点续跑，评分报告位于 `agent测评/论文/实验/reports/zero_shot_full.md`。
- Agent Judge 核心贡献验证设计已写入 `agent测评/论文/实验/核心贡献验证实验设计.md`；Counsel 固定划分为 791/170/170 条 train/dev/test，SkillTV evolution 为 388/90 条 train/dev，全部按 `group_id` 隔离。
- Qwen3-4B 训练上下文已冻结为 65,536 Token：覆盖 Counsel train 100%，覆盖 SkillTV train 379/388 条和 29/29 个任务组；9 条超长训练记录有显式排除清单。
- MetaX 长轨迹 LoRA 链路已打通：使用 flash-attn 2.6.3、最后 8 层 attention LoRA 与目标尾部 logits，65,513 Token 实测完成反向更新，峰值显存约 26.9 GB；完整 masked loss 与尾部 logits loss 实测绝对差为 0。
- 四条件种子 11 的 10 步冒烟训练与 26 条四域 LoRA 推理均完成且严格解析成功；10 步输出与基础模型相同，50 步 `2e-4` 适配器已证实能改变 logits，但当前样本和步数不足以判断负迁移。
- 种子 11 的四条件 300 步训练与 12 个开发集检查点评测已完成，冻结规则选择 300 步；完整报告位于 `agent测评/论文/实验/reports/core_prevalidation_seed11.md/json`。
- 300 步自然混合相对同步数单源的 Counsel/SkillTV Macro-F1 差值为 `-0.0322/+0.0534`，来源均衡混合为 `-0.0442/-0.0105`；但来源曝光不等且多个条件发生单类预测塌缩，当前不能证明异构监督负迁移。
- 已实现源内标签均衡、预测分布与混淆矩阵、逐来源样本/Token/标签曝光审计，以及 `assistant/decision/hybrid` 三种损失模式；本地 67 项测试通过。
- SkillTV 标签均衡门禁三检查点已完成，最佳 step-50 仅复现零样本 Macro-F1 `0.3716`，step-100/150 又接近全预测 `pass`，因此标签均衡不足以解决该来源的类别塌缩。
- Counsel 标签均衡 step-67 在约 201 条曝光时恢复三类预测，Macro-F1 从同曝光旧基线 `0.2201` 提升到 `0.3474`，`Incorrect F1` 从 `0` 提升到 `0.4156`；step-100 仍在评测。
- 决策 Token 全量审计覆盖 1,170 条 eligible 训练记录和五个来源标签，确认输入 Token 与原编码逐条一致，所有标签监督位置非空且稳定为 1 或 2 个 Token。
- SkillTV 纯 `decision` 损失的 step-50/75/100/150 开发集 Macro-F1 为 `0.3350/0.2764/0.2645/0.2645`，最终 fail F1 为 `0` 且有 11 条 Markdown 代码围栏导致的严格解析失败；该方法已作为失败消融归档，不得作为候选创新点。
- 已实现无手调权重的 `hybrid` 损失：完整 JSON Token 保持监督，标签 Token 总权重自动等于其余格式 Token 总权重；真实 GPU 尾部等价性和两个单源门禁仍待队列验证。
- `reference_matched` 日程已验证混合训练中两个来源各 300 条样本的 Prompt 子序列和 Token 总量逐条等于对应单源日程；每个 12 微样本更新固定覆盖 Counsel 三类各 2 条和 SkillTV 两类各 3 条。
- SOTA 基线调研与新实验设计已归档至 `memory/research/2026-08-25-agent-judge-sota-baseline-review.md` 和 `agent测评/论文/实验/SOTA基线复现与优化设计.md`；WebJudge-7B 固定提交已在服务器通过镜像源以 16 并发下载。

## 当前资源与环境

记录服务器、本地环境、依赖版本、运行条件、容量判断和限制。

如果项目不涉及部署或服务运行，可以省略本节。

- SQL 学习课程统一存放在 `sql课程/`，其中包含教程 HTML、配套资源目录和 `sql.md`。
- Agent 测评笔记目录：`agent测评/`，已建立九阶段面试学习路线；已完成软件测试流程和传统测试度量两篇指南；Anthropic《Demystifying evals for AI agents》已加入阶段 2 资料（本地存档 `agent测评/原始资料/anthropic-agent-evals/`）。下一篇聚焦 Agent 的随机性与版本判断，再进入端到端 Agent 测评实战。
- MySQL 八股学习目录：`mysql/`，包含 README 和十份阅读型 notebook（`mysql/notebooks/`）。每份只保留“八股问答、面试话术、自查清单”两个 Markdown 单元格，无自建练习单元格、代码格、执行输出或内核依赖。基础篇“增删改查”一节只保留查询关键字逻辑执行顺序，以及通过 LeetCode SQL 题补充练习的概括说明，不指定题号或题名；其后新增“数据库三大设计范式”，包含 1NF、2NF、3NF 的判断标准、拆表示例和反范式取舍；1NF 明确为字段原子性，不是“看到多个值就一定拆表”，单值字段直接保存，只有一实体多同类值时才拆子表，并补充不满足各范式时的查询、更新、插入、删除异常；2NF 使用学生/课程/选课表的联合主键例子说明部分依赖。`08-存储引擎.ipynb` 已删除与 0.1 重复的“0.2 InnoDB 逻辑存储结构”节，只保留“0.1 存储划分与查询检索”，并新增 InnoDB / MyISAM / Memory 十维度对比表。生成脚本为 `mysql/scripts/build_notebooks.py`；`10-SQL优化.md` 是当前同名 Markdown 源，其他 Notebook 在源缺失时从现有 notebook 恢复讲解。原练习源、SQL 文件和项目 SQL 内核已删除。
- MySQL `02-索引.ipynb` 现有 10 节说明：第 1 节保留主键索引、唯一索引、常规索引、联合索引、全文索引五类索引分类表格，并补充 InnoDB 按存储形式划分的聚簇索引、二级索引表格，表格下方引用 `mysql/notebooks/assets/index-clustered-secondary.png`，图下说明“为什么叫二级”的查询链路 `name 索引 → id → 聚簇索引 → 完整行`，含义列明确两者底层都是 B+ 树、区别在叶子节点存储内容，不再列出按实现或按用途的完整分类；第 2 节“B+ 树的结构”依次引用 `mysql/notebooks/assets/b-plus-tree-structure.png` 和 `mysql/notebooks/assets/b-plus-tree-height-calculation.png`；第 3 节仅简述 Hash 索引的等值比较、范围查询、排序和检索效率特点；第 6 节覆盖索引引用 `mysql/notebooks/assets/covering-index-example.png`，注释按聚簇索引查询、覆盖索引查询、缺 `gender` 回表三类查询对比，并新增 `6.1 前缀索引`，引用 `mysql/notebooks/assets/prefix-index-example.png`，以 `email(5)` 示例说明前缀索引与回表；第 7 节最左前缀原则新增 `idx(a,b,c)` 的简单 WHERE 条件示例表，并注明换条件顺序无效、需改等值条件或改为 `(a,c,b)` 索引；第 8 节索引失效精简为“不要在索引上运算”，配 `SUBSTR(name, 1, 3)`、字符串不加引号隐式转换、尾部模糊 `LIKE 'abc%'`、头部模糊 `LIKE '%abc'` 与 `OR` 连接非索引列五个示例，并注明 OR 两侧列都有索引才可能继续用索引、MySQL 评估走索引比全表慢时也不会使用索引；第 9 节适合建索引、不适合建索引和设计建议的每条均补了简单示例。
- MySQL `09-性能优化与EXPLAIN.ipynb` 八股问答新增第 7 节“SQL 执行频次、慢查询日志、Profile 和 EXPLAIN 怎么配合？”，包含四类工具对比、配合顺序，以及从 `Com_select/Com_insert`、慢查询日志、Profile 到 EXPLAIN 的完整排查示例；第 2 节 EXPLAIN 列表保留 id、type、possible_keys、key、key_len、Extra，不包含 rows、filtered、ref，type 关注点注明 system 访问系统表、const 主键/唯一索引。
- MySQL `10-SQL优化.ipynb` 为新增阅读型 Notebook，Markdown 源为 `mysql/10-SQL优化.md`，覆盖插入数据（批量 INSERT、ON DUPLICATE、LOAD DATA）、主键设计（含页分裂与页合并）、ORDER BY、GROUP BY、LIMIT、UPDATE 六个优化主题；生成器已支持 Markdown 源没有 `## 实操` 时在 `## 面试话术` 前切分讲解。
- MySQL `02-索引.ipynb` 在二级索引/回表部分固定补充“二级索引叶子节点一定包含主键吗？”：常规表叶子保存“联合索引字段 + 主键字段”，无主键时可能使用 `NOT NULL UNIQUE` 聚簇键或隐藏行 ID，因此实际保存的是聚簇索引键。`mysql/scripts/test_build_notebooks.py` 7 项测试通过。
- MySQL `04-锁.ipynb` 的全局锁讲解明确 FTWRL 后查询可继续，写 DML、DDL 和已更新事务的 `COMMIT` 会阻塞；以库存、订单与订单日志的全库逻辑备份为例说明一致性，配图位于 `mysql/notebooks/assets/global-lock-backup-consistency.png`。表级锁按表锁、MDL、意向锁、AUTO-INC 自增锁分类，表锁再区分表共享读锁和表独占写锁。
- MySQL `10-SQL优化.md` 的 ORDER BY 小节补充“Using filesort 是什么”：解释 `EXPLAIN.Extra` 提示、sort buffer 与临时文件、常见触发原因及是否值得优化。
- Redis 八股学习目录：`redis/`，包含 `README.md`、`sources/` 六份 Markdown 源和 `notebooks/` 六份阅读型 `.ipynb`（基础、数据结构、持久化、功能与淘汰、高可用、缓存），由 `redis/scripts/build_notebooks.py` 生成；`redis/scripts/test_build_notebooks.py` 两项测试通过。
- RedSkill CLI `0.1.0` 仍安装在 `/Users/leimingze/.local/bin/redskill`，配置位于 `/Users/leimingze/.redskill`；`offerlens-interview-data@1.0.0` 已安装到当前目录 `/Users/leimingze/notes/skills/offerlens-interview-data`，`offerlens-jobs` 已从 GitHub clone 到 `/Users/leimingze/notes/skills/offerlens-jobs` 并建立 `.venv` 安装依赖，两个技能不再安装在 Codex 全局。RedSkill 下发的 interview-data 包带一层多余同名目录，已提升为顶层 `SKILL.md`。
- `OFFERLENS_API_KEY` 已写入 `/Users/leimingze/.zshenv`（权限 600），并通过 `launchctl setenv` 配置到当前用户会话；项目记忆不保存 Key 明文。
- OfferLens 面经 API（`https://skill.mnls.cloud/api/v1/interviews`）未公开底层数据来源，官网仅声明“持续更新的真实面经库、每日更新”，API 不返回原始来源 ID 或链接；使用时应视为第三方面经证据，而不是公司官方题库或可验证的原始出处。
- dsh 使用源码安装：`/Users/leimingze/agent-project/deepseek-harness`，当前 `master` 提交 `b150a551b8`，版本 `0.1.1-rc.2`。
- dsh-TUI 使用源码安装：`/Users/leimingze/agent-project/dsh-TUI`，当前 `main` 提交 `4c1d09f12596`，包版本 `0.9.3`；命令与 `dsh-tui` profile 均链接该源码目录。
- 服务器 `120.48.147.164`：根分区 `/` 100% 满，Docker 数据与 demo 均放在 `/data`；
  containerd 根目录已迁移到 `/data/containerd`；mysql8 容器已重建（端口 13306，数据 `/data/mysql/data`）。
- 服务器 Python 无 `python3-venv`，虚拟环境用 `/root/.local/bin/uv` 创建。
- Agent Judge 显存服务器为 `root@10.251.1.121:32416`，有 1 张 MetaX C500（64 GB）、驱动 3.0.11 和 MACA 3.5.3.20；`/opt/conda/envs/qwen3` 已验证沐曦版 `torch 2.8.0+metax3.5.3.9` 可识别 GPU 和 BF16，并包含 transformers 4.57.6、vLLM 0.15.0、accelerate 1.13.0 及已激活的 `vllm_metax` 插件，服务器任务统一使用 `/opt/conda/bin/conda run -n qwen3`。
- 服务器 `qwen3` 环境已新增 `peft 0.17.1`，并确认 `flash-attn 2.6.3` 可用于 Qwen3 训练；普通 SDPA 未编译 memory-efficient attention，32K 会申请约 127.5 GiB，禁止用于正式长轨迹训练。

## 架构方向

记录系统拓扑、模块边界、核心依赖、数据流、调用链路和后续演进方向。

- Agent Judge 数据统一只统一 `task`、`trajectory`、`evaluation`、`evidence` 与 `source` 容器，四类监督标签保持独立；所有数据划分以 `source.group_id` 为最小隔离单位，依据 `memory/decisions/ADR-0001-agent-judge-unified-data-schema.md` 执行。
- `PulseFrame` 采用单仓库模块化架构：普通业务先由核心 API 承载，事件 Worker、直播网关和内容安全 Agent 按运行特征预留独立部署边界。初期不为目录美观拆成全量微服务，后续拆分必须由容量测试或故障证据驱动；治理基线覆盖身份传递、追踪、超时取消、重试幂等、错误与事件契约、数据归属、健康退出、可观测性和安全审计。
- `PulseFrame` 核心 API 使用 Gin 处理 HTTP 路由、中间件、参数解析和响应；Gin 仅限服务启动层与业务模块的 HTTP 适配层，Application 和 Domain 不得依赖 `gin.Context`。Worker 不使用 Gin，直播网关、跨进程通信和内容安全 Agent 后续单独选型。

## 产品与体验方向

记录 UI、交互、用户路径、品牌、输出形态和体验原则。

如果项目不是产品或前端项目，可以省略本节。

## 安全边界

记录权限、认证、密钥、数据隔离、外部访问、危险能力和禁止事项。

## 记忆管理策略

记录本项目如何使用 `memory/`，以及哪些记忆文件是最高依据。

这里不要重复写完整协议，只保留项目内的执行规则和指向：

- 记忆协议：`memory/MEMORY_PROTOCOL.md`
- 稳定事实：`memory/PROJECT_MEMORY.md`
- 会话记录：`memory/session-notes/`
- 决策记录：`memory/decisions/`
- 调研记录：`memory/research/`
- 重要产物：`memory/artifacts/`

## 开放问题

记录还没有定论、需要用户确认、需要实验验证或后续设计的问题。

- 格式-决策等质量 `hybrid` 损失能否在 SkillTV 恢复稳定的 `fail` 类判别、并保持 Counsel 已由标签均衡获得的三类收益，决定是否进入逐来源参考匹配混合训练；纯 `decision` 损失已失败。
- 正式对照必须同时匹配每来源样本数和真实非 padding Token；相同步数比较只能作为总预算对照，不能单独归因监督兼容性。
- 当前尚无证据证明异构监督负迁移成立；在单源基线不再塌缩、曝光匹配和多种子统计完成前不得形成该结论。
- OS-Shepherd-9B 所需 Qwen3.5 架构尚不被服务器现有 `transformers 4.57.6` 和 `vLLM 0.15.0` 识别；需在隔离 Conda 环境验证 MetaX 兼容性，不得破坏现有 Qwen3 实验环境。

## 来源参考

记录支撑当前项目记忆的重要文档、链接、仓库路径、调研来源和关联记录。

- `memory/session-notes/2026-08-21-dsh-source-update.md`
- `memory/session-notes/2026-08-23-api-test-demo.md`
- `memory/session-notes/2026-08-26-mysql-storage-notebook-simplification.md`
- `memory/session-notes/2026-08-25-anthropic-evals-learning-plan.md`
- `memory/session-notes/2026-08-25-mysql-八股学习目录.md`
- `memory/artifacts/2026-08-19-agent-eval-notes.md`
- `memory/artifacts/2026-08-23-api-test-demo.md`
- `memory/artifacts/2026-08-24-agent-eval-paper-direction.md`
- `memory/research/2026-08-24-agent-eval-dataset-usability-review.md`
- `memory/decisions/ADR-0001-agent-judge-unified-data-schema.md`
- `memory/decisions/ADR-0002-agent-judge-context-overflow-policy.md`
- `实习项目/PulseFrame/PulseFrame项目总纲.md`
- `实习项目/PulseFrame/设计文档/01-总体架构与目录设计.md`
- `实习项目/PulseFrame/设计文档/02-工程基础设计.md`
