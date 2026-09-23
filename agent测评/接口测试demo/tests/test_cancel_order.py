"""
脚本功能：
覆盖取消接口的核心业务规则：成功、鉴权、越权、状态拒绝、幂等、并发与数据库故障。

启动命令：
在项目根目录运行 `pytest tests/test_cancel_order.py -v`。
"""

from concurrent.futures import ThreadPoolExecutor

from sqlalchemy import func, select

from app.models import IdempotencyRecord, Order, OrderStatus, OutboxEvent


def _event_count(db, order_id: int) -> int:
    """输入：db，数据库会话；order_id，订单主键。输出：该订单的取消事件数量。功能：断言数据副作用。"""
    stmt = select(func.count()).select_from(OutboxEvent).where(OutboxEvent.order_id == order_id)
    return db.scalar(stmt)


def test_cancel_pending_payment_success(db_session, api, redis_client, create_order, fresh_db):
    """正常取消待支付订单：响应成功，状态与事件同事务提交。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_PAYMENT)

    response = api["cancel"](order.id)

    assert response.status_code == 200
    assert response.json()["success"] is True
    with fresh_db() as db:
        assert db.get(Order, order.id).status == OrderStatus.CANCELLED.value
        assert _event_count(db, order.id) == 1


def test_cancel_pending_shipment_success(db_session, api, redis_client, create_order, fresh_db):
    """正常取消待发货订单：与待支付订单行为一致。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_SHIPMENT)

    response = api["cancel"](order.id)

    assert response.status_code == 200
    assert response.json()["success"] is True
    with fresh_db() as db:
        assert db.get(Order, order.id).status == OrderStatus.CANCELLED.value


def test_cancel_rejects_missing_user(db_session, api, redis_client, create_order):
    """未登录取消：返回 401，订单无变化。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_PAYMENT)

    response = api["cancel"](order.id, user_id=None)

    assert response.status_code == 401
    assert response.json()["detail"]["code"] == "UNAUTHORIZED"
    assert db_session.get(Order, order.id).status == OrderStatus.PENDING_PAYMENT.value


def test_cancel_other_users_order_returns_404(db_session, api, redis_client, create_order):
    """越权取消他人订单：统一 404，不泄露订单信息。"""
    order = create_order(user_id="u_owner", status=OrderStatus.PENDING_PAYMENT)

    response = api["cancel"](order.id, user_id="u_other")

    assert response.status_code == 404
    assert response.json()["detail"]["code"] == "ORDER_NOT_FOUND"
    assert db_session.get(Order, order.id).status == OrderStatus.PENDING_PAYMENT.value
    assert _event_count(db_session, order.id) == 0


def test_cancel_shipped_order_returns_409(db_session, api, redis_client, create_order):
    """已发货订单拒绝取消：409，状态与事件均无变化。"""
    order = create_order(user_id="u_test", status=OrderStatus.SHIPPED)

    response = api["cancel"](order.id)

    assert response.status_code == 409
    assert response.json()["detail"]["code"] == "ORDER_NOT_CANCELLABLE"
    assert db_session.get(Order, order.id).status == OrderStatus.SHIPPED.value
    assert _event_count(db_session, order.id) == 0


def test_cancel_already_cancelled_returns_existing_result(db_session, api, redis_client, create_order):
    """已取消订单再次取消：返回已有结果，不产生新事件。"""
    order = create_order(user_id="u_test", status=OrderStatus.CANCELLED)

    response = api["cancel"](order.id)

    assert response.status_code == 200
    assert response.json()["success"] is False
    assert _event_count(db_session, order.id) == 0


def test_cancel_idempotent_by_key(db_session, api, redis_client, create_order):
    """同一个幂等键重复请求：只执行一次取消副作用。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_PAYMENT)
    key = "idem-demo"

    first = api["cancel"](order.id, idempotency_key=key)
    second = api["cancel"](order.id, idempotency_key=key)

    assert first.status_code == 200 and first.json()["success"] is True
    assert second.status_code == 200 and second.json()["success"] is False
    assert _event_count(db_session, order.id) == 1
    assert (
        db_session.scalar(
            select(func.count()).select_from(IdempotencyRecord).where(IdempotencyRecord.idempotency_key == key)
        )
        == 1
    )


def test_concurrent_cancel_single_effect(db_session, api, redis_client, create_order, fresh_db):
    """并发取消：只有一次生效，事件只有一条，最终状态合法。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_SHIPMENT)
    order_id = order.id

    def send(i: int):
        return api["cancel"](order_id, idempotency_key=f"concurrent-{i}")

    with ThreadPoolExecutor(max_workers=4) as pool:
        responses = list(pool.map(send, range(4)))

    assert all(response.status_code == 200 for response in responses)
    assert sum(response.json()["success"] for response in responses) == 1
    with fresh_db() as db:
        assert db.get(Order, order.id).status == OrderStatus.CANCELLED.value
        assert _event_count(db, order.id) == 1


def test_cancel_db_fault_rolls_back(db_session, api, redis_client, create_order):
    """数据库故障注入：返回 500，订单保持原状态且无事件。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_PAYMENT)

    response = api["cancel"](order.id, fault_db=True)

    assert response.status_code == 500
    assert response.json()["detail"]["code"] == "DB_FAULT_SIMULATED"
    assert db_session.get(Order, order.id).status == OrderStatus.PENDING_PAYMENT.value
    assert _event_count(db_session, order.id) == 0


def test_get_order_reads_and_invalidates_cache(db_session, api, redis_client, create_order):
    """查询订单：首次读库写缓存，取消后缓存删除，再次查询返回新状态。"""
    order = create_order(user_id="u_test", status=OrderStatus.PENDING_PAYMENT)

    first = api["get"](order.id)
    assert first.status_code == 200
    assert redis_client.exists(f"order:{order.id}") == 1

    cancel = api["cancel"](order.id)
    assert cancel.status_code == 200
    assert redis_client.exists(f"order:{order.id}") == 0

    second = api["get"](order.id)
    assert second.status_code == 200
    assert second.json()["status"] == OrderStatus.CANCELLED.value
