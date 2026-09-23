"""
脚本功能：
初始化测试数据，为不同状态与归属创建订单。

启动命令：
在项目根目录运行 `python -m app.seed`。
"""

from datetime import datetime

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy import select

from app.config import load_settings
from app.models import Base, Order, OrderStatus


# ==================== 常量与配置 ====================

SEED_ORDERS = [
    {
        "status": OrderStatus.PENDING_PAYMENT,
        "user_id": "10001",
        "order_no": "2026082300000001",
        "amount": "249.00",
        "subject": "小米手环9 NFC版 智能手环 黑色标准版",
        "created_at": "2026-08-23T09:12:33+08:00",
    },
    {
        "status": OrderStatus.PENDING_SHIPMENT,
        "user_id": "10001",
        "order_no": "2026082300000002",
        "amount": "599.00",
        "subject": "罗技（Logitech）MX Keys 无线蓝牙键盘 深灰色",
        "created_at": "2026-08-23T10:05:18+08:00",
    },
    {
        "status": OrderStatus.SHIPPED,
        "user_id": "10001",
        "order_no": "2026082300000003",
        "amount": "159.00",
        "subject": "膳魔师（THERMOS）保温杯 500ml 不锈钢本色",
        "created_at": "2026-08-23T11:40:02+08:00",
    },
    {
        "status": OrderStatus.COMPLETED,
        "user_id": "10001",
        "order_no": "2026082300000004",
        "amount": "2299.00",
        "subject": "索尼（SONY）WH-1000XM5 头戴式降噪耳机 黑色",
        "created_at": "2026-08-23T14:22:47+08:00",
    },
    {
        "status": OrderStatus.CANCELLED,
        "user_id": "10001",
        "order_no": "2026082300000005",
        "amount": "129.00",
        "subject": "绿联（UGREEN）100W 氮化镓充电器套装 含快充线",
        "created_at": "2026-08-23T15:03:56+08:00",
    },
    {
        "status": OrderStatus.PENDING_PAYMENT,
        "user_id": "10002",
        "order_no": "2026082300000006",
        "amount": "68.00",
        "subject": "无印良品（MUJI）亚克力桌面收纳盒 透明",
        "created_at": "2026-08-23T16:38:21+08:00",
    },
]


# ==================== 输入输出、加载与保存 ====================


def seed_database(database_url: str) -> list[Order]:
    """
    输入：database_url，MySQL 连接串。
    输出：实际入库的订单列表；已存在的订单号跳过。
    功能：按种子定义创建测试订单，保证重复执行不产生重复数据。
    """
    engine = create_engine(database_url, pool_pre_ping=True)
    Base.metadata.create_all(engine)
    session_factory = sessionmaker(bind=engine, expire_on_commit=False)
    created: list[Order] = []
    with session_factory() as db:
        for spec in SEED_ORDERS:
            if db.scalar(select(Order).where(Order.order_no == spec["order_no"])) is not None:
                continue
            order = Order(
                order_no=spec["order_no"],
                user_id=spec["user_id"],
                status=spec["status"].value,
                amount=spec["amount"],
                subject=spec["subject"],
                created_at=datetime.fromisoformat(spec["created_at"]),
            )
            db.add(order)
            created.append(order)
        db.commit()
    return created


def main() -> None:
    """
    输入：无。
    输出：向数据库写入种子订单并打印结果；连接失败时抛出异常。
    功能：命令行入口，调用 seed_database 初始化测试数据。
    """
    created = seed_database(load_settings().database_url)
    print(f"种子数据完成：本次新增 {len(created)} 笔订单")
    for order in created:
        print(
            f"  order_id={order.id} order_no={order.order_no} user={order.user_id} "
            f"status={order.status} subject={order.subject}"
        )


if __name__ == "__main__":
    main()
