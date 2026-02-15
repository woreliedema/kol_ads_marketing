import os
from fastapi import FastAPI

# 导入路由文件:b站爬虫、cookie刷新
from data_collection_service.app.api.endpoints import bilibili_web,cookie_system
# 假设您将刚刚写的 cookie 刷新接口放在了 system.py 里


# 1. 初始化 FastAPI 实例
app = FastAPI(
    title="Data Collection Service",
    description="数据采集微服务 (包含抖音、TikTok、B站等UCG平台数据采集与配置)",
    version="1.0.0"
)

# 2. Nacos 连接配置
# (为了代码健壮性，这里建议使用 os.getenv 读取环境变量，赋予默认值以匹配本地开发)
NACOS_IP = os.getenv("NACOS_IP", "127.0.0.1")
NACOS_PORT = int(os.getenv("NACOS_PORT", 8848))
SERVICE_NAME = os.getenv("SERVICE_NAME", "data-collection-service")
SERVICE_IP = os.getenv("SERVICE_IP", "127.0.0.1")
SERVICE_PORT = int(os.getenv("SERVICE_PORT", 8000))

# 3. FastAPI 生命周期事件：启动时注册到 Nacos
from fastapi import FastAPI
from contextlib import asynccontextmanager
import uvicorn
# 导入封装好的 Nacos 注册实例
from data_collection_service.app.core.nacos_config import nacos_registry



@asynccontextmanager
async def lifespan(app: FastAPI):
    # 1. 启动时：注册服务
    await nacos_registry.register()
    yield
    # 2. 关闭时：注销服务 (可选，但推荐)
    await nacos_registry.deregister()

# 初始化 FastAPI
app = FastAPI(
    title="Data Collection Service",
    description="数据采集微服务",
    version="1.0.0",
    lifespan=lifespan
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
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=True)