import json
from datetime import datetime
from typing import List, Dict, Any


class DataCleaningService:
    """
    数据清洗服务：负责将爬虫获取的各种异构原始数据转换为 ClickHouse 强类型标准字典
    """

    @classmethod
    def clean_bilibili_comments(cls, raw_comments: List[Dict[str, Any]], bvid: str, oid: int) -> List[Dict[str, Any]]:
        cleaned_data = []
        if not raw_comments:
            return cleaned_data

        for item in raw_comments:
            # 处理主评论
            root_record = cls._parse_single_comment(item, bvid, oid)
            if cls._validate_data(root_record):
                cleaned_data.append(root_record)

            # 处理子评论 (楼中楼)
            replies = item.get('replies', [])
            for reply in replies:
                sub_record = cls._parse_single_comment(reply, bvid, oid)
                if cls._validate_data(sub_record):
                    cleaned_data.append(sub_record)

        return cleaned_data

    @classmethod
    def _parse_single_comment(cls, raw: Dict[str, Any], bvid: str, oid: int) -> Dict[str, Any]:
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
        mentions = content.get('members', [])
        mentions_mids = [cls._safe_int(m.get('mid', 0)) for m in mentions if m.get('mid')]

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
            'official_verify': json.dumps(member.get('official_verify', {}), ensure_ascii=False),

            # 指标及时间维度
            'like_count': cls._safe_int(raw.get('like', 0)),
            'count': cls._safe_int(raw.get('count', 0)),
            'reply_count': cls._safe_int(raw.get('rcount', 0)),
            'ctime': cls._safe_datetime(raw.get('ctime', 0)),
            'ctime_ts': cls._safe_int(raw.get('ctime', 0))
        }

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