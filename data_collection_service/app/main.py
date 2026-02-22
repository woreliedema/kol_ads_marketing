import os
from fastapi import FastAPI
from dotenv import load_dotenv
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager

# 导入路由文件:b站爬虫、cookie刷新、clickhouse连接
from data_collection_service.app.api.endpoints import bilibili_web,cookie_system
from data_collection_service.app.core.nacos_config import nacos_registry
from data_collection_service.app.db.clickhouse import ClickHouseManager
# 1. Nacos 连接配置
# (为了代码健壮性，这里使用 os.getenv 并结合本地 .env 文件读取环境变量，赋予默认值以匹配本地开发)
load_dotenv()
NACOS_IP = os.getenv("NACOS_IP", "127.0.0.1")
NACOS_PORT = int(os.getenv("NACOS_PORT", 8848))
SERVICE_NAME = os.getenv("SERVICE_NAME", "data-collection-service")
SERVICE_IP = os.getenv("SERVICE_IP", "127.0.0.1")
SERVICE_PORT = int(os.getenv("SERVICE_PORT", 8000))

# 2. FastAPI 生命周期事件：启动时注册到 Nacos

# 导入封装好的 Nacos 注册实例
@asynccontextmanager
async def lifespan(app: FastAPI):
    # 1. 启动时：注册服务
    await nacos_registry.register()
    # 2. 初始化 ClickHouse 数据库连接池
    # 根据 docker-compose.yml 提取环境变量，若本地独立运行则默认 127.0.0.1
    ch_host = os.getenv("CLICKHOUSE_HOST", "127.0.0.1")
    ch_port = int(os.getenv("CLICKHOUSE_PORT", 9000))
    ch_user = os.getenv("CLICKHOUSE_USER", "default")
    ch_password = os.getenv("CLICKHOUSE_PASSWORD", "")

    ClickHouseManager.init_db(
        host=ch_host,
        port=ch_port,
        user=ch_user,
        password=ch_password,
        database="ods"
    )
    # 交出控制权给 FastAPI 应用
    yield

    # 3. 断开 ClickHouse 数据库连接
    ClickHouseManager.close_db()
    # 4. 注销 Nacos 服务
    await nacos_registry.deregister()

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
# 4. 路由注册中心 (严格遵循文档规范：统一前缀 /api/v1/crawler)
# ==========================================

# 挂载 B 站的采集接口
app.include_router(
    bilibili_web.router,
    prefix="/api/v1/crawler",     # 所有此路由下的接口都会加上该前缀
    tags=["Bilibili Data Collection"]
)

# 挂载 系统通用/内部接口 (包含 RESTful 风格的 Cookie 刷新 webhook)
app.include_router(
    cookie_system.router,
    prefix="/api/v1/crawler",
    tags=["System Operations"]
)

if __name__ == "__main__":
    import uvicorn
    # 本地调试启动命令，对应文档中的 uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
    uvicorn.run("data_collection_service.app.main:app", host="0.0.0.0", port=8000, reload=True)