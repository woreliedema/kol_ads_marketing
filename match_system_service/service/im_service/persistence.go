package im_service

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/dal/db"
	user_rpc "kol_ads_marketing/match_system_service/rpc/user_center"
	"strconv"
	"time"
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
	//var payload IMMessagePayload
	//if err := json.Unmarshal(kafkaVal, &payload); err != nil {
	//	return fmt.Errorf("反序列化Kafka消息失败: %w", err)
	//}
	var kafkaMsg model.KafkaChatMsg
	if err := json.Unmarshal(kafkaVal, &kafkaMsg); err != nil {
		return err // 真正的非预期脏数据，才会抛给外层触发跳过逻辑
	}
	dbMsg := db.IMMessage{
		MsgID:      kafkaMsg.MsgID,
		SessionID:  kafkaMsg.SessionID,
		SenderID:   kafkaMsg.SenderID,   // uint64 -> uint64，完美匹配！
		ReceiverID: kafkaMsg.ReceiverID, // uint64 -> uint64，完美匹配！
		MsgType:    kafkaMsg.MsgType,
		Content:    kafkaMsg.Content,
		Status:     0,
		CreatedAt:  time.UnixMilli(kafkaMsg.SendTime),
	}

	// 3. 【持久化层】：开启数据库事务，保证消息与会话的强一致性
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		// 3.1 落盘具体的聊天消息
		if err := tx.Create(&dbMsg).Error; err != nil {
			hlog.CtxErrorf(ctx, "插入聊天消息失败 MsgID:%d, Err:%v", dbMsg.MsgID, err)
			return err
		}

		// 3.2 动态计算未读红点属于哪一方
		var unreadBrandInc, unreadKolInc int
		if kafkaMsg.SenderID == kafkaMsg.BrandID {
			// 品牌方发给红人，红人的未读数 +1
			unreadKolInc = 1
		} else {
			// 红人发给品牌方，品牌方的未读数 +1
			unreadBrandInc = 1
		}

		// 3.3 构造会话 Upsert 的基础数据
		session := db.IMSession{
			SessionID:   kafkaMsg.SessionID,
			BrandUserID: kafkaMsg.BrandID,
			KolUserID:   kafkaMsg.KolID,
			LatestMsg:   kafkaMsg.Content, // 冗余最新消息内容，提升会话列表渲染性能
			UpdatedAt:   dbMsg.CreatedAt,  // 以最后一条消息的发送时间为准
		}

		// 3.4 执行极客 UPSERT 操作 (MySQL: Insert ... On Duplicate Key Update)
		// 含义：如果 SessionID 不存在就插入新行；如果已存在，则只更新最新消息、时间，并在原有未读数上 +1
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "session_id"}}, // 冲突检测的唯一索引列
			DoUpdates: clause.Assignments(map[string]interface{}{
				"latest_msg": session.LatestMsg,
				"updated_at": session.UpdatedAt,
				// 利用 gorm.Expr 进行原子的数据库层面加法，防止高并发下更新丢失 (Lost Update)
				"unread_brand": gorm.Expr("unread_brand + ?", unreadBrandInc),
				"unread_kol":   gorm.Expr("unread_kol + ?", unreadKolInc),
			}),
		}).Create(&session).Error

		if err != nil {
			hlog.CtxErrorf(ctx, "更新/创建会话失败 SessionID:%s, Err:%v", session.SessionID, err)
			return err
		}

		return nil // 事务提交成功
	})
}

// RemoteBaseInfo 适配 user_center 接口返回的单个用户完整聚合模型
type RemoteBaseInfo struct {
	Username string `json:"username"`
	Role     int    `json:"role"`
}

type RemoteProfile struct {
	RealName  string `json:"real_name"`
	AvatarURL string `json:"avatar_url"`
}

type RemoteUGCAccount struct {
	Platform string `json:"platform"`
	Nickname string `json:"nickname"`
}

type BaseUserInfo struct {
	BaseInfo    RemoteBaseInfo     `json:"base_info"`
	Profile     RemoteProfile      `json:"profile"`
	UGCAccounts []RemoteUGCAccount `json:"ugc_accounts"`
}

// GetDisplayName 智能提取展示名称 (瀑布流降级策略: UGC昵称 > 真实姓名 > 基础用户名)
func (u BaseUserInfo) GetDisplayName() string {
	if len(u.UGCAccounts) > 0 && u.UGCAccounts[0].Nickname != "" {
		return u.UGCAccounts[0].Nickname
	}
	if u.Profile.RealName != "" {
		return u.Profile.RealName
	}
	if u.BaseInfo.Username != "" {
		return u.BaseInfo.Username
	}
	return "未知用户"
}

// GetDisplayAvatar 获取显示头像
func (u BaseUserInfo) GetDisplayAvatar() string {
	if u.Profile.AvatarURL != "" {
		return u.Profile.AvatarURL
	}
	// 返回系统默认头像兜底
	return ""
}

// GetUserSessions 拉取会话列表，并适配返回给前端的结构
func (s *IMPersistenceService) GetUserSessions(ctx context.Context, userID uint64, role int) ([]map[string]interface{}, error) {
	var sessions []db.IMSession

	// 1. 根据当前用户ID，查出所有相关的会话（使用复合索引查询）
	err := s.DB.WithContext(ctx).
		Where("brand_user_id = ? OR kol_user_id = ?", userID, userID).
		Order("updated_at DESC"). // 按照最新消息时间降序
		Find(&sessions).Error

	if err != nil {
		return nil, err
	}

	// 2. 收集所有 TargetID，准备跨服务批量获取头像与昵称
	targetIDs := make([]uint64, 0, len(sessions))
	for _, sess := range sessions {
		if userID == sess.BrandUserID {
			targetIDs = append(targetIDs, sess.KolUserID)
		} else {
			targetIDs = append(targetIDs, sess.BrandUserID)
		}
	}
	// 3. 执行内部 RPC/HTTP 批量调用，消灭 N+1 查询问题
	userInfoMap := user_rpc.BatchGetUserInfo(ctx, targetIDs)

	// 4. 数据转化组装 (DTO)
	var result []map[string]interface{}
	for _, sess := range sessions {
		var targetID uint64
		var unreadCount int

		if userID == sess.BrandUserID {
			targetID = sess.KolUserID
			unreadCount = sess.UnreadBrand
		} else {
			targetID = sess.BrandUserID
			unreadCount = sess.UnreadKol
		}

		targetName := "未知用户"
		targetAvatar := ""
		if info, ok := userInfoMap[targetID]; ok {
			targetName = info.GetDisplayName()
			targetAvatar = info.GetDisplayAvatar()
		}

		result = append(result, map[string]interface{}{
			"session_id": sess.SessionID,
			// 【核心修改】：手动强转为字符串，阻断前端 JS 精度丢失！
			"target_user_id":   strconv.FormatUint(targetID, 10),
			"target_user_name": targetName,
			"target_avatar":    targetAvatar,
			"latest_msg":       sess.LatestMsg,
			"unread_count":     unreadCount,
			// 如果 sess.UpdatedAt 是 time.Time 类型，这样写是对的
			"updated_at": sess.UpdatedAt.UnixMilli(),
		})
	}

	return result, nil
}

// GetChatHistory 游标拉取历史消息记录
func (s *IMPersistenceService) GetChatHistory(ctx context.Context, sessionID string, cursor int64, limit int) ([]db.IMMessage, int64, error) {
	var messages []db.IMMessage

	query := s.DB.WithContext(ctx).Where("session_id = ?", sessionID)

	// 如果 Cursor > 0，说明是前端上拉加载更多历史消息 (要求 msg_id 严格小于当前屏幕最老那条的 msg_id)
	if cursor > 0 {
		query = query.Where("msg_id < ?", cursor)
	}

	// 按照 MsgID 降序拉取最新的一批
	err := query.Order("msg_id DESC").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	// 计算 NextCursor
	var nextCursor int64 = 0
	if len(messages) > 0 {
		// 因为是降序排序，数组最后一条就是这批数据里最老的一条
		nextCursor = messages[len(messages)-1].MsgID
	}

	// 原地反转切片，适配前端聊天气泡从上到下的渲染顺序，[最老消息, 较老消息, 最新消息]
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nextCursor, nil
}

// ClearUnreadCount 清空当前用户在该会话中的未读红点 (支持异步调用)
func (s *IMPersistenceService) ClearUnreadCount(ctx context.Context, sessionID string, role int, currentUserID uint64) error {
	// 根据角色决定清空哪个字段
	updateField := "unread_kol"
	if role == 2 { // 1是kol，2 是品牌方
		updateField = "unread_brand"
	}

	// 极客操作：只在未读数 > 0 时才触发 UPDATE，减少不必要的 MySQL 写入开销和 Binlog 污染
	err := s.DB.WithContext(ctx).
		Model(&db.IMSession{}).
		Where("session_id = ? AND "+updateField+" > 0", sessionID).
		Update(updateField, 0).Error
	if err != nil {
		return err
	}

	err = s.DB.WithContext(ctx).Model(&db.IMMessage{}).
		Where("session_id = ? AND receiver_id = ? AND status = 0", sessionID, currentUserID).
		Update("status", 1).Error

	return err
}
