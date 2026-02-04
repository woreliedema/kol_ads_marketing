import json
from datetime import datetime
from sqlalchemy.orm import Session
from clickhouse_driver import Client
from data_collection_service.app.db.models import CrawlerRecord, CrawlerTask
from data_collection_service.crawlers.utils.logger import logger


class StorageService:
    """
        存储服务：负责 MySQL 和 ClickHouse 的持久化操作
        """

    def __init__(self, db_session: Session, ch_client: Client):
        self.db = db_session
        self.ch = ch_client

    def save_comments_to_clickhouse(self, comments: list):
        """批量写入评论到 ClickHouse"""
        if not comments:
            return

        insert_sql = """
            INSERT INTO bilibili_comments 
            (rpid, video_id, parent_id, root_id, user_id, user_name, content, like_count, create_time, is_sub)
            VALUES
            """
        try:
            self.ch.execute(insert_sql, comments)
            logger.info(f"[ClickHouse] 成功写入 {len(comments)} 条评论数据")
        except Exception as e:
            logger.error(f"[ClickHouse] 写入失败: {str(e)}")
            raise e

    def update_task_status(self, task_id: int, status: int, result_summary: dict = None):
        """更新 MySQL 中的任务状态和记录"""
        try:
            # 更新任务表
            task = self.db.query(CrawlerTask).filter(CrawlerTask.id == task_id).first()
            if task:
                task.task_status = status
                task.update_time = datetime.now()

            # 插入执行记录
            if result_summary:
                record = CrawlerRecord(
                    task_id=task_id,
                    resource_id=result_summary.get('resource_id'),
                    resource_type=1,  # 1=视频
                    parsed_data=json.dumps(result_summary.get('data'), ensure_ascii=False),
                    create_time=datetime.now()
                )
                self.db.add(record)

            self.db.commit()
        except Exception as e:
            self.db.rollback()
            logger.error(f"[MySQL] 状态更新失败: {str(e)}")