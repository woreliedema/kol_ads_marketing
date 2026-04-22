package router

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"kol_ads_marketing/data_monitor_service/biz/handler"
	"kol_ads_marketing/data_monitor_service/middleware"

	// Swagger 相关依赖
	"github.com/hertz-contrib/swagger"
	swaggerFiles "github.com/swaggo/files"

	// 匿名引入 swag init 生成的 docs 包
	_ "kol_ads_marketing/data_monitor_service/docs"
)

func RegisterRoutes(h *server.Hertz) {
	h.GET("/swagger/*any", swagger.WrapHandler(swaggerFiles.Handler))
	// 全局 API 前缀
	apiV1 := h.Group("/api/v1/monitor")

	// Dashboard 相关接口群组
	dashboard := apiV1.Group("/dashboard")
	// 挂载中间件
	dashboard.Use(middleware.AuthMiddleware())

	// 原有的基础大盘
	dashboard.GET("/overview", handler.GetDashboardOverview)
	// 时间趋势折线图 API
	dashboard.GET("/trend", handler.GetDashboardTrend)
	// 商单分析接口
	dashboard.GET("/ads_analysis", handler.GetDashboardAdsAnalysis)
}
