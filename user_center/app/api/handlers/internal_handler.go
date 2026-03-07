package handlers

import (
	"context"
	"strconv"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

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

	// 1. 查询基础表获取角色
	var user models.SysUser
	if err := db.DB.Select("id", "username", "role", "status").First(&user, userID).Error; err != nil {
		response.Error(ctx, response.ErrUserNotFound)
		return
	}

	// 2. 组装返回数据 (与面向前端的 GetUserInfo 类似，但这里不需要从 Token 拿状态)
	responseData := map[string]interface{}{
		"base_info": user,
	}

	// 3. 根据角色拉取对应的扩展资料
	if user.Role == models.RoleKOL {
		var profile models.KOLProfile
		if err := db.DB.Where("user_id = ?", userID).First(&profile).Error; err != nil {
			hlog.CtxWarnf(c, "[Internal] 找不到 KOL[%d] 的扩展资料", userID)
		} else {
			responseData["profile"] = profile
		}
	} else if user.Role == models.RoleBrand {
		var profile models.BrandProfile
		if err := db.DB.Where("user_id = ?", userID).First(&profile).Error; err != nil {
			hlog.CtxWarnf(c, "[Internal] 找不到 品牌方[%d] 的扩展资料", userID)
		} else {
			responseData["profile"] = profile
		}
	}

	response.Success(ctx, responseData)
}
