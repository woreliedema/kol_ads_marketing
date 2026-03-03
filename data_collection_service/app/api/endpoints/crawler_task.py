from fastapi import APIRouter, Request, HTTPException, Depends
from sqlalchemy.orm import Session

from data_collection_service.app.api.models.APIResponseModel import ResponseModel, ErrorResponseModel
from data_collection_service.app.api.models.TaskModel import TaskCreateRequest
from data_collection_service.app.db.session import get_db
from data_collection_service.app.services.task_service import TaskService

router = APIRouter()


@router.post("/task/create", response_model=ResponseModel, summary="创建并异步投递爬虫任务")
async def create_crawler_task(
        request: Request,
        payload: TaskCreateRequest,  # Pydantic 模型自动做参数校验
        db: Session = Depends(get_db)  # 自动获取并管理 MySQL 连接
):
    """
    **用途**: 供前台或外部品牌方调用，将长耗时爬虫任务异步化。
    """
    try:
        # 1. 实例化 Service 并注入数据库 Session
        task_service = TaskService(db_session=db)

        # 2. 调用核心业务逻辑 (雪花算法生成ID -> 落库MySQL -> 发送Kafka -> 提交事务)
        task_id = await task_service.create_and_dispatch_task(
            platform_type=payload.platform_type,
            resource_type=payload.resource_type,
            resource_ids=payload.resource_ids,
            task_name=payload.task_name,
            params=payload.params
        )

        # 3. 封装标准成功响应
        return ResponseModel(
            code=200,
            router=request.url.path,
            data={
                "task_id": task_id,
                "status": "pending",
                "message": "任务已受理并进入后台调度队列"
            }
        )

    except Exception as e:
        status_code = 500
        detail = ErrorResponseModel(
            code=status_code,
            router=request.url.path,
            message=f"任务创建失败: {str(e)}"
        )
        raise HTTPException(status_code=status_code, detail=detail.dict())