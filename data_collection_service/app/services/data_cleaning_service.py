import json
from datetime import datetime
from typing import List, Dict, Any


class DataCleaningService:
    """
    数据清洗服务：负责将爬虫获取的各种异构原始数据转换为 ClickHouse 强类型标准字典
    """

    @classmethod
    def clean_bilibili_video_comments(cls, raw_comments: List[Dict[str, Any]], bvid: str, oid: int) -> List[Dict[str, Any]]:
        cleaned_data = []
        if not raw_comments:
            return cleaned_data

        for item in raw_comments:
            # 处理主评论
            root_record = cls._parse_video_single_comment(item, bvid, oid)
            if cls._validate_data(root_record):
                cleaned_data.append(root_record)

            # 处理子评论 (楼中楼), 加上 or [] 防止 replies 为 None
            replies = item.get('replies') or []
            for reply in replies:
                sub_record = cls._parse_video_single_comment(reply, bvid, oid)
                if cls._validate_data(sub_record):
                    cleaned_data.append(sub_record)

        return cleaned_data

    @classmethod
    def _parse_video_single_comment(cls, raw: Dict[str, Any], bvid: str, oid: int) -> Dict[str, Any]:
        """

        :param raw:
        :param bvid:
        :param oid:
        :return:
        """
        member = raw.get('member', {})
        fans_detail = member.get('fans_detail') or {}
        content = raw.get('content', {})

        # 提取提及用户和跳转链接
        mentions = content.get('members') or []
        mentions_mids = [cls._safe_string(m.get('mid', '')) for m in mentions if m.get('mid')]

        jump_url = content.get('jump_url', {})

        return {
            'rpid': cls._safe_int(raw.get('rpid_str', raw.get('rpid', 0))),
            'oid': oid,
            'bvid': bvid,
            'root_id': cls._safe_int(raw.get('root_str', raw.get('root', 0))),
            'parent_id': cls._safe_int(raw.get('parent_str', raw.get('parent', 0))),
            'dialog_id': cls._safe_int(raw.get('dialog_str', raw.get('dialog', 0))),
            'state': cls._safe_int(raw.get('state', 0)),

            # 用户维度
            'mid': cls._safe_int(member.get('mid_str', member.get('mid', 0))),
            'uname': cls._safe_string(member.get('uname', '')),
            'sign': cls._safe_string(member.get('sign', '')),
            'user_level': cls._safe_int(member.get('level_info', {}).get('current_level', 0)),
            'user_sex': cls._safe_string(member.get('sex', '保密')),
            'vip_type': cls._safe_int(member.get('vip', {}).get('vipType', 0)),

            # 粉丝牌维度
            'medal_uid': cls._safe_int(fans_detail.get('uid', 0)),
            'medal_id': cls._safe_int(fans_detail.get('medal_id', 0)),
            'medal_name': cls._safe_string(fans_detail.get('medal_name', '')),
            'medal_level': cls._safe_int(fans_detail.get('level', 0)),
            'medal_guard_level': cls._safe_int(fans_detail.get('guard_level', 0)),

            # 内容维度
            'message': cls._safe_string(content.get('message', raw.get('msg', ''))),
            'mentions_mids': mentions_mids,
            'jump_url_title': cls._safe_string(jump_url.get('title', '')),
            'jump_url': cls._safe_string(jump_url.get('url', '')),
            # 'official_verify': json.dumps(member.get('official_verify', {}), ensure_ascii=False),

            # 指标及时间维度
            'like_count': cls._safe_int(raw.get('like', 0)),
            'count': cls._safe_int(raw.get('count', 0)),
            'reply_count': cls._safe_int(raw.get('rcount', 0)),
            'ctime': cls._safe_datetime(raw.get('ctime', 0)),
            'ctime_ts': cls._safe_int(raw.get('ctime', 0))
        }

    @classmethod
    def clean_user_info(cls, raw_data: Dict[str, Any]) -> List[Dict[str, Any]]:
        """
        清洗 B站 UP主个人信息，适配 ods.bilibili_user_info 表结构
        :param raw_data: fetch_user_profile 接口返回的原始 JSON
        """
        if not raw_data or raw_data.get('code') != 0:
            return []

        data = raw_data.get('data', {})
        if not data:
            return []

        # 1. 提取嵌套对象，使用 get 防止 NoneType 报错
        vip = data.get('vip') or {}
        official = data.get('official') or {}
        live_room = data.get('live_room') or {}
        # 优先获取 top_photo_v2 中的高清图，如果不存在则降级使用 top_photo
        top_photo_v2 = data.get('top_photo_v2') or {}
        elec = data.get('elec', {}).get('show_info') or {}

        # 2. 构造符合 ClickHouse DDL 的字典
        user_info = {
            # --- 基础信息 ---
            'mid': cls._safe_int(data.get('mid')),
            'uname': cls._safe_string(data.get('name')),  # 注意：API返回name，库表字段uname
            'sign': cls._safe_string(data.get('sign')),
            'level': cls._safe_int(data.get('level')),
            'sex': cls._safe_string(data.get('sex')),
            'face': cls._safe_string(data.get('face')),
            # 优先取 v2 的 l_img，没有则取旧字段
            'top_photo': cls._safe_string(top_photo_v2.get('l_img') or data.get('top_photo')),

            # --- 认证与身份 ---
            'official_role': cls._safe_int(official.get('role')),
            'official_title': cls._safe_string(official.get('title')),
            'vip_type': cls._safe_int(vip.get('type')),
            'vip_status': cls._safe_int(vip.get('status')),

            # --- 直播数据 ---
            'live_room_id': cls._safe_int(live_room.get('roomid')),
            'live_room_title': cls._safe_string(live_room.get('title')),
            'live_status': cls._safe_int(live_room.get('liveStatus')),  # 0=未播, 1=直播中
            'live_url': cls._safe_string(live_room.get('url')),

            # --- 商业潜力 ---
            'charge_total': cls._safe_int(elec.get('total')),  # 充电人数
            'tags': data.get('tags') or []  # 已经是 List[str]，CK Array(String) 可直接接收，同时防止NoneType报错
        }

        return [user_info]

    @classmethod
    def clean_video_info(cls, raw_data: Dict[str, Any]) -> List[Dict[str, Any]]:
        """
        清洗逻辑 - 适配 ods.bilibili_video_info 新版数据字典
        """
        if not raw_data or raw_data.get('code') != 0:
            return []

        data = raw_data.get('data', {})
        if not data:
            return []

        owner = data.get('owner', {})
        stat = data.get('stat', {})
        rights = data.get('rights', {})

        # 处理 pages 和 honor 为 JSON 字符串
        honor_reply_json = json.dumps(data.get('honor_reply', {}), ensure_ascii=False)

        video_info = {
            # --- 基础信息 ---
            'bvid': cls._safe_string(data.get('bvid')),
            'oid': cls._safe_int(data.get('aid')),  # 映射: aid -> oid
            'title': cls._safe_string(data.get('title')),
            'introduction': cls._safe_string(data.get('desc')),  # 映射: desc -> introduction
            'introduction_v2': cls._safe_string(data.get('desc_v2')),
            'videos': cls._safe_int(data.get('videos')),
            'tid': cls._safe_int(data.get('tid')),
            'tid_v2': cls._safe_int(data.get('tid_v2')),
            'tname': cls._safe_string(data.get('tname')),
            'tname_v2': cls._safe_string(data.get('tname_v2')),
            'copyright': cls._safe_int(data.get('copyright')),
            'pic': cls._safe_string(data.get('pic')),
            'duration': cls._safe_int(data.get('duration')),
            'state': cls._safe_int(data.get('state')),
            'mission_id': cls._safe_int(data.get('mission_id')),
            # --- UP主信息 ---
            'mid': cls._safe_int(owner.get('mid')),
            'uname': cls._safe_string(owner.get('name')),  # 映射: name -> uname
            'face': cls._safe_string(owner.get('face')),
            # --- 权限信息 ---
            'is_cooperation': cls._safe_int(rights.get('is_cooperation')),
            'no_reprint': cls._safe_int(rights.get('no_reprint')),
            'elec': cls._safe_int(rights.get('elec')),
            'is_stein_gate': cls._safe_int(rights.get('is_stein_gate')),
            # --- 统计指标 (注意重命名) ---
            'views_count': cls._safe_int(stat.get('view')),
            'danmaku_count': cls._safe_int(stat.get('danmaku')),
            'replys_count': cls._safe_int(stat.get('reply')),
            'favorites_count': cls._safe_int(stat.get('favorite')),
            'coin_count': cls._safe_int(stat.get('coin')),
            'share_count': cls._safe_int(stat.get('share')),
            'likes_count': cls._safe_int(stat.get('like')),
            'now_rank': cls._safe_int(stat.get('now_rank')),
            'his_rank': cls._safe_int(stat.get('his_rank')),
            'honor_reply_json': honor_reply_json,
            # --- 时间维度 ---
            'ctime': cls._safe_datetime(data.get('ctime')),
            'ctime_ts': cls._safe_int(data.get('ctime')),
            'pubdate': cls._safe_datetime(data.get('pubdate')),
            'pubdate_ts': cls._safe_int(data.get('pubdate'))
        }

        return [video_info]

    @classmethod
    def clean_video_pages_info(cls, raw_data: Dict[str, Any]) -> List[Dict[str, Any]]:
        """
        清洗 B站 视频分P列表数据 (pages)
        适配 ods.bilibili_video_pages 表结构
        """
        # 1. 基础校验
        if not raw_data or raw_data.get('code') != 0:
            return []
        data = raw_data.get('data', {})
        if not data:
            return []
        # 获取主视频的关联键
        bvid = cls._safe_string(data.get('bvid'))
        oid = cls._safe_int(data.get('aid'))
        # 获取 pages 列表
        pages = data.get('pages', [])
        if not pages:
            return []
        cleaned_pages = []
        # 2. 遍历 pages 列表，生成多条记录
        for p in pages:
            dimension = p.get('dimension', {})
            page_record = {
                # --- 关联键 ---
                'cid': cls._safe_int(p.get('cid')),
                'bvid': bvid,
                'oid': oid,
                # --- 分P详情 ---
                'page_num': cls._safe_int(p.get('page')),
                'part_title': cls._safe_string(p.get('part')),
                'duration': cls._safe_int(p.get('duration')),
                'from': cls._safe_string(p.get('from')),
                # --- 画质与预览 ---
                'width': cls._safe_int(dimension.get('width')),
                'height': cls._safe_int(dimension.get('height')),
                'rotate': cls._safe_int(dimension.get('rotate')),
                'first_frame': cls._safe_string(p.get('first_frame'))
            }
            cleaned_pages.append(page_record)

        return cleaned_pages

    @classmethod
    def _validate_data(cls, data: Dict[str, Any]) -> bool:
        return data.get('rpid', 0) != 0 and data.get('oid', 0) != 0 and bool(data.get('bvid'))

    @staticmethod
    def _safe_int(value: Any, default: int = 0) -> int:
        try:
            return int(value)
        except (ValueError, TypeError):
            return default

    @staticmethod
    def _safe_string(value: Any, default: str = '') -> str:
        if value is None: return default
        try:
            return str(value)
        except Exception:
            return default

    @staticmethod
    def _safe_datetime(timestamp: Any) -> datetime:
        try:
            return datetime.fromtimestamp(int(timestamp))
        except (ValueError, TypeError):
            return datetime.now()