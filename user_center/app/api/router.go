package api

import (
	"context"

	"kol_ads_marketing/user_center/app/api/handlers"
	"kol_ads_marketing/user_center/app/api/middleware"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/hertz-contrib/swagger"
	swaggerFiles "github.com/swaggo/files"
	_ "kol_ads_marketing/user_center/docs" // 必须引入 swag 生成的 docs 包
)

// RegisterRoutes 统一挂载和管理微服务的所有 API 路由
func RegisterRoutes(h *server.Hertz) {
	// 0. 基础设施与文档路由
	// 挂载 Swagger UI
	h.GET("/swagger/*any", swagger.WrapHandler(swaggerFiles.Handler))

	// 微服务健康检查接口 (Nacos 或 K8s 探针使用)
	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(200, utils.H{"message": "pong", "service": "user_center"})
	})

	// 1. 面向前端/用户的公网路由组 (API V1)
	v1 := h.Group("/api/v1")

	// 1.1 开放路由 (无需登录鉴权)
	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/register", handlers.Register)
		authGroup.POST("/login", handlers.Login)
	}

	// 1.2 受保护路由 (必须携带合法 Token)
	protectedGroup := v1.Group("/user", middleware.AuthMiddleware())
	{
		protectedGroup.POST("/ugc/bind", handlers.BindUGCAccount)
		protectedGroup.POST("/password/reset", handlers.ResetPassword)
		protectedGroup.GET("/info", handlers.GetUserInfo)
		protectedGroup.PUT("/kol/profile", handlers.UpdateKOLProfile)
		protectedGroup.PUT("/brand/profile", handlers.UpdateBrandProfile)
	}

	// 2. 面向内部微服务的 RPC/HTTP 路由组
	// 无 AuthMiddleware，仅限内网调用
	internalGroup := h.Group("/api/internal/v1")
	{
		internalGroup.GET("/user/:id/profile", handlers.GetInternalUserProfile)
	}
}
