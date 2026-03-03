import json
from datetime import datetime
from sqlalchemy.orm import Session

# 导入项目中定义的模型和工具
from data_collection_service.app.db.models import CrawlerTask
from data_collection_service.app.services.kafka_service import kafka_producer
from data_collection_service.crawlers.utils.logger import logger
# 导入花算法生成器工具类
from data_collection_service.crawlers.utils.snowflake import snowflake_gen


class TaskService:
    """
    爬虫任务调度服务 (支持批量快照与时序追踪)
    职责: 负责任务的持久化登记与 Kafka 异步下发，保证 MySQL 与 Kafka 的状态一致性
    """
    def __init__(self, db_session: Session):
        self.db = db_session

    async def create_and_dispatch_task(
            self,
            platform_type: int,
            resource_type: str,
            resource_ids: list,
            task_name: str = "未命名时序任务",
            params: dict = None
    ) -> str:
        """
        创建并派发批量/时序任务
        :param task_type: 任务动作 (1=全量评论, 2=画像快照, 3=指标监测)
        :param platform_type: 平台 (3=B站, 4=抖音)
        :param resource_type: 资源维度 ('video', 'user', 'keyword')
        :param resource_ids: 目标ID列表 (如 ['BV1xx', 'BV2xx'])
        :param task_name: 任务备注名
        :param params: 附加限制参数
        :return: 返回生成的雪花算法批次号 (字符串格式)
        """
        # 1. 使用雪花算法生成全局唯一且趋势递增的纯数字 ID (用作 batch_id)
        # 示例: 735628193746194432
        batch_id_numeric = snowflake_gen.generate_id()

        # PS: 将其转换为字符串形式
        # 原因：由于 64 位整型在传给前端浏览器时，会超出 JavaScript 的 MAX_SAFE_INTEGER (2^53 - 1)，
        # 导致前端拿到的数字末尾几位全变成 0 (精度丢失)。因此 API 交互中 64 位 ID 必须转为字符串。
        task_id_str = str(batch_id_numeric)

        # 2. 构造 MySQL 任务记录 (此时状态为 0: 待执行)
        new_task = CrawlerTask(
            task_id=task_id_str,
            task_name=task_name,
            platform_type=platform_type,
            resource_type=resource_type,
            # 将资源列表封装进 payload JSON 中，完美契合 MySQL 的 JSON 字段
            resource_payload={"ids": resource_ids},
            task_status=0,
            params=params
        )

        try:
            # 【本地事务防护】先将对象 add 到 Session 中，但先不 commit
            # 使用 flush 将 SQL 推送到 MySQL 检查约束，但不提交事务
            self.db.add(new_task)
            self.db.flush()
            # 3. 组装并向 Kafka 投递消息
            # 注意：这里的结构必须与我们之前写的 kafka_consumer 中的 `_process_task` 解析逻辑完全对齐！
            kafka_message = {
                "task_id": task_id_str,  # 透传给消费者的批次号 (batch_id)
                "task_name": task_name,
                "platform_type": platform_type,
                "resource_type": resource_type,
                "resource_payload": {"ids": resource_ids},
                "params": params or {},
                "submit_time": datetime.now().strftime("%Y-%m-%d %H:%M:%S")
            }
            # 使用单例 producer 发送消息
            await kafka_producer.send_task_message(topic="crawler_task_queue", message=kafka_message)
            # 4. 【关键步骤】只有当 Kafka 确定收到消息（没有抛出网络异常）后，我们才正式提交数据库事务
            self.db.commit()
            logger.info(
                f"[TaskService] 批次 {task_id_str} 已成功入库并下发至 Kafka。包含 {len(resource_ids)} 个采集目标。")
            return task_id_str

        except Exception as e:
            # 5. 异常回滚：如果 Kafka 宕机或网络异常，回滚 MySQL 事务
            # 彻底防止数据库里产生永远不会被消费者读取到的“幽灵待执行任务”
            self.db.rollback()
            logger.error(f"[TaskService] 任务创建失败，事务已回滚。原因: {str(e)}")
            raise e