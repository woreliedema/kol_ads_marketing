package kol

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/pkg/response"
	"kol_ads_marketing/match_system_service/service/matcher"
)

// FilterKOLs 品牌方筛选红人接口
// @Summary 品牌方按标签筛选红人
// @Description 根据领域标签、粉丝量级、价格区间等多维度筛选红人列表 (对标B站花火“找红人”功能)
// @Tags KOL匹配系统
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param field_tag query string false "领域标签，如'美妆'"
// @Param fan_level query string false "粉丝量级，格式'min,max'，如'10000,100000'"
// @Param price_min query int false "最低报价限制"
// @Param price_max query int false "最高报价限制"
// @Param page query int false "页码，默认1"
// @Param size query int false "每页数量，默认20"
// @Success 200 {object} response.Response{data=model.KolFilterResp} "成功返回红人列表及总数"
// @Failure 400 {object} response.Response "参数解析失败"
// @Failure 401 {object} response.Response "Token无效或已过期"
// @Failure 403 {object} response.Response "权限不足，仅限品牌方调用"
// @Failure 500 {object} response.Response "系统检索异常"
// @Router /api/v1/match/kol/filter [get]
func FilterKOLs(ctx context.Context, c *app.RequestContext) {
	var req model.KolFilterReq

	// 1. 绑定并校验前端查询参数
	if err := c.BindAndValidate(&req); err != nil {
		// 使用自定义的响应包和错误字典，动态拼接校验错误细节
		response.ErrorWithMsg(c, response.ErrInvalidParams, "参数解析失败: "+err.Error())
		return
	}

	// 2. 调用 Service 层进行业务处理
	resp, err := matcher.SearchKOLsService(ctx, &req)
	if err != nil {
		// 命中搜索引擎服务异常
		response.ErrorWithMsg(c, response.ErrESQueryFailed, "系统检索异常: "+err.Error())
		return
	}

	// 3. 返回成功结果，自动包装 code: 0 和 msg: success
	response.Success(c, resp)
}
