from fastapi import APIRouter

# 导入外部操作接口和内部微服务接口
from data_collection_service.app.api.endpoints import bilibili_web, inner_bilibili, cookie_system, crawler_task


# 创建一个全局的 APIRouter 实例
router = APIRouter()

# 1. 挂载对外暴露的用户端采集接口
# 根据 ADR 文档规范，对外接口统一前缀为 /api/v1/crawler
router.include_router(
    bilibili_web.router,
    prefix="/api/v1/crawler/bilibili",
    tags=["Bilibili Data Collection"]
)
# 2. 挂载动态更新cookie接口
router.include_router(
    cookie_system.router,
    prefix="/api/v1/crawler",
    tags=["System Operations"]
)
# 3. 挂载对内暴露的微服务数据读取接口
# 注意：inner_bilibili.py 内部已经定义了 prefix="/inner"，所以这里不需要再加前缀
router.include_router(
    inner_bilibili.router,
    prefix="/api/v1/inner",
    tags=["Inner Bilibili Data Collection"]
    # 内部鉴权依赖:
    # dependencies=[Depends(verify_service_signature)]
)

# 挂载爬虫任务接口
router.include_router(
    crawler_task.router,
    tags=["Task Scheduler"]
)