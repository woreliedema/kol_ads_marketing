package matcher

import (
	"context"
	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/dal/es"
)

// SearchKOLsService 处理 KOL 搜索的核心业务逻辑
func SearchKOLsService(ctx context.Context, req *model.KolFilterReq) (*model.KolFilterResp, error) {
	// 1. 设置分页默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	// 2. 调用 ES 进行多路召回 [cite: 181]
	resp, err := es.SearchKOLs(ctx, req)
	if err != nil {
		return nil, err
	}

	// 3. 预留：精排阶段逻辑 (如果需要计算重合度打分，在这里处理) [cite: 190]
	// for _, kol := range resp.List {
	//     overlapScore := calculateCosineSimilarity(...)
	//     kol.CreditScore = ...
	// }

	return resp, nil
}
