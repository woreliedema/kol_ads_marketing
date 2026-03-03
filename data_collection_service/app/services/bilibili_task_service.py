import asyncio
import math


from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler
from data_collection_service.app.services.data_cleaning_service import DataCleaningService
from data_collection_service.app.services.storage_service import StorageService
from data_collection_service.crawlers.utils.logger import logger



class BilibiliTaskService:
    def __init__(self, crawler: BilibiliWebCrawler, storage: StorageService):
        self.crawler = crawler
        self.storage = storage

    async def collect_and_store_video_comments(self, bvid: str, batch_id: str):
        """
        整合b站视频下所有评论的爬取、清洗、入库的完整流程
        """
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

        success = self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_video_comments",
            data_list=cleaned_data
        )

        if success:
            logger.info(f"[Task {batch_id}] 视频 {bvid} 全量评论信息入库成功")
        return success

    async def collect_and_store_user_info(self, mid: str, batch_id: str):
        """
        采集 B站 UP主基本信息并写入 ClickHouse
        """
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
        success = self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_user_info",
            data_list=cleaned_data
        )
        if success:
            logger.info(f"[Task {batch_id}] 用户 {mid} ({cleaned_data[0]['uname']}) 信息入库成功")
        return success

    async def collect_and_store_user_relation(self, mid: str, batch_id: str):
        """
        采集 B站 UP主基本信息并写入 ClickHouse
        """
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
        success = self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_user_relation",
            data_list=cleaned_data
        )
        if success:
            logger.info(f"[Task {batch_id}] 用户 {mid} 信息入库成功")
        return success

    async def collect_and_store_video_info(self, bvid: str, batch_id: str):
        """
        [Task] 采集 B站 视频基本信息并写入 ClickHouse
        """
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
        if not pages_data:
            logger.warning(f"[Task {batch_id}] 视频 {bvid} 无分P数据或解析为空")
        # 3. 写入 ClickHouse
        # 注意: table_name 必须与 ClickHouse 的表名一致
        success = self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_video_info",
            data_list=cleaned_data
        )
        pages_info_success = self.storage.save_data_to_clickhouse(
            table_name="ods.bilibili_video_pages_info",
            data_list=pages_data
        )
        if success:
            title = cleaned_data[0].get('title', 'Unknown')
            logger.info(f"[Task {batch_id}] 视频 {bvid}： ({title}) 信息入库成功")
        if pages_info_success:
            logger.info(f"[Task {batch_id}] 视频 {bvid} 分P数据入库成功，共 {len(pages_data)} 集")
        return True