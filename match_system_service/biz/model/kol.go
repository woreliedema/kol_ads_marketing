package model

// KolFilterReq 品牌方搜索红人的请求结构体
type KolFilterReq struct {
	FieldTag string `query:"field_tag" json:"field_tag"` // 领域标签，如"美妆"
	FanLevel string `query:"fan_level" json:"fan_level"` // 粉丝量级，格式建议为 "min,max"，如 "10000,100000"
	PriceMin int64  `query:"price_min" json:"price_min"` // 最低报价
	PriceMax int64  `query:"price_max" json:"price_max"` // 最高报价
	Page     int    `query:"page" json:"page"`           // 当前页码，默认 1
	Size     int    `query:"size" json:"size"`           // 每页数量，默认 20
}

// KolFilterResp 返回给前端的红人列表和分页信息
type KolFilterResp struct {
	Total int64          `json:"total"`
	List  []*KolSyncData `json:"list"`
}

// UgcAccountDetail UGC 平台账号详情子结构体
type UgcAccountDetail struct {
	FollowersCount int    `json:"followers_count"`
	Platform       string `json:"platform"`
	PlatformUID    string `json:"platform_uid"`
}

// KolSyncData 红人核心数据模型 (与 ES 存储映射保持一致)
type KolSyncData struct {
	KolUserID      int64    `json:"kol_user_id"` // 对齐 kol_user_id
	Username       string   `json:"username"`    // 对齐 username
	Status         int8     `json:"status"`
	KolAvatarURL   string   `json:"kol_avatar_url"`
	BaseQuote      float64  `json:"base_quote"`
	TotalFollowers int      `json:"total_followers"`
	Tags           []string `json:"tags"`
	UgcPlatforms   []string `json:"ugc_platforms"`

	// 嵌套的账号详情数组
	UgcAccountsDetail []*UgcAccountDetail `json:"ugc_accounts_detail"`
}
