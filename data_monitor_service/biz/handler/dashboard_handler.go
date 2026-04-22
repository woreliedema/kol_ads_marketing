package handler

import (
	"context"
	"kol_ads_marketing/data_monitor_service/biz/service"
	"kol_ads_marketing/data_monitor_service/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// getAuthContext 极客封装：统一提取并校验鉴权上下文
// 返回值：userID, role, 是否校验通过
func getAuthContext(ctx *app.RequestContext) (uint64, int, bool) {
	userIDVal, exists1 := ctx.Get("user_id")
	roleVal, exists2 := ctx.Get("role")

	if !exists1 || !exists2 {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "缺少用户鉴权上下文")
		return 0, 0, false
	}

	return userIDVal.(uint64), roleVal.(int), true
}

// ---------------- Handler 实现 ----------------

// GetDashboardOverview 处理前端获取大盘总览的请求
// @Summary 获取 KOL 数据大盘核心概览指标
// @Description 结合 Redis 缓存，毫秒级获取指定 KOL 近30天的均播、中位数以及互动率等基础大盘数据。
// @Tags Dashboard 大盘数据
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=model.KOLOverviewDTO} "成功返回大盘数据"
// @Security ApiKeyAuth
// @Router /dashboard/overview [get]
func GetDashboardOverview(c context.Context, ctx *app.RequestContext) {
	// 1. 提取鉴权上下文
	UserID, UserRole, ok := getAuthContext(ctx)
	if !ok {
		return
	}

	// 2. 将所有业务重担交给 Service 层处理
	data, err := service.GetDashboardOverviewService(c, UserRole, UserID)
	if err != nil {
		hlog.CtxErrorf(c, "业务逻辑层执行失败 [UID:%d, Role:%d]: %v", UserID, UserRole, err)
		// 将 Service 层抛出的精炼错误提示直接返回给前端
		response.ErrorWithMsg(ctx, response.ErrSystemError, err.Error())
		return
	}

	// 3. 成功返回统一结构体
	response.Success(ctx, data)
}

// GetDashboardTrend
// @Summary 获取 KOL 数据大盘的时间窗口趋势图(折线图)
// @Description 返回前端 ECharts 能够直接渲染的 Category 和 Series 格式数据。
// @Tags Dashboard 大盘数据
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=model.TrendChartDTO} "成功返回趋势数据"
// @Security ApiKeyAuth
// @Router /dashboard/trend [get]
func GetDashboardTrend(c context.Context, ctx *app.RequestContext) {
	// 1. 提取鉴权上下文
	UserID, UserRole, ok := getAuthContext(ctx)
	if !ok {
		return
	}

	// 2. 调度 Service 层获取折线图专属结构
	data, err := service.GetDashboardTrendService(c, UserRole, UserID)
	if err != nil {
		hlog.CtxErrorf(c, "趋势计算执行失败 [UID:%d]: %v", UserID, err)
		response.ErrorWithMsg(ctx, response.ErrSystemError, err.Error())
		return
	}

	// 3. 响应给前端
	response.Success(ctx, data)
}

// GetDashboardAdsAnalysis
// @Summary 获取 KOL 近30天商单 AI 分析结果
// @Description 提取包含品牌偏好饼图、卖点词云及视频明细的大盘数据。
// @Tags Dashboard 大盘数据
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=model.AdsAnalysisDTO} "成功返回商单分析数据"
// @Security ApiKeyAuth
// @Router /dashboard/ads_analysis [get]
func GetDashboardAdsAnalysis(c context.Context, ctx *app.RequestContext) {
	// 1. 提取入站鉴权上下文
	UserID, UserRole, ok := getAuthContext(ctx)
	if !ok {
		return
	}
	// 2. 调用 Service 层进行 ClickHouse OLAP 计算
	data, err := service.GetDashboardAdsAnalysisService(c, UserRole, UserID)
	if err != nil {
		hlog.CtxErrorf(c, "商单分析计算失败 [UID:%d]: %v", UserID, err)
		response.ErrorWithMsg(ctx, response.ErrSystemError, err.Error())
		return
	}
	// 3. 返回数据
	response.Success(ctx, data)
}
