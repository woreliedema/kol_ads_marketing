package matcher

import (
	"context"
	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/dal/es"
)

// SearchBrandsService 处理红人搜索品牌方的核心业务逻辑
func SearchBrandsService(ctx context.Context, req *model.BrandFilterReq) (*model.BrandFilterResp, error) {
	// 1. 设置分页默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	// 2. 调用 ES 进行多路召回
	resp, err := es.SearchBrands(ctx, req)
	if err != nil {
		return nil, err
	}

	// 3. 预留精排层：可以根据品牌方历史投放转化率 (ROI) 进行二次排序重塑
	// ...

	return resp, nil
}
