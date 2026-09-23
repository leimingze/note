"""
脚本功能：
实现取消订单的核心业务逻辑：身份校验、状态校验、事务提交、幂等与缓存失效。

启动命令：
由 app/api/orders.py 调用，无需单独执行。
"""

from dataclasses import dataclass

from redis import Redis
from sqlalchemy import select, update
from sqlalchemy.orm import Session

from app.cache import invalidate_order_cache
from app.models import IdempotencyRecord, Order, OrderStatus, OutboxEvent, utc_now


# ==================== 常量与配置 ====================

ALLOW_CANCEL_STATUSES = frozenset(
    {OrderStatus.PENDING_PAYMENT.value, OrderStatus.PENDING_SHIPMENT.value}
)


class OrderNotFoundError(Exception):
    """订单不存在或不属于当前用户。"""


class OrderNotCancellableError(Exception):
    """订单当前状态不允许取消。"""

    def __init__(self, status: str):
        self.status = status
        super().__init__(f"订单状态 {status} 不允许取消")


class DBSimulationError(Exception):
    """故障注入用异常，模拟数据库提交阶段失败。"""


@dataclass(frozen=True)
class CancelResult:
    """取消结果：already_cancelled 表示本次请求未执行新的状态变更。"""

    order_id: int
    status: str
    success: bool
    already_cancelled: bool
    message: str


# ==================== 核心逻辑 ====================


def get_order_for_user(db: Session, order_id: int, user_id: str) -> Order | None:
    """
    输入：db，数据库会话；order_id，订单主键；user_id，当前用户标识。
    输出：订单对象；订单不存在或不属于该用户时返回 None。
    功能：按归属查询订单，供查询与取消接口共用同一套身份过滤。
    """
    order = db.get(Order, order_id)
    if order is None or order.user_id != user_id:
        return None
    return order


def cancel_order(
    db: Session,
    cache: Redis,
    order_id: int,
    user_id: str,
    idempotency_key: str | None,
    fault_db: bool,
) -> CancelResult:
    """
    输入：db，数据库会话；cache，Redis 客户端；order_id，订单主键；user_id，用户标识；
        idempotency_key，可选的幂等键；fault_db，是否模拟数据库提交失败。
    输出：CancelResult；越权或订单不存在时抛出 OrderNotFoundError，
        状态不允许时抛出 OrderNotCancellableError，故障注入时抛出 DBSimulationError 并回滚事务。
    功能：执行取消订单核心流程，保证订单状态与待发送事件在同一事务中提交。
    """
    existing = _find_idempotency_record(db, order_id, idempotency_key)
    if existing is not None:
        return CancelResult(
            order_id=order_id,
            status=existing.result_status,
            success=False,
            already_cancelled=True,
            message="该请求之前已处理，返回已有结果",
        )

    order = db.get(Order, order_id)
    if order is None or order.user_id != user_id:
        raise OrderNotFoundError()
    if order.status == OrderStatus.CANCELLED.value:
        return CancelResult(
            order_id=order.id,
            status=order.status,
            success=False,
            already_cancelled=True,
            message="订单已取消，无需重复操作",
        )
    if order.status not in ALLOW_CANCEL_STATUSES:
        raise OrderNotCancellableError(order.status)

    updated = _try_apply_cancellation(db, order_id, order.user_id)
    if updated != 1:
        fresh = db.get(Order, order_id)
        if fresh is None:
            raise OrderNotFoundError()
        if fresh.status == OrderStatus.CANCELLED.value:
            return CancelResult(
                order_id=order_id,
                status=fresh.status,
                success=False,
                already_cancelled=True,
                message="订单已取消，无需重复操作",
            )
        raise OrderNotCancellableError(fresh.status)

    event = OutboxEvent.build_cancelled_event(order)
    db.add(event)
    if idempotency_key:
        db.add(
            IdempotencyRecord(
                idempotency_key=idempotency_key,
                order_id=order_id,
                result_status=OrderStatus.CANCELLED.value,
            )
        )
    if fault_db:
        db.rollback()
        raise DBSimulationError()

    db.commit()
    invalidate_order_cache(cache, order_id)
    return CancelResult(
        order_id=order_id,
        status=OrderStatus.CANCELLED.value,
        success=True,
        already_cancelled=False,
        message="取消成功",
    )


# ==================== 校验与错误处理 ====================


def _find_idempotency_record(
    db: Session, order_id: int, idempotency_key: str | None
) -> IdempotencyRecord | None:
    """
    输入：db，数据库会话；order_id，订单主键；idempotency_key，幂等键或 None。
    输出：幂等记录；无幂等键或未找到时返回 None。
    功能：查询指定订单与幂等键的既有处理记录，用于重试请求直接返回已有结果。
    """
    if not idempotency_key:
        return None
    stmt = select(IdempotencyRecord).where(
        IdempotencyRecord.idempotency_key == idempotency_key,
        IdempotencyRecord.order_id == order_id,
    )
    return db.scalar(stmt)


def _try_apply_cancellation(db: Session, order_id: int, user_id: str) -> int:
    """
    输入：db，数据库会话；order_id，订单主键；user_id，用户标识。
    输出：受影响行数，0 或 1。
    功能：以条件更新方式把订单改为已取消，避免并发请求同时生效。
    """
    result = db.execute(
        update(Order)
        .where(
            Order.id == order_id,
            Order.user_id == user_id,
            Order.status.in_(ALLOW_CANCEL_STATUSES),
        )
        .values(status=OrderStatus.CANCELLED.value, cancelled_at=utc_now())
    )
    return result.rowcount
