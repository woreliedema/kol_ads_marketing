import os
import httpx
import json
import asyncio
from dotenv import load_dotenv
from data_collection_service.crawlers.utils.multimodal_data import align_and_chunk_multimodal_data
from data_collection_service.app.db.clickhouse import ClickHouseManager
from data_collection_service.app.services.storage_service import StorageService
load_dotenv()


class CozeFileTest:
    def __init__(self):
        self.api_key = os.getenv("COZE_API_KEY", "")
        self.base_url = "https://api.coze.cn"
        self.ai_workflow_id = os.getenv("COZE_AI_WORKFLOW_ID", "")
        self.headers = {
            "Authorization": f"Bearer {self.api_key}",
        }

    async def test_upload(self, file_path: str):
        """测试上传文件，打印完整响应"""
        url = f"{self.base_url}/v1/files/upload"

        print("=" * 60)
        print(f"测试上传文件: {file_path}")
        print("=" * 60)

        async with httpx.AsyncClient() as client:
            with open(file_path, "rb") as f:
                files = {"file": f}
                response = await client.post(url,
                                             headers=self.headers,
                                             files=files,
                                             timeout=60.0)

                print("\n【响应状态码】")
                print(f"Status: {response.status_code}")

                print("\n【完整响应体】")
                data = response.json()
                print(json.dumps(data, ensure_ascii=False, indent=2))

                if data.get("code") == 0:
                    file_data = data.get("data", {})
                    print("\n【提取的file_id】")
                    print(f"file_id = {file_data.get('id')}")

                    return file_data.get("id")
                else:
                    print("\n【上传失败】")
                    print(f"错误信息: {data.get('msg')}")
                    return None

    async def test_workflow_with_file(self, workflow_id: str, file_id: str):
        """测试用file_id调用工作流"""
        url = f"{self.base_url}/v1/workflow/run"

        print("\n" + "=" * 60)
        print(f"测试调用工作流: {workflow_id}")
        print(f"file_id: {file_id}")
        print("=" * 60)

        file_param_str = json.dumps({"file_id": file_id})
        payload = {
            "workflow_id": workflow_id,
            "parameters": {
                "audio": file_param_str
            }
        }

        print("\n【请求payload】")
        print(json.dumps(payload, ensure_ascii=False, indent=2))

        timeout = httpx.Timeout(300.0, connect=10.0)

        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.post(url, headers=self.headers, json=payload)

            print("\n【响应状态码】")
            print(f"Status: {response.status_code}")

            # print("\n【完整响应体】")
            # data = response.json()
            # 获取原始响应文本
            raw_text = response.text
            print(f"\n【原始响应类型】: {type(raw_text)}")
            print(f"【原始响应前500字符】:\n{raw_text[:500]}")

            # 解析响应
            data = self._safe_parse_json(raw_text)

            print(f"\n【解析后类型】: {type(data)}")

            if isinstance(data, dict):
                print(f"【data.keys】: {list(data.keys())}")

                # 检查 code
                if data.get("code") != 0:
                    print(f"【API调用失败】: {data.get('msg')}")
                    return

                # 获取 output
                inner_data = data.get("data", {})
                print(f"\n【data.data 类型】: {type(inner_data)}")

                if isinstance(inner_data, dict):
                    output = inner_data.get("output")
                    print(f"【output 类型】: {type(output)}")

                    if output:
                        # 解析 output
                        parsed_output = self._safe_parse_json(output)
                        print(f"\n【output 解析后类型】: {type(parsed_output)}")

                        if isinstance(parsed_output, dict):
                            print(f"【output.keys】: {list(parsed_output.keys())}")

                            # 提取 timeline
                            timeline = parsed_output.get("timeline")
                            print(f"\n【timeline 类型】: {type(timeline)}")

                            if isinstance(timeline, dict):
                                timelines = timeline.get("timelines", [])
                                print(f"【timelines 数量】: {len(timelines) if isinstance(timelines, list) else 'N/A'}")

                                if timelines and isinstance(timelines, list) and len(timelines) > 0:
                                    print(f"\n【第一条字幕示例】:")
                                    print(json.dumps(timelines[0], ensure_ascii=False, indent=2))
            else:
                print(f"【data 不是 dict】: {data}")

    async def test_align_and_chunk(self, bvid: str, cid: str) -> list:
        print("\n" + "=" * 60)
        print(f"开始测试数据组装: {bvid}_{cid}")
        print("=" * 60)

        # 在独立脚本中借用连接池
        async with ClickHouseManager.pool.connection() as ch_client:
            storage = StorageService(ch_client=ch_client)
            # 1. 查关键帧
            kf_query = f"""
                SELECT 
                    toUInt64OrZero(extract(file_name, '(\\\\d+)\\\\.jpg$')) AS timestamp_us, 
                    coze_file_id, 
                    file_name 
                FROM ods.oss2coze_filename_info 
                WHERE file_name LIKE 'images/bilibili/{bvid}_{cid}/%' AND file_type = 1
                ORDER BY timestamp_us ASC
            """
            keyframes = await storage.query_clickhouse(kf_query)
            print(f"✅ 查询到 {len(keyframes)} 张关键帧")
            # 2. 查字幕
            sub_query = f"""
                SELECT start_time_us, end_time_us, text 
                FROM ods.bilibili_audio_info 
                WHERE bvid = '{bvid}' AND cid = '{cid}'
                ORDER BY start_time_us ASC
            """
            subtitles = await storage.query_clickhouse(sub_query)
            print(f"✅ 查询到 {len(subtitles)} 条字幕")

            if not keyframes or not subtitles:
                print("⚠️ 数据不全，请确保 ASR 和抽帧都已经入库！")
                return []

            # 3. 运行算法组装
            chunks = align_and_chunk_multimodal_data(keyframes, subtitles, chunk_size=15)
            print(f"🍔 汉堡包组装完毕，共切分为 {len(chunks)} 个批次")

            return chunks

    async def test_multimodal_llm_workflow(self, bvid: str, chunks: list):
        """
        测试 2: 循环调用大模型工作流，传入批次数据
        """
        url = f"{self.base_url}/v1/workflow/run"
        print("\n" + "=" * 60)
        print(f"开始测试多模态大模型分析流水线")
        print("=" * 60)
        async with ClickHouseManager.pool.connection() as ch_client:
            storage = StorageService(ch_client=ch_client)
            query = f"""
            select title,introduction 
            From ods.bilibili_video_info
            where bvid='{bvid}'
            order by batch_id DESC
            limit 1
            """
            video_info_list = await storage.query_clickhouse(query)
            video_title = "未知标题"
            video_intro = ""
            if video_info_list:
                video_title = video_info_list[0].get("title", "未知标题")
                video_intro = video_info_list[0].get("introduction", "")
        previous_response = ""
        total_batches = len(chunks)
        # 为了防止测试太久，你可以加一个切片比如 chunks[:2] 只测前两批
        for index, chunk in enumerate(chunks):
            current_batch = index + 1
            is_final_batch = (current_batch == total_batches)
            print(f"\n🚀 正在发送批次 {current_batch}/{total_batches} (is_final={is_final_batch}) ...")
            # 🌟 破解限制：平行数组拆解法
            image_list = []
            text_list = []
            for idx, item in enumerate(chunk):
                file_id = item.get("coze_file_id")
                if not file_id:
                    continue
                # 🎯 核心测试：仿照 test_workflow_with_file 的逻辑
                # 将单个图片组装成 {"file_id": "xxx"} 的对象字典
                image_list.append({"file_id": file_id})
                # 提前把文本在 Python 侧排版好
                start = item.get("start_time", "")
                end = item.get("end_time", "")
                text = item.get("text", "（画面无语音）")
                formatted_text = f"【画面 {idx + 1}】[时间: {start} - {end}] 口播字幕: {text}"
                text_list.append(formatted_text)
            # 构建 Coze API 需要的 parameters
            payload = {
                "workflow_id":self.ai_workflow_id,
                "parameters": {
                    "previous_response": previous_response,
                    "batch_index": str(current_batch),
                    "is_final_batch": "true" if is_final_batch else "false",
                    "video_metadata": json.dumps({
                        "platform": "bilibili",
                        "video_title": video_title,
                        "video_introduction": video_intro
                    }, ensure_ascii=False),
                    "image_list": json.dumps(image_list, ensure_ascii=False),
                    "text_list": json.dumps(text_list, ensure_ascii=False)
                }
            }
            timeout = httpx.Timeout(600.0, connect=10.0)  # 这一步很耗时，设置 10 分钟超时
            try:
                async with httpx.AsyncClient(timeout=timeout) as client:
                    response = await client.post(url, headers=self.headers, json=payload)
                    response.raise_for_status()
                    raw_text = response.text
                    data = self._safe_parse_json(raw_text)
                    debug_url = data.get("debug_url") or data.get("data", {}).get("debug_url", "未提供")
                    if data.get("code") != 0:
                        print(f"❌ API 调用失败: {data.get('msg')}")
                        print(f"🔗 Debug 链接: {debug_url}")
                        break
                    # 成功也打印链接，方便在浏览器看执行瀑布流
                    print(f"🔗 本轮工作流 Debug 链接: {debug_url}")
                    # Coze 真实输出内容在 data 字段的字符串里
                    inner_data_str = data.get("data")
                    if not inner_data_str:
                        print("⚠️ 工作流成功执行，但未返回 data 字段。")
                        break
                    # 精准解析大模型返回的 JSON 对象
                    try:
                        parsed_inner = json.loads(inner_data_str)
                        # 兼容你在结束节点定义的变量名，可能是 output 或 kol_ads
                        kol_ads_result = parsed_inner.get("output") or parsed_inner.get("kol_ads") or parsed_inner
                    except json.JSONDecodeError:
                        print("⚠️ 大模型返回的数据无法反序列化，降级为原生字符串处理。")
                        kol_ads_result = {"summary_for_next": str(inner_data_str)}
                    if not is_final_batch:
                        # 【核心修正】：只把本轮的精华总结（summary）丢给下一轮，绝不透传冗余外壳！，注意字段名称最长20个字符
                        previous_response = kol_ads_result.get("summary_for_next", "")
                        print(f"\n【本轮提炼线索】: {previous_response}")
                        print("✅ 已保存本轮总结，等待 3 秒后发送下一批...")
                        await asyncio.sleep(3)  # 防 Coze QPS 限流
                    else:
                        print("\n" + "🎉" * 20)
                        print("【最终商单分析报告】:")
                        # 打印漂亮的最终 JSON 结果
                        print(json.dumps(kol_ads_result, ensure_ascii=False, indent=2))
                        print("🎉" * 20)
            except Exception as e:
                print(f"❌ 本批次执行异常: {e}")
                break

    def _safe_parse_json(self, data, max_depth=10):
        """安全解析嵌套的JSON字符串"""
        parse_count = 0
        while isinstance(data, str) and parse_count < max_depth:
            try:
                data = json.loads(data)
                parse_count += 1
                print(f"  [解析第{parse_count}次] 类型变为: {type(data)}")
            except json.JSONDecodeError as e:
                print(f"  [解析失败] 第{parse_count + 1}次: {e}")
                break
        return data

# 运行测试


async def main():
    # 1. 独立脚本必须初始化 ClickHouse 连接池
    ch_host = os.getenv("CLICKHOUSE_HOST", "127.0.0.1")
    ch_port = int(os.getenv("CLICKHOUSE_PORT", 9000))
    ch_user = os.getenv("CLICKHOUSE_USER", "default")
    ch_password = os.getenv("CLICKHOUSE_PASSWORD", "")
    await ClickHouseManager.init_db(
        host=ch_host,
        port=ch_port,
        user=ch_user,
        password=ch_password,
        database="ods"
    )
    tester = CozeFileTest()
    # 替换为你刚才成功跑完 ASR 并入库的一个真实 BVID 和 CID
    test_bvid = "BV1HK9MBfEq6"
    test_cid = "37211342930"
    # ===== 环节 1: 测试拉取数据和组装 =====
    chunks = await tester.test_align_and_chunk(test_bvid, test_cid)
    # ===== 环节 2: 测试连续丢给大模型 =====
    if chunks:
        # 为了节约 Token 和时间，沙盒里可以先只测试前 2 批数据
        # await tester.test_multimodal_llm_workflow(test_bvid, chunks[:2])
        # 如果你想跑全量，解除下面的注释
        await tester.test_multimodal_llm_workflow(test_bvid, chunks)
    # 关闭连接池
    await ClickHouseManager.close_db()

if __name__ == "__main__":
    asyncio.run(main())