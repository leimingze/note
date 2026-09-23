"""
脚本功能：
定义订单服务的数据模型：订单、待发送事件表（outbox）、幂等记录。

启动命令：
由 app.main 在启动时通过 create_all 建表，无需单独执行。
"""

import enum
import json
from datetime import datetime, timezone
from decimal import Decimal
from uuid import uuid4

from sqlalchemy import DateTime, Enum, ForeignKey, Numeric, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


# ==================== 常量与配置 ====================


def utc_now() -> datetime:
    """返回当前 UTC 时间，作为所有时间字段的默认值。"""
    return datetime.now(timezone.utc)


class OrderStatus(str, enum.Enum):
    """订单状态枚举，与业务规则中的状态名称一一对应。"""

    PENDING_PAYMENT = "PENDING_PAYMENT"
    PENDING_SHIPMENT = "PENDING_SHIPMENT"
    SHIPPED = "SHIPPED"
    COMPLETED = "COMPLETED"
    CANCELLED = "CANCELLED"


class OutboxStatus(str, enum.Enum):
    """待发送事件状态：PENDING 表示等待发送，SENT 表示已确认发送。"""

    PENDING = "PENDING"
    SENT = "SENT"


class Base(DeclarativeBase):
    """SQLAlchemy 声明式基类。"""


class Order(Base):
    """订单表，MySQL 中保存最终可信的订单状态。"""

    __tablename__ = "orders"

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    order_no: Mapped[str] = mapped_column(String(32), unique=True, index=True)
    user_id: Mapped[str] = mapped_column(String(64), index=True)
    status: Mapped[str] = mapped_column(
        Enum(OrderStatus, values_callable=lambda x: [item.value for item in x]),
        default=OrderStatus.PENDING_PAYMENT.value,
    )
    amount: Mapped[Decimal] = mapped_column(Numeric(10, 2), default=Decimal("0.00"))
    subject: Mapped[str] = mapped_column(String(128), default="")
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=utc_now)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=utc_now, onupdate=utc_now
    )
    cancelled_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class OutboxEvent(Base):
    """待发送事件表，与订单状态在同一事务中写入，由后台任务发送到 RabbitMQ。"""

    __tablename__ = "outbox_events"

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    event_id: Mapped[str] = mapped_column(String(36), unique=True)
    order_id: Mapped[int] = mapped_column(ForeignKey("orders.id"), index=True)
    event_type: Mapped[str] = mapped_column(String(64))
    payload: Mapped[str] = mapped_column(Text)
    status: Mapped[str] = mapped_column(
        Enum(OutboxStatus, values_callable=lambda x: [item.value for item in x]),
        default=OutboxStatus.PENDING.value,
    )
    attempt_count: Mapped[int] = mapped_column(default=0)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=utc_now)
    sent_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    @classmethod
    def build_cancelled_event(cls, order: Order) -> "OutboxEvent":
        """
        输入：order，已确定要取消的订单对象。
        输出：一个新的 PENDING 状态取消事件，尚未入库。
        功能：构造取消事件，保证事件 ID 与 payload 格式统一。
        """
        payload = {
            "event_id": str(uuid4()),
            "order_id": order.id,
            "order_no": order.order_no,
            "user_id": order.user_id,
            "event_type": "order.cancelled",
            "cancelled_at": utc_now().isoformat(),
        }
        return cls(
            event_id=payload["event_id"],
            order_id=order.id,
            event_type=payload["event_type"],
            payload=json.dumps(payload, ensure_ascii=False),
        )


class IdempotencyRecord(Base):
    """幂等记录表，用幂等键识别同一次取消请求的重试。"""

    __tablename__ = "idempotency_records"

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    idempotency_key: Mapped[str] = mapped_column(String(64), unique=True)
    order_id: Mapped[int] = mapped_column(ForeignKey("orders.id"), index=True)
    result_status: Mapped[str] = mapped_column(String(32))
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=utc_now)
