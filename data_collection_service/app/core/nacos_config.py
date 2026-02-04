
from fastapi import FastAPI
from contextlib import asynccontextmanager
from v2.nacos import NacosNamingService, ClientConfigBuilder, RegisterInstanceParam



async def register_service():

    # 1. 构建 Nacos 客户端配置（新版必须先配置，旧版是直接传地址）
    # Nacos 服务端地址
    # 命名空间（默认public，根据你的实际情况修改）
    # 超时时间（毫秒）
    client_config = ClientConfigBuilder().server_address("localhost:8848").namespace_id("public").timeout_ms(5000).build()
    # 2. 实例化 Nacos Naming 服务（替代旧版的 NacosClient）
    naming_service = await NacosNamingService.create_naming_service(client_config)
    # 3. 注册服务实例（方法名和参数略有调整，核心逻辑一致）
    instance_param = RegisterInstanceParam(
        service_name="data-collection-service", # 服务名（和旧版一致）
        ip="localhost",  # 服务IP
        port=8000, # 服务端口
        cluster_name="DEFAULT", # 集群名（和旧版一致）
        group_name="DEFAULT_GROUP",  # 建议显式添加组名，默认为 DEFAULT_GROUP
        # 可选：添加权重、健康检查等参数
        healthy=True,
        weight=1.0
    )
    await naming_service.register_instance(instance_param)

    print("服务成功注册到 Nacos！")

# 2. 使用 lifespan 替代过时的 on_event
@asynccontextmanager
async def lifespan(app: FastAPI):
    try:
        await register_service()
    except Exception as e:
        print(f"Nacos 注册失败: {e}")
    yield

# 3. 初始化 FastAPI 并绑定 lifespan
app = FastAPI(lifespan=lifespan)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="localhost", port=8000)