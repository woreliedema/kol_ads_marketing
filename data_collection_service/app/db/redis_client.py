import os
import redis.asyncio as aioredis
from typing import Optional

class RedisClient:
    def __init__(self):
        self.pool: Optional[aioredis.Redis] = None

    async def init_pool(self):
        """
        初始化全局异步 Redis 连接池
        """
        redis_host = os.getenv("REDIS_HOST", "127.0.0.1")
        redis_port = int(os.getenv("REDIS_PORT", 6379))
        redis_password = os.getenv("REDIS_PASSWORD", None)

        # 创建异步连接池
        self.pool = aioredis.Redis(
            host=redis_host,
            port=redis_port,
            password=redis_password,
            db=0,
            decode_responses=True, # 自动将 bytes 解码为 str (极度推荐)
            max_connections=100,   # 防止高并发把 Redis 连接打满
            socket_timeout=5.0     # 超时快失败机制
        )
        # 测试连接
        await self.pool.ping()
        print(f" Async Redis connection pool initialized at {redis_host}:{redis_port}")

    async def close_pool(self):
        """
        关闭连接池，释放资源
        """
        if self.pool:
            await self.pool.aclose()
            print("🛑 Async Redis connection pool closed")

# 导出全局单例实例
redis_client_mgr = RedisClient()