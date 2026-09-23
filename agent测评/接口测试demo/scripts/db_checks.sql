-- 数据库检查脚本：通过 docker exec 在 MySQL 容器内执行。
-- 启动命令：
--   docker exec -i api-demo-mysql mysql -udemo -pdemo_pass order_demo < scripts/db_checks.sql

SELECT id, order_no, user_id, status, amount, cancelled_at
FROM orders
ORDER BY id;

SELECT id, event_id, order_id, event_type, status, attempt_count, created_at, sent_at
FROM outbox_events
ORDER BY id;

SELECT status, COUNT(*) AS event_count
FROM outbox_events
GROUP BY status;
