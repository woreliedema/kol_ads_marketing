package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/redis/go-redis/v9"

	"kol_ads_marketing/match_system_service/dal/cache"
	"kol_ads_marketing/match_system_service/pkg/constants"
)

const (
	TokenPrefix      = "auth:token:"
	UserHashPrefix   = "auth:user:"
	TokenExpiration  = 7 * 24 * time.Hour
	RenewalThreshold = 24 * time.Hour // 续期阈值：当过期时间小于 1 天时触发自动续期
)

var (
	ErrTokenInvalid = errors.New("token expired or invalid")
	ErrRedisSys     = errors.New("redis execution error")
	ErrDataParse    = errors.New("session data parse error")
)

// CheckAndGetSession 核心逻辑：校验 Token 并返回 SessionInfo（包含滑动续期机制）
func CheckAndGetSession(ctx context.Context, token string) (*constants.SessionInfo, error) {
	tokenKey := fmt.Sprintf("%s%s", TokenPrefix, token)

	// 1. 使用 Redis Pipeline 优化网络 I/O，同时获取 Data 和 TTL
	pipe := cache.RDB.Pipeline()
	getCmd := pipe.Get(ctx, tokenKey)
	ttlCmd := pipe.TTL(ctx, tokenKey)

	_, err := pipe.Exec(ctx)
	if errors.Is(err, redis.Nil) {
		return nil, ErrTokenInvalid
	} else if err != nil {
		hlog.CtxErrorf(ctx, "Redis Pipeline err: %v", err)
		return nil, ErrRedisSys
	}

	// 2. 解析 Session 数据
	sessionJSON := getCmd.Val()
	var sessionInfo constants.SessionInfo
	if err := json.Unmarshal([]byte(sessionJSON), &sessionInfo); err != nil {
		hlog.CtxErrorf(ctx, "Failed to unmarshal session: %v", err)
		return nil, ErrDataParse
	}

	// 3. Sliding Session：无感续期逻辑 [cite: 1, 2, 4]
	ttl := ttlCmd.Val()
	if ttl > 0 && ttl < RenewalThreshold {
		// 采用异步 Goroutine 续期，不阻塞当前请求的关键路径
		go asyncRenewSession(tokenKey, sessionInfo)
	}

	return &sessionInfo, nil
}

// asyncRenewSession 异步续期正向和反向映射
func asyncRenewSession(tokenKey string, info constants.SessionInfo) {
	// 注意：必须使用脱离原 HTTP 生命周期的 Context，避免请求结束导致 Context Canceled
	bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	userHashKey := fmt.Sprintf("%s%d", UserHashPrefix, info.UserID)

	pipe := cache.RDB.Pipeline()
	pipe.Expire(bgCtx, tokenKey, TokenExpiration)
	pipe.Expire(bgCtx, userHashKey, TokenExpiration)

	if _, err := pipe.Exec(bgCtx); err != nil {
		// 仅记录日志，续期失败不影响当次请求
		hlog.CtxErrorf(bgCtx, "Failed to renew session for token [%s]: %v", tokenKey, err)
	}
}

// Context 提取工具函数
// 在 Controller/Handler 层调用这些函数可以极大地保持代码整洁

// GetUserID 从 Hertz 请求上下文中提取用户 ID
func GetUserID(ctx *app.RequestContext) uint64 {
	if id, exists := ctx.Get("user_id"); exists {
		// 根据你在中间件注入的具体类型断言
		if uid, ok := id.(uint64); ok {
			return uid
		}
	}
	return 0
}

// GetUserRole 从 Hertz 请求上下文中提取用户角色
func GetUserRole(ctx *app.RequestContext) int {
	if role, exists := ctx.Get("role"); exists {
		if r, ok := role.(int); ok {
			return r
		}
	}
	return 0
}
