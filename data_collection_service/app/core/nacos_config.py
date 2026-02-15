
from v2.nacos import NacosNamingService, ClientConfigBuilder, RegisterInstanceParam
import logging

# 配置日志
logger = logging.getLogger(__name__)

class NacosRegistry:
    def __init__(self):
        self.naming_service = None
        # Nacos 配置信息 (建议从环境变量读取，这里保持你原有的硬编码或默认值)
        self.server_address = "localhost:8848"
        self.namespace_id = "public"
        self.service_name = "data-collection-service"
        self.ip = "localhost"
        self.port = 8000
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
                weight=1.0
            )
            await self.naming_service.register_instance(instance_param)
            logger.info(f"服务已成功注册到 Nacos: {self.service_name}@{self.ip}:{self.port}")
        except Exception as e:
            logger.error(f"Nacos 注册失败: {e}")
            # 根据需求决定是否抛出异常阻断启动
            # raise e

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
                logger.info(f"服务已从 Nacos 注销")
            except Exception as e:
                logger.error(f"Nacos 注销失败: {e}")


nacos_registry = NacosRegistry()