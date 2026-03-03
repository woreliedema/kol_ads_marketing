import uuid
from datetime import datetime
from fastapi import APIRouter, Request, HTTPException
from pydantic import BaseModel

from data_collection_service.app.api.models.APIResponseModel import ResponseModel, ErrorResponseModel
from data_collection_service.app.api.models.TaskModel import TaskCreateRequest
from data_collection_service.app.services.kafka_service import kafka_producer

router = APIRouter(tags=["Task Scheduler"])



@router.post("/task/create", response_model=ResponseModel, summary="异步提交爬虫任务")
async def create_crawler_task(request: Request, payload: TaskCreateRequest):
    """
    接收耗时爬虫任务请求，生成任务ID，打入 Kafka 后立即返回给前端。
    """
    try:
        # 1. 模拟生成全局唯一任务 ID (实战中这里通常是先 INSERT 到 MySQL 拿到自增ID)
        task_id = f"TASK_{uuid.uuid4().hex[:8].upper()}"

        # 2. 组装要发送给 Kafka 的消息载荷
        kafka_message = {
            "task_id": task_id,
            "task_type": payload.task_type,
            "platform_type": payload.platform_type,
            "target_id": payload.target_id,
            "submit_time": datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        }

        # 3. 极速异步投递到 Kafka 的采集任务专用 Topic
        # 根据 ADR 或数据采集文档，Topic 名字通常定义在环境变量或配置中
        topic_name = "crawler_task_queue"
        await kafka_producer.send_task_message(topic=topic_name, message=kafka_message)

        # 4. 毫秒级响应前端！无需等待爬虫执行完毕！
        return ResponseModel(
            code=200,
            router=request.url.path,
            data={
                "task_id": task_id,
                "status": "pending",
                "message": "任务已成功提交到后台队列，请稍后通过 /task/status 查询进度"
            }
        )

    except Exception as e:
        status_code = 500
        detail = ErrorResponseModel(code=status_code, router=request.url.path, message=str(e))
        raise HTTPException(status_code=status_code, detail=detail.dict())