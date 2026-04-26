import os
import httpx
import json
from typing import Optional, Any
from data_collection_service.crawlers.utils.logger import logger


class CozeApiService:
    def __init__(self):
        # 请在 .env 中配置这些鉴权信息
        self.api_key = os.getenv("COZE_API_KEY", "")
        self.asr_workflow_id = os.getenv("COZE_ASR_WORKFLOW_ID", "")
        self.ai_workflow_id = os.getenv("COZE_AI_WORKFLOW_ID", "")
        self.base_url = "https://api.coze.cn"  # 使用国内版Coze
        self.workflow_base_url = f"{self.base_url}/v1/workflow/run"
        self.headers = {
            "Authorization": f"Bearer {self.api_key}",
        }

    def _safe_parse_json(self, data: Any, max_depth: int = 5) -> Any:
        """【内部防弹衣】安全解析 Coze 返回的嵌套 JSON 字符串"""
        parse_count = 0
        while isinstance(data, str) and parse_count < max_depth:
            try:
                data = json.loads(data)
                parse_count += 1
            except json.JSONDecodeError:
                break
        return data

    async def upload_file(self, file_path: str) -> Optional[str]:
        """上传本地文件到 Coze 并返回 file_id"""
        url = f"{self.base_url}/v1/files/upload"
        try:
            async with httpx.AsyncClient() as client:
                with open(file_path, "rb") as f:
                    files = {"file": f}
                    response = await client.post(url, headers=self.headers, files=files, timeout=60.0)
                    response.raise_for_status()

                    data = response.json()
                    if data.get("code") == 0:
                        file_id = data.get("data", {}).get("id")
                        logger.info(f"[CozeService] 文件上传成功: {file_path} -> file_id: {file_id}")
                        return file_id
                    else:
                        logger.error(f"[CozeService] 上传失败: {data}")
                        return None
        except Exception as e:
            logger.error(f"[CozeService] 请求上传接口异常: {e}")
            return None

    async def run_asr_workflow(self, file_id: str) -> dict:
        """调用 Coze 工作流进行音视频分离与 ASR 解析"""

        file_param_str = json.dumps({"file_id": file_id})
        payload = {
            "workflow_id": self.asr_workflow_id,
            "parameters": {
                "audio": file_param_str # 传入刚才上传的视频 file_id
            }
        }

        # 【核心】AI 任务可能长达几分钟，必须设置长超时！
        timeout = httpx.Timeout(300.0, connect=10.0)

        try:
            async with httpx.AsyncClient(timeout=timeout) as client:
                logger.info(f"[CozeService] 正在投递至 Coze ASR 工作流, file_id: {file_id} ...")
                response = await client.post(self.workflow_base_url, headers=self.headers, json=payload)
                response.raise_for_status()

                # 注意：Coze 可能会返回嵌套的 JSON 字符串，需要二次解析
                data = response.json()
                logger.info(f"[CozeService] 工作流执行完毕, file_id: {file_id}")
                return data
        except Exception as e:
            logger.error(f"[CozeService] 执行工作流异常: {e}")
            return {}

    async def run_multimodal_workflow(self, parameters: dict) -> dict:
        """
        调用 Coze 多模态大模型工作流 (耗时长，逻辑复杂)
        :param parameters: 已经组装好的顶层参数字典
        :return: 解析后的最终 JSON 对象
        """
        payload = {
            "workflow_id": self.ai_workflow_id,
            "parameters": parameters
        }
        # 视频多模态分析极耗时，设置 10 分钟 (600秒) 超时
        timeout = httpx.Timeout(600.0, connect=10.0)
        try:
            async with httpx.AsyncClient(timeout=timeout) as client:
                response = await client.post(self.workflow_base_url, headers=self.headers, json=payload)
                response.raise_for_status()
                # 1. 获取外层响应
                res_json = response.json()
                # 2. 提取 Debug 链接 (极其重要)
                debug_url = res_json.get("debug_url") or res_json.get("data", {}).get("debug_url", "未提供")
                # 3. 校验业务状态码
                if res_json.get("code") != 0:
                    raise RuntimeError(f"API 业务报错: {res_json.get('msg')} | Debug URL: {debug_url}")
                # 4. 提取内层核心数据字符串
                inner_data_str = res_json.get("data")
                if not inner_data_str:
                    raise RuntimeError(f"工作流执行成功，但未返回 data 字段 | Debug URL: {debug_url}")
                # 5. 安全反序列化并兼容字段名
                parsed_inner = self._safe_parse_json(inner_data_str)
                if isinstance(parsed_inner, dict):
                    # 兼容不同时期在结束节点设置的返回变量名
                    kol_ads_result = parsed_inner.get("output") or parsed_inner.get("kol_ads") or parsed_inner
                else:
                    logger.warning(f"[CozeService] 大模型返回数据无法解析为字典，降级处理。数据: {parsed_inner}")
                    kol_ads_result = {"summary_for_next": str(parsed_inner)}
                return kol_ads_result
        except Exception as e:
            logger.error(f"[CozeService] 执行多模态工作流异常: {e}")
            raise e


# 单例模式
coze_client = CozeApiService()