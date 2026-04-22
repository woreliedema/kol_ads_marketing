from typing import List, Dict


def align_and_chunk_multimodal_data(keyframes: List[Dict], subtitles: List[Dict], chunk_size: int = 15) -> List[List[Dict]]:
    """
    汉堡式无损多模态数据对齐算法
    :param keyframes: [{'timestamp_us': 2300000, 'coze_file_id': 'xxx'}, ...] (需按时间升序)
    :param subtitles: [{'start_time_us': 1000, 'end_time_us': 4000, 'text': 'hello'}, ...] (需按时间升序)
    :param chunk_size: 每个批次最多包含的关键帧数量（受限于大模型上限）
    :return: 按照 chunk_size 切分好的 frames_and_subs 批次数组
    """

    # 1. 初始化关键帧的“汉堡桶”
    aligned_results = []
    for k in keyframes:
        aligned_results.append({
            'keyframe_time_us': k['timestamp_us'],
            'coze_file_id': k['coze_file_id'],
            'sub_start_time_us': None,
            'sub_end_time_us': None,
            'merged_text': []
        })

    # 2. 遍历字幕，将其“塞入”距离最近的关键帧汉堡中
    for sub in subtitles:
        # 计算当前字幕的中心时间点
        sub_mid_time = (sub['start_time_us'] + sub['end_time_us']) / 2

        # 寻找距离该字幕最近的关键帧
        closest_idx = 0
        min_diff = float('inf')

        for i, k in enumerate(keyframes):
            diff = abs(k['timestamp_us'] - sub_mid_time)
            if diff < min_diff:
                min_diff = diff
                closest_idx = i

        # 将字幕分配给最近的关键帧
        target_bucket = aligned_results[closest_idx]
        target_bucket['merged_text'].append(sub['text'])

        # 动态扩张汉堡的面包边（更新这段区间的极小起始和极大结束时间）
        if target_bucket['sub_start_time_us'] is None or sub['start_time_us'] < target_bucket['sub_start_time_us']:
            target_bucket['sub_start_time_us'] = sub['start_time_us']

        if target_bucket['sub_end_time_us'] is None or sub['end_time_us'] > target_bucket['sub_end_time_us']:
            target_bucket['sub_end_time_us'] = sub['end_time_us']

    # 3. 格式化清洗，处理那些没有匹配到任何字幕的“孤立关键帧”（比如纯风景空镜头）
    final_flattened = []
    for res in aligned_results:
        # 将微秒转换为易读的秒数（可选，大模型对秒的感知比微秒好）
        start_sec = round((res['sub_start_time_us'] or res['keyframe_time_us']) / 1_000_000, 2)
        end_sec = round((res['sub_end_time_us'] or res['keyframe_time_us']) / 1_000_000, 2)
        kf_sec = round(res['keyframe_time_us'] / 1_000_000, 2)

        final_flattened.append({
            "start_time": start_sec,
            "end_time": end_sec,
            "text": " ".join(res['merged_text']) if res['merged_text'] else "（画面无语音）",
            "keyframe_time": kf_sec,
            "coze_file_id": res['coze_file_id']
        })

    # 4. 按照大模型的限制（如 20 张图）进行数组分块 (Chunking)
    chunks = [final_flattened[i:i + chunk_size] for i in range(0, len(final_flattened), chunk_size)]

    return chunks