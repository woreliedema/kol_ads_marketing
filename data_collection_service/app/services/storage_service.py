from asynch.connection import Connection
from asynch.cursors import DictCursor

from data_collection_service.app.api.models.QueryModel import OperatorEnum,ComplexSearchRequest
from data_collection_service.crawlers.utils.logger import logger

class StorageService:
    def __init__(self, ch_client: Connection):
        self.ch = ch_client

    async def save_data_to_clickhouse(self, table_name: str, data_list: list[dict]) -> bool:
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

            async with self.ch.cursor() as cursor:
                await cursor.execute(query, data_list)
            logger.info(f"[ClickHouse] 成功批量写入 {len(data_list)} 条数据到 {table_name}")
            return True
        except Exception as e:
            logger.error(f"[ClickHouse] 写入 {table_name} 失败: {str(e)}", exc_info=True)
            return False

    async def search_data_from_clickhouse(self, table_name: str, query_req: ComplexSearchRequest) -> dict:
        """
        通用复杂查询接口
        """
        try:
            where_clauses = []
            params = {}

            # 1. 遍历筛选规则，构建 WHERE 子句
            for idx, rule in enumerate(query_req.filters):
                # 为了防止参数名冲突，使用 param_0, param_1 这样的唯一key
                param_key = f"p_{idx}"

                # 安全校验：防止字段名注入 (仅允许字母数字下划线)
                if not rule.field.replace("_", "").isalnum():
                    continue

                if rule.op == OperatorEnum.EQ:
                    where_clauses.append(f"{rule.field} = %({param_key})s")
                    params[param_key] = rule.value

                elif rule.op == OperatorEnum.GT:
                    where_clauses.append(f"{rule.field} > %({param_key})s")
                    params[param_key] = rule.value

                elif rule.op == OperatorEnum.GTE:
                    where_clauses.append(f"{rule.field} >= %({param_key})s")
                    params[param_key] = rule.value

                elif rule.op == OperatorEnum.LIKE:
                    where_clauses.append(f"{rule.field} LIKE %({param_key})s")
                    params[param_key] = f"%{rule.value}%"  # 自动加 %

                elif rule.op == OperatorEnum.IN:
                    # ClickHouse 的 IN 需要元组或列表
                    where_clauses.append(f"{rule.field} IN %({param_key})s")
                    params[param_key] = tuple(rule.value) if isinstance(rule.value, list) else rule.value

                elif rule.op == OperatorEnum.BETWEEN:
                    # between 需要两个值
                    if isinstance(rule.value, list) and len(rule.value) == 2:
                        p_start = f"{param_key}_start"
                        p_end = f"{param_key}_end"
                        where_clauses.append(f"{rule.field} >= %({p_start})s AND {rule.field} <= %({p_end})s")
                        params[p_start] = rule.value[0]
                        params[p_end] = rule.value[1]

            # 2. 拼接最终 SQL
            where_sql = " AND ".join(where_clauses)
            if where_sql:
                where_sql = f"WHERE {where_sql}"

            # 分页计算，当前不需要，后续优化时考虑添加
            # offset = (query_req.page - 1) * query_req.page_size
            # sql添加：LIMIT {query_req.page_size} OFFSET {offset}

            # 3. 执行查询 (查数据)
            sql = f"""
                SELECT * FROM {table_name}
                {where_sql}
                ORDER BY {query_req.order_by}
            """

            # 4. 执行计数 (查总数 - 用于分页显示)
            count_sql = f"SELECT count(*) FROM {table_name} {where_sql}"

            # 执行
            # data_result, col_types = self.ch.execute(sql, params, with_column_types=True)
            async with self.ch.cursor(cursor=DictCursor) as cursor:
                await cursor.execute(sql, params)
                items = await cursor.fetchall()
            async with self.ch.cursor() as cursor:
                await cursor.execute(count_sql, params)
                count_result = await cursor.fetchone()
                total_count = count_result[0] if count_result else 0

            # 格式化结果
            # columns = [col[0] for col in col_types]
            # items = [dict(zip(columns, row)) for row in data_result]

            return {
                "total": total_count,
                "page": query_req.page,
                "items": items
            }

        except Exception as e:
            logger.error(f"[ClickHouse] 复杂查询失败: {str(e)}", exc_info=True)
            return {"total": 0, "items": [], "error": str(e)}