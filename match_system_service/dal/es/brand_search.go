package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"kol_ads_marketing/match_system_service/biz/model"
)

// SearchBrands 执行 Elasticsearch DSL 查询获取品牌方列表
func SearchBrands(ctx context.Context, req *model.BrandFilterReq) (*model.BrandFilterResp, error) {
	mustQueries := []map[string]interface{}{}

	// 1. 行业精准或模糊匹配 (Match Query)
	if req.FieldTag != "" {
		mustQueries = append(mustQueries, map[string]interface{}{
			"match": map[string]interface{}{
				"industry": req.FieldTag,
			},
		})
	}

	// 2. 资质认证过滤 (Term Query) - 注意此处利用指针判断是否传入了筛选条件
	if req.IsVerified != nil {
		mustQueries = append(mustQueries, map[string]interface{}{
			"term": map[string]interface{}{
				"is_verified": *req.IsVerified,
			},
		})
	}

	// 构建完整 DSL
	var query map[string]interface{}

	if len(mustQueries) == 0 {
		// 如果前端没有任何筛选条件，执行全量匹配 (等同于你在浏览器里直接回车)
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"match_all": map[string]interface{}{},
			},
			"from": (req.Page - 1) * req.Size,
			"size": req.Size,
		}
	} else {
		// 如果有条件，才执行 bool 多路召回
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"must": mustQueries,
				},
			},
			"from": (req.Page - 1) * req.Size,
			"size": req.Size,
		}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("error encoding query: %w", err)
	}

	// 假设品牌方索引为 brand_search_index
	res, err := ESClient.Search(
		ESClient.Search.WithContext(ctx),
		ESClient.Search.WithIndex(BrandIndexName),
		ESClient.Search.WithBody(&buf),
		ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	return parseBrandESResponse(res)
}

// 辅助解析结构体
type esBrandSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source *model.BrandSyncData `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// parseBrandESResponse 解析品牌方 ES 响应
func parseBrandESResponse(res *esapi.Response) (*model.BrandFilterResp, error) {
	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch 返回错误状态: %s", res.String())
	}

	var esResp esBrandSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("解析 ES 返回报文失败: %w", err)
	}

	brandList := make([]*model.BrandSyncData, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		brandList = append(brandList, hit.Source)
	}

	return &model.BrandFilterResp{
		Total: esResp.Hits.Total.Value,
		List:  brandList,
	}, nil
}
