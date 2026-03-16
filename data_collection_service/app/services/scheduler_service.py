import asyncio
from datetime import datetime, timedelta
from data_collection_service.crawlers.utils.logger import logger
from data_collection_service.app.db.session import SessionLocal
from data_collection_service.app.db.models import CrawlerTarget
from data_collection_service.app.services.task_service import TaskService


class SchedulerDaemon:
    """定时任务引擎：扫描目标总表，按批次生成快照任务"""
    def __init__(self):
        self._running = False
        self._task = None

    async def start(self):
        self._running = True
        self._task = asyncio.create_task(self._schedule_loop())
        logger.info(" [Scheduler] 定时扫描引擎已启动...")

    async def stop(self):
        self._running = False
        if self._task:
            self._task.cancel()
            logger.info(" [Scheduler] 定时扫描引擎已关闭。")

    async def _schedule_loop(self):
        while self._running:
            try:
                await self._check_and_dispatch()
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f" [Scheduler] 扫描异常: {str(e)}")

            # 每隔 60 秒巡检一次
            await asyncio.sleep(60)

    async def _check_and_dispatch(self):
        db = SessionLocal()
        try:
            now = datetime.now()
            # 1. 查询所有到期需要执行的任务
            due_targets = db.query(CrawlerTarget).filter(
                CrawlerTarget.is_active == True,
                CrawlerTarget.next_run_time <= now
            ).limit(500).all()  # 一次最多捞 500 个防 OOM

            if not due_targets:
                return

            # 2. 按照 (platform_type, resource_type) 将任务分组
            grouped_targets = {}
            for t in due_targets:
                key = (t.platform_type, t.resource_type)
                if key not in grouped_targets:
                    grouped_targets[key] = []
                grouped_targets[key].append(t)

            task_service = TaskService(db_session=db)

            # 3. 批量生成执行快照 (CrawlerTask) 并发送给 Kafka
            for (p_type, r_type), targets in grouped_targets.items():
                target_ids = [t.target_id for t in targets]

                # 复用我们写好的绝杀方法！它会自动生成 batch_id, 写入 MySQL 并发给 Kafka
                batch_id = await task_service.create_and_dispatch_task(
                    platform_type=p_type,
                    resource_type=r_type,
                    resource_ids=target_ids,
                    task_name=f"Cron调度_{r_type}_{now.strftime('%H:%M')}"
                )

                # 4. 更新总表的 next_run_time (计算下次执行时间)
                for t in targets:
                    t.last_run_time = now
                    t.next_run_time = now + timedelta(minutes=t.cron_interval_minutes)

                logger.info(f" [Scheduler] 触发 {len(target_ids)} 个 {r_type} 定时任务，生成批次号: {batch_id}")

            db.commit()

        except Exception as e:
            db.rollback()
            raise e
        finally:
            db.close()


scheduler_daemon = SchedulerDaemon()