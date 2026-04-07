import os
import json
import asyncio
import traceback
from aiokafka import AIOKafkaConsumer
from dotenv import load_dotenv
# 导入基础组件
from data_collection_service.crawlers.utils.logger import logger
from data_collection_service.app.db.session import SessionLocal
from data_collection_service.app.db.models import CrawlerTask
from data_collection_service.app.db.clickhouse import ClickHouseManager

# 导入业务服务层
from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler
from data_collection_service.app.services.storage_service import StorageService
from data_collection_service.app.services.bilibili_task_service import BilibiliTaskService

load_dotenv()


class KafkaConsumerManager:
    """
    Kafka 异步消费者守护进程 (时序批次化升级版)
    职责: 监听批次任务队列，组装服务依赖，循环调度抓取任务，透传 batch_id，维护 MySQL 状态机
    """
    def __init__(self):
        self.bootstrap_servers = os.getenv("KAFKA_BOOTSTRAP_SERVERS", "127.0.0.1:9092")
        self.topic = "crawler_task_queue"
        self.consumer = None
        self._consume_task = None

    async def start(self):
        """启动消费者并挂载到后台"""
        try:
            self.consumer = AIOKafkaConsumer(
                self.topic,
                bootstrap_servers=self.bootstrap_servers,
                group_id="crawler_worker_group",
                value_deserializer=lambda m: json.loads(m.decode('utf-8')),
                auto_offset_reset="earliest"
            )
            await self.consumer.start()
            logger.info(f"🎧 [Kafka Consumer] 已启动，正在监听 Topic: {self.topic}")
            # 放入后台事件循环
            self._consume_task = asyncio.create_task(self._consume_loop())
        except Exception as e:
            logger.error(f"❌ [Kafka Consumer] 启动失败: {str(e)}")
            raise e

    async def stop(self):
        """优雅停机"""
        if self._consume_task:
            self._consume_task.cancel()
        if self.consumer:
            await self.consumer.stop()
            logger.info("🛑 [Kafka Consumer] 已安全关闭")

    async def _consume_loop(self):
        """持续消费死循环"""
        try:
            async for msg in self.consumer:
                payload = msg.value
                task_id = payload.get('task_id')
                logger.info(f"📥 [Kafka Consumer] 收到批次任务: {task_id}")
                await self._process_task(payload)
        except asyncio.CancelledError:
            logger.info("⚠️ [Kafka Consumer] 接收到停机信号，退出消费循环")
        except Exception as e:
            logger.error(f"❌ [Kafka Consumer] 消费循环发生致命错误: {str(e)}")

    async def _process_task(self, payload: dict):
        """
        核心调度与状态机维护逻辑 (支持批量与雪花算法 batch_id 透传)
        """
        # 1. 解析 Kafka 载荷 (与 task_service.py 的拼装逻辑严格对齐)
        task_id = payload.get("task_id")  # 雪花算法生成的纯数字字符串 (作为 batch_id)
        # task_type = payload.get("task_type")
        platform_type = payload.get("platform_type", 3)
        resource_type = payload.get("resource_type")  # 'video', 'user', 'keyword'
        resource_payload = payload.get("resource_payload", {})
        target_ids = resource_payload.get("ids", [])  # 提取批量目标ID数组
        if not target_ids:
            logger.warning(f"⚠️ [Task] 批次 {task_id} 没有有效的抓取目标(ids为空)，跳过。")
            return
        # 独立申请 MySQL 事务连接
        db = SessionLocal()
        try:
            # 状态机步骤 1: 认领任务 (0 -> 1)
            task_record = db.query(CrawlerTask).filter(CrawlerTask.task_id == task_id).first()
            if not task_record or task_record.task_status != 0:
                logger.warning(f"⚠️ [Task] 批次 {task_id} 不存在或已被处理，跳过。")
                return
            task_record.task_status = 1
            db.commit()
            logger.info(f"🔄 [Task] 批次 {task_id} 状态 -> 1(执行中)，包含 {len(target_ids)} 个目标，开始装配链路...")
            # 状态机步骤 2: 装配组件并循环执行核心链路
            success_count = 0
            if platform_type == 3:  # 平台：B站
                # 从异步全局连接池中安全“借用”一个连接
                # 这样写保证了这批任务执行完毕后，连接会自动归还给连接池
                async with ClickHouseManager.pool.connection() as ch_client:
                    # 注入依赖 (传入异步连接)
                    storage = StorageService(ch_client=ch_client)
                    crawler_instance = BilibiliWebCrawler()
                    task_service = BilibiliTaskService(crawler=crawler_instance, storage=storage)

                    # 动态路由映射表 (Action Map)
                    bilibili_action_map = {
                        "scrape_and_store_video_comments": task_service.collect_and_store_video_comments,
                        "scrape_and_store_user_info": task_service.collect_and_store_user_info,
                        "scrape_and_store_user_relation": task_service.collect_and_store_user_relation,
                        "scrape_and_store_video_info": task_service.collect_and_store_video_info,
                        # 新增的爬取用户近一年所有投稿视频作品信息
                        'scrape_and_store_user_videos': task_service.collect_and_store_user_videos
                    }
                    # 获取对应的处理函数
                    action_handler = bilibili_action_map.get(resource_type)

                    if not action_handler:
                        # 如果传了一个未知的 resource_type，直接报错退出
                        raise ValueError(f"未知的 resource_type: {resource_type}，无法匹配底层处理函数")

                    # 遍历处理目标
                    for target_id in target_ids:
                        try:
                            logger.info(f"⏳ [Task:{task_id}] 动态执行动作 [{resource_type}], 目标ID: {target_id}...")

                            # 动态调用：不论是评论还是画像，因为入参形式统一，直接调用 action_handler 即可！
                            is_ok = await action_handler(target_id, task_id)
                            if is_ok:
                                success_count += 1
                            else:
                                logger.warning(f"⚠️ [Task:{task_id}] 目标 {target_id} 业务层返回采集失败。")
                            # 强制加入休眠，防止触发高频封控
                            await asyncio.sleep(1.5)

                        except Exception as loop_e:
                            logger.error(f"❌ [Task:{task_id}] 抓取目标 {target_id} 时发生异常: {str(loop_e)}")
                            continue

            elif platform_type == 1:  # 预留：抖音平台
                pass
            elif platform_type == 2:  # 预留：Tiktok平台
                pass

            # 状态机步骤 3: 依据批次整体 success_count 更新 MySQL 状态
            if success_count > 0:
                task_record.task_status = 2
                task_record.error_msg = f"部分或全部成功: {success_count}/{len(target_ids)}"
                db.commit()
                logger.info(
                    f"🎉 [Task] 批次 {task_id} 执行结束，成功率 {success_count}/{len(target_ids)}，状态 -> 2(成功)")
            else:
                task_record.task_status = 3
                task_record.error_msg = f"该批次共 {len(target_ids)} 个目标全部采集失败"
                db.commit()
                logger.warning(f"⚠️ [Task] 批次 {task_id} 全军覆没，状态 -> 3(失败)")
        except Exception as e:
            # 状态机步骤 4: 意外崩溃兜底回滚
            db.rollback()
            error_stack = traceback.format_exc()
            logger.error(f"❌ [Task] 批次 {task_id} 发生未捕获的系统崩溃:\n{error_stack}")
            try:
                task_record = db.query(CrawlerTask).filter(CrawlerTask.task_id == task_id).first()
                if task_record:
                    task_record.task_status = 3
                    task_record.error_msg = str(e)[:500]
                    db.commit()
            except Exception as inner_e:
                logger.error(f"❌ [Task] 崩溃兜底更新 MySQL 失败: {inner_e}")
        finally:
            # 安全释放 MySQL 事务连接
            db.close()


# 导出单例，交由 main.py 管理其生命周期
kafka_consumer = KafkaConsumerManager()