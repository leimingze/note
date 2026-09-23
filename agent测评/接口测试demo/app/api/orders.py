"""
脚本功能：
定义订单相关 HTTP 路由：查询订单详情、取消订单。

启动命令：
由 app.main 挂载，无需单独执行。
"""

from typing import Annotated

from fastapi import APIRouter, Depends, Header, HTTPException
from sqlalchemy.orm import Session

from app.cache import RedisDep, get_cached_order, set_cached_order
from app.config import SETTINGS
from app.db import get_db
from app.schemas import CancelResponse, ErrorResponse, OrderResponse
from app.services.order_service import (
    DBSimulationError,
    OrderNotCancellableError,
    OrderNotFoundError,
    cancel_order as cancel_order_service,
    get_order_for_user,
)


# ==================== 常量与配置 ====================

router = APIRouter(prefix="/api/orders", tags=["orders"])


def _error_response(code: str, message: str) -> dict:
    """
    输入：code，错误码；message，错误说明。
    输出：可被 FastAPI 序列化的错误字典。
    功能：统一构造 HTTPException 的错误响应体。
    """
    return ErrorResponse(code=code, message=message).model_dump()


def _get_current_user(x_user_id: Annotated[str | None, Header()] = None) -> str:
    """
    输入：X-User-Id 请求头，可选。
    输出：用户标识；缺失时抛出 401。
    功能：从请求头读取当前登录用户，模拟登录态。
    """
    if not x_user_id:
        raise HTTPException(status_code=401, detail=_error_response("UNAUTHORIZED", "缺少 X-User-Id 请求头"))
    return x_user_id


@router.get("/{order_id}", response_model=OrderResponse, responses={401: {"model": ErrorResponse}, 404: {"model": ErrorResponse}})
def read_order(
    order_id: int,
    db: Annotated[Session, Depends(get_db)],
    cache: RedisDep,
    user_id: Annotated[str, Depends(_get_current_user)],
) -> OrderResponse:
    """
    输入：order_id，路径参数；user_id，登录用户；db，数据库会话；cache，Redis 客户端。
    输出：订单详情；未登录返回 401，越权或不存在返回 404。
    功能：先读缓存，未命中时读数据库并写缓存，用于验证取消后的状态。
    """
    cached = get_cached_order(cache, order_id)
    if cached is not None and cached.get("user_id") == user_id:
        return OrderResponse(**cached)
    order = get_order_for_user(db, order_id, user_id)
    if order is None:
        raise HTTPException(status_code=404, detail=_error_response("ORDER_NOT_FOUND", "订单不存在"))
    response = OrderResponse.model_validate(order)
    set_cached_order(cache, order_id, response.model_dump(), SETTINGS.cache_ttl_seconds)
    return response


@router.post("/{order_id}/cancel", response_model=CancelResponse, responses={401: {"model": ErrorResponse}, 404: {"model": ErrorResponse}, 409: {"model": ErrorResponse}, 500: {"model": ErrorResponse}})
def cancel_order(
    order_id: int,
    db: Annotated[Session, Depends(get_db)],
    cache: RedisDep,
    user_id: Annotated[str, Depends(_get_current_user)],
    x_idempotency_key: Annotated[str | None, Header()] = None,
    x_fault_injection: Annotated[str | None, Header()] = None,
) -> CancelResponse:
    """
    输入：order_id，路径参数；user_id；db；cache；
        x_idempotency_key，可选幂等键；x_fault_injection，可选故障注入标记（db）。
    输出：取消结果；未登录 401，越权或不存在 404，状态不允许 409，数据库故障注入 500。
    功能：调用取消订单核心服务并转换为 HTTP 响应。
    """
    try:
        result = cancel_order_service(
            db=db,
            cache=cache,
            order_id=order_id,
            user_id=user_id,
            idempotency_key=x_idempotency_key,
            fault_db=x_fault_injection == "db",
        )
    except OrderNotFoundError:
        raise HTTPException(status_code=404, detail=_error_response("ORDER_NOT_FOUND", "订单不存在"))
    except OrderNotCancellableError as exc:
        raise HTTPException(status_code=409, detail=_error_response("ORDER_NOT_CANCELLABLE", str(exc)))
    except DBSimulationError:
        raise HTTPException(status_code=500, detail=_error_response("DB_FAULT_SIMULATED", "数据库故障注入：事务已回滚"))
    return CancelResponse(
        order_id=result.order_id,
        status=result.status,
        success=result.success,
        message=result.message,
    )
