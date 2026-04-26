from pydantic import BaseModel, Field
from typing import Optional

class CookieUpdatePayload(BaseModel):
    # cookie: str
    # timestamp: str
    # test: bool = False
    # message: str = ""
    cookie: str = Field(..., description="平台最新的 Cookie 字符串")
    timestamp: str = Field(..., description="采集时间戳")
    test: bool = Field(False, description="是否为测试回调")
    message: Optional[str] = Field(None, description="测试消息")

    # --- 边缘节点指纹字段 ---
    browser_id: str = Field(..., description="浏览器唯一ID (UUID)，用于隔离节点")
    platform: str = Field(..., description="所属平台，如 bilibili, douyin")