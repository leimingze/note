"""
脚本功能：
验证取消事件进入 RabbitMQ，以及 MQ 故障注入后的暂停与恢复。

启动命令：
在项目根目录运行 `pytest tests/test_mq_fault.py -v`。
"""

import json

import pika
from sqlalchemy import func, select

from app.models import Order, OrderStatus, OutboxEvent, OutboxStatus

from tests.conftest import wait_until


def _event(db, order_id: int) -> OutboxEvent:
    """输入：db，数据库会话；order_id。输出：该订单的取消事件。功能：读取事件用于状态断言。"""
    stmt = select(OutboxEvent).where(OutboxEvent.order_id == order_id)
    return db.scalar(stmt)


def _purge_queue(rabbitmq_url: str) -> None:
    """输入：rabbitmq_url，AMQP 连接串。输出：清空取消事件队列。功能：保证测试只读到本次消息。"""
    connection = pika.BlockingConnection(pika.URLParameters(rabbitmq_url))
    try:
        channel = connection.channel()
        channel.queue_declare(queue="order.cancel.events", durable=True)
        channel.queue_purge(queue="order.cancel.events")
    finally:
        connection.close()


def _consume_one(rabbitmq_url: str) -> dict | None:
    """输入：rabbitmq_url。输出：队列中的一条消息体；无消息时返回 None。功能：断言事件已进入 MQ。"""
    connection = pika.BlockingConnection(pika.URLParameters(rabbitmq_url))
    try:
        channel = connection.channel()
        method, _, body = channel.basic_get(queue="order.cancel.events", auto_ack=True)
        if method is None:
            return None
        return json.loads(body)
    finally:
        connection.close()


def test_outbox_event_sent_to_rabbitmq(db_session, api, redis_client, settings, create_order):
    """取消后事件最终发送到 RabbitMQ，且消息体包含订单信息。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_SHIPMENT)
    _purge_queue(settings.rabbitmq_url)

    assert api["cancel"](order.id).status_code == 200
    wait_until(
        lambda: _event(db_session, order.id).status == OutboxStatus.SENT.value,
        timeout=15,
        message="取消事件未在 15 秒内发送到 RabbitMQ",
    )
    message = _consume_one(settings.rabbitmq_url)
    assert message is not None
    assert message["order_id"] == order.id
    assert message["event_type"] == "order.cancelled"


def test_mq_fault_keeps_event_pending_and_recovers(db_session, api, redis_client, settings, create_order, fresh_db):
    """MQ 故障注入：事件保持 PENDING；关闭开关后事件最终发送。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_SHIPMENT)
    _purge_queue(settings.rabbitmq_url)
    redis_client.set("fault:mq_publish", "1")

    assert api["cancel"](order.id).status_code == 200
    wait_until(lambda: _event(db_session, order.id) is not None, timeout=5, message="事件未写入 outbox")
    with fresh_db() as db:
        assert db.get(Order, order.id).status == OrderStatus.CANCELLED.value
    assert _event(db_session, order.id).status == OutboxStatus.PENDING.value

    redis_client.delete("fault:mq_publish")
    wait_until(
        lambda: _event(db_session, order.id).status == OutboxStatus.SENT.value,
        timeout=15,
        message="关闭故障开关后事件未恢复发送",
    )
    assert _consume_one(settings.rabbitmq_url) is not None
