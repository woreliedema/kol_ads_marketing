package router

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"kol_ads_marketing/match_system_service/biz/handlers/brand"
	"kol_ads_marketing/match_system_service/biz/handlers/im"
	"kol_ads_marketing/match_system_service/biz/handlers/kol"
	"kol_ads_marketing/match_system_service/middleware"
	"kol_ads_marketing/match_system_service/pkg/constants"

	// Swagger 相关依赖
	"github.com/hertz-contrib/swagger"
	swaggerFiles "github.com/swaggo/files"

	// 匿名引入 swag init 生成的 docs 包
	_ "kol_ads_marketing/match_system_service/docs"
)

// RegisterRoutes 统一挂载和管理微服务的所有 API 路由
func RegisterRoutes(h *server.Hertz) {
	// 0. 基础设施与文档路由
	// 挂载 Swagger UI
	h.GET("/swagger/*any", swagger.WrapHandler(swaggerFiles.Handler))

	// 微服务健康检查接口
	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(200, map[string]interface{}{"message": "pong", "service": "match_system_service"})
	})

	// 前端连接地址: ws://<host>:<port>/ws/im_service
	h.GET("/ws/im_service", im.Connect)

	// 1. 定义全局 /api/v1 路由组
	v1 := h.Group("/api/v1")

	matchGroup := v1.Group("/match")
	{
		// 3. 在 /kol 组上挂载鉴权中间件，传入允许的角色：2(品牌方，前端测试), 99(管理员，用于本地测试)
		kolGroup := matchGroup.Group("/kol", middleware.AuthMiddleware(constants.RoleBrand, constants.RoleAdmin))
		{
			// 绑定 GET /api/v1/match/kol/filter 路由到 handler
			kolGroup.GET("/filter", kol.FilterKOLs)
		}

		brandGroup := matchGroup.Group("/brand", middleware.AuthMiddleware(constants.RoleKOL, constants.RoleAdmin))
		{
			// 绑定 GET /api/v1/match/brand/filter
			brandGroup.GET("/filter", brand.FilterBrands)
		}

		// 新增：IM 通讯模块路由组
		// 挂载鉴权中间件 (不传参数代表只需要 Token 有效，不限制角色)
		imGroup := matchGroup.Group("/im", middleware.AuthMiddleware())
		{
			// 获取左侧会话列表
			imGroup.GET("/sessions", im.GetSessionList)
			// 获取与特定用户的历史聊天记录
			imGroup.GET("/history", im.GetHistoryMessages)
			// 挂载清空未读数路由
			imGroup.POST("read", im.ClearSessionUnread)
		}

		// 如果有不需要鉴权的公开接口，可以直接挂载在 matchGroup 下
	}
	// 2. 面向内部微服务的 RPC/HTTP 路由组...
}
