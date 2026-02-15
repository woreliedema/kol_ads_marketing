from fastapi import APIRouter, Body, Query, Request, HTTPException  # 导入FastAPI组件


from data_collection_service.app.api.models.APIResponseModel import ResponseModel, ErrorResponseModel  # 导入响应模型
from data_collection_service.crawlers.bilibili.web_crawler import BilibiliWebCrawler  # 导入哔哩哔哩web爬虫
from data_collection_service.app.services.bilibili_task_service import BilibiliCommentService # 导入哔哩哔哩评论全量抓取爬虫

router = APIRouter()
BilibiliWebCrawler = BilibiliWebCrawler()
comment_service = BilibiliCommentService()

# 获取单个视频详情信息
@router.get("/fetch_one_video", response_model=ResponseModel, summary="获取单个视频详情信息/Get single video data")
async def fetch_one_video(request: Request,
                          bv_id: str = Query(example="BV1SEBxBSE8Q", description="作品id/Video id")):
    """
    # [中文]
    ### 用途:
    - 获取单个视频详情信息
    ### 参数:
    - bv_id: 作品id
    ### 返回:
    - 视频详情信息

    # [示例/Example]
    bv_id = "BV1SEBxBSE8Q"
    up主：风采 含广告插入，时间段：02:51~03:54，完整视频全长：07:41
    """
    try:
        data = await BilibiliWebCrawler.fetch_one_video(bv_id)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取视频流地址
@router.get("/fetch_video_playurl", response_model=ResponseModel, summary="获取视频流地址/Get video playurl")
async def fetch_video_playurl(request: Request,
                          bv_id: str = Query(example="BV1SEBxBSE8Q", description="作品id/Video id"),
                          cid:str = Query(example="171776208", description="作品cid/Video cid")):
    """
    # [中文]
    ### 用途:
    - 获取视频流地址
    ### 参数:
    - bv_id: 作品id
    - cid: 作品cid
    ### 返回:
    - 视频流地址

    # [示例/Example]
    bv_id = "BV1SEBxBSE8Q"
    cid = "171776208" 忘记了，得重新获取
    """
    try:
        data = await BilibiliWebCrawler.fetch_video_playurl(bv_id, cid)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取用户发布视频作品数据
@router.get("/fetch_user_post_videos", response_model=ResponseModel,
            summary="获取用户主页作品数据/Get user homepage video data")
async def fetch_user_post_videos(request: Request,
                                 uid: str = Query(example="178360345", description="用户UID"),
                                 pn: int = Query(default=1, description="页码/Page number"),):
    """
    # [中文]
    ### 用途:
    - 获取用户发布的视频数据
    ### 参数:
    - uid: 用户UID
    - pn: 页码
    ### 返回:
    - 用户发布的视频数据

    # [示例/Example]
    uid = "178360345"
    pn = 1
    """
    try:
        data = await BilibiliWebCrawler.fetch_user_post_videos(uid, pn)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取用户所有收藏夹信息
@router.get("/fetch_collect_folders", response_model=ResponseModel,
            summary="获取用户所有收藏夹信息/Get user collection folders")
async def fetch_collect_folders(request: Request,
                                uid: str = Query(example="178360345", description="用户UID")):
    """
    # [中文]
    ### 用途:
    - 获取用户收藏作品数据
    ### 参数:
    - uid: 用户UID
    ### 返回:
    - 用户收藏夹信息

    # [示例/Example]
    uid = "178360345"
    """
    try:
        data = await BilibiliWebCrawler.fetch_collect_folders(uid)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定收藏夹内视频数据
@router.get("/fetch_user_collection_videos", response_model=ResponseModel,
            summary="获取指定收藏夹内视频数据/Gets video data from a collection folder")
async def fetch_user_collection_videos(request: Request,
                                       folder_id: str = Query(example="1756059545",
                                                              description="收藏夹id/collection folder id"),
                                       pn: int = Query(default=1, description="页码/Page number")
                                       ):
    """
    # [中文]
    ### 用途:
    - 获取指定收藏夹内视频数据
    ### 参数:
    - folder_id: 用户UID
    - pn: 页码
    ### 返回:
    - 指定收藏夹内视频数据

    # [示例/Example]
    folder_id = "1756059545"
    pn = 1
    """
    try:
        data = await BilibiliWebCrawler.fetch_folder_videos(folder_id, pn)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定用户的信息
@router.get("/fetch_user_profile", response_model=ResponseModel,
            summary="获取指定用户的信息/Get information of specified user")
async def fetch_user_profile(request: Request,
                                uid: str = Query(example="178360345", description="用户UID")):
    """
    # [中文]
    ### 用途:
    - 获取指定用户的信息
    ### 参数:
    - uid: 用户UID
    ### 返回:
    - 指定用户的个人信息

    # [示例/Example]
    uid = "178360345"
    """
    try:
        data = await BilibiliWebCrawler.fetch_user_profile(uid)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取综合热门视频信息
@router.get("/fetch_com_popular", response_model=ResponseModel,
            summary="获取综合热门视频信息/Get comprehensive popular video information")
async def fetch_com_popular(request: Request,
                                pn: int = Query(default=1, description="页码/Page number")):
    """
    # [中文]
    ### 用途:
    - 获取综合热门视频信息
    ### 参数:
    - pn: 页码
    ### 返回:
    - 综合热门视频信息

    # [示例/Example]
    pn = 1
    """
    try:
        data = await BilibiliWebCrawler.fetch_com_popular(pn)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定视频的评论
@router.get("/fetch_video_comments", response_model=ResponseModel,
            summary="获取指定视频的评论/Get comments on the specified video")
async def fetch_video_comments(request: Request,
                                bv_id: str = Query(example="BV1SEBxBSE8Q", description="作品id/Video id"),
                                pn: int = Query(default=1, description="页码/Page number")):
    """
    # [中文]
    ### 用途:
    - 获取指定视频的评论
    ### 参数:
    - bv_id: 作品id
    - pn: 页码
    ### 返回:
    - 指定视频的评论数据

    # [示例/Example]
    bv_id = "BV1SEBxBSE8Q"
    pn = 1
    """
    try:
        data = await BilibiliWebCrawler.fetch_video_comments(bv_id, pn)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取视频下指定评论的回复
@router.get("/fetch_comment_reply", response_model=ResponseModel,
            summary="获取视频下指定评论的回复/Get reply to the specified comment")
async def fetch_comment_reply(request: Request,
                                bv_id: str = Query(example="BV1SEBxBSE8Q", description="作品id/Video id"),
                                pn: int = Query(default=1, description="页码/Page number"),
                                rpid: str = Query(example="237109455120", description="回复id/Reply id")):
    """
    # [中文]
    ### 用途:
    - 获取视频下指定评论的回复
    ### 参数:
    - bv_id: 作品id
    - pn: 页码
    - rpid: 回复id
    ### 返回:
    - 指定评论的回复数据

    # [示例/Example]
    bv_id = "BV1SEBxBSE8Q"
    pn = 1
    rpid = "237109455120"
    """
    try:
        data = await BilibiliWebCrawler.fetch_comment_reply(bv_id, pn, rpid)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定用户动态
@router.get("/fetch_user_dynamic", response_model=ResponseModel,
            summary="获取指定用户动态/Get dynamic information of specified user")
async def fetch_user_dynamic(request: Request,
                                uid: str = Query(example="16015678", description="用户UID"),
                                offset: str = Query(default="", example="953154282154098691",
                                                    description="开始索引/offset")):
    """
    # [中文]
    ### 用途:
    - 获取指定用户动态
    ### 参数:
    - uid: 用户UID
    - offset: 开始索引
    ### 返回:
    - 指定用户动态数据

    # [示例/Example]
    uid = "178360345"
    offset = "953154282154098691"
    """
    try:
        data = await BilibiliWebCrawler.fetch_user_dynamic(uid, offset)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取视频实时弹幕
@router.get("/fetch_video_danmaku", response_model=ResponseModel, summary="获取视频实时弹幕/Get Video Danmaku")
async def fetch_video_danmaku(request: Request,
                          cid: str = Query(example="1639235405", description="作品cid/Video cid")):
    """
    # [中文]
    ### 用途:
    - 获取视频实时弹幕
    ### 参数:
    - cid: 作品cid
    ### 返回:
    - 视频实时弹幕

    # [示例/Example]
    cid = "1639235405"
    """
    try:
        data = await BilibiliWebCrawler.fetch_video_danmaku(cid)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定直播间信息
@router.get("/fetch_live_room_detail", response_model=ResponseModel,
            summary="获取指定直播间信息/Get information of specified live room")
async def fetch_live_room_detail(request: Request,
                                room_id: str = Query(example="22816111", description="直播间ID/Live room ID")):
    """
    # [中文]
    ### 用途:
    - 获取指定直播间信息
    ### 参数:
    - room_id: 直播间ID
    ### 返回:
    - 指定直播间信息

    # [示例/Example]
    room_id = "22816111"
    """
    try:
        data = await BilibiliWebCrawler.fetch_live_room_detail(room_id)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定直播间视频流
@router.get("/fetch_live_videos", response_model=ResponseModel,
            summary="获取直播间视频流/Get live video data of specified room")
async def fetch_live_videos(request: Request,
                                room_id: str = Query(example="1815229528", description="直播间ID/Live room ID")):
    """
    # [中文]
    ### 用途:
    - 获取指定直播间视频流
    ### 参数:
    - room_id: 直播间ID
    ### 返回:
    - 指定直播间视频流

    # [示例/Example]
    room_id = "1815229528"
    """
    try:
        data = await BilibiliWebCrawler.fetch_live_videos(room_id)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取指定分区正在直播的主播
@router.get("/fetch_live_streamers", response_model=ResponseModel,
            summary="获取指定分区正在直播的主播/Get live streamers of specified live area")
async def fetch_live_streamers(request: Request,
                                area_id: str = Query(example="9", description="直播分区id/Live area ID"),
                                pn: int = Query(default=1, description="页码/Page number")):
    """
    # [中文]
    ### 用途:
    - 获取指定分区正在直播的主播
    ### 参数:
    - area_id: 直播分区id
    - pn: 页码
    ### 返回:
    - 指定分区正在直播的主播

    # [示例/Example]
    area_id = "9"
    pn = 1
    """
    try:
        data = await BilibiliWebCrawler.fetch_live_streamers(area_id, pn)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取所有直播分区列表
@router.get("/fetch_all_live_areas", response_model=ResponseModel,
            summary="获取所有直播分区列表/Get a list of all live areas")
async def fetch_all_live_areas(request: Request,):
    """
    # [中文]
    ### 用途:
    - 获取所有直播分区列表
    ### 参数:
    ### 返回:
    - 所有直播分区列表

    # [示例/Example]
    """
    try:
        data = await BilibiliWebCrawler.fetch_all_live_areas()
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 通过bv号获得视频aid号
@router.get("/bv_to_aid", response_model=ResponseModel, summary="通过bv号获得视频aid号/Generate aid by bvid")
async def bv_to_aid(request: Request,
                          bv_id: str = Query(example="BV1SEBxBSE8Q", description="作品id/Video id")):
    """
    # [中文]
    ### 用途:
    - 通过bv号获得视频aid号
    ### 参数:
    - bv_id: 作品id
    ### 返回:
    - 视频aid号

    # [示例/Example]
    bv_id = "BV1SEBxBSE8Q"
    """
    try:
        data = await BilibiliWebCrawler.bv_to_aid(bv_id)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 通过bv号获得视频分p信息
@router.get("/fetch_video_parts", response_model=ResponseModel, summary="通过bv号获得视频分p信息/Get Video Parts By bvid")
async def fetch_video_parts(request: Request,
                          bv_id: str = Query(example="BV1vf421i7hV", description="作品id/Video id")):
    """
    # [中文]
    ### 用途:
    - 通过bv号获得视频分p信息
    ### 参数:
    - bv_id: 作品id
    ### 返回:
    - 视频分p信息

    # [示例/Example]
    bv_id = "BV1vf421i7hV"
    """
    try:
        data = await BilibiliWebCrawler.fetch_video_parts(bv_id)
        return ResponseModel(code=200,
                             router=request.url.path,
                             data=data)
    except Exception as e:
        status_code = 400
        detail = ErrorResponseModel(code=status_code,
                                    router=request.url.path,
                                    params=dict(request.query_params),
                                    )
        raise HTTPException(status_code=status_code, detail=detail)


# 获取b站全量评论数据
@router.get("/scrape_all_comments", response_model=ResponseModel, summary="全量抓取视频评论(含子评论)")
async def scrape_all_comments(request: Request,
                                bv_id: str = Query(..., description="视频BV号")):
    """
    注意：此接口耗时较长，建议异步调用或仅用于测试。
    生产环境应通过 /task/create 接口提交任务到 Kafka 后台执行。
    """
    try:
        # 调用业务层逻辑
        data = await comment_service.scrape_all_comments(bv_id)

        return ResponseModel(
            code=200,
            router=request.url.path,
            data={
                "total_count": len(data),
                "comments": data  # 包含嵌套的 sub_comments
            }
        )
    except Exception as e:
        status_code = 500
        detail = ErrorResponseModel(
            code=status_code,
            router=request.url.path,
            message=str(e)
        )
        raise HTTPException(status_code=status_code, detail=detail)