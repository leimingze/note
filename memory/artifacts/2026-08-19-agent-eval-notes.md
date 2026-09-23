# 产物：Agent 测评学习笔记框架与原始资料

日期：2026-08-19
类型：学习笔记 / README 框架 / 原始资料归档

## 路径

- `agent测评/README.md`：Agent 测评学习笔记入口与模板
- `agent测评/学习指南/01-软件测试基础.md`：以取消订单案例串联需求评审、测试设计、执行、发布与复盘
- `agent测评/学习指南/02-软件测试度量与发布判断.md`：传统软件测试的覆盖、执行、缺陷、性能、可靠性和线上质量指标
- `agent测评/原始资料/`：已下载的书籍配套资料、翻译版、论文和公众号文章转载

## 用途

作为 Agent 评测主题的笔记目录入口，统一整理以下来源：

- 《AI Engineering》→ `ai-engineering/`（官方配套仓库 chiphuyen/aie-book）
- 《AI Agents in Action》→ `ai-agents-in-action/`（中文翻译版 2nd edition）
- 《Agentic Design Patterns》→ `agentic-design-patterns/`（中英双语仓库）
- VitaBench 论文 → `vitabench/`（arXiv PDF + 摘要页）
- 腾讯《AI Agent & Skill 测评方案及落地实践》→ `tencent-agent-skill/tencent-agent-skill.html`（微信原文）
- 阿里《Agent 评测：方法论与体系设计》→ `alibaba-agent-eval/alibaba-agent-eval.html`（微信原文）
- 美团《Agent评测漫谈 —— 由浅入深讲解Agent评测》→ `meituan-agent-eval/meituan-agent-eval.html`（微信原文）
- 得物《得物推荐系统诊断 Agent：从 “调接口” 到 “会思考”》→ 按用户要求不再需要，已移除

## 状态

已按用户要求移除 CSDN 转载，并下载腾讯、阿里、美团三篇微信原文；得物不再需要。微信文章中的图片已下载到各目录 `images/` 并改写为本地引用。

学习指南已完成软件测试流程与传统测试度量两篇。软件测试基础指南按需求评审、测试设计、开发准备、提测准入、执行、回归、发布和上线复盘组织；测试设计先演示从一条规则生成一条完整用例，再使用标准测试技术和技术风险扩展为完整测试集。下一篇为“Agent 评测中的随机性与版本判断”，只处理 Agent 重复运行、评分器校准和版本差异判断，不重复传统软件测试指标；之后再编写端到端 Agent 测评实战。

## 维护说明

后续阅读时按 README 中的“单篇资料阅读模板”填充；学习指南的实时执行顺序以 `agent测评/TODO.csv` 为准。如需微信原文，再手动导出替换转载版。
