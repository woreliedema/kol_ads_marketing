package es

// ESKolDocument 推送到 ES `kol_profiles` 索引的文档结构
type ESKolDocument struct {
	KOLUserID      uint64 `json:"kol_user_id"`
	Username       string `json:"username"`
	Status         int8   `json:"status"`
	KOLAvatarURL   string `json:"kol_avatar_url"`
	TotalFollowers uint32 `json:"total_followers"`

	// MySQL 中的 JSON 数组在这里映射为原生 Slice，方便 ES 进行倒排索引
	Tags         []string `json:"tags"`
	UGCPlatforms []string `json:"ugc_platforms"`

	// 对于复杂的账号详情（前端花火界面展示需要），使用 interface{} 让 json.Marshal 自动处理
	UGCAccountsDetail interface{} `json:"ugc_accounts_detail,omitempty"`

	// 预留的 AI 受众重合度向量字段 (MVP 阶段由于 omitempty 会被自动忽略)
	AudienceEmbedding []float64 `json:"audience_embedding,omitempty"`
}

// ESBrandDocument 推送到 ES `brand_profiles` 索引的文档结构
type ESBrandDocument struct {
	BrandUserID    uint64 `json:"brand_user_id"`
	Status         int8   `json:"status"`
	Username       string `json:"username"`
	CompanyName    string `json:"company_name"`
	BrandAvatarURL string `json:"brand_avatar_url"`
	Industry       string `json:"industry"`
	IsVerified     int8   `json:"is_verified"`
}
