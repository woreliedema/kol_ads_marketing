import os
import asyncio
from fastapi import FastAPI
from dotenv import load_dotenv
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager

# 导入路由文件:b站爬虫、clickhouse连接
from data_collection_service.app.api.router import router
from data_collection_service.app.core.nacos_config import nacos_registry
from data_collection_service.app.db.clickhouse import ClickHouseManager
from data_collection_service.crawlers.utils.logger import logger
from data_collection_service.app.services.kafka_service import kafka_producer
from data_collection_service.app.services.kafka_consumer import kafka_consumer
from data_collection_service.app.services.scheduler_service import scheduler_daemon
from data_collection_service.app.db.redis_client import redis_client_mgr

# 1. Nacos 连接配置
# (为了代码健壮性，这里使用 os.getenv 并结合本地 .env 文件读取环境变量，赋予默认值以匹配本地开发)
load_dotenv()

# 2. FastAPI 生命周期事件：启动时注册到 Nacos

# 导入封装好的 Nacos 注册实例
@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    管理 FastAPI 的启动与关闭事件
    最佳实践架构原则：
    - 启动：底层资源(DB) -> 生产者(Kafka Producer) -> 服务发现(Nacos) -> 流量入口(Consumer/Scheduler)
    - 停机：切断流量入口(Scheduler/Consumer) -> 注销服务(Nacos) -> 断开底层(Producer/DB)
    """
    logger.info("🚀 ================== 数据采集服务准备启动 ================== 🚀")
    try:
        # 步骤 1: 初始化 ClickHouse 数据库连接池
        ch_host = os.getenv("CLICKHOUSE_HOST", "127.0.0.1")
        ch_port = int(os.getenv("CLICKHOUSE_PORT", 9000))
        ch_user = os.getenv("CLICKHOUSE_USER", "default")
        ch_password = os.getenv("CLICKHOUSE_PASSWORD", "")

        await ClickHouseManager.init_db(
            host=ch_host,
            port=ch_port,
            user=ch_user,
            password=ch_password,
            database="ods"  # or: os.getenv("CLICKHOUSE_DB", "ods")
        )
        logger.info("[Init] ClickHouse 数据库连接初始化成功。")
        # 初始化 Redis 异步连接池 (支撑高并发热缓存)
        await redis_client_mgr.init_pool()
        logger.info("[Init] Redis 热缓存连接池初始化成功。")

        # 步骤 2: 启动 Kafka 生产者
        await kafka_producer.start()
        logger.info("[Init] Kafka 生产者启动成功。")
        # 步骤 3: 注册到 Nacos 注册中心 (服务就绪，开始接收流量)
        await nacos_registry.register()
        logger.info("[Init] Nacos 服务注册成功。")
        # 步骤 3: 启动 Kafka 消费
        await kafka_consumer.start()
        logger.info("[Init] Kafka 消费者后台守护进程启动成功。")

        await scheduler_daemon.start()
        logger.info("[Init] 定时扫描引擎启动成功。")

        logger.info("✅ ================== 数据采集服务启动完成 ================== ✅ ")

        # 交出控制权给 FastAPI 应用
        yield

    except Exception as e:
        logger.error(f"❌ 服务启动过程发生严重异常: {str(e)}")
        # 抛出异常阻止服务在半残状态下启动
        raise e

    finally:
        logger.info("🛑 ================== 数据采集服务正在关闭 ================== 🛑")

        # 步骤 1: 停机第一步：停止定时任务引擎 (从源头掐断自身产生的新任务)
        try:
            await scheduler_daemon.stop()
            logger.info("[Cleanup] 定时扫描引擎已安全关闭。")
        except Exception as e:
            logger.error(f"[Cleanup] 定时扫描引擎关闭异常: {str(e)}")

        # 步骤 2. 停止 Kafka 消费者 (不再从队列拉取新任务，允许正在执行的任务跑完)
        try:
            await kafka_consumer.stop()
            logger.info("[Cleanup] Kafka 消费者已安全关闭。")
        except Exception as e:
            logger.error(f"[Cleanup] Kafka 消费者关闭异常: {str(e)}")

        # 步骤 3: 注销 Nacos 服务 (告诉 API 网关/其他微服务不要再派发新 HTTP 流量)
        try:
            await nacos_registry.deregister()
            logger.info("[Cleanup] Nacos 服务注销成功。")
        except Exception as e:
            logger.error(f"[Cleanup] Nacos 注销异常: {str(e)}")

        # 优雅停机缓冲期：给当前还在处理中的请求留出 1 秒收尾时间
        await asyncio.sleep(1)

        # 步骤 4. 安全关闭 Kafka 生产者 (确保状态更新等最后一条消息被 flush 到 Broker)
        try:
            await kafka_producer.stop()
            logger.info("[Cleanup] Kafka 生产者已安全关闭。")
        except Exception as e:
            logger.error(f"[Cleanup] Kafka 生产者关闭异常: {str(e)}")

        # 步骤 5: 断开 ClickHouse 等、redis底层数据库连接
        try:
            await ClickHouseManager.close_db()
            logger.info("[Cleanup] ClickHouse 数据库连接已安全关闭。")
        except Exception as e:
            logger.error(f"[Cleanup] ClickHouse 关闭异常: {str(e)}")

        try:
            await redis_client_mgr.close_pool()
            logger.info("[Cleanup] Redis 热缓存连接池已安全关闭。")
        except Exception as e:
            logger.error(f"[Cleanup] Redis 关闭异常: {str(e)}")

        logger.info("👋 ================== 数据采集服务已安全退出 ================== 👋")

# 3. 初始化 FastAPI
app = FastAPI(
    title="Data Collection Service",
    description="数据采集微服务",
    version="1.0.0",
    lifespan=lifespan
)

# [新增] 配置 CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # 生产环境建议改为具体的扩展 ID 或域名，开发环境可以用 "*"
    allow_credentials=True,
    allow_methods=["*"],  # 允许所有方法，包括 OPTIONS, PUT
    allow_headers=["*"],
)

# ==========================================
# 4. 路由注册中心 (严格遵循文档规范：统一前缀 /api/v1)
# ==========================================
app.include_router(router)

if __name__ == "__main__":
    import uvicorn
    # 本地调试启动命令，对应文档中的 uvicorn data_collection_service.app.main:app --host 0.0.0.0 --port 8000
    uvicorn.run("data_collection_service.app.main:app", host="0.0.0.0", port=8000, reload=False)