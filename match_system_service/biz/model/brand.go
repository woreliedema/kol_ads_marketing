package model

// BrandFilterReq 红人搜索品牌方的请求结构体
type BrandFilterReq struct {
	FieldTag   string `query:"field_tag" json:"field_tag"`     // 行业标签
	IsVerified *int8  `query:"is_verified" json:"is_verified"` // 是否认证 (传指针以区分 0-未认证, 1-已认证 和 未传值)
	Page       int    `query:"page" json:"page"`               // 当前页码
	Size       int    `query:"size" json:"size"`               // 每页数量
}

// BrandFilterResp 返回给前端的品牌方列表和分页信息
type BrandFilterResp struct {
	Total int64            `json:"total"`
	List  []*BrandSyncData `json:"list"`
}

// BrandSyncData 品牌方核心数据模型 (与 ES 存储及 match_brand_wide_index 映射对齐)
type BrandSyncData struct {
	BrandUserID int64  `json:"brand_user_id"` // 对应 sys_users.id
	Username    string `json:"username"`
	CompanyName string `json:"company_name"`
	AvatarURL   string `json:"avatar_url"` // 对应 brand_profiles.avatar_url
	Tags        string `json:"tags"`
	IsVerified  int8   `json:"is_verified"` // 1-是 0-否
}
