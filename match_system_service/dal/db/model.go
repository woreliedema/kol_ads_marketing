package db

import (
	"time"

	"gorm.io/datatypes"
)

// MatchKolWideIndex 红人匹配检索宽表
type MatchKolWideIndex struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	KOLUserID uint64 `gorm:"column:kol_user_id;uniqueIndex;not null" json:"kol_user_id"`

	// 业务字段
	Username          string         `gorm:"column:username;size:64;not null" json:"username"`
	Status            int8           `gorm:"column:status;not null" json:"status"` // 1-正常 0-封禁
	KOLAvatarURL      string         `gorm:"column:kol_avatar_url;size:255" json:"kol_avatar_url"`
	Tags              datatypes.JSON `gorm:"column:tags" json:"tags"` // 例: ["数码", "美妆"]
	BaseQuote         float64        `gorm:"column:base_quote;default:0;comment:'基础报价'"`
	UGCPlatforms      datatypes.JSON `gorm:"column:ugc_platforms" json:"ugc_platforms"` // 例: ["bilibili", "douyin"]
	TotalFollowers    uint32         `gorm:"column:total_followers;default:0" json:"total_followers"`
	UGCAccountsDetail datatypes.JSON `gorm:"column:ugc_accounts_detail" json:"ugc_accounts_detail"` // 完整UGC账号详情

	// 同步控制字段
	SourceUpdatedAt time.Time `gorm:"column:source_updated_at;not null;index:idx_source_time" json:"source_updated_at"`
	SyncStatus      int8      `gorm:"column:sync_status;not null;default:0;index:idx_sync_status" json:"sync_status"`
	SyncRetryCount  int8      `gorm:"column:sync_retry_count;not null;default:0" json:"sync_retry_count"`
	ErrorMsg        string    `gorm:"column:error_msg;size:255" json:"error_msg"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime;not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime;not null" json:"updated_at"`
}

func (MatchKolWideIndex) TableName() string {
	return "match_kol_wide_index"
}

// MatchBrandWideIndex 品牌方匹配检索宽表
type MatchBrandWideIndex struct {
	ID          uint64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	BrandUserID uint64 `gorm:"column:brand_user_id;uniqueIndex;not null" json:"brand_user_id"`

	// 业务字段
	Status         int8   `gorm:"column:status;not null" json:"status"` // 1-正常 0-封禁
	Username       string `gorm:"column:username;size:64;not null" json:"username"`
	CompanyName    string `gorm:"column:company_name;size:128;not null" json:"company_name"`
	BrandAvatarURL string `gorm:"column:brand_avatar_url;size:255" json:"brand_avatar_url"`
	//Industry       string `gorm:"column:industry;size:64" json:"industry"`
	Tags       string `gorm:"column:tags;type:json;comment:领域标签" json:"tags"`
	IsVerified int8   `gorm:"column:is_verified;default:0" json:"is_verified"` // 企业资质是否已认证

	// 同步控制字段
	SourceUpdatedAt time.Time `gorm:"column:source_updated_at;not null" json:"source_updated_at"`
	SyncStatus      int8      `gorm:"column:sync_status;not null;default:0;index:idx_sync_status" json:"sync_status"`
	SyncRetryCount  int8      `gorm:"column:sync_retry_count;not null;default:0" json:"sync_retry_count"`
	ErrorMsg        string    `gorm:"column:error_msg;size:255" json:"error_msg"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime;not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime;not null" json:"updated_at"`
}

func (MatchBrandWideIndex) TableName() string {
	return "match_brand_wide_index"
}
