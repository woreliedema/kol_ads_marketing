package im_service

import (
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kol_ads_marketing/match_system_service/dal/db"
)

// IMPersistenceService 提供IM消息持久化能力
type IMPersistenceService struct {
	DB *gorm.DB
}

func NewIMPersistenceService(db *gorm.DB) *IMPersistenceService {
	return &IMPersistenceService{DB: db}
}

type IMMessagePayload struct {
	db.IMMessage        // 嵌套 GORM 的 Message 模型
	BrandID      uint64 `json:"brand_id"` // 由API层/WS层投递时注入
	KolID        uint64 `json:"kol_id"`   // 由API层/WS层投递时注入
}

// ProcessKafkaMessage 处理从Kafka拉取到的单条消息并落库
func (s *IMPersistenceService) ProcessKafkaMessage(ctx context.Context, kafkaVal []byte) error {
	var payload IMMessagePayload
	if err := json.Unmarshal(kafkaVal, &payload); err != nil {
		return fmt.Errorf("反序列化Kafka消息失败: %w", err)
	}

	// 1. 在内存中先完成所有业务逻辑计算，不占用 DB 事务时间
	var brandUnreadInc, kolUnreadInc int
	// 根据 Payload 明确的上下文判断未读数该加给谁
	if payload.ReceiverID == payload.BrandID {
		brandUnreadInc = 1
	} else if payload.ReceiverID == payload.KolID {
		kolUnreadInc = 1
	}

	// 提取消息摘要
	latestMsgDigest := payload.Content
	if payload.MsgType == 2 {
		latestMsgDigest = "[图片]"
	} else if payload.MsgType == 3 {
		latestMsgDigest = "[合作意向卡片]"
	}
	if len(latestMsgDigest) > 500 {
		latestMsgDigest = latestMsgDigest[:500] + "..."
	}

	// 2. 开启事务 (遵循行锁延迟原则)
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		// 【步骤 A】：先做无锁冲突的 Append 操作 (Insert Message)
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "msg_id"}},
			DoNothing: true, // 幂等处理
		}).Create(&payload.IMMessage).Error
		if err != nil {
			return fmt.Errorf("写入消息流水失败: %w", err)
		}

		// 【步骤 B】：最后做需要获取行锁的操作 (Upsert Session)
		session := db.IMSession{
			SessionID:   payload.SessionID,
			BrandUserID: payload.BrandID, // 直接使用上下文传递的明确 ID
			KolUserID:   payload.KolID,   // 直接使用上下文传递的明确 ID
			LatestMsg:   latestMsgDigest,
			UnreadBrand: brandUnreadInc,
			UnreadKol:   kolUnreadInc,
		}

		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "session_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"latest_msg":   latestMsgDigest,
				"unread_brand": gorm.Expr("unread_brand + ?", brandUnreadInc),
				"unread_kol":   gorm.Expr("unread_kol + ?", kolUnreadInc),
				"updated_at":   gorm.Expr("CURRENT_TIMESTAMP(3)"),
			}),
		}).Create(&session).Error

		if err != nil {
			return fmt.Errorf("更新会话状态失败: %w", err)
		}

		return nil // 事务提交，释放 session_id 上的行锁
	})

	return err
}
