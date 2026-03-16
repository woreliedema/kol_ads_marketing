import json
from datetime import datetime, timedelta

from data_collection_service.crawlers.utils.logger import logger
from data_collection_service.app.db.redis_client import redis_client_mgr
from data_collection_service.app.services.query_service import QueryService


class DataProxyService:
    # 业务规则：多长时间内的数据认为是"鲜活"的 (例如1天)
    FRESHNESS_THRESHOLD_DAYS = 1

    @classmethod
    async def check_and_fetch_fresh_profile(cls, platform: str, uid: str, query_service: QueryService) -> tuple[bool, dict]:
        redis_key = f"kol:profile:fresh:{platform}:{uid}"

        redis_pool = redis_client_mgr.pool
        if not redis_pool:
            logger.error("Redis 连接池未初始化，已降级跳过 L1 缓存")
            redis_pool = None
        # L1: 尝试从 Redis 获取热数据
        try:
            cached_data = await redis_pool.get(redis_key)
            if cached_data:
                logger.info(f"[L1 Hit] Platform: {platform}, UID: {uid}")
                return True, json.loads(cached_data)
        except Exception as e:
            logger.error(f"Redis 查询异常: {e}")
            # Redis 挂了不阻断，降级去 CK 查

        # L2: Redis Miss，从 ClickHouse 查询温数据快照
        try:
            ck_data = await query_service.get_latest_profile_snapshot(platform, uid)
            if not ck_data:
                return False, {}

            last_update_time = ck_data.get('last_update_time')

            # 鲜活度计算
            # 判断入库时间与当前时间的差值
            time_diff = datetime.now() - last_update_time
            if time_diff > timedelta(days=cls.FRESHNESS_THRESHOLD_DAYS):
                logger.info(f"[Data Expired] {uid} 数据已过期 (距今 {time_diff.days} 天)")
                return False, {}  # 返回False触发Go端的Kafka爬虫抓取

            # 字段标准化 (适配 Go 端的需求)
            # 确保返回的键与 Go 端的 `freshData["nickname"]` 匹配
            standard_data = {
                "nickname": str(ck_data.get("nickname", "")),
                "followers_count": float(ck_data.get("followers_count", 0))  # Go端使用float64接收JSON数字
            }

            # 缓存回写 (Cache-Aside 查漏补缺)
            try:
                # 回写Redis，设置 TTL (缓存有效期设为 鲜活度剩余的时间 或 固定时间)
                # 这里设为剩余的新鲜时间
                remaining_ttl = int((timedelta(days=cls.FRESHNESS_THRESHOLD_DAYS) - time_diff).total_seconds())
                if remaining_ttl > 0:
                    await redis_pool.setex(redis_key, remaining_ttl, json.dumps(standard_data))
            except Exception as e:
                logger.error(f"Redis 回写异常: {e}")

            logger.info(f"[L2 Hit] 从 CK 加载鲜活数据并回写 Redis: {uid}")
            return True, standard_data

        except Exception as e:
            logger.error(f"ClickHouse 查询异常: {e}")
            return False, {}