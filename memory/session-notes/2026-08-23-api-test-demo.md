# 会话记录：接口测试 Demo 全量部署

日期：2026-08-23

## 背景

用户学完《软件测试基础》指南后，要求做一个“真实的全量 demo”并真正走一遍流程。
最终确认：本地订单服务 + Docker 真实 MySQL/Redis/RabbitMQ，复刻指南中的取消订单场景。

## 用户目标

- 被测接口：`POST /api/orders/{order_id}/cancel`，配套 `GET /api/orders/{order_id}`。
- 种子数据要贴近真实电商场景，但全部为虚构数据。
- 部署到服务器 `120.48.147.164` 的 `/data/leimingze/接口测试demo`。

## 相关上下文

- 服务器根分区 `/` 100% 满（40G），`/data` 有 425G 空闲，Docker 数据目录已配置在 `/data/docker`。
- Docker 使用 containerd snapshotter，解压镜像时写 `/var/lib/containerd`（根分区），导致拉镜像报“no space left”。
- 服务器 30050 被 mbti-server.js 占用、30053 被既有 uvicorn 服务占用。

## 决策

- 端口：API 30051、MySQL 30052、Redis 30054、RabbitMQ AMQP 30055、RabbitMQ 管理台 30056。
- 服务器 compose 文件：`/data/leimingze/docker-compose.yml`，与 demo 项目内 docker-compose.yml 内容一致。
- 迁移 containerd 根目录到 `/data/containerd`（修改 `/etc/containerd/config.toml`），备份在 `/data/leimingze/containerd-config.toml.bak`。
- mysql8 容器重建：原容器删除后遗留孤儿 mysqld 进程占用 `/data/mysql/data`，已优雅停止并重建；
  备份在 `/data/leimingze/mysql8-backup.json`，数据卷 `/data/mysql/data` 未丢失，端口 13306 恢复。

## 证据

- pytest 全量 12 个用例通过，覆盖成功、鉴权、越权、状态拒绝、幂等、并发、数据库回滚、缓存失效、MQ 发送与故障恢复。
- 种子数据 6 笔订单已入库（order_id 因测试自增从 90 开始，手工脚本已改为按订单号动态查询 id）。
- 容器：api-demo-mysql / api-demo-redis / api-demo-rabbitmq 全部 healthy；mysql8 healthy。
- API 健康检查：`http://127.0.0.1:30051/api/health` 返回 `{"status":"ok"}`。
- 2026-08-24 新增度量与发布流程：`docs/05-测试度量与发布流程.md`、
  `scripts/metrics.py`（指标与门禁计算）、`scripts/load_test.py`（RPS/P95/P99/错误率压测）；
  示例结果文件 `scripts/test_results.example.json` 六项门禁全绿。

## 踩坑记录（后续不要重复）

- 重启进程或搜索 uvicorn 时，pkill 模式必须带端口，例如
  `pkill -f 'uvicorn app.main:app --host 0.0.0.0 --port 30051'`；
  曾因不带端口误杀服务器上 30053 的既有服务（已被自动拉起）。
- pytest 断言外部提交的数据时，不要复用创建订单的会话：MySQL 默认 REPEATABLE READ 快照会读到旧值。
  使用独立新会话 + `isolation_level="READ COMMITTED"` + `NullPool`。
- 并发线程内不要访问 SQLAlchemy ORM 对象属性（会触发懒加载并共享会话冲突），先在线程外取出 id。
- Python 环境缺 `python3-venv`，服务器上改用 `/root/.local/bin/uv` 创建虚拟环境；临时目录和 pip/uv 缓存都指向 `/data`。
- FastAPI 的 HTTPException detail 会包一层 `detail`，测试断言用 `response.json()["detail"]["code"]`。

## 结果

- demo 已部署并自测通过，用户可按 README 的 6 步流程手动走一遍。
- API 以 nohup 后台运行在 30051；中间件由 `docker compose` 管理。

## 后续动作

- 旧的 `/var/lib/containerd`（约 1.2G 半成品镜像）迁移后不再使用，删除需用户确认。
- 服务器根分区仍 100% 满，后续可清理 `/root/.cache/uv`（8.4G）等缓存，需用户确认。
- 手工脚本 `scripts/manual_requests.sh` 会消费真实事件并修改订单状态，重跑前建议重新 `python -m app.seed`。

## 需要同步到长期记忆的内容

- demo 目录、端口、部署位置和运行方式（见 `memory/PROJECT_MEMORY.md`）。
