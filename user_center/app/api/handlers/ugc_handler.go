package handlers

import (
	"context"
	"errors"
	"kol_ads_marketing/user_center/app/models"
	service "kol_ads_marketing/user_center/app/services"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"kol_ads_marketing/user_center/app/api/response"
)

// BindUGCAccountReq 绑定请求参数
type BindUGCAccountReq struct {
	Platform         string `json:"platform" vd:"$=='bilibili'||$=='douyin'||$=='tiktok';msg:'目前仅支持 bilibili, douyin, tiktok'"`
	PlatformSpaceURL string `json:"platform_space_url" vd:"required;msg:'个人主页链接不能为空'"`
}

// SyncUGCProfileReq 前端请求参数 (放 Query 里)
type SyncUGCProfileReq struct {
	Platform string `query:"platform" vd:"required"`
}

// BindUGCAccount 处理 KOL 绑定第三方平台账号的逻辑
// @Summary 绑定第三方 UGC 账号 (受保护)
// @Description 绑定 B站/抖音/小红书 账号。支持 MVP 阶段不传 auth_token 自动从 Redis 读取。
// @Tags UGC
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body BindUGCAccountReq true "绑定参数"
// @Success 200 {object} map[string]interface{} "成功返回绑定的平台和UID"
// @Router /api/v1/user/ugc/bind [post]
func BindUGCAccount(c context.Context, ctx *app.RequestContext) {
	// 1. 越权防御
	roleAny, _ := ctx.Get("role")
	if roleAny.(models.RoleType) != models.RoleKOL {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：仅限红人(KOL)使用")
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 2. 参数绑定与校验
	var req BindUGCAccountReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	// 3. 下推给 Service 层处理初始状态并分发任务
	isFresh, err := service.SubmitUGCBindTask(c, userID, req.Platform, req.PlatformSpaceURL)
	if err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	// Step 4: 前端交互降级
	if isFresh {
		// 命中了热点数据，直接秒级绑定成功
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"code":    200,
			"message": "绑定成功！您的平台数据已同步完毕",
			"data": map[string]interface{}{
				"auth_status": 1,
			},
		})
	} else {
		// 触发了异步爬虫，返回 202 Accepted 表示已接受请求并在后台处理
		ctx.JSON(http.StatusAccepted, map[string]interface{}{
			"code":    202,
			"message": "链接校验成功。数据正在后台排队抓取中，请稍后刷新页面查看",
			"data": map[string]interface{}{
				"auth_status": 0,
			},
		})
	}
}

// GetUGCBind 供前端轮询/刷新最新 UGC 数据的接口
// @Summary 手动/自动同步 UGC 最新数据
// @Tags UGC
// @Security ApiKeyAuth
// @Param platform query string true "平台名称 (如 bilibili)"
// @Router /api/v1/user/ugc/bind/result [get]
func GetUGCBind(c context.Context, ctx *app.RequestContext) {
	var req SyncUGCProfileReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 调用 Service
	account, err := service.GetUGCBindService(c, userID, req.Platform)
	if err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	// 把清洗后的干净数据丢给前端
	response.Success(ctx, map[string]interface{}{
		"platform":   account.Platform,
		"status":     account.AuthStatus,
		"nickname":   account.Nickname,
		"fans_count": account.FansCount,
		"bound_at":   account.BoundAt,
	})
}
