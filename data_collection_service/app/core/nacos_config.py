from dotenv import load_dotenv
from v2.nacos import NacosNamingService, ClientConfigBuilder, RegisterInstanceParam
import os

from data_collection_service.crawlers.utils.logger import logger
# 加载环境变量
load_dotenv()
# 配置日志

class NacosRegistry:
    def __init__(self):
        self.naming_service = None
        # Nacos 配置信息 (建议从环境变量读取，这里保持你原有的硬编码或默认值)
        nacos_host = os.getenv("NACOS_HOST", "localhost")
        nacos_port = os.getenv("NACOS_PORT", "8848")
        self.server_address = f"{nacos_host}:{nacos_port}"

        self.namespace_id = os.getenv("NACOS_NAMESPACE", "public")
        self.service_name = os.getenv("SERVICE_NAME", "data-collection-service")

        self.ip = os.getenv("SERVICE_IP", "127.0.0.1")
        self.port = int(os.getenv("SERVICE_PORT", 8000))

        self.group_name = "DEFAULT_GROUP"
        self.cluster_name = "DEFAULT"

    async def register(self):
        """注册服务到 Nacos"""
        try:
            # 1. 构建客户端配置
            client_config = ClientConfigBuilder().server_address(self.server_address).namespace_id(self.namespace_id).timeout_ms(5000).build()

            # 2. 创建 Naming 服务
            self.naming_service = await NacosNamingService.create_naming_service(client_config)

            # 3. 注册实例
            instance_param = RegisterInstanceParam(
                service_name=self.service_name,
                ip=self.ip,
                port=self.port,
                cluster_name=self.cluster_name,
                group_name=self.group_name,
                healthy=True,
                weight=1.0,
                # 设置为临时实例，服务挂掉时 Nacos 会基于心跳超时自动剔除节点
                ephemeral=True
            )
            await self.naming_service.register_instance(instance_param)
            logger.info(f"[Nacos] 服务已成功注册到 Nacos: {self.service_name}@{self.ip}:{self.port}")
        except Exception as e:
            logger.error(f"[Nacos] 注册失败: {e}")
            # 根据需求决定是否抛出异常阻断启动
            raise e

    async def deregister(self):
        """从 Nacos 注销服务"""
        if self.naming_service:
            try:
                # v2.nacos 库通常提供 deregister_instance 方法，参数与注册类似
                # 注意：具体方法名需确认 v2.nacos 文档，通常是 deregister_instance
                from v2.nacos import DeregisterInstanceParam

                param = DeregisterInstanceParam(
                    service_name=self.service_name,
                    ip=self.ip,
                    port=self.port,
                    cluster_name=self.cluster_name,
                    group_name=self.group_name
                )
                await self.naming_service.deregister_instance(param)
                logger.info(f"[Nacos] 服务{self.service_name} 已安全注销")
            except Exception as e:
                logger.error(f"[Nacos] 注销失败: {e}")


nacos_registry = NacosRegistry()