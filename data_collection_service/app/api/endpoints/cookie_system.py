from fastapi import APIRouter, Body, Request, HTTPException, Path
from pydantic import BaseModel


from data_collection_service.app.api.models.APIResponseModel import ResponseModel, ErrorResponseModel
from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler
# 导入其他爬虫...

router = APIRouter()

# 此时 Payload 就不需要传 service 字段了，因为它已经在 URL 路径里了
class CookieUpdatePayload(BaseModel):
    cookie: str
    timestamp: str
    test: bool = False
    message: str = ""

@router.put("/platforms/{platform_name}/cookie", response_model=ResponseModel, summary="更新平台Cookie Webhook")
async def update_platform_cookie(
    request: Request,
    platform_name: str = Path(..., description="平台名称，如: bilibili, douyin, tiktok"),
    payload: CookieUpdatePayload = Body(...)
):
    """接收 Chrome 拓展发送的最新 Cookie 并更新对应平台的配置"""
    try:
        # 如果是测试回调，直接返回成功
        if payload.test:
            return ResponseModel(code=200, router=request.url.path, data={"message": "Test successful"})

        # 根据路径参数 platform_name 分发逻辑
        if platform_name == "bilibili":
            crawler = BilibiliWebCrawler()
            await crawler.update_cookie(payload.cookie)
        # elif platform_name == "douyin":
        #     crawler = DouyinWebCrawler()
        #     await crawler.update_cookie(payload.cookie)
        # elif platform_name == "tiktok":
        #     crawler = TikTokWebCrawler()
        #     await crawler.update_cookie(payload.cookie)
        else:
            raise ValueError(f"不支持的平台: {platform_name}")

        return ResponseModel(
            code=200,
            router=request.url.path,
            data={"message": f"{platform_name} Cookie updated successfully"}
        )

    except Exception as e:
        detail = ErrorResponseModel(code=400, router=request.url.path, message=str(e))
        raise HTTPException(status_code=400, detail=detail.dict())