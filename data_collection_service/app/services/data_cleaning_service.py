from datetime import datetime

class DataCleaningService:
    """
    数据清洗服务：负责将原始采集数据转换为标准格式
    """

    @staticmethod
    def clean_bilibili_comments(raw_comments: list, video_id: str) -> list:
        """
        清洗B站评论数据，将嵌套结构扁平化
        """
        cleaned_data = []
        if not raw_comments:
            return cleaned_data

        for item in raw_comments:
            # 1. 处理主评论
            root = DataCleaningService._parse_single_comment(item, video_id, is_sub=0)
            cleaned_data.append(root)

            # 2. 处理子评论 (如果有)
            if 'sub_comments' in item and item['sub_comments']:
                for sub in item['sub_comments']:
                    sub_record = DataCleaningService._parse_single_comment(sub, video_id, is_sub=1)
                    cleaned_data.append(sub_record)

        return cleaned_data

    @staticmethod
    def _parse_single_comment(item: dict, video_id: str, is_sub: int) -> dict:
        """内部辅助方法：解析单条评论"""
        return {
            'rpid': str(item.get('rpid')),
            'video_id': video_id,
            'parent_id': str(item.get('parent_id', 0)),
            'root_id': str(item.get('root_id', 0)),
            'user_id': str(item.get('user_id') or item.get('mid')),
            'user_name': item.get('user_name') or item.get('uname'),
            'content': item.get('content'),
            'like_count': int(item.get('like_count') or item.get('like', 0)),
            'create_time': item.get('create_time') or datetime.now(),  # 需确保格式正确
            'is_sub': is_sub
        }