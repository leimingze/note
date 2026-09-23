"""
脚本功能：
创建 SQLAlchemy 引擎与会话工厂，并提供一个 FastAPI 依赖生成数据库会话。

启动命令：
由 app.main 在服务启动时调用，无需单独执行。
"""

from collections.abc import Generator

from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker


# ==================== 常量与配置 ====================

SESSION_FACTORY: sessionmaker[Session] | None = None


def init_db(database_url: str) -> sessionmaker[Session]:
    """
    输入：database_url，形如 mysql+pymysql://user:password@host:port/db 的字符串。
    输出：初始化并返回全局会话工厂；连接失败时由调用方捕获异常。
    功能：根据连接串创建带 pool_pre_ping 的引擎和会话工厂，供依赖注入与服务启动复用。
    """
    global SESSION_FACTORY
    engine = create_engine(database_url, pool_pre_ping=True)
    SESSION_FACTORY = sessionmaker(bind=engine, autoflush=False, expire_on_commit=False)
    return SESSION_FACTORY


def get_db() -> Generator[Session, None, None]:
    """
    输入：无。
    输出：每次请求产生一个新数据库会话，请求结束后关闭。
    功能：FastAPI 依赖，保证每个请求使用独立会话且最终释放连接。
    """
    if SESSION_FACTORY is None:
        raise RuntimeError("数据库会话工厂尚未初始化")
    session = SESSION_FACTORY()
    try:
        yield session
    finally:
        session.close()
