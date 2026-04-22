import asyncio
import math
import json
import traceback
from datetime import datetime, timedelta
from typing import Optional

from data_collection_service.app.db.clickhouse import ClickHouseManager
from data_collection_service.app.services.kafka_service import kafka_producer
from data_collection_service.app.services.data_cleaning_service import DataCleaningService
from data_collection_service.app.services.storage_service import StorageService
from data_collection_service.app.services.video_processor_service import VideoProcessorService
from data_collection_service.app.services.storage_video_service import minio_video_client
from data_collection_service.app.services.coze_service import coze_client
from data_collection_service.app.db.target_repository import cascade_register_videos_to_target
from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler
from data_collection_service.crawlers.utils.logger import logger
from data_collection_service.crawlers.utils.multimodal_data import align_and_chunk_multimodal_data


class BilibiliTaskService:
    def __init__(self, crawler: Optional[BilibiliWebCrawler], storage: Optional[StorageService]):
        self.crawler = crawler
        self.storage = storage

    async def collect_and_store_video_comments(self, target_id: str, batch_id: str):
        """
        整合b站视频下所有评论的爬取、清洗、入库的完整流程
        """
        bvid = target_id
        logger.info(f"[Task {batch_id}] 开始执行视频 {bvid} 的评论采集任务")

        # 1. 获取视频 aid
        video_info = await self.crawler.fetch_one_video(bvid)
        if not video_info or video_info.get('code') != 0:
            logger.error(f"获取视频信息失败，无法提取 aid: bvid={bvid}")
            return

        aid = video_info.get('data', {}).get('aid')
        if not aid:
            logger.error("视频信息中缺失 aid")
            return

        all_comments = []

        # 2. 采集第一页并计算分页
        first_page = await self.crawler.fetch_video_comments_new(bvid, pn=1, aid=aid)
        if not first_page or first_page.get('code') != 0:
            logger.error(f"获取首页评论失败: bvid={bvid}")
            return

        page_info = first_page.get('data', {}).get('page', {})
        total_count = page_info.get('count', 0)
        page_size = page_info.get('size', 20) or 20
        total_pages = math.ceil(total_count / page_size)

        logger.info(f"视频 {bvid} 共发现 {total_count} 条评论，需采集 {total_pages} 页")

        replies = first_page.get('data', {}).get('replies', [])
        if replies:
            all_comments.extend(replies)

        # 3. 采集主评论剩余页
        for pn in range(2, total_pages + 1):
            try:
                page_data = await self.crawler.fetch_video_comments_new(bvid, pn=pn, aid=aid)
                if page_data.get('code') == 0:
                    page_replies = page_data.get('data', {}).get('replies', [])
                    if page_replies:
                        all_comments.extend(page_replies)
                    else:
                        break

                # 【核心移植】：动态自适应延时策略防封
                sleep_time = min(1.5 + (total_count / 1000), 3.0)
                await asyncio.sleep(sleep_time)
            except Exception as e:
                logger.error(f"爬取第 {pn} 页异常: {e}")
                await asyncio.sleep(3.0)
                continue

        # 4. 采集楼中楼（子评论）
        for comment in all_comments:
            rpid = comment.get('rpid')
            rcount = comment.get('rcount', 0)

            if rpid and rcount > 0:
                reply_page = await self.crawler.fetch_comment_reply_new(bvid, pn=1, rpid=str(rpid), aid=aid)
                if reply_page.get('code') == 0:
                    reply_data = reply_page.get('data', {})
                    # sub_replies = reply_data.get('replies', [])
                    # 如果 get 到的是 None，强制转为空列表 []
                    sub_replies = reply_data.get('replies') or []
                    comment['replies'] = sub_replies

                    reply_count = reply_data.get('page', {}).get('count', rcount)
                    reply_size = reply_data.get('page', {}).get('size', 10) or 10
                    reply_total_pages = math.ceil(reply_count / reply_size)

                    for reply_pn in range(2, reply_total_pages + 1):
                        await asyncio.sleep(0.8)
                        try:
                            reply_page_more = await self.crawler.fetch_comment_reply_new(bvid, pn=reply_pn, rpid=str(rpid),aid=aid)
                            if reply_page_more.get('code') == 0:
                                more_replies = reply_page_more.get('data', {}).get('replies', [])
                                if more_replies:
                                    comment['replies'].extend(more_replies)
                        except Exception as e:
                            logger.error(f"获取子评论异常: {e}")
                            continue
                await asyncio.sleep(1.0)

        # 5. 数据清洗与格式化
        cleaned_data = DataCleaningService.clean_bilibili_video_comments(all_comments, bvid, aid, batch_id)
        logger.info(f"[清洗] 完成数据转换，清洗后产生 {len(cleaned_data)} 条标准化数据 (含子评论)")

        # 6. 通用化写入 ClickHouse
        # 即使未来换成动态评论、直播弹幕，这行代码都不用变，只需换 table_name 即可
        if not cleaned_data:
            logger.warning(f"[Task {batch_id}] 清洗 {bvid} 视频数据为空或接口返回错误，跳过入库")
            return False

        success = await self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_video_comments",
            data_list=cleaned_data
        )

        if success:
            logger.info(f"[Task {batch_id}] 视频 {bvid} 全量评论信息入库成功")
        return success

    async def collect_and_store_user_info(self, target_id: str, batch_id: str):
        """
        采集 B站 UP主基本信息并写入 ClickHouse
        """
        mid = target_id
        logger.info(f"[Task {batch_id}] 开始采集用户 {mid} 的档案信息")

        # 1. 调用爬虫获取原始数据
        try:
            # 使用已有的 fetch_user_profile 接口
            raw_data = await self.crawler.fetch_user_profile(uid=mid)
        except Exception as e:
            logger.error(f"[Task {batch_id}] 用户 {mid} 爬取失败: {str(e)}")
            return False

        # 2. 数据清洗
        cleaned_data = DataCleaningService.clean_user_info(raw_data, batch_id)
        if not cleaned_data:
            logger.warning(f"[Task {batch_id}] 用户 {mid} 数据为空或接口返回错误，跳过入库")
            return False
        # 3. 写入 ClickHouse
        # 注意：table_name 必须与 ClickHouse 的表名一致
        success = await self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_user_info",
            data_list=cleaned_data
        )
        if success:
            logger.info(f"[Task {batch_id}] 用户 {mid} ({cleaned_data[0]['uname']}) 信息入库成功")
        return success

    async def collect_and_store_user_relation(self, target_id: str, batch_id: str):
        """
        采集 B站 UP主关系信息并写入 ClickHouse
        """
        mid = target_id
        logger.info(f"[Task {batch_id}] 开始采集用户 {mid} 的档案信息")

        # 1. 调用爬虫获取原始数据
        try:
            # 使用已有的 fetch_user_profile 接口
            raw_data = await self.crawler.fetch_user_relation(uid=mid)
        except Exception as e:
            logger.error(f"[Task {batch_id}] 用户 {mid} 爬取失败: {str(e)}")
            return False

        # 2. 数据清洗
        cleaned_data = DataCleaningService.clean_user_relation(raw_data, batch_id)
        if not cleaned_data:
            logger.warning(f"[Task {batch_id}] 用户 {mid} 数据为空或接口返回错误，跳过入库")
            return False
        # 3. 写入 ClickHouse
        # 注意：table_name 必须与 ClickHouse 的表名一致
        success = await self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_user_relation",
            data_list=cleaned_data
        )
        if success:
            logger.info(f"[Task {batch_id}] 用户 {mid} 信息入库成功")
        return success

    async def collect_and_store_video_info(self, target_id: str, batch_id: str):
        """
        [Task] 采集 B站 视频基本信息并写入 ClickHouse
        """
        bvid = target_id
        logger.info(f"[Task {batch_id}] 开始采集视频 {bvid} 的基本信息")
        try:
            raw_data = await self.crawler.fetch_one_video(bv_id=bvid)
        except Exception as e:
            logger.error(f"[Task {batch_id}] 视频 {bvid} 网络请求失败: {str(e)}")
            return False
        # 2. 数据清洗 (解析 JSON -> 扁平字典)
        cleaned_data = DataCleaningService.clean_video_info(raw_data, batch_id)
        pages_data = DataCleaningService.clean_video_pages_info(raw_data, batch_id)
        if not cleaned_data:
            logger.warning(f"[Task {batch_id}] 视频 {bvid} 数据为空或接口返回错误，跳过入库")
            return False
        # 3. 写入 ClickHouse
        is_main_success = await self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_video_info",
            data_list=cleaned_data
        )
        if not is_main_success:
            logger.error(f"[Task {batch_id}] 视频 {bvid} 主表(video_info)落盘失败，触发快速失败。")
            return False

        title = cleaned_data[0].get('title', 'Unknown')
        logger.info(f"[Task {batch_id}] 视频 {bvid}： ({title}) 主表信息入库成功")

        # 4. 写入 ClickHouse (分P视频信息)
        if pages_data:
            is_pages_success = await self.storage.save_data_to_clickhouse(
                table_name="ods.bilibili_video_pages_info",
                data_list=pages_data
            )
            # 第二层防线：主表已成功，但从表（分P）失败，依然要触发快速失败
            if not is_pages_success:
                logger.error(f"[Task {batch_id}] 视频 {bvid} 从表(video_pages)落盘失败，触发快速失败。")
                # 此时返回 False，调度器会再次下发该任务
                return False
            logger.info(f"[Task {batch_id}] 视频 {bvid} 分P数据入库成功，共 {len(pages_data)} 集")
        else:
            logger.warning(f"[Task {batch_id}] 视频 {bvid} 无分P数据或解析为空（单P视频）")
        # 5. 当且仅当所有的必要 I/O 操作均无异常时，才向队列返回成功 ACK
        return True

    async def collect_and_store_video_to_minio(self, target_id: str, batch_id: str) -> bool:
        """
        [专项任务] 针对近30天内发布的视频：解析无水印直链 -> 下载/合并音视频 -> 上传至本地 MinIO -> 触发 AI 分析
        """
        bvid = target_id
        logger.info(f"[Task {batch_id}] 开始执行视频 {bvid} 的下载与云端存储任务")

        try:
            # 1. 任务自闭环：独立获取视频信息，提取 cid
            video_info = await self.crawler.fetch_one_video(bvid)
            if not video_info or video_info.get('code') != 0:
                logger.error(f"[Task {batch_id}] 获取视频信息失败，无法提取 cid: bvid={bvid}")
                return False
            # B站视频可能有多P，我们通常针对 AI 分析只取第一P的 cid
            cid = video_info.get('data', {}).get('cid')
            if not cid:
                logger.error(f"[Task {batch_id}] 视频信息中缺失 cid: bvid={bvid}")
                return False

            # 使用 bvid 和 cid 共同作为文件名，天然支持版本追踪
            composite_video_id = f"{bvid}_{cid}"
            # 秒传校验改为检查纯音频文件是否存在
            audio_object_name = f"audios/bilibili/{composite_video_id}.m4a"
            # object_name = f"videos/bilibili/{bvid}_{cid}.mp4"
            # 去 MinIO 查一下这个特定版本的文件是否已经下过了
            if minio_video_client.check_file_exists(audio_object_name):
                logger.info(f"[Task {batch_id}] ⚡ 触发秒传！音频版本 {composite_video_id} 已存在，跳过下载。")

                return True

            logger.info(f"[Task {batch_id}] 发现新版本 {composite_video_id}，请求底层分离流...")
            # 2. 调用底层的视频流接口获取播放地址
            # qn="64" 代表 720P，这对大模型分析(尤其是抽帧和语音识别)已经足够清晰且极大地节省带宽
            play_data = await self.crawler.fetch_video_playurl(bv_id=bvid, cid=str(cid))

            if not play_data or play_data.get('code') != 0:
                logger.error(f"[Task {batch_id}] 获取播放流地址失败: bvid={bvid}")
                return False

            # B站返回的音视频分离流 (dash 格式)
            dash_info = play_data.get('data', {}).get('dash')
            if not dash_info:
                # 有些老视频可能是 durl 格式（FLV/MP4单文件），如果遇到需要写额外逻辑处理
                # 此处为防崩溃，暂时跳过不支持 dash 格式的老视频
                logger.warning(f"[Task {batch_id}] 视频不包含 dash 音视频分离流，暂时跳过: bvid={bvid}")
                return False

            # 提取视频流 (取第一种清晰度的首个备用地址)
            video_url = dash_info.get('video', [{}])[0].get('baseUrl')
            # 提取音频流
            audio_url = dash_info.get('audio', [{}])[0].get('baseUrl')

            if not video_url or not audio_url:
                logger.error(f"[Task {batch_id}] 无法解析具体的音视频 baseUrl: bvid={bvid}")
                return False

            # 3. 获取 B 站专属防盗链 Headers
            headers_dict = await self.crawler.get_bilibili_headers()
            # 组装数据包
            video_mock_data = {
                'nwm_video_url_HQ': video_url,
                'audio_url': audio_url
            }
            # 4. 委托 VideoProcessorService 执行重度 I/O 操作
            composite_video_id = f"{bvid}_{cid}"
            storage_result = await VideoProcessorService.process_and_upload_video(
                platform='bilibili',
                video_id=composite_video_id,
                video_data=video_mock_data,
                headers=headers_dict.get('headers', {})
            )

            if not storage_result:
                logger.error(f"[Task {batch_id}] 视频 {bvid} 下载合并或上传 MinIO 失败")
                return False

            # 本地测试时的临时数据清洗
            audio_file_name = storage_result["file_name"]
            coze_aud_id = storage_result["coze_audio_file_id"]
            frames_data = storage_result.get("frame_urls", [])

            mapping_data = []
            # 追加音频映射 (file_type: 2 代表音频)
            mapping_data.append({
                "file_url": storage_result["audio_url"],
                "coze_file_id": coze_aud_id,
                "coze_base_url": "https://api.coze.cn/",
                "oss_base_url": "http://localhost:19000/",
                "file_type": 2,
                "file_name": audio_file_name
            })
            # 追加关键帧映射 (file_type: 1)
            for frame in frames_data:
                mapping_data.append({
                    "file_url": frame["file_url"],
                    "coze_file_id": frame["coze_file_id"],
                    "coze_base_url": "https://api.coze.cn/",
                    "oss_base_url": "http://localhost:19000/",
                    "file_type": 1,
                    "file_name": frame["file_name"]
                })
            if mapping_data:
                await self.storage.save_data_to_clickhouse("ods.oss2coze_filename_info", mapping_data)
                logger.info(f"[Task {batch_id}] 成功将 1 个视频和 {len(frames_data)} 张关键帧的 Coze 映射关系落盘。")

            # 向AI处理 队列生产消息
            ai_asr_payload = {
                "batch_id": batch_id,
                "bvid": bvid,
                "cid": str(cid),
                "coze_file_id": coze_aud_id  # 传递核心介质 ID
            }
            await kafka_producer.send_task_message("bilibili_coze_asr_tasks", ai_asr_payload)
            logger.info(f"[Task {batch_id}] 阶段 A 完成，已将视频 {bvid} (CozeID: {coze_aud_id}) 推入阶段 B (ASR) 队列。")

            return True

        except Exception as e:
            logger.error(f"[Task {batch_id}] 下载存储视频 {bvid} 任务发生未捕获异常: {e}")
            return False

    async def process_coze_asr_workflow(self, bvid: str, cid: str, coze_file_id: str, batch_id: str) -> bool:
        """
        [阶段B专属任务] 调用 Coze 执行 ASR -> 数据清洗截断 -> 压入 ClickHouse
        """
        logger.info(f"[ASR Pipeline] 开始处理视频 {bvid}，调起 Coze 工作流...")

        # 1. 挂起等待 1-3 分钟，执行大模型音频提取
        ai_workflow_response = await coze_client.run_asr_workflow(file_id=coze_file_id)

        # 2. 清洗裁剪掉 100KB 的冗余字级时间戳
        cleaned_subtitles = DataCleaningService.clean_and_flatten_asr_data(
            ai_workflow_response=ai_workflow_response,
            bvid=bvid,
            cid=cid,
            batch_id=batch_id
        )
        if not cleaned_subtitles:
            logger.warning(f"[ASR Pipeline] 视频 {bvid} ASR 字幕为空或接口失败。")
            return False

        # 3. 极速写入 ClickHouse 底表
        success = await self.storage.save_data_to_clickhouse("ods.bilibili_audio_info", cleaned_subtitles)

        if success:
            logger.info(f"🎉 [ASR Pipeline] 视频 {bvid} 的 ASR 字幕已成功入库！")
            # 组装阶段 C (多模态大模型分析) 的载荷
            analysis_payload = {
                "batch_id": batch_id,
                "bvid": bvid,
                "cid": str(cid)
            }
            # 发送给 Topic C
            await kafka_producer.send_task_message("bilibili_multimodal_analysis_tasks", analysis_payload)
            logger.info(f"[ASR Pipeline] 阶段 B 闭环完成，已将视频 {bvid} 推入阶段 C (多模态深度分析) 队列。")
        return success

    async def collect_and_store_user_videos(self, target_id: str, batch_id: str) -> bool:
        """
        采集用户投稿视频作品信息，只抓取近一年的数据，并写入 ClickHouse
        """
        mid = target_id
        page = 1
        all_videos = []
        bvid_list = []
        recent_30_days_bvid_list = []
        has_more = True

        # 计算时间边界：一年（365天）前的时间戳
        one_year_ago_ts = int((datetime.now() - timedelta(days=365)).timestamp())
        thirty_days_ago_ts = int((datetime.now() - timedelta(days=30)).timestamp())

        while has_more:
            try:
                logger.info(f"[Task:{batch_id}] 正在拉取 UID:{mid} 投稿视频 第 {page} 页...")
                data_ori = await self.crawler.fetch_user_post_videos(uid=mid,pn=page)

                raw_data = data_ori.get('data', {}).get('list', {}).get('vlist', [])
                if not raw_data:
                    # 已经没有更多视频了
                    break

                for v in raw_data:
                    pubdate_ts = v.get('created', 0)
                    bvid = v.get('bvid', '')

                    if any(v.get(key, 0) != 0 for key in ['is_pay', 'is_lesson_video', 'is_charging_arc', 'is_live_playback']):
                        logger.debug(f"[Task:{batch_id}] 视频 {bvid} 不符合UGC常规投放特征，跳过")
                        continue

                    # B站接口返回是按时间倒序的。一旦遍历到一年前的视频，直接腰斩整个爬虫循环！
                    if pubdate_ts < one_year_ago_ts:
                        logger.info(f"[Task:{batch_id}] UID:{mid} 触发一年前的时间边界，提前结束爬取。")
                        has_more = False
                        break

                    all_videos.append(v)
                    bvid_list.append(bvid)

                    if pubdate_ts >= thirty_days_ago_ts:
                        recent_30_days_bvid_list.append(bvid)

                if not has_more:
                    break

                page += 1
                # 防护风控机制：翻页休眠 1.5 - 3 秒
                await asyncio.sleep(2)

            except Exception as e:
                logger.error(f"[Task:{batch_id}] 拉取 UID:{mid} 视频页数 {page} 发生异常: {str(e)}")
                break

        # --- 数据落盘与状态判定 ---
        is_ch_success = True
        if all_videos:
            cleaned_data = DataCleaningService.clean_upload_video_info(all_videos, batch_id)
            is_ch_success = await self.storage.save_data_to_clickhouse(
                table_name="ods.bilibili_upload_videos_info",
                data_list=cleaned_data
            )
            if not is_ch_success:
                logger.error(f"[Task:{batch_id}] ClickHouse 写入失败，阻断级联任务触发。")
                return False  # 快速失败，保护数据完整性

        # --- 触发级联调度 ---
        is_cascade_success = True
        if recent_30_days_bvid_list and is_ch_success:
            try:
                # 独立 DB Session 的操作，必须捕获其可能抛出的异常
                affected_rows = cascade_register_videos_to_target(
                    uid=mid,
                    platform_type=3,
                    bvid_list=recent_30_days_bvid_list,
                    r_type="scrape_and_store_video_to_minio",
                    interval_minutes=720
                )
                logger.info(f"[Task:{batch_id}] 级联目标注册完成，触发 {affected_rows} 个后台调度。")
            except Exception as e:
                logger.error(f"[Task:{batch_id}] 级联目标注册(MySQL)发生崩溃: {str(e)}")
                is_cascade_success = False

        if bvid_list and is_ch_success:
            try:
                # 独立 DB Session 的操作，必须捕获其可能抛出的异常
                affected_rows = cascade_register_videos_to_target(
                    uid=mid,
                    platform_type=3,
                    bvid_list=bvid_list,
                    r_type="scrape_and_store_video_info",
                    interval_minutes=720
                )
                logger.info(f"[Task:{batch_id}] 级联目标注册完成，触发 {affected_rows} 个后台调度。")
            except Exception as e:
                logger.error(f"[Task:{batch_id}] 级联目标注册(MySQL)发生崩溃: {str(e)}")
                is_cascade_success = False

        # 只有当两个核心 I/O 操作都成功时，才向队列返回 ACK(True)
        return is_ch_success and is_cascade_success

    async def run_multimodal_analysis_loop(self, bvid: str, cid: str, batch_id: str) -> bool:
        """
        阶段 C：多模态大模型分析流水线 (长耗时后台任务)
        """
        try:
            logger.info(f"🚀 [Phase C] 启动深度分析: {bvid} (CID: {cid})")
            # 2. 从 ClickHouse 拉取关键数据
            async with ClickHouseManager.pool.connection() as ch_client:
                storage = StorageService(ch_client=ch_client)
                # 查关键帧
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
                # 查字幕
                sub_query = f"""
                            SELECT start_time_us, end_time_us, text 
                            FROM ods.bilibili_audio_info 
                            WHERE bvid = '{bvid}' AND cid = '{cid}'
                            ORDER BY start_time_us ASC
                        """
                subtitles = await storage.query_clickhouse(sub_query)
                # 查视频元数据
                info_query = f"""
                            SELECT title, introduction 
                            FROM ods.bilibili_video_info
                            WHERE bvid='{bvid}'
                            ORDER BY batch_id DESC
                            LIMIT 1
                        """
                video_info_list = await storage.query_clickhouse(info_query)
                video_title = video_info_list[0].get("title", "未知标题") if video_info_list else "未知标题"
                video_intro = video_info_list[0].get("introduction", "") if video_info_list else ""

            if not keyframes or not subtitles:
                logger.warning("⚠️ 数据不全，ClickHouse 中缺少当前视频的关键帧或字幕记录。")
                return False

            # 3. 运行汉堡包组装算法
            chunks = align_and_chunk_multimodal_data(keyframes, subtitles, chunk_size=15)
            logger.info(f"🍔 [Task {batch_id}] 汉堡包组装完毕，视频 {bvid} 共切分为 {len(chunks)} 个批次")

            previous_response = ""
            total_batches = len(chunks)
            final_report = None

            # 5. 循环推进大模型工作流
            for index, chunk in enumerate(chunks):
                current_batch = index + 1
                is_final_batch = (current_batch == total_batches)
                logger.info(f" [Task {batch_id}] 正在发送 {bvid} 批次 {current_batch}/{total_batches} (is_final={is_final_batch}) ...")

                # 平行数组拆解
                image_list = [{"file_id": item.get("coze_file_id")} for item in chunk if item.get("coze_file_id")]
                text_list = [f"【画面 {idx + 1}】[时间: {item.get('start_time', '')} - {item.get('end_time', '')}] 口播字幕: {item.get('text', '（画面无语音）')}" for idx, item in enumerate(chunk)]

                api_parameters = {
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

                kol_ads_result = await coze_client.run_multimodal_workflow(api_parameters)
                if not is_final_batch:
                    # 提取并继承线索
                    previous_response = kol_ads_result.get("summary_for_next", "")
                    logger.info(f"📝 [Phase C] 本轮提炼线索: {previous_response}")
                    await asyncio.sleep(3)  # 防限流
                else:
                    # 最后一轮，赋值给 final_report 准备落库
                    final_report = kol_ads_result
                    final_report['raw_ai_response'] = json.dumps(kol_ads_result, ensure_ascii=False)
                    logger.info(f"🎉 [Phase C] 最终商单分析完成: {bvid}")

            # 6. 状态机流转：标记为【分析成功】并写回数据库
            if not final_report:
                return False

            cleaned_data = DataCleaningService.clean_video_ai_analysis(final_report, bvid, cid, batch_id)
            async with ClickHouseManager.pool.connection() as ch_client:
                storage = StorageService(ch_client=ch_client)
                is_success = await storage.save_data_to_clickhouse(
                    table_name="ods.bilibili_video_ai_analysis",
                    data_list=cleaned_data
                )

            if not is_success:
                logger.error(f"❌ [Phase C] 视频 {bvid} 分析结果落盘失败！")
                return False

            logger.info(f"💾 [Phase C] 视频 {bvid} 分析结果已成功归档至 ClickHouse。")
            return True
        except Exception as e:
            error_trace = f"{str(e)}\n{traceback.format_exc()}"
            logger.error(f"❌ [Phase C] 视频 {bvid} (CID: {cid}) 分析崩溃: {error_trace}")
            return False