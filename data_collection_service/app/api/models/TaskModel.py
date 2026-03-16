from pydantic import BaseModel, Field
from typing import Optional,List


class TaskCreateRequest(BaseModel):
    task_name: Optional[str] = Field(default="未命名时序任务", description="任务备注")
    # task_type: int = Field(..., description="1=全量评论, 2=画像快照, 3=指标监测")
    platform_type: int = Field(3, description="3=B站, 4=抖音")

    # 明确了资源类型和资源列表
    resource_type: str = Field(..., description="可选: 'video', 'user', 'keyword'")
    resource_ids: List[str] = Field(..., description="目标ID数组，例如 ['BV1xx', 'BV2xx'] 或 ['178360345']")

    params: Optional[dict] = Field(default_factory=dict, description="附加参数", json_schema_extra={"example": {}})

class UserProfileBindRequest(BaseModel):
    sys_uid: str = Field(..., description="系统内部用户UID")
    platform: str = Field(..., description="平台名称英文标识 (如: bilibili, douyin, tiktok)")
    profile_url: str = Field(..., description="用户填写的第三方平台主页链接")