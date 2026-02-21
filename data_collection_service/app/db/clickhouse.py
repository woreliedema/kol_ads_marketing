import logging
from clickhouse_driver import Client

logger = logging.getLogger(__name__)

class ClickHouseManager:
    """ClickHouse 连接管理器 (单例模式思想)"""
    client: Client = None

    @classmethod
    def init_db(cls, host: str, port: int, user: str, password: str, database: str = "ods"):
        try:
            # clickhouse_driver 底层会自动维护连接池
            cls.client = Client(
                host=host,
                port=port,        # 注意：此处应填写 TCP 端口 (默认9000)
                user=user,
                password=password,
                database=database,
                connect_timeout=10,
                send_receive_timeout=300
            )
            # 测试连接
            cls.client.execute('SELECT 1')
            logger.info("✅ ClickHouse 数据库连接初始化成功！")
        except Exception as e:
            logger.error(f"❌ ClickHouse 连接失败: {e}", exc_info=True)
            raise e

    @classmethod
    def close_db(cls):
        if cls.client:
            cls.client.disconnect()
            logger.info("🛑 ClickHouse 数据库连接已安全关闭。")

#  核心：这就是提供给 FastAPI 路由的依赖注入函数
def get_ch_client() -> Client:
    if not ClickHouseManager.client:
        raise RuntimeError("ClickHouse 客户端尚未初始化")
    return ClickHouseManager.client