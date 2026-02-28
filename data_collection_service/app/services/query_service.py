from clickhouse_driver import Client
from typing import Dict, Any, Optional


from data_collection_service.crawlers.utils.logger import logger


class QueryService:
    """
    前期先写死，中期慢慢动态化封装修改
    数据查询服务：
    1. 负责为微服务集群（报价引擎、监控服务）提供底层 ClickHouse 数据的查询与封装
    2. 负责为前端提供基础的信息查询与封装
    3. 复杂查询通过其他特定服务完成
    """
    def __init__(self, ch_client: Client):
        self.ch = ch_client

    def get_kol_base_data(self, user_id: str) -> Optional[Dict[str, Any]]:
        """查询红人基础画像数据"""
        query = f"""
        select * 
        from dwd.bilibili_user_info_unique
        where mid=Uint64({user_id})
        """
        try:
            logger.info(f"[Inner API] 准备从 ClickHouse 读取红人数据 UID: {user_id}")
            result = self.ch.execute(query, {'user_id': user_id}, with_column_types=True)
            data, columns = result[0], result[1]

            if not data:
                logger.warning(f"[Inner API] 未查找到 UID: {user_id} 的红人数据")
                return None

            col_names = [c[0] for c in columns]
            raw_dict = dict(zip(col_names, data[0]))

            logger.info(f"[Inner API] 成功读取 UID: {user_id} 的红人数据")
            return self._parse_kol_data(raw_dict)

        except Exception as e:
            logger.error(f"[ClickHouse] 读取红人数据失败 UID={user_id}: {str(e)}")
            raise e

    def get_video_metrics_data(self, video_id: str) -> Optional[Dict[str, Any]]:
        """查询视频核心互动指标"""
        query = f"""
        select * 
        from dwd.video_metrics
        where bvid={video_id}
        """
        try:
            logger.info(f"[Inner API] 准备读取视频监控数据 BV号: {video_id}")
            result = self.ch.execute(query, {'video_id': video_id}, with_column_types=True)
            data, columns = result[0], result[1]

            if not data:
                logger.warning(f"[Inner API] 未查找到 BV号: {video_id} 的监控数据")
                return None

            col_names = [c[0] for c in columns]
            raw_dict = dict(zip(col_names, data[0]))

            logger.info(f"[Inner API] 成功读取 BV号: {video_id} 的监控数据")
            return self._parse_video_metrics(raw_dict)

        except Exception as e:
            logger.error(f"[ClickHouse] 读取视频数据失败 BV={video_id}: {str(e)}")
            raise e

    @classmethod
    def _parse_kol_data(self, raw: dict) -> Dict[str, Any]:
        """数据清洗：格式化红人基础信息"""
        return {
            "platform_type": raw.get("platform_type", 3),
            "user_id": str(raw.get("mid", "")),
            "user_nickname": raw.get("uname", ""),
            # "follower_count": int(raw.get("follower_count", 0)),
            "sign": raw.get("sign", "")
        }

    def _parse_video_metrics(self, raw: dict) -> Dict[str, Any]:
        """数据清洗：封装供 AI 或监控大盘使用的结构"""
        return {
            "video_id": str(raw.get("video_id", "")),
            "title": raw.get("title", ""),
            "metrics": {
                "view_count": int(raw.get("view_count", 0)),
                "danmaku_count": int(raw.get("danmaku_count", 0)),
                "reply_count": int(raw.get("reply_count", 0)),
                "like_count": int(raw.get("like_count", 0)),
                "coin_count": int(raw.get("coin_count", 0)),
                "share_count": int(raw.get("share_count", 0)),
            },
            # 兼容 datetime 对象转换为字符串
            "last_updated": raw.get("fetch_time").strftime("%Y-%m-%d %H:%M:%S") if raw.get("fetch_time") else None
        }