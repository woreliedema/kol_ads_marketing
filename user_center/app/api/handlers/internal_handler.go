package handlers

import (
	"context"
	"errors"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"
	"strconv"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/services"

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
