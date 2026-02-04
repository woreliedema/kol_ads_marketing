import json
import datetime
from kafka import KafkaProducer
from data_collection_service.crawlers.utils.logger import logger


class KafkaService:
    """
    消息队列服务：负责将采集结果推送到 Kafka
    """
    def __init__(self, bootstrap_servers="localhost:9092"):
        self.producer = KafkaProducer(
            bootstrap_servers=bootstrap_servers,
            value_serializer=lambda v: json.dumps(v).encode("utf-8")
        )
        self.topic = "crawler_video_data" # 对应文档[4]定义的Topic

    def send_completion_event(self, task_id: int, video_id: str, total_count: int):
        """发送采集完成事件"""
        message = {
            "task_id": task_id,
            "platform": "bilibili",
            "video_id": video_id,
            "data_type": "comment",
            "count": total_count,
            "timestamp": str(datetime.now())
        }
        try:
            self.producer.send(self.topic, value=message)
            self.producer.flush()
            logger.info(f"[Kafka] 消息推送成功: {message}")
        except Exception as e:
            logger.error(f"[Kafka] 推送失败: {str(e)}")