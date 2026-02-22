import asyncio
import math


from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler
from data_collection_service.app.services.data_cleaning_service import DataCleaningService
from data_collection_service.app.services.storage_service import StorageService
from data_collection_service.crawlers.utils.logger import logger


class BilibiliCommentService:
    def __init__(self):
        self.crawler = BilibiliWebCrawler()

    async def scrape_video_all_comments(self, bv_id: str):
        """
        全量抓取指定视频的所有评论（含二级子评论）
        """
        all_comments = []

        # 1. 预先获取 AV 号 (oid)，避免在循环中重复请求
        try:
            aid = await self.crawler.bv_to_aid(bv_id)
        except Exception as e:
            logger.error(f"BV号转换失败: {e}")
            return []

        # --- 外层循环：获取主评论 ---
        page = 1
        while True:
            logger.info(f"正在抓取主评论第 {page} 页...")
            try:
                # 调用底层爬虫，传入 aid
                res = await self.crawler.fetch_video_comments_new(bv_id=bv_id, pn=page, aid=aid)

                # 检查数据有效性
                if not res or 'data' not in res or not res['data']['replies']:
                    logger.info("主评论抓取结束 (无更多数据)")
                    break

                replies = res['data']['replies']

                # 处理当前页的每一条主评论
                for root_reply in replies:
                    # 解析主评论基础信息
                    comment_data = self._parse_comment(root_reply, bv_id, is_sub=False)

                    # --- 内层检查：是否有子评论需要展开 ---
                    rcount = root_reply.get('rcount', 0)  # 回复总数
                    root_id = root_reply.get('rpid')

                    # 如果有子评论，且 B 站折叠了（通常 rcount > 3 需要翻页），或者为了保险起见，
                    # 我们可以把主评论自带的 `replies` 字段先存下来，如果还有更多页再抓取

                    sub_comments = []
                    # 先保存主接口返回的前几条子评论
                    if root_reply.get('replies'):
                        for sub in root_reply['replies']:
                            sub_comments.append(self._parse_comment(sub, bv_id, root_id=root_id))

                    # 如果子评论数量多，需要单独分页抓取子评论
                    if rcount > 0:
                        fetched_subs = await self._scrape_video_sub_comments(bv_id, aid, root_id, rcount)
                        # 去重合并（API返回的replies有时会和子评论接口第一页重复）
                        existing_ids = {s['rpid'] for s in sub_comments}
                        for fs in fetched_subs:
                            if fs['rpid'] not in existing_ids:
                                sub_comments.append(fs)

                    comment_data['sub_comments'] = sub_comments
                    all_comments.append(comment_data)

                # B站分页逻辑检查：通过 page 对象判断是否最后一页
                page_info = res['data'].get('page', {})
                if page * 20 >= page_info.get('count', 0):
                    break

                page += 1
                await asyncio.sleep(0.5)  # 稍微延时防止风控

            except Exception as e:
                logger.error(f"抓取主评论第 {page} 页异常: {e}")
                break

        return all_comments

    async def _scrape_video_sub_comments(self, bv_id, aid, root_id, total_count):
        """
        分页抓取指定主评论下的所有子评论
        """
        subs = []
        sub_page = 1
        # B站子评论一页通常 10-20 条，计算最大页数防止死循环
        max_page = (total_count // 20) + 2

        while sub_page <= max_page:
            try:
                # 调用底层爬虫获取子评论
                res = await self.crawler.fetch_comment_reply_new(bv_id, sub_page, root_id, aid=aid)

                if not res or 'data' not in res or not res['data']['replies']:
                    break

                for r in res['data']['replies']:
                    subs.append(self._parse_comment(r, bv_id, root_id=root_id))

                # 检查翻页
                page_info = res['data'].get('page', {})
                if sub_page * page_info.get('size', 20) >= page_info.get('count', 0):
                    break

                sub_page += 1
                await asyncio.sleep(0.2)  # 子评论请求频率可以稍快

            except Exception as e:
                logger.error(f"抓取子评论异常 rpid={root_id}: {e}")
                break
        return subs

    def _parse_comment(self, raw, bv_id, is_sub=True, root_id=None):
        """
        数据清洗：提取核心字段
        """
        return {
            "bv_id": bv_id,
            "rpid": str(raw.get('rpid')),
            "root_id": str(root_id) if is_sub else str(raw.get('rpid')),
            "parent_id": str(raw.get('parent', 0)),
            "content": raw.get('content', {}).get('message', ''),
            "mid": str(raw.get('mid')),
            "uname": raw.get('member', {}).get('uname'),
            "like": raw.get('like', 0),
            "ctime": raw.get('ctime'),  # 时间戳
            "is_sub": is_sub
        }


class BilibiliTaskService:
    def __init__(self, crawler: BilibiliWebCrawler, storage: StorageService):
        self.crawler = crawler
        self.storage = storage

    async def collect_and_store_video_comments(self, bvid: str):
        """
        整合爬取、清洗、入库的完整流程
        """
        logger.info(f"[Task] 开始执行视频 {bvid} 的评论采集任务")

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
                    sub_replies = reply_data.get('replies', [])
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
        cleaned_data = DataCleaningService.clean_bilibili_comments(all_comments, bvid, aid)
        logger.info(f"[清洗] 完成数据转换，清洗后产生 {len(cleaned_data)} 条标准化数据 (含子评论)")

        # 6. 通用化写入 ClickHouse
        # 即使未来换成动态评论、直播弹幕，这行代码都不用变，只需换 table_name 即可
        if cleaned_data:
            self.storage.save_data_to_clickhouse(
                table_name="ods.bilibili_video_comments",
                data_list=cleaned_data
            )



