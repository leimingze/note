#!/usr/bin/env bash
# 脚本功能：
# 手工走一遍取消订单接口的核心用例，覆盖成功、越权、状态拒绝、幂等、故障注入与 MQ 恢复。
#
# 启动命令：
# 先启动服务并完成种子数据，再执行 `bash scripts/manual_requests.sh`。

BASE_URL="${API_BASE_URL:-http://127.0.0.1:30051}"
USER="10001"
OTHER="10002"

get_order_id() {
  # 输入：order_no，订单号。输出：该订单在 MySQL 中的 id。功能：动态获取订单 id，避免硬编码。
  docker exec api-demo-mysql mysql -N -udemo -pdemo_pass order_demo \
    -e "SELECT id FROM orders WHERE order_no='$1' LIMIT 1;" 2>/dev/null
}

ID_PENDING_PAYMENT=$(get_order_id 2026082300000001)
ID_PENDING_SHIPMENT=$(get_order_id 2026082300000002)
ID_SHIPPED=$(get_order_id 2026082300000003)

echo "== 1. 健康检查 =="
curl -sS "${BASE_URL}/api/health"
echo

echo "== 2. 查询待支付订单（id=${ID_PENDING_PAYMENT}）=="
curl -sS -H "X-User-Id: ${USER}" "${BASE_URL}/api/orders/${ID_PENDING_PAYMENT}"
echo

echo "== 3. 正常取消待支付订单（id=${ID_PENDING_PAYMENT}）=="
curl -sS -X POST -H "X-User-Id: ${USER}" -H "X-Idempotency-Key: demo-cancel-1" \
  "${BASE_URL}/api/orders/${ID_PENDING_PAYMENT}/cancel"
echo

echo "== 4. 未登录取消（应 401）=="
curl -sS -X POST "${BASE_URL}/api/orders/${ID_PENDING_SHIPMENT}/cancel"
echo

echo "== 5. 越权取消他人订单（应 404）=="
curl -sS -X POST -H "X-User-Id: ${OTHER}" "${BASE_URL}/api/orders/${ID_PENDING_SHIPMENT}/cancel"
echo

echo "== 6. 取消已发货订单（应 409）=="
curl -sS -X POST -H "X-User-Id: ${USER}" "${BASE_URL}/api/orders/${ID_SHIPPED}/cancel"
echo

echo "== 7. 同一个幂等键重复取消（第二次应 success=false）=="
curl -sS -X POST -H "X-User-Id: ${USER}" -H "X-Idempotency-Key: demo-cancel-1" \
  "${BASE_URL}/api/orders/${ID_PENDING_PAYMENT}/cancel"
echo

echo "== 8. 数据库故障注入（应 500，订单不变）=="
curl -sS -X POST -H "X-User-Id: ${USER}" -H "X-Fault-Injection: db" \
  "${BASE_URL}/api/orders/${ID_PENDING_SHIPMENT}/cancel"
echo

echo "== 9. 打开 MQ 故障开关并取消待发货订单（id=${ID_PENDING_SHIPMENT}）=="
docker exec api-demo-redis redis-cli set fault:mq_publish 1
curl -sS -X POST -H "X-User-Id: ${USER}" "${BASE_URL}/api/orders/${ID_PENDING_SHIPMENT}/cancel"
echo
echo "现在检查 outbox_events：事件应为 PENDING（见下方 SQL）"

echo "== 10. 关闭 MQ 故障开关，等待后台任务发送 =="
docker exec api-demo-redis redis-cli del fault:mq_publish
sleep 3
docker exec -i api-demo-mysql mysql -udemo -pdemo_pass order_demo \
  -e "SELECT id, order_id, status, attempt_count, sent_at FROM outbox_events ORDER BY id;"

echo "== 11. 从 RabbitMQ 消费一条取消事件 =="
python scripts/consume_event.py
echo

echo
echo "完成。请对照 scripts/db_checks.sql 复核数据库状态。"
