from fastapi import APIRouter, Query, Request, HTTPException, Depends
from clickhouse_driver import Client

# 导入项目中定义的标准响应模型
from data_collection_service.app.api.models.APIResponseModel import ResponseModel, ErrorResponseModel
# 导入数据库会话依赖 (假设路径如下，负责 yield ch_client)
from data_collection_service.app.db.clickhouse import get_ch_client
# 导入重构后的服务层
from data_collection_service.app.services.query_service import QueryService

router = APIRouter()


@router.get("/kol/data", response_model=ResponseModel, summary="查询红人基础数据(供报价引擎)")
async def inner_fetch_kol_data(
        request: Request,
        user_id: str = Query(..., examples=["2687303"], description="B站用户UID"),
        ch_client: Client = Depends(get_ch_client)
):
    try:
        # 依赖注入：初始化 Service
        data_service = QueryService(ch_client=ch_client)

        # 调用核心业务逻辑
        data = data_service.get_kol_base_data(user_id)

        if not data:
            return ResponseModel(
                code=404,
                router=request.url.path,
                data={"message": f"KOL data for user {user_id} not found."}
            )

        return ResponseModel(
            code=200,
            router=request.url.path,
            data=data
        )
    except Exception as e:
        status_code = 500
        detail = ErrorResponseModel(
            code=status_code,
            router=request.url.path,
            message="Internal Server Error during KOL data extraction."
        )
        raise HTTPException(status_code=status_code, detail=detail.dict())


@router.get("/video/data", response_model=ResponseModel, summary="查询视频数据(供数据监控服务)")
async def inner_fetch_video_data(
        request: Request,
        video_id: str = Query(..., examples=["BV1SEBxBSE8Q"], description="作品BV号"),
        ch_client: Client = Depends(get_ch_client)
):
    try:
        data_service = QueryService(ch_client=ch_client)
        data = data_service.get_video_metrics_data(video_id)

        if not data:
            return ResponseModel(
                code=404,
                router=request.url.path,
                data={"message": f"Video metrics for {video_id} not found."}
            )

        return ResponseModel(
            code=200,
            router=request.url.path,
            data=data
        )
    except Exception as e:
        status_code = 500
        detail = ErrorResponseModel(
            code=status_code,
            router=request.url.path,
            message="Internal Server Error during video metrics extraction."
        )
        raise HTTPException(status_code=status_code, detail=detail.dict())