"""
脚本功能：
FastAPI 应用入口，负责初始化数据库、Redis、后台任务并挂载路由。

启动命令：
在项目根目录运行 `uvicorn app.main:app --reload --port 30051`。
"""

from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.orders import router as orders_router
from app.cache import close_cache, init_cache
from app.config import load_settings
from app.db import init_db
from app.models import Base
from app.mq import OutboxWorker


# ==================== 常量与配置 ====================

WORKER: OutboxWorker | None = None


@asynccontextmanager
async def lifespan(_: FastAPI):
    """
    输入：FastAPI 应用实例。
    输出：启动时初始化数据表、Redis、后台任务，关闭时停止并释放资源。
    功能：管理应用级资源的生命周期。
    """
    global WORKER
    settings = load_settings()
    session_factory = init_db(settings.database_url)
    engine = session_factory.kw["bind"]
    Base.metadata.create_all(engine)
    cache = init_cache(settings.redis_url)
    WORKER = OutboxWorker(session_factory, cache, settings.rabbitmq_url)
    WORKER.start()
    yield
    if WORKER is not None:
        WORKER.stop()
    close_cache()


app = FastAPI(title="订单服务接口测试 Demo", lifespan=lifespan)
app.include_router(orders_router)


@app.get("/api/health")
def health() -> dict:
    """
    输入：无。
    输出：固定健康检查响应 {"status": "ok"}。
    功能：供测试与手工验证确认服务已启动。
    """
    return {"status": "ok"}
