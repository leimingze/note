"""
脚本功能：
定义接口请求与响应的 Pydantic 模型。

启动命令：
由 FastAPI 路由自动加载，无需单独执行。
"""

from datetime import datetime
from decimal import Decimal

from pydantic import BaseModel, ConfigDict


# ==================== 类型、结构与数据模型 ====================


class OrderResponse(BaseModel):
    """订单详情响应，用于 GET /api/orders/{order_id}。"""

    model_config = ConfigDict(from_attributes=True)

    id: int
    order_no: str
    user_id: str
    status: str
    amount: Decimal
    subject: str
    created_at: datetime
    cancelled_at: datetime | None = None


class CancelResponse(BaseModel):
    """取消订单响应，success 表示本次请求是否真正执行了取消。"""

    order_id: int
    status: str
    success: bool
    message: str


class ErrorResponse(BaseModel):
    """统一错误响应体。"""

    code: str
    message: str
