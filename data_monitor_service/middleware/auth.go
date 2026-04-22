package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kol_ads_marketing/data_monitor_service/biz/dal/db"
	"kol_ads_marketing/data_monitor_service/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	goredis "github.com/redis/go-redis/v9"
)

type SessionInfo struct {
	UserID     uint64 `json:"user_id"`
	Role       int    `json:"role"`
	ClientType string `json:"client_type"`
}

func AuthMiddleware() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		authHeader := string(ctx.Request.Header.Peek("Authorization"))
		if authHeader == "" {
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "缺失 Authorization 请求头")
			ctx.Abort()
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		tokenKey := fmt.Sprintf("auth:token:%s", token)

		sessionJSON, err := db.RDB.Get(c, tokenKey).Result()
		if err == goredis.Nil {
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "登录状态已失效，请重新登录")
			ctx.Abort()
			return
		} else if err != nil {
			hlog.CtxErrorf(c, "Redis 查询 Token 异常: %v", err)
			response.ErrorWithMsg(ctx, response.ErrSystemError, "系统错误")
			ctx.Abort()
			return
		}

		var sessionInfo SessionInfo
		if err := json.Unmarshal([]byte(sessionJSON), &sessionInfo); err != nil {
			response.ErrorWithMsg(ctx, response.ErrSystemError, "会话解析失败")
			ctx.Abort()
			return
		}

		// 将用户信息注入上下文，供后续业务层 (Dashboard聚合计算) 消费
		ctx.Set("user_id", sessionInfo.UserID)
		ctx.Set("role", sessionInfo.Role)
		ctx.Next(c)
	}
}
