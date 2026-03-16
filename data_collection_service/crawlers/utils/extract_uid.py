import re
PLATFORM_MAPPING = {
    "bilibili": 3,
    "douyin": 4,
    "tiktok": 2
}


# --- 2. 链接解析策略函数 ---
def extract_target_id_from_url(platform: str, url: str) -> str:
    """
    根据不同平台，使用正则从 URL 中提取唯一的资源 ID
    """
    if platform == "bilibili":
        # 匹配逻辑: 找到 space.bilibili.com/ 后面的连续数字
        # 示例 1: https://space.bilibili.com/292039314 -> 292039314
        # 示例 2: https://space.bilibili.com/2687303?spm_id_from=333 -> 2687303
        match = re.search(r"space\.bilibili\.com/(\d+)", url)
        if match:
            return match.group(1)
        raise ValueError("无法从该链接中解析出有效的 B站 UID，请检查链接格式")

    elif platform == "douyin":
        # 预留给抖音的解析逻辑，例如 www.douyin.com/user/MS4wLj...
        match = re.search(r"douyin\.com/user/([A-Za-z0-9_-]+)", url)
        if match:
            return match.group(1)
        raise ValueError("无法解析抖音主页链接")

    else:
        raise ValueError(f"暂不支持解析该平台的链接: {platform}")