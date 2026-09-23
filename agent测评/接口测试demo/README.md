# 接口测试 Demo：取消订单

这是一个完整的接口测试练习项目，用于走完《软件测试基础》学习指南第 10 章“第二遍”的全部流程：
业务规则 → 用例设计 → 手工验证 → 自动化 → 故障注入 → 测试报告。

项目使用真实中间件：FastAPI 订单服务 + MySQL 8 + Redis 7 + RabbitMQ 3，
复刻指南中的“取消订单”场景，被测接口为 `POST /api/orders/{order_id}/cancel`。

## 系统链路

```text
用户发起取消
→ 校验 X-User-Id 与订单归属
→ MySQL 同一事务：订单状态改 CANCELLED + 写入待发送事件
→ 事务提交
  ├→ 当前请求删除 Redis 订单缓存并返回真实结果
  └→ 后台任务轮询 outbox，把事件发送到 RabbitMQ
```

## 目录结构

```text
接口测试demo/
├── docker-compose.yml          # MySQL、Redis、RabbitMQ
├── app/                        # 订单服务代码
│   ├── main.py                 # 应用入口
│   ├── models.py               # 订单、outbox、幂等记录
│   ├── services/order_service.py  # 取消订单核心逻辑
│   ├── mq.py                   # outbox 后台任务
│   ├── seed.py                 # 种子数据
│   └── api/orders.py           # 查询与取消接口
├── tests/                      # pytest 集成测试
├── scripts/                    # 手工验证、数据库检查、压测与指标计算
│   ├── manual_requests.sh
│   ├── metrics.py
│   └── load_test.py
├── docs/
│   ├── 00-设计方案.md
│   ├── 01-业务规则与风险.md
│   ├── 02-接口契约.md
│   ├── 03-决策表与用例设计.md
│   ├── 04-测试报告模板.md
│   ├── 05-测试度量与发布流程.md
│   └── 06-源码与测试学习指南.md
└── README.md
```

## 前置条件

- Python 3.10+
- Docker Desktop（需启动 Docker daemon）
- 端口 30051（API）、30052（MySQL）、30054（Redis）、30055（RabbitMQ）、30056（RabbitMQ 管理台）未被占用

## 快速启动

```bash
cd /Users/leimingze/notes/agent测评/接口测试demo

# 1. 启动中间件（首次拉镜像需要几分钟）
docker compose up -d --wait

# 2. 创建虚拟环境并安装依赖
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# 3. 初始化种子数据（6 笔不同状态的订单）
python -m app.seed

# 4. 启动订单服务
uvicorn app.main:app --reload --port 30051
```

启动后先确认健康检查：`curl http://127.0.0.1:30051/api/health`，返回 `{"status":"ok"}`。

## 服务器部署（120.48.147.164）

服务器上已按同一套端口方案部署完成：

- 中间件 compose：`/data/leimingze/docker-compose.yml`，在 `/data/leimingze` 下执行 `docker compose up -d --wait`
- 项目代码与虚拟环境：`/data/leimingze/接口测试demo/`
- API 地址：`http://120.48.147.164:30051`
- RabbitMQ 管理台：`http://120.48.147.164:30056`（guest/guest）
- API 重启命令：

```bash
cd /data/leimingze/接口测试demo
pkill -f 'uvicorn app.main:app --host 0.0.0.0 --port 30051'
nohup .venv/bin/uvicorn app.main:app --host 0.0.0.0 --port 30051 > uvicorn.log 2>&1 &
```

## 一步一步走完流程

### 第 1 步：先读源码、规则和用例设计

先按 `docs/06-源码与测试学习指南.md` 第一部分走完请求链路，再打开
`docs/01-业务规则与风险.md` 和 `docs/03-决策表与用例设计.md`，确认每条规则如何变成用例。

### 第 2 步：手工验证核心用例

```bash
bash scripts/manual_requests.sh
```

脚本会依次演示：健康检查、查询订单、正常取消、未登录、越权、状态拒绝、
幂等重复、数据库故障注入、MQ 故障注入与恢复。每一步都有注释说明预期结果。

### 第 3 步：查数据库验证证据

取消后订单状态、待发送事件、幂等记录都在 MySQL：

```bash
docker exec -i api-demo-mysql mysql -udemo -pdemo_pass order_demo < scripts/db_checks.sql
```

Redis 缓存键检查：

```bash
docker exec api-demo-redis redis-cli keys 'order:*'
```

### 第 4 步：跑自动化测试

```bash
pytest -v
```

覆盖范围见 `docs/03-决策表与用例设计.md`，核心规则全部有自动化断言，
并包含并发取消、数据库回滚、MQ 故障恢复。

### 第 5 步：亲手制造一次故障并提交缺陷

数据库故障：

```bash
curl -X POST -H "X-User-Id: 10001" -H "X-Fault-Injection: db" \
  http://127.0.0.1:30051/api/orders/6/cancel
```

预期：返回 500；查库发现订单仍是 PENDING_PAYMENT，且没有 outbox 事件。
请把“接口返回 500 后订单是否保持原状态”写成一条可复核的缺陷记录。

MQ 故障：

```bash
docker exec api-demo-redis redis-cli set fault:mq_publish 1
docker exec api-demo-redis redis-cli del fault:mq_publish
```

打开开关后取消订单，outbox 事件保持 PENDING；关闭开关后，等待 1-3 秒应变为 SENT。
这对应指南中的“MQ 中断恢复”可靠性用例。

### 第 6 步：写测试报告

复制 `docs/04-测试报告模板.md`，按模板填写版本、范围、执行结果、缺陷和遗留风险，
最后给出发布建议。

## 常用命令速查

| 操作 | 命令 |
| --- | --- |
| 查看中间件状态 | `docker compose ps` |
| 查看订单服务日志 | `docker compose logs -f`（另开终端） |
| 重置全部数据 | `docker compose down -v` 后重新 `docker compose up -d --wait`，再 `python -m app.seed` |
| 单独重置数据表 | `pytest` 的 db_session 夹具会自动清空订单相关表 |
| 查看 RabbitMQ 队列 | `docker exec api-demo-rabbitmq rabbitmqctl list_queues` |
| 查看 RabbitMQ 管理台 | 浏览器打开 `http://127.0.0.1:30056`（guest/guest） |

## 学习路线对照

| 学习指南章节 | 本项目对应物 |
| --- | --- |
| 第 2 章 需求评审 | `docs/01-业务规则与风险.md` |
| 第 3 章 测试设计 | `docs/03-决策表与用例设计.md` |
| 第 4 章 开发阶段 | `app/`、`docker-compose.yml` |
| 第 5 章 提测准入 | 冒烟用例 C02、C05、C11 |
| 第 6 章 测试执行 | `scripts/manual_requests.sh` |
| 第 7 章 回归与专项 | `pytest -v` 全套用例 |
| 第 8 章 发布评估 | `docs/04-测试报告模板.md` |
| 度量与发布判断指南 | `docs/05-测试度量与发布流程.md`、`scripts/metrics.py`、`scripts/load_test.py` |
| 源码与测试串联学习 | `docs/06-源码与测试学习指南.md` |

## 已知设计取舍

- Redis 不可用时读取降级到数据库，由 5 秒 TTL 兜底保证最终一致；日志会记录告警。
- 登录态用 `X-User-Id` 请求头模拟，真实项目应替换为完整鉴权。
- 待支付订单取消不模拟退款流程，只发送取消事件。
