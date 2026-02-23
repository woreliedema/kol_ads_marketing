import logging
from clickhouse_driver import Client

logger = logging.getLogger(__name__)

class StorageService:
    def __init__(self, ch_client: Client):
        self.ch = ch_client

    def save_data_to_clickhouse(self, table_name: str, data_list: list[dict]) -> bool:
        """
        通用化 ClickHouse 批量写入方法
        利用 clickhouse-driver 的字典插入特性，只要字典 key 和列名一致即可自动映射
        """
        if not data_list:
            logger.warning(f"[{table_name}] 接收到的写入数据为空，跳过写入")
            return False

        try:
            # 此处可以根据需要决定是否在通用层做初步过滤
            # 动态获取字典的 keys，显式指定要插入的列名
            # 例如将 keys 拼接成: "rpid, oid, bvid, mid, message, ..."
            columns = ', '.join(data_list[0].keys())

            query = f"INSERT INTO {table_name}({columns}) VALUES"

            self.ch.execute(query, data_list)
            logger.info(f"[ClickHouse] 成功批量写入 {len(data_list)} 条数据到 {table_name}")
            return True
        except Exception as e:
            logger.error(f"[ClickHouse] 写入 {table_name} 失败: {str(e)}", exc_info=True)
            return False

# 假设你在项目初始化时注入了 client
# ch_client = Client(host='...', port=9000, ...)
# storage_service = StorageService(ch_client)