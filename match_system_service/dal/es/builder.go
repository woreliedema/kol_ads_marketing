package es

import (
	"encoding/json"

	"kol_ads_marketing/match_system_service/dal/db"
)

// BuildESKolDoc 将 MySQL 的宽表对象安全地转换为推送到 ES 的文档对象
func BuildESKolDoc(model *db.MatchKolWideIndex) (*ESKolDocument, error) {
	doc := &ESKolDocument{
		KOLUserID:      model.KOLUserID,
		Username:       model.Username,
		Status:         model.Status,
		KOLAvatarURL:   model.KOLAvatarURL,
		BaseQuote:      model.BaseQuote,
		TotalFollowers: model.TotalFollowers,
	}

	// 解析标签数组
	if len(model.Tags) > 0 {
		if err := json.Unmarshal(model.Tags, &doc.Tags); err != nil {
			return nil, err
		}
	}

	// 解析平台数组
	if len(model.UGCPlatforms) > 0 {
		if err := json.Unmarshal(model.UGCPlatforms, &doc.UGCPlatforms); err != nil {
			return nil, err
		}
	}

	// 解析复杂的账号详情对象（如果有数据的话）
	if len(model.UGCAccountsDetail) > 0 {
		var detail interface{}
		if err := json.Unmarshal(model.UGCAccountsDetail, &detail); err != nil {
			return nil, err
		}
		doc.UGCAccountsDetail = detail
	}

	return doc, nil
}

// BuildESBrandDoc 转换品牌方文档 (由于没有复杂 JSON 字段，转换非常轻量)
func BuildESBrandDoc(model *db.MatchBrandWideIndex) *ESBrandDocument {
	return &ESBrandDocument{
		BrandUserID:    model.BrandUserID,
		Status:         model.Status,
		Username:       model.Username,
		CompanyName:    model.CompanyName,
		BrandAvatarURL: model.BrandAvatarURL,
		//Industry:       model.Industry,
		Tags:       model.Tags,
		IsVerified: model.IsVerified,
	}
}
