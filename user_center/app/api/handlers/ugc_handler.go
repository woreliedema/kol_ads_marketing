package handlers

import (
	"context"
	"errors"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/go-sql-driver/mysql"
	//"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BindUGCAccountReq 绑定请求参数
type BindUGCAccountReq struct {
	Platform    string `json:"platform" vd:"$=='bilibili'||$=='douyin'||$=='tiktok';msg:'目前仅支持 bilibili, douyin, tiktok'"`
	PlatformUID string `json:"platform_uid" vd:"required;msg:'平台 UID 不能为空'"`
	Nickname    string `json:"nickname"`

	// MVP 阶段允许传入的AuthToken，通过代码在本地读取Redis中的cookie替代
	//AuthToken   string `json:"auth_token" vd:"required;msg:'授权 Token 不能为空'"`
	AuthToken string `json:"auth_token"`
}

// BindUGCAccount 处理 KOL 绑定第三方平台账号的逻辑
// @Summary 绑定第三方 UGC 账号 (受保护)
// @Description 绑定 B站/抖音/小红书 账号。支持 MVP 阶段不传 auth_token 自动从 Redis 读取。
// @Tags UGC
// @Accept json
// @Produce json
// @Security ApiKeyAuth   <-- 🚀 核心：告诉 Swagger 这个接口右上角需要一把小锁！
// @Param request body BindUGCAccountReq true "绑定参数"
// @Success 200 {object} map[string]interface{} "成功返回绑定的平台和UID"
// @Router /api/v1/user/ugc/bind [post]
func BindUGCAccount(c context.Context, ctx *app.RequestContext) {
	// 1. 从上下文中提取由 AuthMiddleware 注入的 user_id
	userIDAny, exists := ctx.Get("user_id")
	if !exists {
		response.Error(ctx, response.ErrUnauthorized)
		return
	}
	userID := userIDAny.(uint64)

	// 2. 参数绑定与校验
	var req BindUGCAccountReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	// MVP 阶段自动挂载 Redis 中的最新 Cookie=
	if req.AuthToken == "" {
		// 拼接你在 Python 爬虫里定义的 Redis Key (例如: "cookie:bilibili")
		redisKey := "cookie:" + req.Platform
		cookieStr, err := db.RDB.Get(c, redisKey).Result()

		if err == nil && cookieStr != "" {
			req.AuthToken = cookieStr
			hlog.CtxInfof(c, "🔥 [MVP Hack] 成功从 Redis 读取到 [%s] 的自动刷新 Cookie", req.Platform)
		} else {
			hlog.CtxWarnf(c, "Redis 中未找到可用 Cookie, Key: %s, Err: %v", redisKey, err)
			response.ErrorWithMsg(ctx, response.ErrInvalidParams, "未提供授权 Token，且系统中暂无该平台的可用 Cookie")
			return
		}
	}

	// 3. 构建模型实体
	ugcAccount := models.UserUGCAccount{
		UserID:      userID,
		Platform:    req.Platform,
		PlatformUID: req.PlatformUID,
		Nickname:    req.Nickname,
		AuthToken:   req.AuthToken,
	}

	// 4. 极客写法：使用 GORM 的 Upsert (OnConflict) 处理冲突
	// 语义：如果发现 UserID 和 Platform 冲突了（该用户已经绑过该平台），则更新他的 UID、Nickname 和 Token
	err := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "platform"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"platform_uid", "nickname", "auth_token", "bound_at"}),
	}).Create(&ugcAccount).Error

	if err != nil {
		// 拦截第二层唯一索引冲突：该平台的这个 UID 已经被别人绑了！
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			response.ErrorWithMsg(ctx, response.ErrInvalidParams, "绑定失败：该第三方平台账号已被其他用户绑定！")
			return
		}

		hlog.CtxErrorf(c, "绑定 UGC 账号失败: %v", err)
		response.Error(ctx, response.ErrDatabaseError)
		return
	}

	response.Success(ctx, map[string]interface{}{
		"message":      "账号绑定成功",
		"platform":     req.Platform,
		"platform_uid": req.PlatformUID,
	})
}
