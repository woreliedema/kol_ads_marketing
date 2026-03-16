
from typing import Dict, Any, Optional
from asynch.connection import Connection
from asynch.cursors import DictCursor


from data_collection_service.crawlers.utils.logger import logger


class QueryService:
    """
    前期先写死，中期慢慢动态化封装修改
    数据查询服务：
    1. 负责为微服务集群（报价引擎、监控服务）提供底层 ClickHouse 数据的查询与封装
    2. 负责为前端提供基础的信息查询与封装
    3. 复杂查询通过其他特定服务完成
    """
    def __init__(self, ch_client: Connection):
        self.ch = ch_client

    async def get_latest_profile_snapshot(self, platform: str, uid: str) -> Optional[Dict[str, Any]]:
        # ... 动态表名和 SQL 拼接逻辑保持不变 ...
        uid_param = int(uid) if platform == "bilibili" else uid
        query = f"""
            SELECT 
                mid,
                argMaxMerge(name) as nickname,
                argMaxMerge(fans) as followers_count,
                max(last_update_time) as last_update_time
            FROM dwd.bilibili_user_latest_profile
            WHERE mid = {uid_param}
            GROUP BY mid
        """

        try:
            # 【核心改造】使用 DictCursor 获取字典格式的异步游标
            async with self.ch.cursor(cursor=DictCursor) as cursor:
                await cursor.execute(query)
                # fetchall() 直接返回 [ {'mid':123456,'nickname': '...', 'followers_count': 100}, ... ]
                data = await cursor.fetchall()
            if not data:
                return None
            # 直接获取第一行数据字典
            return data[0]

        except Exception as e:
            logger.error(f"[ClickHouse] 读取快照数据失败 Platform={platform}, UID={uid}: {str(e)}")
            raise e

    async def get_kol_base_data(self, user_id: str) -> Optional[Dict[str, Any]]:
        """查询红人基础画像数据"""
        query = f"""
        select mid,uname,sign,official_role,official_title
        from dwd.bilibili_user_info_unique
        where mid={int(user_id)}
        """
        try:
            logger.info(f"[Inner API] 准备从 ClickHouse 读取红人数据 UID: {user_id}")
            async with self.ch.cursor(cursor=DictCursor) as cursor:
                await cursor.execute(query)
                data = await cursor.fetchall()

            if not data:
                logger.warning(f"[Inner API] 未查找到 UID: {user_id} 的红人数据")
                return None
            logger.info(f"[Inner API] 成功读取 UID: {user_id} 的红人数据")
            return self._parse_kol_data(data[0])

        except Exception as e:
            logger.error(f"[ClickHouse] 读取红人数据失败 UID={user_id}: {str(e)}")
            raise e

    async def get_video_metrics_data(self, video_id: str) -> Optional[Dict[str, Any]]:
        """查询视频核心互动指标"""
        query = f"""
        select bvid,title,views_count,danmaku_count,replys_count,likes_count,coin_count,share_count,favorites_count
            ,insert_datetime
        from dwd.bilibili_video_info_unqiue
        where bvid='{video_id}'
        """
        try:
            logger.info(f"[Inner API] 准备读取视频监控数据 BV号: {video_id}")
            async with self.ch.cursor(cursor=DictCursor) as cursor:
                await cursor.execute(query)
                data = await cursor.fetchall()

            if not data:
                logger.warning(f"[Inner API] 未查找到 BV号: {video_id} 的监控数据")
                return None

            logger.info(f"[Inner API] 成功读取 BV号: {video_id} 的监控数据")
            return self._parse_video_metrics(data[0])

        except Exception as e:
            logger.error(f"[ClickHouse] 读取视频数据失败 BV={video_id}: {str(e)}")
            raise e

    @classmethod
    def _parse_kol_data(self, raw: dict) -> Dict[str, Any]:
        """数据清洗：格式化红人基础信息"""
        return {
            # "platform_type": raw.get("platform_type", 3),
            "user_id": str(raw.get("mid", "")),
            "user_nickname": raw.get("uname", ""),
            # "follower_count": int(raw.get("follower_count", 0)),
            "sign": raw.get("sign", ""),
            "official_role": raw.get("official_role", ""),
            "official_title": raw.get("official_title", "")
        }

    @classmethod
    def _parse_video_metrics(self, raw: dict) -> Dict[str, Any]:
        """数据清洗：封装供 AI 或监控大盘使用的结构"""
        return {
            "video_id": str(raw.get("bvid", "")),
            "title": raw.get("title", ""),
            "metrics": {
                "views_count": int(raw.get("views_count", 0)),
                "danmaku_count": int(raw.get("danmaku_count", 0)),
                "replys_count": int(raw.get("replys_count", 0)),
                "likes_count": int(raw.get("likes_count", 0)),
                "coin_count": int(raw.get("coin_count", 0)),
                "share_count": int(raw.get("share_count", 0)),
                "favorites_count": int(raw.get("favorites_count", 0))
            },
            # 兼容 datetime 对象转换为字符串
            "insert_datetime": raw.get("insert_datetime").strftime("%Y-%m-%d %H:%M:%S") if raw.get("insert_datetime") else None
        }

    @classmethod
    def _parse_latest_profile_snapshot_video_metrics(self, raw: dict) -> Dict[str, Any]:
        """数据清洗：封装供 UGC 绑定使用的结构"""
        return {
            "mid": int(raw.get("mid", 0)),
            "nickname": raw.get("nickname", ""),
            "followers_count": int(raw.get("followers_count", 0)),
        }