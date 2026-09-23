# 产物：接口测试 Demo

日期：2026-08-23
类型：学习练习项目 / 测试自动化示例

## 路径

- 本地：`/Users/leimingze/notes/agent测评/接口测试demo/`
- 服务器：`/data/leimingze/接口测试demo/`，compose 在 `/data/leimingze/docker-compose.yml`
- 度量与发布流程：`docs/05-测试度量与发布流程.md`；配套 `scripts/metrics.py`、`scripts/load_test.py`
- 源码与测试学习指南：`docs/06-源码与测试学习指南.md`

## 用途

用于先沿请求链路学习源码，再走完业务规则 → 决策表 → 手工验证 → 自动化 → 故障注入 → 测试报告的完整流程。

## 状态

已部署并通过 12 个 pytest 用例；API 运行在服务器 30051。源码与测试学习指南已落盘；现有
`load_test.py` 尚未按 `--target-rps` 限速，完善负载控制前不能作为固定 200 RPS 的达标证据。
`scripts/test_results.example.json` 已改为非实测格式示例，性能默认未验证，避免示例误判发布通过。

## 关联记忆

- `memory/session-notes/2026-08-23-api-test-demo.md`

## 维护说明

- 修改服务代码后同步到服务器 `app/` 目录，并重启 30051 的 uvicorn。
- 修改 docker-compose.yml 后同时更新 `/data/leimingze/docker-compose.yml`。
