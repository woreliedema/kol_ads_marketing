package brand

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/pkg/response"
	"kol_ads_marketing/match_system_service/service/matcher"
)

// FilterBrands 红人筛选品牌方接口
// @Summary 红人按条件筛选品牌方
// @Description 根据所属行业、认证状态等维度筛选潜在合作的品牌方 (对标B站花火“找品牌”功能)
// @Tags KOL匹配系统
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param field_tag query string false "所属行业标签，如'数码'"
// @Param is_verified query int false "是否已认证: 1-已认证, 0-未认证"
// @Param page query int false "页码，默认1"
// @Param size query int false "每页数量，默认20"
// @Success 200 {object} response.Response{data=model.BrandFilterResp} "成功返回品牌方列表"
// @Failure 400 {object} response.Response "参数解析失败"
// @Failure 401 {object} response.Response "Token无效或已过期"
// @Failure 403 {object} response.Response "权限不足，仅限红人调用"
// @Failure 500 {object} response.Response "系统检索异常"
// @Router /api/v1/match/brand/filter [get]
func FilterBrands(ctx context.Context, c *app.RequestContext) {
	var req model.BrandFilterReq

	// 1. 参数绑定与校验
	if err := c.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(c, response.ErrInvalidParams, "参数解析失败: "+err.Error())
		return
	}

	// 2. 调起匹配引擎
	resp, err := matcher.SearchBrandsService(ctx, &req)
	if err != nil {
		// 这里沿用你 errors.go 里定义的 ES 服务异常常量
		response.ErrorWithMsg(c, response.ErrESQueryFailed, "品牌检索引擎异常: "+err.Error())
		return
	}

	// 3. 成功返回
	response.Success(c, resp)
}
