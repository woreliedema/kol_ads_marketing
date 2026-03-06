import os
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from dotenv import load_dotenv
from data_collection_service.crawlers.utils.logger import logger

load_dotenv()

# 从环境变量中读取 MySQL 连接配置，提供本地测试的默认值
MYSQL_URL = os.getenv("MYSQL_URL_PYTHON")

try:
    # pool_pre_ping=True: 每次从连接池获取连接时都会 ping 一下数据库，防止 MySQL 隔夜断开导致 2006 错误
    engine = create_engine(MYSQL_URL, pool_pre_ping=True, pool_size=10, max_overflow=20)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    logger.info("[DB] MySQL 引擎及连接池初始化完成")
except Exception as e:
    logger.error(f"[DB] MySQL 连接引擎初始化失败: {str(e)}")
    raise e

def get_db():
    """
    FastAPI 依赖注入：获取数据库会话
    通过 yield 生成器模式，确保在 HTTP 请求结束（或发生异常）时安全关闭 Session
    """
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()