package db

import (
	"time"
)

// IMSession 会话表：记录品牌方与红人的聊天窗口关系及未读数
// 极客设计：前端的“消息列表”就是查这张表，通过 UpdatedAt 倒序排列
type IMSession struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID   string    `gorm:"type:varchar(64);uniqueIndex;not null;comment:会话全局唯一ID(规则: minID_maxID)" json:"session_id"`
	BrandUserID uint64    `gorm:"index;not null;comment:品牌方UserID" json:"brand_user_id"`
	KolUserID   uint64    `gorm:"index;not null;comment:红人UserID" json:"kol_user_id"`
	LatestMsg   string    `gorm:"type:varchar(512);comment:最后一条消息摘要" json:"latest_msg"` // 用于前端会话列表展示
	UnreadBrand int       `gorm:"default:0;comment:品牌方未读消息数" json:"unread_brand"`
	UnreadKol   int       `gorm:"default:0;comment:红人未读消息数" json:"unread_kol"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;index;comment:最后活跃时间" json:"updated_at"` // 强依赖此字段进行会话列表排序
}

// IMMessage 消息流水表：记录每一条具体的聊天或信令消息
type IMMessage struct {
	ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// 极客提示：前端 JS 处理 64 位整型会丢失精度，因此 JSON 序列化时自动转为 string
	MsgID      int64     `gorm:"type:bigint;uniqueIndex;not null;comment:雪花算法全局唯一消息ID" json:"msg_id,string"`
	SessionID  string    `gorm:"type:varchar(64);index;not null;comment:所属会话ID" json:"session_id"`
	SenderID   uint64    `gorm:"index;not null;comment:发送方UserID" json:"sender_id"`
	ReceiverID uint64    `gorm:"index;not null;comment:接收方UserID" json:"receiver_id"`
	MsgType    int8      `gorm:"type:tinyint;default:1;comment:消息类型(1:纯文本, 2:图片, 3:合作意向卡片, 4:系统通知)" json:"msg_type"`
	Content    string    `gorm:"type:text;not null;comment:消息内容(支持存入JSON字符串格式的复杂卡片)" json:"content"`
	Status     int8      `gorm:"type:tinyint;default:0;comment:消息状态(0:已发送/未读, 1:已读, 2:撤回, 3:发送失败)" json:"status"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index;comment:消息发送时间" json:"created_at"` // 强依赖此字段拉取历史消息
}
