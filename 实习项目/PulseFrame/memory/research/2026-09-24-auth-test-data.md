# 调研：用户认证模块测试数据

日期：2026-09-24

## 摘要

没有发现适合拿来创建账号或回放注册、登录流程的公开用户名密码数据集。公开登录事件数据可用于后续风险识别研究，但不能替代认证链路测试；测试账号必须由隔离环境生成。

## 结论

- SuspiciousLoginDataset 描述约 73,528 条匿名化 Google Workspace 成功登录事件，适用方向是可疑登录行为分析，不包含可用于本项目的用户凭据。项目 README 声明 CC BY-NC 4.0，但 GitHub 仓库许可元数据为 NOASSERTION；用于研究前仍需核实许可及数据字段，不应在首版认证模块中下载或导入。
- Have I Been Pwned Pwned Passwords 提供泄露密码检查 API，不是用户账号或登录行为数据集。它不能证明认证流程正确，也不适合作为测试账号密码来源；当前模块不需要调用该服务。
- 前端组件和 API 客户端测试使用临时合成账号及受控 HTTP 响应；后端使用应用测试替身、SQL mock、miniredis 与 HTTP 测试。
- 首轮真实集成在专用 `pulseframe` MySQL 数据库、独立最小权限 MySQL 账号和 Redis `pulseframe:auth:*` ACL 键前缀中执行，并通过 SSH 隧道连接；注册、登录、本人信息、CSRF 退出及会话撤销均通过。测试创建的两条合成账号已删除，未导入公开数据集。
- 合成账号只用于可复现的功能与边界测试，不代表真实用户分布。若后续进行风控模型或登录行为负载研究，可单独评估事件数据的字段、许可、隐私和适用性。

## 来源

- SuspiciousLoginDataset 仓库与 README：https://github.com/salomaopena/SuspiciousLoginDataset
- Pwned Passwords API 文档：https://haveibeenpwned.com/API/v3#PwnedPasswords
- Unsplash License：https://unsplash.com/license（仅适用于网页视觉图片，不是测试数据来源）

## 风险与边界

- SuspiciousLoginDataset 的 README 声明与 GitHub 许可元数据不一致，不能据此将数据直接纳入项目或分发。
- 泄露密码清单属于高风险凭据材料，不应保存、导入账号表或用于成功登录测试。
- 本次使用共享远端服务，但隔离到 PulseFrame 专用数据库、账号和 Redis 键前缀；这不能替代专用测试实例的故障和容量验证。
- 远端 MySQL/Redis 映射端口的公网访问策略尚未核查或收紧；持续使用前需限制来源地址并保留 SSH 隧道接入方式。

## 建议

首版以合成测试身份验证账号规则、哈希存储、会话、CSRF、限流与失败路径；首轮真实存储联调已完成。后续继续使用专用测试数据边界，并补充依赖故障与容量验证；公开事件数据留给后续风控或行为负载研究评估。
