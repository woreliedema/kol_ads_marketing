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
    Kafka 异步消费者守护进程 (Saga 管道拆分版)
    职责:
    1. [阶段A引擎]: 监听常规爬虫与视频下载队列 (crawler_task_queue)
    2. [阶段B引擎]: 监听长耗时 AI 处理队列 (bilibili_coze_asr_tasks)
    """
    def __init__(self):
        self.bootstrap_servers = os.getenv("KAFKA_BOOTSTRAP_SERVERS", "127.0.0.1:9092")
        self.topic_crawler = "crawler_task_queue"
        self.topic_asr = "bilibili_coze_asr_tasks"
        self.topic_analysis = "bilibili_multimodal_analysis_tasks"
        self.crawler_consumer = None
        self.asr_consumer = None
        self.analysis_consumer = None
        self._tasks = []

    async def start(self):
        """启动消费者并挂载到后台"""
        try:
            # 1. 爬虫与下载消费者 (I/O密集型)
            self.crawler_consumer = AIOKafkaConsumer(
                self.topic_crawler,
                bootstrap_servers=self.bootstrap_servers,
                group_id="crawler_worker_group",
                value_deserializer=lambda m: json.loads(m.decode('utf-8')),
                auto_offset_reset="earliest"
            )
            # 2. AI ASR 工作流消费者 (CPU/第三方API限制型)
            # 这里的 session_timeout 和 max_poll_interval 可以适度调长，应对 Coze 的 API 延迟
            self.asr_consumer = AIOKafkaConsumer(
                self.topic_asr,
                bootstrap_servers=self.bootstrap_servers,
                group_id="ai_asr_worker_group",  # 【关键】不同的消费组，实现算力隔离
                value_deserializer=lambda m: json.loads(m.decode('utf-8')),
                auto_offset_reset="earliest",
                max_poll_interval_ms=600000  # 允许单次处理最长 10 分钟
            )
            self.analysis_consumer = AIOKafkaConsumer(
                self.topic_analysis,
                bootstrap_servers=self.bootstrap_servers,
                group_id="ai_multimodal_worker_group",
                value_deserializer=lambda m: json.loads(m.decode('utf-8')),
                auto_offset_reset="earliest",
                max_poll_interval_ms=600000  # 10分钟防掉线
            )
            await self.crawler_consumer.start()
            await self.asr_consumer.start()
            await self.analysis_consumer.start()
            logger.info(f"🎧 [Kafka] 阶段 A (Crawler) 已监听: {self.topic_crawler}")
            logger.info(f"🎧 [Kafka] 阶段 B (AI ASR) 已监听: {self.topic_asr}")
            logger.info(f"🎧 [Kafka] 阶段 C (AI 多模态分析) 已监听: {self.topic_analysis}")
            # 放入后台事件循环
            self._tasks.append(asyncio.create_task(self._consume_crawler_loop()))
            self._tasks.append(asyncio.create_task(self._consume_asr_loop()))
            self._tasks.append(asyncio.create_task(self._consume_analysis_loop()))
        except Exception as e:
            logger.error(f"❌ [Kafka Consumer] 启动失败: {str(e)}")
            raise e

    async def stop(self):
        """优雅停机"""
        for task in self._tasks:
            task.cancel()
        if self.crawler_consumer:
            await self.crawler_consumer.stop()
        if self.asr_consumer:
            await self.asr_consumer.stop()
        if self.analysis_consumer:
            await self.analysis_consumer.stop()
        logger.info("🛑 [Kafka Consumer] 已安全关闭所有通道")

    async def _consume_crawler_loop(self):
        """持续消费死循环"""
        try:
            async for msg in self.crawler_consumer:
                payload = msg.value
                logger.info(f"📥 [Kafka Consumer A] 收到爬虫/下载批次: {payload.get('task_id')}")
                await self._process_crawler_task(payload)
        except asyncio.CancelledError:
            logger.info("⚠️ [Kafka Consumer A] 接收到停机信号，退出消费循环")
        except Exception as e:
            logger.error(f"❌ [Kafka Consumer A] 消费循环发生致命错误: {str(e)}")

    async def _process_crawler_task(self, payload: dict):
        """
        核心调度与状态机维护逻辑 (支持批量与雪花算法 batch_id 透传)
        """
        # 1. 解析 Kafka 载荷 (与 task_service.py 的拼装逻辑严格对齐)
        task_id = payload.get("task_id")  # 雪花算法生成的纯数字字符串 (作为 batch_id)
        platform_type = payload.get("platform_type", 3)
        resource_type = payload.get("resource_type")
        target_ids = payload.get("resource_payload", {}).get("ids", []) # 提取批量目标ID数组
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
                        "scrape_and_store_user_videos": task_service.collect_and_store_user_videos,
                        # 下载用户近30天的所有投稿视频，这里的下载方法执行完双写后，而是生产一条消息发给 Topic B
                        "scrape_and_store_video_to_minio": task_service.collect_and_store_video_to_minio
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

            # 状态机步骤 3:
            # 如果是纯爬虫任务，这里可以直接标为2(成功)。
            # 如果是下载任务，这里标为2则代表"阶段A已成功交接给阶段B"
            if success_count > 0:
                task_record.task_status = 2
                task_record.error_msg = f"阶段A执行完毕: {success_count}/{len(target_ids)}"
                db.commit()
                logger.info(f"🎉 [Task] 批次 {task_id} 执行结束，成功率 {success_count}/{len(target_ids)}，状态 -> 2(成功)")
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

    async def _consume_asr_loop(self):
        try:
            async for msg in self.asr_consumer:
                payload = msg.value
                logger.info(f" [Kafka Consumer B] 收到 AI 解析任务: {payload.get('coze_file_id')}")
                await self._process_asr_task(payload)
        except asyncio.CancelledError:
            pass
        except Exception as e:
            logger.error(f"❌ [Kafka Consumer B] 消费循环致命错误: {e}")


    async def _process_asr_task(self, payload: dict):
        """
        专门处理长耗时的 Coze ASR 调用和数据清洗
        """
        batch_id = payload.get("batch_id")
        bvid = payload.get("bvid")
        cid = payload.get("cid")
        coze_file_id = payload.get("coze_file_id")

        if not all([bvid, cid, coze_file_id]):
            logger.error(f"⚠️ [阶段 B] 参数不全，无法执行 ASR 任务: {payload}")
            return

        # 我们可以在这里实例化依赖，或者写在 BilibiliTaskService 里的一个新方法中
        try:
            async with ClickHouseManager.pool.connection() as ch_client:
                storage = StorageService(ch_client=ch_client)
                task_service = BilibiliTaskService(crawler=None, storage=storage)

                # 调用 task_service 中专门处理 ASR 的方法 (将在下面补充)
                await task_service.process_coze_asr_workflow(
                    bvid=bvid,
                    cid=cid,
                    coze_file_id=coze_file_id,
                    batch_id=batch_id
                )

        except Exception as e:
            logger.error(f"❌ [阶段 B] 执行 ASR 任务发生崩溃: {e}")

    async def _consume_analysis_loop(self):
        """阶段 C 的持续消费死循环"""
        try:
            async for msg in self.analysis_consumer:
                payload = msg.value
                logger.info(f"📥 [Kafka Consumer C] 收到 AI 多模态分析任务: {payload.get('bvid')}")

                # 🚨 极度关键：使用 create_task 后台运行！
                # 因为阶段 C 耗时极长，绝对不能在这里 await 阻塞 Kafka 消费者接收心跳！
                asyncio.create_task(self._process_analysis_task(payload))

        except asyncio.CancelledError:
            logger.info("⚠️ [Kafka Consumer C] 接收到停机信号，退出消费循环")
        except Exception as e:
            logger.error(f"❌ [Kafka Consumer C] 消费循环发生致命错误: {str(e)}")

    async def _process_analysis_task(self, payload: dict):
        """
        专门处理长耗时的多模态 AI 分析调用
        """
        batch_id = payload.get("batch_id", "unknown_batch")
        bvid = payload.get("bvid")
        cid = payload.get("cid")

        if not all([bvid, cid]):
            logger.error(f"⚠️ [阶段 C] 参数不全，无法执行多模态分析任务: {payload}")
            return

        try:
            # 独立申请 ClickHouse 客户端，实例化 BilibiliTaskService
            # async with ClickHouseManager.pool.connection() as ch_client:
            #     storage = StorageService(ch_client=ch_client)
            task_service = BilibiliTaskService(crawler=None, storage=None)

            # 调用阶段 C 核心逻辑
            is_success = await task_service.run_multimodal_analysis_loop(
                bvid=bvid,
                cid=cid,
                batch_id=batch_id
            )

            if is_success:
                logger.info(f"🎉 [阶段 C] 批次 {batch_id} - 视频 {bvid} 多模态分析链路全部圆满结束！")
            else:
                logger.warning(f"⚠️ [阶段 C] 批次 {batch_id} - 视频 {bvid} 分析失败，请排查日志。")

        except Exception as e:
            logger.error(f"❌ [阶段 C] 执行多模态任务发生崩溃: {e}")


# 导出单例，交由 main.py 管理其生命周期
kafka_consumer = KafkaConsumerManager()