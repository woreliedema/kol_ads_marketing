import os
from fastapi import Header, HTTPException, Request
from dotenv import load_dotenv

from data_collection_service.crawlers.utils.logger import logger

load_dotenv()

# 1. 启动时从环境变量或 .env 读取内部通讯秘钥 (与 Go 保持绝对一致)
INTERNAL_SECRET_KEY = os.getenv("INTERNAL_SECRET_KEY")

if not INTERNAL_SECRET_KEY:
    logger.error("致命错误：未配置 INTERNAL_SECRET_KEY，无法启动内部服务防线！")
    # 为了保证安全，如果没有配置秘钥，你可以选择直接抛出异常阻止程序启动
    # raise RuntimeError("Missing INTERNAL_SECRET_KEY in environment variables")


async def verify_internal_secret(
        request: Request,
        # 2. FastAPI 会自动去 Header 里找 X-Internal-Secret
        x_internal_secret: str = Header(None, alias="X-Internal-Secret")
):
    """
    FastAPI 依赖注入：内部微服务通信专属鉴权
    """
    if not x_internal_secret or x_internal_secret != INTERNAL_SECRET_KEY:
        client_ip = request.client.host if request.client else "Unknown IP"
        logger.warning(f"🚨 拦截到非法的内部接口调用请求！来源 IP: {client_ip}")

        # 抛出 403 权限不足
        raise HTTPException(
            status_code=403,
            detail="非法内部调用：凭证不匹配"
        )

    # 秘钥匹配，放行！
    return x_internal_secret