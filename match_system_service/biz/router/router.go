package router

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

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

	// 2. 面向内部微服务的 RPC/HTTP 路由组...
}
