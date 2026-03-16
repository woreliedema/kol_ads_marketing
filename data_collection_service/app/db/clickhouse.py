import asynch
from asynch.pool import Pool
from typing import AsyncGenerator
from asynch.connection import Connection

from data_collection_service.crawlers.utils.logger import logger

class ClickHouseManager:
    """ClickHouse 异步连接池管理器"""
    pool: Pool = None

    @classmethod
    async def init_db(cls, host: str, port: int, user: str, password: str, database: str = "ods"):
        try:
            cls.pool = Pool(
                host=host,
                port=port,        # 注意：此处应填写 TCP 端口 (默认9000)
                user=user,
                password=password,
                database=database,
                minsize=1,  # 最小空闲连接数
                maxsize=50, # 最大并发连接数 (根据并发采集量调整)
                connect_timeout=10,
                send_receive_timeout=300
            )
            await cls.pool.startup()
            # 测试连接
            async with cls.pool.connection() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute('SELECT 1')

            logger.info("✅ ClickHouse 数据库连接初始化成功！")
        except Exception as e:
            logger.error(f"❌ ClickHouse 连接失败: {e}", exc_info=True)
            raise e

    @classmethod
    async def close_db(cls):
        if cls.pool:
            await cls.pool.shutdown()
            logger.info("🛑 ClickHouse 数据库连接已安全关闭。")

#  核心：这就是提供给 FastAPI 路由的依赖注入函数
async def get_ch_client() -> AsyncGenerator[Connection, None]:
    if not ClickHouseManager.pool:
        raise RuntimeError("ClickHouse 客户端尚未初始化")
    async with ClickHouseManager.pool.connection() as conn:
        yield conn

