"""
脚本功能：
实现 RabbitMQ 发布与 outbox 后台任务，把待发送事件发送到消息队列。

启动命令：
由 app.main 在服务启动时以后台线程运行，无需单独执行。
"""

import json
import logging
import threading
import time

import pika
from redis import Redis
from sqlalchemy import select
from sqlalchemy.orm import Session, sessionmaker

from app.models import OutboxEvent, OutboxStatus, utc_now


# ==================== 常量与配置 ====================

LOGGER = logging.getLogger(__name__)

EVENT_EXCHANGE = "order.events"
CANCEL_QUEUE = "order.cancel.events"
CANCEL_ROUTING_KEY = "order.cancelled"
POLL_INTERVAL_SECONDS = 1
BATCH_SIZE = 10
MQ_FAULT_KEY = "fault:mq_publish"


# ==================== 外部服务与接口调用 ====================


def publish_event(rabbitmq_url: str, routing_key: str, payload: str) -> None:
    """
    输入：rabbitmq_url，AMQP 连接串；routing_key，路由键；payload，消息体 JSON 字符串。
    输出：发布持久化消息到队列；连接或发布失败时抛出异常。
    功能：声明交换机、队列与绑定关系，并以 publisher confirm 方式发送取消事件。
    """
    connection = pika.BlockingConnection(pika.URLParameters(rabbitmq_url))
    try:
        channel = connection.channel()
        channel.confirm_delivery()
        channel.exchange_declare(exchange=EVENT_EXCHANGE, exchange_type="topic", durable=True)
        channel.queue_declare(queue=CANCEL_QUEUE, durable=True)
        channel.queue_bind(queue=CANCEL_QUEUE, exchange=EVENT_EXCHANGE, routing_key=CANCEL_ROUTING_KEY)
        channel.basic_publish(
            exchange=EVENT_EXCHANGE,
            routing_key=routing_key,
            body=payload.encode("utf-8"),
            properties=pika.BasicProperties(delivery_mode=2),
        )
    finally:
        connection.close()


class OutboxWorker:
    """
    输入：session_factory，数据库会话工厂；cache，Redis 客户端；
        rabbitmq_url，AMQP 连接串；fault_key，MQ 故障注入开关键。
    输出：以后台线程轮询 outbox 表并发送待发送事件。
    功能：实现事务性发件箱模式，保证消息最终进入 RabbitMQ。
    """

    def __init__(
        self,
        session_factory: sessionmaker[Session],
        cache: Redis,
        rabbitmq_url: str,
        fault_key: str = MQ_FAULT_KEY,
    ):
        self._session_factory = session_factory
        self._cache = cache
        self._rabbitmq_url = rabbitmq_url
        self._fault_key = fault_key
        self._stop_event = threading.Event()
        self._thread = threading.Thread(target=self._run_loop, name="outbox-worker", daemon=True)

    def start(self) -> None:
        """启动后台线程；重复启动时无副作用。"""
        if not self._thread.is_alive():
            self._thread.start()

    def stop(self) -> None:
        """设置停止标记并等待线程结束。"""
        self._stop_event.set()
        self._thread.join(timeout=5)

    def _run_loop(self) -> None:
        """后台线程主循环，按固定间隔轮询待发送事件。"""
        while not self._stop_event.is_set():
            try:
                self.publish_pending()
            except Exception:
                LOGGER.exception("outbox 轮询失败，稍后重试")
            time.sleep(POLL_INTERVAL_SECONDS)

    def publish_pending(self) -> int:
        """
        输入：无。
        输出：本次处理的待发送事件数量；发布失败的事件保留 PENDING 并累加尝试次数。
        功能：读取并发送一批待发送事件，发送成功后把状态更新为 SENT。
        """
        if self._mq_fault_enabled():
            LOGGER.warning("MQ 故障注入开关已打开，本次跳过消息发送")
            return 0
        session = self._session_factory()
        try:
            events = session.scalars(
                select(OutboxEvent)
                .where(OutboxEvent.status == OutboxStatus.PENDING.value)
                .order_by(OutboxEvent.id)
                .limit(BATCH_SIZE)
            ).all()
            for event in events:
                self._publish_one(session, event)
            session.commit()
            return len(events)
        finally:
            session.close()

    def _publish_one(self, session: Session, event: OutboxEvent) -> None:
        """
        输入：session，数据库会话；event，待发送事件。
        输出：发送成功则把事件标记为 SENT；失败则累加 attempt_count 并记录异常。
        功能：发送单个取消事件，并在同一次会话中更新发送状态。
        """
        try:
            payload = json.loads(event.payload)
            publish_event(self._rabbitmq_url, payload["event_type"], event.payload)
            event.status = OutboxStatus.SENT.value
            event.sent_at = utc_now()
        except Exception:
            event.attempt_count += 1
            LOGGER.exception("事件 %s 发送失败，保留 PENDING 待重试", event.event_id)

    def _mq_fault_enabled(self) -> bool:
        """读取 Redis 中的故障注入开关；Redis 异常时视为开关未打开。"""
        try:
            return self._cache.get(self._fault_key) == b"1"
        except Exception:
            LOGGER.exception("读取 MQ 故障开关失败，按未打开处理")
            return False
