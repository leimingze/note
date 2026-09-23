"""
脚本功能：
读取环境变量并生成服务配置，配置项均提供本地开发默认值。

启动命令：
在项目根目录运行 `uvicorn app.main:app --reload --port 30051`；
如需覆盖默认值，可设置 DATABASE_URL、REDIS_URL、RABBITMQ_URL 等环境变量。
"""

import os
from dataclasses import dataclass


# ==================== 常量与配置 ====================

DEFAULT_DATABASE_URL = (
    "mysql+pymysql://demo:demo_pass@127.0.0.1:30052/order_demo"
)
DEFAULT_REDIS_URL = "redis://127.0.0.1:30054/0"
DEFAULT_RABBITMQ_URL = "amqp://guest:guest@127.0.0.1:30055/%2F"


@dataclass(frozen=True)
class Settings:
    """
    输入：无参数，全部字段来自环境变量或默认值。
    输出：不可变配置对象，包含数据库、Redis、RabbitMQ 连接信息及缓存 TTL。
    功能：集中管理服务运行所需配置，避免业务代码直接读取环境变量。
    """

    database_url: str
    redis_url: str
    rabbitmq_url: str
    cache_ttl_seconds: int


def load_settings() -> Settings:
    """
    输入：环境变量 DATABASE_URL、REDIS_URL、RABBITMQ_URL、CACHE_TTL_SECONDS。
    输出：Settings 实例；数值解析失败时抛出 ValueError。
    功能：读取环境变量并组装配置对象，未设置的项使用本地开发默认值。
    """
    return Settings(
        database_url=os.getenv("DATABASE_URL", DEFAULT_DATABASE_URL),
        redis_url=os.getenv("REDIS_URL", DEFAULT_REDIS_URL),
        rabbitmq_url=os.getenv("RABBITMQ_URL", DEFAULT_RABBITMQ_URL),
        cache_ttl_seconds=int(os.getenv("CACHE_TTL_SECONDS", "5")),
    )


SETTINGS = load_settings()
