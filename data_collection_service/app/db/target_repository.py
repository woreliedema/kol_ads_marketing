from datetime import datetime
from sqlalchemy.dialects.mysql import insert

from data_collection_service.app.db.session import SessionLocal
from data_collection_service.app.db.models import CrawlerTarget
from data_collection_service.crawlers.utils.logger import logger


def cascade_register_videos_to_target(uid: str, platform_type: int, bvid_list: list, r_type: str, interval_minutes: int = 720):
    """
    专门用于级联写入新视频 ID 到调度总表，自我管理 DB Session。
    遇到老数据仅保持激活，绝不重置下一次调度时间！
    """
    if not bvid_list:
        return 0

    now = datetime.now()
    values_to_insert = []

    # 同时给新视频注册“详情”任务
    for bvid in bvid_list:
        values_to_insert.append({
            "platform_type": platform_type,
            "uid": uid,
            "resource_type": r_type,
            "target_id": bvid,
            "cron_interval_minutes": interval_minutes,
            "is_active": True,
            "next_run_time": now  # 仅对首次 Insert 生效，新视频立刻跑
        })

    # 创建独立的数据库会话
    db = SessionLocal()
    try:
        stmt = insert(CrawlerTarget).values(values_to_insert)

        # 【核心逻辑】：遇到重复键时，只保证处于激活状态，坚决不更新 next_run_time，保护老视频调度周期
        on_duplicate_stmt = stmt.on_duplicate_key_update(
            is_active=True
        )

        result = db.execute(on_duplicate_stmt)
        db.commit()
        return result.rowcount
    except Exception as e:
        db.rollback()
        logger.error(f"级联写入视频 Target 失败 (UID: {uid}): {str(e)}")
        return 0
    finally:
        # 确保释放连接池
        db.close()