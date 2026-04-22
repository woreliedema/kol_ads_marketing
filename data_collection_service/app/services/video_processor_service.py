import os
import aiofiles
import asyncio
import httpx
import tempfile
import subprocess
import glob
import shutil
import re
from fastapi import Request
from typing import Optional

from data_collection_service.crawlers.utils.logger import logger
from data_collection_service.app.services.storage_video_service import minio_video_client
from data_collection_service.app.services.coze_service import coze_client


class VideoProcessorService:
    @staticmethod
    async def fetch_data_stream(url: str, request: Request = None, headers: dict = None, file_path: str = None) -> bool:
        """流式下载文件到本地临时目录"""
        default_headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36'
        }
        headers = headers or default_headers

        try:
            async with httpx.AsyncClient() as client:
                async with client.stream("GET", url, headers=headers) as response:
                    response.raise_for_status()
                    async with aiofiles.open(file_path, 'wb') as out_file:
                        async for chunk in response.aiter_bytes():
                            # 仅当 Request 存在时才检查断开连接 (兼容后台任务无 Request 的场景)
                            if request and await request.is_disconnected():
                                logger.warning(f"[VideoProcessor] Client disconnected, cleaning up: {file_path}")
                                return False
                            await out_file.write(chunk)
            return True
        except Exception as e:
            logger.error(f"[VideoProcessor] Stream download failed: {e}")
            return False

    @staticmethod
    async def download_bilibili_streams(video_url: str, audio_url: str, video_path: str, audio_path: str, headers: dict) -> Optional[bool]:
        try:
            logger.info(f"[VideoProcessor] 并发下载 m4v 和 m4a 分离流到本地...")
            # 并发执行两个下载任务
            v_task = VideoProcessorService.fetch_data_stream(video_url, headers=headers, file_path=video_path)
            a_task = VideoProcessorService.fetch_data_stream(audio_url, headers=headers, file_path=audio_path)

            results = await asyncio.gather(v_task, a_task)

            if not all(results):
                logger.error("[VideoProcessor] 视频流或音频流下载失败。")
                return False
            return True
        except Exception as e:
            logger.error(f"[VideoProcessor] 并发下载异常: {e}")
            return False

    @staticmethod
    async def merge_bilibili_video_audio(video_url: str, audio_url: str, output_path: str, headers: dict,
                                         request: Optional[Request] = None) -> Optional[bool]:
        """下载并合并 Bilibili 的视频和音频流"""
        video_temp_path = tempfile.mktemp(suffix='.m4v')
        audio_temp_path = tempfile.mktemp(suffix='.m4a')

        try:
            logger.info(f"[VideoProcessor] Downloading Bilibili streams to temp files...")
            v_success = VideoProcessorService.fetch_data_stream(video_url, request, headers, video_temp_path)
            a_success = VideoProcessorService.fetch_data_stream(audio_url, request, headers, audio_temp_path)

            if not v_success or not a_success:
                logger.error("[VideoProcessor] Failed to download Bilibili streams.")
                return False

            logger.info("[VideoProcessor] Starting FFmpeg merge...")
            ffmpeg_cmd = [
                'ffmpeg', '-y',
                '-i', video_temp_path,
                '-i', audio_temp_path,
                '-c:v', 'copy',
                '-c:a', 'copy',
                '-f', 'mp4',
                output_path
            ]
            result = subprocess.run(ffmpeg_cmd, capture_output=True, text=True)

            if result.returncode != 0:
                logger.error(f"[VideoProcessor] FFmpeg error: {result.stderr}")
                return False

            return True
        except Exception as e:
            # 新增：捕获未知异常（如系统没装 ffmpeg），保证必定返回 bool，消除 IDE 警告
            logger.error(f"[VideoProcessor] Exception during Bilibili merge: {e}")
            return False
        finally:
            # 无论成功失败，必须清理流文件
            for p in [video_temp_path, audio_temp_path]:
                if os.path.exists(p):
                    os.remove(p)

    @staticmethod
    async def process_and_upload_video(platform: str, video_id: str, video_data: dict, headers: dict) -> Optional[dict]:
        temp_dir = "/tmp/kol_videos"
        os.makedirs(temp_dir, exist_ok=True)

        # 定义分离的本地临时路径
        local_m4v_path = os.path.join(temp_dir, f"{platform}_{video_id}.m4v")
        local_m4a_path = os.path.join(temp_dir, f"{platform}_{video_id}.m4a")

        try:
            video_url = video_data.get('nwm_video_url_HQ')
            audio_url = video_data.get('audio_url')

            # 1. 并发下载分离流 (I/O 密集型)
            success = await VideoProcessorService.download_bilibili_streams(
                video_url, audio_url, local_m4v_path, local_m4a_path, headers
            )
            if not success:
                logger.error(f"[VideoProcessor] 下载环节失败: {platform}_{video_id}")
                return None

            audio_object_name = f"audios/{platform}/{video_id}.m4a"

            # ==========================================
            # 2. 🚀 双轨并行处理阶段 (算力与网络密集型)
            # ==========================================
            # 轨道 A: 纯音频双写 (极速，给下游 ASR 备料)
            async def process_audio():
                minio_task = asyncio.to_thread(minio_video_client.upload_file, local_m4a_path, audio_object_name)
                coze_task = coze_client.upload_file(local_m4a_path)
                return await asyncio.gather(minio_task, coze_task)

            # 轨道 B: 纯视频流关键帧抽取与双写
            async def process_video_frames():
                # 注意这里传入的是 m4v 纯视频流的路径，FFmpeg 照样能完美抽帧
                return await VideoProcessorService.extract_and_upload_keyframes(
                    video_path=local_m4v_path, platform=platform, video_id=video_id
                )

            logger.info(f"[VideoProcessor] 开启双轨并行：音频极速双写 VS 视频智能抽帧...")
            # 并发执行两个轨道，彻底榨干服务器网络带宽和 CPU
            audio_results, frame_urls = await asyncio.gather(process_audio(), process_video_frames())

            minio_audio_url, coze_audio_file_id = audio_results

            if not minio_audio_url or not coze_audio_file_id:
                logger.error(f"[VideoProcessor] 音频 {video_id} 双写云端失败")
                return None

            # 返回音频 ID 用于 ASR，返回图片列表用于映射表
            return {
                "audio_url": minio_audio_url,
                "coze_audio_file_id": coze_audio_file_id,
                "file_name": audio_object_name,
                "frame_urls": frame_urls
            }

        finally:
            # 清理两个临时文件
            for p in [local_m4v_path, local_m4a_path]:
                if os.path.exists(p):
                    os.remove(p)
    # @staticmethod
    # async def process_and_upload_video(platform: str, video_id: str, video_data: dict, headers: dict) -> Optional[dict]:
    #     """
    #     核心业务流：下载 -> (B站合并) -> 存入临时文件 -> 上传 MinIO -> 清理临时文件 -> 返回云端 URL
    #     """
    #     temp_dir = "/tmp/kol_videos"
    #     os.makedirs(temp_dir, exist_ok=True)
    #     local_mp4_path = os.path.join(temp_dir, f"{platform}_{video_id}.mp4")
    #
    #     try:
    #         # 1. 执行下载/合并
    #         if platform == 'bilibili':
    #             video_url = video_data.get('nwm_video_url_HQ')
    #             audio_url = video_data.get('audio_url')
    #             success = await VideoProcessorService.merge_bilibili_video_audio(
    #                 video_url, audio_url, local_mp4_path, headers
    #             )
    #         else:
    #             video_url = video_data.get('nwm_video_url_HQ')
    #             success = await VideoProcessorService.fetch_data_stream(
    #                 video_url, headers=headers, file_path=local_mp4_path
    #             )
    #
    #         if not success:
    #             logger.error(f"[VideoProcessor] Processing failed for {platform}_{video_id}")
    #             return None
    #
    #         # 2. 上传到 MinIO 对象存储
    #         object_name = f"videos/{platform}/{video_id}.mp4"
    #         minio_task = asyncio.to_thread(minio_video_client.upload_file, local_mp4_path, object_name)
    #         coze_task = coze_client.upload_file(local_mp4_path)
    #
    #         minio_url, coze_file_id = await asyncio.gather(minio_task, coze_task)
    #
    #         # 3. 🔥【新增】在本地文件销毁前，执行关键帧提取与上传！
    #         frame_urls = await VideoProcessorService.extract_and_upload_keyframes(
    #             video_path=local_mp4_path,
    #             platform=platform,
    #             video_id=video_id
    #         )
    #
    #         return {
    #             "video_url": minio_url,
    #             "coze_file_id": coze_file_id,
    #             "file_name": object_name,
    #             "frame_urls": frame_urls
    #         }
    #
    #     finally:
    #         # 3. 极客好习惯：清理本地磁盘
    #         if os.path.exists(local_mp4_path):
    #             os.remove(local_mp4_path)

    @staticmethod
    async def extract_and_upload_keyframes(video_path: str, platform: str, video_id: str) -> Optional[list[dict]]:
        """
        核心动作：对本地视频进行场景感知抽帧，并批量上传至 MinIO
        :param platform: 平台名称
        :param video_path: 本地视频的临时路径 (如 /tmp/kol_videos/bilibili_BV1xx_123.mp4)
        :param video_id: 复合 ID (bvid_cid)
        :return: 包含所有抽帧图片 MinIO URL 的列表
        """
        # 为该视频创建一个专属的抽帧临时目录，防止并发冲突
        frames_dir = f"/tmp/kol_videos/frames_{platform}_{video_id}"
        os.makedirs(frames_dir, exist_ok=True)

        try:
            logger.info(f"[Keyframe Extraction] 正在为 {video_id} 执行智能场景抽帧...")

            # 极客级 FFmpeg 抽帧命令解析：
            # -vf "select='gt(scene,0.3)',scale=-1:480" -> 当场景变化率大于30%时抽帧，并等比例缩放至高度480p（极大降低图片体积和大模型Token成本）
            # -vsync vfr -> 可变帧率，配合 select 滤镜使用
            # -q:v 2 -> 保证图片质量清晰（范围2-31，越小越清晰）
            output_pattern = os.path.join(frames_dir, "frame_%04d.jpg")
            ffmpeg_cmd = [
                'ffmpeg', '-y',
                '-i', video_path,
                '-vf', "select='gt(scene,0.3)',showinfo,scale=-1:480",
                '-vsync', '0',
                '-q:v', '2',
                output_pattern
            ]

            # result = await asyncio.to_thread(subprocess.run,ffmpeg_cmd, capture_output=True, text=True)
            # 将 subprocess.run 的完整调用包进 lambda 函数中
            result = await asyncio.to_thread(lambda: subprocess.run(ffmpeg_cmd, capture_output=True, text=True))
            if result.returncode != 0:
                logger.error(f"[Keyframe Extraction] FFmpeg 抽帧失败: {result.stderr}")
                return []

            # 🔥 极客核心逻辑：解析时间戳并与图片映射
            # FFmpeg showinfo 日志格式示例: [Parsed_showinfo_1 @ 0x...] n: 0 pts: 12345 pts_time:2.300000 ...
            # 通过正则精准提取每一张输出图片对应的 pts_time (秒级浮点数)
            pts_times = re.findall(r"pts_time:\s*(\d+\.?\d*)", result.stderr)
            # 收集生成的图片列表
            frame_files = sorted(glob.glob(os.path.join(frames_dir, "*.jpg")))
            logger.info(f"[Keyframe Extraction] 抽帧完成，共提取 {len(frame_files)} 张高价值关键帧。")

            if len(pts_times) != len(frame_files):
                logger.warning(f"[Keyframe Extraction] ⚠️ 时间戳数量与图片数量不一致！采用最后匹配策略。")
            # 批量上传至 MinIO

            frame_results = []

            # 将图片路径与时间戳进行拉链操作 (zip)
            for frame_file, pts_time_str in zip(frame_files, pts_times):
                try:
                    # 转换: 2.300000 秒 -> 2300000 微秒
                    timestamp_us = int(float(pts_time_str) * 1_000_000)

                    # 【终极命名】: images/bilibili/BV1xx_123/2300000.jpg
                    object_name = f"images/{platform}/{video_id}/{timestamp_us}.jpg"
                    # 包装双写任务（minio 的客户端是同步的，所以用 to_thread 扔进线程池防阻塞）
                    minio_task = asyncio.to_thread(minio_video_client.upload_file, frame_file, object_name)
                    coze_task = coze_client.upload_file(frame_file)

                    # 并发等待当前这张图片的双写完成
                    minio_url, coze_file_id = await asyncio.gather(minio_task, coze_task)

                    if minio_url and coze_file_id:
                        frame_results.append({
                            "timestamp_us": timestamp_us,  # 建议保留，供外层参考
                            "file_name": object_name,
                            "file_url": minio_url,
                            "coze_file_id": coze_file_id
                        })
                    else:
                        logger.warning(f"[Keyframe Extraction] 关键帧 {timestamp_us}.jpg 双写异常 (MinIO: {bool(minio_url)}, Coze: {bool(coze_file_id)})")
                    await asyncio.sleep(0.2)
                except Exception as loop_e:
                    logger.error(f"[Keyframe Extraction] 单张关键帧 {frame_file} 处理异常: {loop_e}")
                    continue

            logger.info(f"[Keyframe Extraction] 成功双写上传 {len(frame_results)} 张关键帧。")
            return frame_results

        except Exception as e:
            logger.error(f"[Keyframe Extraction] 抽帧流程发生异常: {e}")
            return []

        finally:
            # 极客好习惯：清理本地图片临时文件夹
            if os.path.exists(frames_dir):
                shutil.rmtree(frames_dir)