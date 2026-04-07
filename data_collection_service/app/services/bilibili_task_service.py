import asyncio
import math
from datetime import datetime, timedelta


from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler
from data_collection_service.app.services.data_cleaning_service import DataCleaningService
from data_collection_service.app.services.storage_service import StorageService
from data_collection_service.app.db.target_repository import cascade_register_videos_to_target
from data_collection_service.crawlers.utils.logger import logger



class BilibiliTaskService:
    def __init__(self, crawler: BilibiliWebCrawler, storage: StorageService):
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
        # 1. 调用底层爬虫获取原始数据 (复用 fetch_one_video)
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

    async def collect_and_store_user_videos(self, target_id: str, batch_id: str) -> bool:
        """
        采集用户投稿视频作品信息，只抓取近一年的数据，并写入 ClickHouse
        """
        mid = target_id
        page = 1
        all_videos = []
        bvid_list = []
        has_more = True

        # 计算时间边界：一年（365天）前的时间戳
        one_year_ago_ts = int((datetime.now() - timedelta(days=365)).timestamp())

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
        if bvid_list and is_ch_success:
            try:
                # 独立 DB Session 的操作，必须捕获其可能抛出的异常
                affected_rows = cascade_register_videos_to_target(
                    uid=mid,
                    platform_type=3,
                    bvid_list=bvid_list,
                    interval_minutes=1440
                )
                logger.info(f"[Task:{batch_id}] 级联目标注册完成，触发 {affected_rows} 个后台调度。")
            except Exception as e:
                logger.error(f"[Task:{batch_id}] 级联目标注册(MySQL)发生崩溃: {str(e)}")
                is_cascade_success = False

        # 只有当两个核心 I/O 操作都成功时，才向队列返回 ACK(True)
        return is_ch_success and is_cascade_success