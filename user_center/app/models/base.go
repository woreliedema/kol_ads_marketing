package models

import (
	"gorm.io/gorm"
	"time"
)

// RoleType 定义系统角色枚举
type RoleType int

const (
	RoleKOL   RoleType = 1
	RoleBrand RoleType = 2
	RoleAdmin RoleType = 99
)

// 利用 init 函数和空白标识符，在包加载时执行一次“无意义赋值”，消耗掉未使用警告
func init() {
	_ = RoleAdmin
}

// SysUser 核心用户表
type SysUser struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string         `gorm:"type:varchar(64);uniqueIndex;not null;comment:登录名(手机/邮箱)" json:"username"`
	PasswordHash string         `gorm:"type:varchar(255);not null;comment:登录密码哈基值" json:"-"` // json:"-" 严防密码泄漏
	Role         RoleType       `gorm:"type:tinyint;not null;comment:1-红人 2-品牌方 99-管理员" json:"role"`
	Status       int8           `gorm:"type:tinyint;default:1;comment:1-正常 0-封禁 -1-未激活" json:"status"`
	LastLoginIP  string         `gorm:"type:varchar(64)" json:"last_login_ip"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	IsDelete     int8           `gorm:"type:tinyint;default:0;comment:0-未删除 1-删除" json:"is_delete"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"` // 软删除时间戳，满足合规与审计要求

	// 关联定义
	KOLProfile   *KOLProfile   `gorm:"foreignKey:UserID" json:"kol_profile,omitempty"`
	BrandProfile *BrandProfile `gorm:"foreignKey:UserID" json:"brand_profile,omitempty"`
}

// KOLProfile 红人业务扩展表
type KOLProfile struct {
	ID          uint64  `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint64  `gorm:"uniqueIndex;not null" json:"user_id"`
	RealName    string  `gorm:"type:varchar(64)" json:"real_name"`
	AvatarURL   string  `gorm:"type:varchar(255)" json:"avatar_url"`
	Tags        string  `gorm:"type:json;comment:领域标签(如[数码,美妆])，MySQL8.0原生支持JSON" json:"tags"`
	BaseQuote   float64 `gorm:"type:decimal(10,2);comment:红人自行设置的基础底价" json:"base_quote"`
	CreditScore int     `gorm:"type:int;default:100;comment:平台信用分(影响排序推荐)" json:"credit_score"`
}

// BrandProfile 品牌方业务扩展表
type BrandProfile struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint64 `gorm:"uniqueIndex;not null" json:"user_id"`
	CompanyName string `gorm:"type:varchar(128);not null;comment:企业主体名称" json:"company_name"`
	Industry    string `gorm:"type:varchar(64);comment:所属行业" json:"industry"`
	LicenseURL  string `gorm:"type:varchar(255);comment:营业执照OSS地址" json:"license_url"`
	IsVerified  bool   `gorm:"type:tinyint(1);default:0;comment:是否通过企业资质认证" json:"is_verified"`
}

// UserUGCAccount 跨平台社交账号绑定表 (与数据采集模块直接联动)
type UserUGCAccount struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint64    `gorm:"index;not null" json:"user_id"`
	Platform    string    `gorm:"type:varchar(32);not null;comment:平台名(douyin, bilibili, tiktok)" json:"platform"`
	PlatformUID string    `gorm:"type:varchar(128);not null;comment:第三方平台的唯一UID" json:"platform_uid"`
	Nickname    string    `gorm:"type:varchar(64)" json:"nickname"`
	AuthToken   string    `gorm:"type:text;comment:用于合规爬虫或API的授权Token(需加密存储)" json:"-"`
	FansCount   int64     `gorm:"type:int;default:0;comment:冗余字段，方便快速查询" json:"fans_count"`
	BoundAt     time.Time `gorm:"autoCreateTime" json:"bound_at"`

	// 联合唯一索引：一个用户在一个平台只能绑定一个核心账号 (或者一个平台UID只能被绑定一次)
	_ struct{} `gorm:"uniqueIndex:idx_user_platform,column:user_id,platform"`
	_ struct{} `gorm:"uniqueIndex:idx_platform_uid,column:platform,platform_uid"`
}
