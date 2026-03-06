package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"

	"github.com/google/uuid"
)

const (
	// TokenExpiration 定义多端登录的有效生命周期为 7 天
	TokenExpiration = 7 * 24 * time.Hour
)

// SessionInfo 存在 Redis 里的正向映射数据结构 (Token -> UserInfo)
type SessionInfo struct {
	UserID     uint64          `json:"user_id"`
	Role       models.RoleType `json:"role"`
	ClientType string          `json:"client_type"` // "pc" 或 "mobile"
}

// GenerateAndSaveToken 核心逻辑：生成 Token 并处理双向映射与踢人逻辑
func GenerateAndSaveToken(ctx context.Context, userID uint64, role models.RoleType, clientType string) (string, error) {
	// 1. 生成高强度随机 Opaque Token (去掉中间的横杠)
	newToken := uuid.New().String()

	// Redis Key 定义
	userHashKey := fmt.Sprintf("auth:user:%d", userID)    // 反向映射 Key (Hash)
	newTokenKey := fmt.Sprintf("auth:token:%s", newToken) // 正向映射 Key (String)

	// 2. 检查并执行“踢人”逻辑 (单端挤兑)
	// 去查该用户在这个端 (如 pc) 是否已经有旧 Token
	oldToken, err := db.RDB.HGet(ctx, userHashKey, clientType).Result()
	if err == nil && oldToken != "" {
		// 发现旧 Token！毫不留情地将其从 Redis 正向映射中抹除，实现强制下线
		oldTokenKey := fmt.Sprintf("auth:token:%s", oldToken)
		db.RDB.Del(ctx, oldTokenKey)
	}

	// 3. 构建新的 Session 载荷
	sessionData := SessionInfo{
		UserID:     userID,
		Role:       role,
		ClientType: clientType,
	}
	sessionJSON, _ := json.Marshal(sessionData)

	// 4. 开启 Redis 管道 (Pipeline) 保证双向映射写入的原子性与高性能
	pipe := db.RDB.TxPipeline()

	// 写入正向映射 (供中间件鉴权查验)，设置 7 天过期
	pipe.Set(ctx, newTokenKey, sessionJSON, TokenExpiration)

	// 写入反向映射 (供后续踢人和状态管理)，不设置过期（或者设置得比 Token 长）
	pipe.HSet(ctx, userHashKey, clientType, newToken)
	pipe.Expire(ctx, userHashKey, TokenExpiration) // 顺便刷新 Hash 的过期时间

	// 执行管道
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("redis 写入 session 失败: %w", err)
	}

	return newToken, nil
}
