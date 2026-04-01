package handlers

import (
	"context"
	"errors"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"
	"strconv"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type UGCAuthCallbackReq struct {
	UserID         uint64 `json:"user_id"`
	Platform       string `json:"platform"`
	AuthStatus     int8   `json:"auth_status"` // 1 成功, -1 失败
	PlatformUID    string `json:"platform_uid"`
	Nickname       string `json:"nickname"`
	FollowersCount int    `json:"followers_count"`
}

// GetInternalUserProfile 供内部微服务调用的用户信息查询接口
// @Summary 内部调用：查询用户详情
// @Description 无需 Token，供报价引擎或匹配系统通过 user_id 直接查询。
// @Tags Internal
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} map[string]interface{} "成功返回用户信息"
// @Router /api/internal/v1/user/{id}/profile [get]
func GetInternalUserProfile(c context.Context, ctx *app.RequestContext) {
	// 从 URL 路径中提取 user_id 参数 (如 /user/3/profile)
	idStr := ctx.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "无效的用户 ID")
		return
	}

	responseData, err := service.GetUserProfileService(c, userID)

	if err != nil {
		// 标准的错误拦截与断言
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			hlog.CtxErrorf(c, "GetInternalUserProfile 未知异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	response.Success(ctx, responseData)
}

// InternalUGCAuthCallback 供 Python 数据采集服务回调的内部接口
// @Router /api/internal/v1/user/ugc/callback [post]
func InternalUGCAuthCallback(c context.Context, ctx *app.RequestContext) {
	// 此接口无需用户 Token，但在中间件里应该校验内部的 INTERNAL_SECRET_KEY

	var req UGCAuthCallbackReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	// 更新 MySQL 底表，将爬虫抓到的数据回填，并把状态改为 1 (成功)
	updateData := map[string]interface{}{
		"auth_status":  req.AuthStatus,
		"platform_uid": req.PlatformUID,
		"nickname":     req.Nickname,
		"fans_count":   req.FollowersCount,
	}

	err := db.DB.Model(&models.UserUGCAccount{}).
		Where("user_id = ? AND platform = ?", req.UserID, req.Platform).
		Updates(updateData).Error

	if err != nil {
		hlog.CtxErrorf(c, "更新 UGC 回调数据失败: %v", err)
		response.Error(ctx, response.ErrDatabaseError)
		return
	}

	response.Success(ctx, map[string]interface{}{"message": "状态回写成功"})
}

// GetInternalUserInfo 获取单个用户详细信息 (内部调用)
// 路由: GET /api/v1/internal/user/info?user_id=123
func GetInternalUserInfo(c context.Context, ctx *app.RequestContext) {
	// 1. 内部接口不走 JWT 中间件，直接从 Query 参数获取目标用户 ID
	userIDStr := ctx.Query("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil || userID == 0 {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "目标 user_id 缺失或非法")
		return
	}

	// 2. 复用底层业务大脑 Service 层
	responseData, err := service.GetUserProfileService(c, userID)
	if err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			hlog.CtxErrorf(c, "GetInternalUserInfo 未知异常 UID: %d: %v", userID, err)
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	response.Success(ctx, responseData)
}

// BatchUserInfoReq 批量请求参数结构体
type BatchUserInfoReq struct {
	UIDs []uint64 `json:"uids"` // 前端/内部服务传入的 ID 列表
}

// BatchGetUserInfo 批量获取用户信息 (专供 IM 会话列表等场景调用)
// 路由: POST /api/v1/internal/users/batch_info
func BatchGetUserInfo(c context.Context, ctx *app.RequestContext) {
	var req BatchUserInfoReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "请求体格式错误")
		return
	}

	if len(req.UIDs) == 0 {
		response.Success(ctx, map[string]interface{}{"users": map[uint64]interface{}{}})
		return
	}

	// MVP 阶段：复用现有的单体 Service 进行循环查询
	// 进阶优化：未来可以在 Service 层写一个 BatchGetUserProfileService，直接用 SQL 的 `IN (?)` 或 Redis MGET 查出，性能更高
	resultMap := make(map[uint64]interface{})
	for _, uid := range req.UIDs {
		data, err := service.GetUserProfileService(c, uid)
		if err == nil {
			// 只有查询成功的才放入 Map，容错处理
			resultMap[uid] = data
		} else {
			hlog.CtxWarnf(c, "内部批量查询异常 UID: %d, Err: %v", uid, err)
		}
	}

	// 返回统一的 Map 结构给调用方（如 match_system_service）
	response.Success(ctx, map[string]interface{}{
		"users": resultMap,
	})
}
