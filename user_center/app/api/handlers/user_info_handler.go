package handlers

import (
	"context"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"golang.org/x/sync/errgroup"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"

	"github.com/cloudwego/hertz/pkg/app"
)

// 1. 查询账号信息接口

// GetUserInfo 获取当前登录用户的详细信息
// @Summary 获取账号详细信息
// @Description 根据当前 Token 获取用户的基本信息及对应角色的扩展资料（KOL或品牌方）
// @Tags User
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]interface{} "成功返回用户信息"
// @Router /api/v1/user/info [get]
func GetUserInfo(c context.Context, ctx *app.RequestContext) {
	userIDAny, _ := ctx.Get("user_id")
	roleAny, _ := ctx.Get("role")
	userID := userIDAny.(uint64)
	role := roleAny.(models.RoleType)

	// 提前声明用于接收数据的变量
	var user models.SysUser
	var profile interface{} // 使用空接口来动态接收 KOL 或 Brand 的结构体
	var ugcAccounts []models.UserUGCAccount

	// 创建一个 errgroup.Group 实例
	var eg errgroup.Group

	// 协程 1：查询主表基础信息
	eg.Go(func() error {
		return db.DB.Select("id", "username", "role", "status", "created_at").First(&user, userID).Error
	})

	// 协程 2：查询角色专属扩展资料表
	eg.Go(func() error {
		if role == models.RoleKOL {
			var kol models.KOLProfile
			err := db.DB.Where("user_id = ?", userID).First(&kol).Error
			profile = kol // 赋值给外部变量
			return err
		} else if role == models.RoleBrand {
			var brand models.BrandProfile
			err := db.DB.Where("user_id = ?", userID).First(&brand).Error
			profile = brand
			return err
		}
		return nil
	})

	// 协程 3：查询绑定的第三方 UGC 账号列表
	eg.Go(func() error {
		// 注意：Find 找不到数据时不会报错（只会返回空切片），这符合业务逻辑
		return db.DB.Where("user_id = ?", userID).Find(&ugcAccounts).Error
	})

	// 屏障等待：等待所有协程全部执行完毕
	if err := eg.Wait(); err != nil {
		// 只要有任何一个协程返回了 error（比如 user 表查不到该用户），就会立刻被这里捕获
		hlog.CtxErrorf(c, "并发聚合用户信息失败: %v", err)
		response.ErrorWithMsg(ctx, response.ErrSystemError, "获取用户全量数据失败")
		return
	}

	// 组装聚合后的超级返回体
	responseData := map[string]interface{}{
		"base_info":    user,
		"profile":      profile,
		"ugc_accounts": ugcAccounts, // 把之前写的绑定账号也返回给前端！
	}

	response.Success(ctx, responseData)
}

// 2. 修改 KOL (红人) 扩展信息接口

type UpdateKOLProfileReq struct {
	AvatarURL string `json:"avatar_url"`
	RealName  string `json:"real_name"`
	Tags      string `json:"tags"`       // JSON字符串，如 "[\"游戏\",\"主播\",\"知识\"]"
	BaseQuote int    `json:"base_quote"` // 基础报价
}

// UpdateKOLProfile 修改 KOL 专属扩展资料
// @Summary 修改 KOL 扩展资料
// @Description 仅限 KOL(role=1) 调用。修改昵称、标签、底价等参数。
// @Tags User
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body UpdateKOLProfileReq true "KOL 资料参数"
// @Success 200 {object} map[string]interface{} "成功提示"
// @Router /api/v1/user/kol/profile [put]
func UpdateKOLProfile(c context.Context, ctx *app.RequestContext) {
	roleAny, _ := ctx.Get("role")
	role := roleAny.(models.RoleType)

	// 极客防御：越权拦截
	if role != models.RoleKOL {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：该接口仅限红人(KOL)调用")
		return
	}

	var req UpdateKOLProfileReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	updateData := map[string]interface{}{
		"avatar_url": req.AvatarURL,
		"real_name":  req.RealName,
		"tags":       req.Tags,
		"base_quote": req.BaseQuote,
	}

	if err := db.DB.Model(&models.KOLProfile{}).Where("user_id = ?", userID).Updates(updateData).Error; err != nil {
		hlog.CtxErrorf(c, "更新 KOL 资料失败: %v", err)
		response.Error(ctx, response.ErrDatabaseError)
		return
	}

	response.Success(ctx, map[string]interface{}{"message": "KOL 资料更新成功"})
}

// 3. 修改 Brand (品牌方) 扩展信息接口

type UpdateBrandProfileReq struct {
	AvatarURL   string `json:"avatar_url"`
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
}

// UpdateBrandProfile 修改品牌方专属扩展资料
// @Summary 修改品牌方扩展资料
// @Description 仅限品牌方(role=2) 调用。修改公司名、所属行业等参数。
// @Tags User
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body UpdateBrandProfileReq true "品牌方资料参数"
// @Success 200 {object} map[string]interface{} "成功提示"
// @Router /api/v1/user/brand/profile [put]
func UpdateBrandProfile(c context.Context, ctx *app.RequestContext) {
	roleAny, _ := ctx.Get("role")
	role := roleAny.(models.RoleType)

	// 极客防御：越权拦截
	if role != models.RoleBrand {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：该接口仅限品牌方调用")
		return
	}

	var req UpdateBrandProfileReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	updateData := map[string]interface{}{
		"avatar_url":   req.AvatarURL,
		"company_name": req.CompanyName,
		"industry":     req.Industry,
	}

	if err := db.DB.Model(&models.BrandProfile{}).Where("user_id = ?", userID).Updates(updateData).Error; err != nil {
		hlog.CtxErrorf(c, "更新品牌方资料失败: %v", err)
		response.Error(ctx, response.ErrDatabaseError)
		return
	}

	response.Success(ctx, map[string]interface{}{"message": "品牌方资料更新成功"})
}
