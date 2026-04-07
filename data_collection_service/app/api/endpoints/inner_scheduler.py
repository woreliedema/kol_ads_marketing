from fastapi import APIRouter, Depends, Request, HTTPException
from sqlalchemy.orm import Session
from sqlalchemy.dialects.mysql import insert
from datetime import datetime
from asynch.connection import Connection

from data_collection_service.app.api.models.APIResponseModel import ResponseModel, ErrorResponseModel
from data_collection_service.crawlers.utils.extract_uid import extract_target_id_from_url
from data_collection_service.app.db.session import get_db
from data_collection_service.app.db.models import CrawlerTarget
from data_collection_service.app.services.data_proxy_service import DataProxyService
from data_collection_service.app.db.clickhouse import get_ch_client
from data_collection_service.app.services.query_service import QueryService


router = APIRouter()



@router.post("/inner/target/register", response_model=ResponseModel)
async def register_crawler_target(
        uid: str,
        resource_type: str,  # 例如: 'scrape_user_info' 或 'scrape_and_store_video_comments' 或 'scrape_and_store_user_videos'
        target_id: str,  # 例如: '123456'(UID) 或 'BV1xx'(BV号)
        platform_type: int = 3,
        interval_minutes: int = 1440,  # 24小时
        db: Session = Depends(get_db)
):
    """
    供业务方调用：将任何维度的抓取目标注册到时序总表中。
    注册后会立即触发第一次抓取，随后每隔 interval_minutes 循环触发。
    """
    try:
        stmt = insert(CrawlerTarget).values(
            platform_type=platform_type,
            uid=uid,  # 归属用户
            resource_type=resource_type,  # 模块
            target_id=target_id,  # 资源ID
            cron_interval_minutes=interval_minutes,
            is_active=True,
            next_run_time=datetime.now()  # 入库即刻触发第一次
        )

        # UPSERT 逻辑：如果这个资源ID之前配过，只是想修改执行频率或重新激活
        stmt = stmt.on_duplicate_key_update(
            is_active=True,
            next_run_time=datetime.now(),
            cron_interval_minutes=interval_minutes
        )

        db.execute(stmt)
        db.commit()

        return ResponseModel(code=200, data={"target_id": target_id, "status": "registered & scheduled"})

    except Exception as e:
        db.rollback()
        raise e

@router.get("/inner/tools/parse_profile_url")
async def parse_profile_url(request: Request,platform: str, url: str):
    """
    提供给用户中心的工具接口：输入主页链接，秒级返回第三方平台的真实 UID
    """
    try:
        uid = extract_target_id_from_url(platform, url)

        if not uid:
            return ErrorResponseModel(code=404,
                                      router=request.url.path,
                                      message="Invalid URL")
        return ResponseModel(
            code=200,
            router=request.url.path,
            data={
                "code":0,
                "message":"success",
                "uid":uid
            }
        )
    except ValueError as e:
        status_code = 500
        detail = ErrorResponseModel(
            code=status_code,
            router=request.url.path,
            message=f"解析失败: {str(e)}"
        ).dict()
        # 如果用户乱填链接，直接抛错，用户中心可以直接把这个错误提示给前端
        raise HTTPException(status_code=status_code, detail=detail)


@router.get("/inner/data/profile/{platform}/{uid}", response_model=ResponseModel)
async def check_profile_freshness(request: Request, platform: str, uid: str, ch_client: Connection = Depends(get_ch_client)):
    """
    内部接口：探测KOL画像数据的鲜活度
    """
    # 平台名称标准化防呆设计
    platform = platform.lower()
    if platform not in ["douyin", "bilibili", "tiktok", "xiaohongshu"]:
        detail = ErrorResponseModel(
            code=400,
            router=request.url.path,
            message="Unsupported platform"
        ).dict()
        raise HTTPException(status_code=400, detail=detail)

    # 核心查询逻辑委托给 Service 层
    query_service = QueryService(ch_client=ch_client)
    is_fresh, data = await DataProxyService.check_and_fetch_fresh_profile(
        platform, uid, query_service
    )

    if not is_fresh or not data:
        # 对应 Go 代码中的 resp.StatusCode == http.StatusNotFound
        detail = ErrorResponseModel(
            code=400,
            router=request.url.path,
            message="Profile not found or expired"
        ).dict()
        raise HTTPException(status_code=404, detail=detail)

    return ResponseModel(
        code=200,
        router=request.url.path,
        data=data
    )