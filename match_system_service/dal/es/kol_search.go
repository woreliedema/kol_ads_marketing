package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"strconv"
	"strings"

	"kol_ads_marketing/match_system_service/biz/model"
)

// esSearchResponse 定义辅助解析 ES 返回原始 JSON 的结构体
// 利用 Go 的匿名嵌套结构体快速映射我们需要的嵌套层级
type esSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"` // 命中的总条数
		} `json:"total"`
		Hits []struct {
			Source *model.KolSyncData `json:"_source"` // ES 里的原始数据存放在 _source 节点
		} `json:"hits"`
	} `json:"hits"`
}

// parseESResponse 解析 Elasticsearch 的 HTTP 响应体
func parseESResponse(res *esapi.Response) (*model.KolFilterResp, error) {
	// 1. 检查 ES 引擎本身是否抛出错误（如语法错误、索引不存在）
	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch 返回错误状态: %s", res.String())
	}

	// 🚀 [核心调试代码] 直接截获 ES 响应的原始字节流
	//bodyBytes, err := io.ReadAll(res.Body)
	//if err != nil {
	//	return nil, fmt.Errorf("读取 ES 报文失败: %w", err)
	//}
	//log.Println("🔥 [DEBUG] ES原始返回报文:", string(bodyBytes))

	// 2. 声明解析结构体并解码 JSON 流
	var esResp esSearchResponse

	//if err := json.Unmarshal(bodyBytes, &esResp); err != nil {
	//	return nil, fmt.Errorf("解析 ES 返回报文失败: %w", err)
	//}
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("解析 ES 返回报文失败: %w", err)
	}

	// 3. 提取 _source 中的业务数据
	kolList := make([]*model.KolSyncData, 0, len(esResp.Hits.Hits)) // 预分配容量，提升性能
	for _, hit := range esResp.Hits.Hits {
		kolList = append(kolList, hit.Source)
	}

	// 4. 组装最终结果返回
	return &model.KolFilterResp{
		Total: esResp.Hits.Total.Value,
		List:  kolList,
	}, nil
}

// SearchKOLs 执行 Elasticsearch DSL 查询
func SearchKOLs(ctx context.Context, req *model.KolFilterReq) (*model.KolFilterResp, error) {
	// 构建 Bool Query DSL
	mustQueries := []map[string]interface{}{}

	// 1. 价格过滤 Range Query
	if req.PriceMax > 0 {
		mustQueries = append(mustQueries, map[string]interface{}{
			"range": map[string]interface{}{
				"base_quote": map[string]interface{}{
					"gte": req.PriceMin,
					"lte": req.PriceMax,
				},
			},
		})
	}

	// 2. 粉丝量级过滤 Range Query
	if req.FanLevel != "" {
		fansRange := strings.Split(req.FanLevel, ",")
		if len(fansRange) == 2 {
			minFans, _ := strconv.Atoi(fansRange[0])
			maxFans, _ := strconv.Atoi(fansRange[1])
			mustQueries = append(mustQueries, map[string]interface{}{
				"range": map[string]interface{}{
					"total_followers": map[string]interface{}{
						"gte": minFans,
						"lte": maxFans,
					},
				},
			})
		}
	}

	// 3. 标签精确匹配 Term/Match Query
	if req.FieldTag != "" {
		mustQueries = append(mustQueries, map[string]interface{}{
			"match": map[string]interface{}{
				"tags": req.FieldTag,
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

	//log.Println("🔥 [DEBUG] 发送给ES的请求DSL:", buf.String())

	// 执行 ES 查询
	res, err := ESClient.Search(
		ESClient.Search.WithContext(ctx),
		ESClient.Search.WithIndex(KolIndexName),
		ESClient.Search.WithBody(&buf),
		ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	// 解析结果 (此处省略详细的 JSON Decode 解析步骤，通常解析 hits.hits 并反序列化为 model.KolSyncData)
	// 返回组装好的 KolFilterResp
	return parseESResponse(res)
}
