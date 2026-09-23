"""
脚本功能：
pytest 公共夹具：等待服务就绪，提供数据库、Redis、HTTP 调用与测试数据清理能力。

启动命令：
在项目根目录运行 `pytest`；需要先启动 Docker 中间件与订单服务。
"""

import time
from collections.abc import Generator
from decimal import Decimal
from uuid import uuid4

import pytest
import requests
from redis import Redis
from sqlalchemy import create_engine, delete
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import NullPool

from app.config import load_settings
from app.models import Base, IdempotencyRecord, Order, OrderStatus, OutboxEvent


# ==================== 常量与配置 ====================

API_BASE_URL = "http://127.0.0.1:30051"
API_WAIT_TIMEOUT_SECONDS = 60


def wait_until(predicate, timeout: float = 15.0, interval: float = 0.3, message: str = "等待超时") -> None:
    """
    输入：predicate，返回真值的函数；timeout，总等待秒数；interval，轮询间隔；message，超时提示。
    输出：条件满足时返回；超时则抛出 AssertionError。
    功能：轮询等待异步结果，例如后台任务把事件发送到 MQ。
    """
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(interval)
    raise AssertionError(message)


def wait_for_api() -> None:
    """等待订单服务健康检查通过；超时抛出异常。"""
    wait_until(
        lambda: requests.get(f"{API_BASE_URL}/api/health", timeout=3).status_code == 200,
        timeout=API_WAIT_TIMEOUT_SECONDS,
        message="订单服务未在 60 秒内就绪，请先启动服务",
    )


# ==================== 夹具 ====================


@pytest.fixture(scope="session")
def settings():
    """输入：无。输出：Settings 配置对象。功能：供数据库与 Redis 夹具复用。"""
    return load_settings()


@pytest.fixture(scope="session")
def db_engine(settings):
    """输入：settings。输出：SQLAlchemy 引擎。功能：创建引擎并确保数据表存在。"""
    wait_for_api()
    engine = create_engine(
        settings.database_url,
        pool_pre_ping=True,
        isolation_level="READ COMMITTED",
        poolclass=NullPool,
    )
    Base.metadata.create_all(engine)
    return engine


@pytest.fixture()
def db_session(db_engine) -> Generator[Session, None, None]:
    """输入：db_engine。输出：清理后的数据库会话。功能：每用例前清空订单相关数据。"""
    session_factory = sessionmaker(bind=db_engine)
    with session_factory() as db:
        db.execute(delete(OutboxEvent))
        db.execute(delete(IdempotencyRecord))
        db.execute(delete(Order))
        db.commit()
        yield db


@pytest.fixture()
def redis_client(settings) -> Generator[Redis, None, None]:
    """输入：settings。输出：清理后的 Redis 客户端。功能：清空订单缓存与故障开关。"""
    cache = Redis.from_url(settings.redis_url)
    for key in cache.scan_iter(match="order:*"):
        cache.delete(key)
    cache.delete("fault:mq_publish")
    yield cache
    cache.close()


def _make_order(
    db: Session,
    *,
    user_id: str,
    status: OrderStatus,
    amount: str = "1.00",
) -> Order:
    """
    输入：db，数据库会话；user_id，用户标识；status，订单状态；amount，金额字符串。
    输出：已提交并刷新主键的 Order 对象。
    功能：为测试创建唯一订单，避免用例之间互相干扰。
    """
    order = Order(
        order_no=f"TEST{uuid4().hex[:12].upper()}",
        user_id=user_id,
        status=status.value,
        amount=Decimal(amount),
    )
    db.add(order)
    db.commit()
    return order


@pytest.fixture()
def api():
    """输入：无。输出：访问被测服务的辅助函数集合。功能：封装取消与查询请求。"""

    def post_cancel(
        order_id: int,
        *,
        user_id: str | None = "u_test",
        idempotency_key: str | None = None,
        fault_db: bool = False,
    ) -> requests.Response:
        headers = {}
        if user_id is not None:
            headers["X-User-Id"] = user_id
        if idempotency_key is not None:
            headers["X-Idempotency-Key"] = idempotency_key
        if fault_db:
            headers["X-Fault-Injection"] = "db"
        return requests.post(
            f"{API_BASE_URL}/api/orders/{order_id}/cancel",
            headers=headers,
            timeout=5,
        )

    def get_order(order_id: int, *, user_id: str = "u_test") -> requests.Response:
        return requests.get(
            f"{API_BASE_URL}/api/orders/{order_id}",
            headers={"X-User-Id": user_id},
            timeout=5,
        )

    return {"cancel": post_cancel, "get": get_order}


@pytest.fixture()
def fresh_db(db_engine):
    """
    输入：db_engine 夹具。
    输出：创建全新数据库会话的工厂函数。
    功能：用独立会话验证外部提交后的数据，避免共享会话的缓存与快照干扰。
    """

    def _open() -> Session:
        return sessionmaker(bind=db_engine)()

    return _open


@pytest.fixture()
def create_order(db_session):
    """
    输入：db_session 夹具。
    输出：创建订单的函数，接受 user_id、status、amount 参数。
    功能：把订单创建函数注入测试，简化用例中的测试数据准备。
    """

    def _create(**kwargs):
        return _make_order(db_session, **kwargs)

    return _create
