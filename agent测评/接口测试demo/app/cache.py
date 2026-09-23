"""
脚本功能：
创建 Redis 客户端，并封装订单缓存键的读写。

启动命令：
由 app.main 在启动时初始化，无需单独执行。
"""

import json
import logging
from typing import Annotated

from fastapi import Depends
from redis import Redis
from redis.exceptions import RedisError


# ==================== 常量与配置 ====================

LOGGER = logging.getLogger(__name__)
REDIS_CLIENT: Redis | None = None


def init_cache(redis_url: str) -> Redis:
    """
    输入：redis_url，形如 redis://host:port/db 的连接串。
    输出：初始化并返回全局 Redis 客户端。
    功能：在应用启动时创建 Redis 客户端，供依赖注入复用。
    """
    global REDIS_CLIENT
    REDIS_CLIENT = Redis.from_url(redis_url, decode_responses=False)
    return REDIS_CLIENT


def close_cache() -> None:
    """关闭全局 Redis 客户端连接；未初始化时不做任何事。"""
    if REDIS_CLIENT is not None:
        REDIS_CLIENT.close()


def get_redis_client() -> Redis:
    """
    输入：无。
    输出：全局 Redis 客户端；未初始化时抛出 RuntimeError。
    功能：FastAPI 依赖，供路由注入 Redis 客户端。
    """
    if REDIS_CLIENT is None:
        raise RuntimeError("Redis 客户端尚未初始化")
    return REDIS_CLIENT


RedisDep = Annotated[Redis, Depends(get_redis_client)]


def order_cache_key(order_id: int) -> str:
    """输入：order_id，订单自增主键。输出：Redis 缓存键。功能：统一缓存键格式。"""
    return f"order:{order_id}"


def get_cached_order(cache: Redis, order_id: int) -> dict | None:
    """
    输入：cache，Redis 客户端；order_id，订单主键。
    输出：缓存中的订单字典；无缓存或 Redis 异常时返回 None 并记录日志。
    功能：读取订单缓存。Redis 故障时降级读数据库，由 TTL 兜底保证最终一致。
    """
    try:
        raw = cache.get(order_cache_key(order_id))
        return json.loads(raw) if raw else None
    except RedisError:
        LOGGER.exception("读取订单缓存失败，降级读取数据库")
        return None


def set_cached_order(cache: Redis, order_id: int, order: dict, ttl_seconds: int) -> None:
    """
    输入：cache，Redis 客户端；order_id；order，订单字典；ttl_seconds，缓存秒数。
    输出：写入缓存；Redis 异常时记录日志，不中断主流程。
    功能：写入订单缓存，TTL 保证旧值最迟在 ttl_seconds 后失效。
    """
    try:
        cache.set(order_cache_key(order_id), json.dumps(order, default=str), ex=ttl_seconds)
    except RedisError:
        LOGGER.exception("写入订单缓存失败，本次读取由数据库兜底")


def invalidate_order_cache(cache: Redis, order_id: int) -> None:
    """
    输入：cache，Redis 客户端；order_id，订单主键。
    输出：删除缓存键；Redis 异常时记录日志，由 TTL 兜底。
    功能：取消成功后立即删除旧缓存，让后续读取尽快回到新状态。
    """
    try:
        cache.delete(order_cache_key(order_id))
    except RedisError:
        LOGGER.exception("删除订单缓存失败，旧值将由 TTL 兜底失效")
