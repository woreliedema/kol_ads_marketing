import os
from datetime import timedelta
from minio import Minio
from minio.error import S3Error
from dotenv import load_dotenv

# 完美复用你已有的日志模块
from data_collection_service.crawlers.utils.logger import logger

# 显式加载根目录的 .env 文件
load_dotenv()


class StorageVideoService:
    def __init__(self):
        """
        初始化视频对象存储客户端 (MinIO/OSS)
        所有配置项均从环境变量或 .env 文件中动态读取
        """
        # 从环境变量读取配置，并提供 fallback 默认值防崩溃
        self.endpoint = os.getenv("MINIO_ENDPOINT", "localhost:19000")
        self.access_key = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
        self.secret_key = os.getenv("MINIO_SECRET_KEY", "minioadmin")
        self.bucket_name = os.getenv("MINIO_BUCKET_NAME", "kol-videos")

        # 将环境变量中的字符串布尔值安全地转换为 Python bool
        secure_str = os.getenv("MINIO_SECURE", "False")
        self.secure = secure_str.lower() in ("true", "1", "t")

        try:
            self.client = Minio(
                endpoint=self.endpoint,
                access_key=self.access_key,
                secret_key=self.secret_key,
                secure=self.secure
            )
            logger.info(f"[StorageVideoService] Initialized connecting to {self.endpoint} (Secure: {self.secure})")

            self._ensure_bucket_exists()
        except Exception as e:
            logger.error(f"[StorageVideoService] Failed to initialize MinIO client: {e}")

    def _ensure_bucket_exists(self):
        """确保存储桶存在，不存在则自动创建"""
        try:
            if not self.client.bucket_exists(self.bucket_name):
                self.client.make_bucket(self.bucket_name)
                logger.info(f"[StorageVideoService] Successfully created bucket: '{self.bucket_name}'")
            else:
                logger.info(f"[StorageVideoService] Bucket '{self.bucket_name}' already exists.")
        except S3Error as e:
            logger.error(f"[StorageVideoService] Failed to check/create bucket '{self.bucket_name}': {e}")

    def check_file_exists(self, object_name: str) -> bool:
        """检查文件是否已经在 MinIO 中存在"""
        try:
            self.client.stat_object(self.bucket_name, object_name)
            return True
        except S3Error as e:
            if e.code == 'NoSuchKey':
                return False
            else:
                logger.error(f"[StorageVideoService] 检查文件存在性时发生异常: {e}")
                return False

    def upload_file(self, local_file_path: str, object_name: str) -> str:
        """
        上传本地视频文件到对象存储
        :param local_file_path: 本地临时文件路径 (如 /tmp/xxx.mp4)
        :param object_name: 存储桶内的路径和文件名 (如 videos/douyin/123.mp4)
        :return: 可供 AI 服务拉取的 URL
        """
        try:
            # 执行文件上传
            self.client.fput_object(
                bucket_name=self.bucket_name,
                object_name=object_name,
                file_path=local_file_path,
            )
            logger.info(f"[StorageVideoService] Successfully uploaded '{local_file_path}' to '{object_name}'")

            # 返回内网/外网可访问的 URL
            return self.get_download_url(object_name)
        except S3Error as e:
            logger.error(f"[StorageVideoService] Upload failed for '{local_file_path}': {e}")
            return ""
        except FileNotFoundError:
            logger.error(f"[StorageVideoService] Local file not found: '{local_file_path}'")
            return ""

    def get_download_url(self, object_name: str, expires_days: int = 7) -> str:
        """
        获取文件的下载/拉流预签名 URL
        """
        try:
            url = self.client.presigned_get_object(
                bucket_name=self.bucket_name,
                object_name=object_name,
                expires=timedelta(days=expires_days)
            )
            logger.info(f"[StorageVideoService] Generated presigned URL for '{object_name}'")
            return url
        except S3Error as e:
            logger.error(f"[StorageVideoService] Failed to generate URL for '{object_name}': {e}")
            return ""


# 实例化一个单例供外部 router 或 worker 调用
minio_video_client = StorageVideoService()