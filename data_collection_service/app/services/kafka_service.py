import os
import json
from aiokafka import AIOKafkaProducer
from dotenv import load_dotenv

# 导入项目中标准的日志记录器
from data_collection_service.crawlers.utils.logger import logger

load_dotenv()

class KafkaProducerManager:
    """
    Kafka 异步生产者管理器
    负责维持与 Kafka 集群的长连接，并提供极速的异步消息投递能力
    """
    def __init__(self):
        # 从环境变量获取 Kafka Broker 地址，默认适配 docker-compose 里的配置
        self.bootstrap_servers = os.getenv("KAFKA_BOOTSTRAP_SERVERS", "127.0.0.1:9092")
        self.producer = None

    async def start(self):
        """
        初始化并启动 Producer (将被挂载到 main.py 的 lifespan 中)
        """
        try:
            # 实例化 AIOKafkaProducer
            # value_serializer: 自动将 Python 字典序列化为 UTF-8 编码的 JSON 字节流
            self.producer = AIOKafkaProducer(
                bootstrap_servers=self.bootstrap_servers,
                value_serializer=lambda v: json.dumps(v, ensure_ascii=False).encode('utf-8'),
                # acks='1' 代表只要 Leader 写入成功就返回，兼顾高吞吐与安全性
                acks='1'
            )
            await self.producer.start()
            logger.info(f"🚀 [Kafka] Producer 已成功连接到集群: {self.bootstrap_servers}")
        except Exception as e:
            logger.error(f"❌ [Kafka] Producer 启动失败: {str(e)}")
            raise e

    async def stop(self):
        """
        优雅关闭 Producer，确保缓存在内存中的消息被 flush 到 Broker
        """
        if self.producer:
            await self.producer.stop()
            logger.info("🛑 [Kafka] Producer 已安全关闭，内存消息已 Flush")

    async def send_task_message(self, topic: str, message: dict):
        """
        通用异步消息发送方法
        """
        if not self.producer:
            error_msg = "[Kafka] Producer 尚未初始化，无法发送消息！"
            logger.error(error_msg)
            raise RuntimeError(error_msg)

        try:
            # send_and_wait() 会等待 Broker 的 ACK 回执，确保消息不丢失
            record_metadata = await self.producer.send_and_wait(topic, message)
            logger.info(
                f"✉️ [Kafka] 消息投递成功 -> Topic: {topic}, "
                f"Partition: {record_metadata.partition}, "
                f"Offset: {record_metadata.offset}"
            )
            return True
        except Exception as e:
            logger.error(f"❌ [Kafka] 消息发送至 {topic} 失败: {str(e)}, 载荷: {message}")
            raise e

# 导出单例对象
kafka_producer = KafkaProducerManager()