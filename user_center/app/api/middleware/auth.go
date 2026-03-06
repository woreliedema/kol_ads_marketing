package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/utils/auth"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/redis/go-redis/v9"
)

// AuthMiddleware Opaque Token 鉴权与拦截中间件
func AuthMiddleware() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		// 1. 提取 HTTP Header 中的 Authorization 字段
		authHeader := string(ctx.Request.Header.Peek("Authorization"))
		if authHeader == "" {
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "缺失 Authorization 请求头")
			ctx.Abort() // 拦截请求，不再往下游传递
			return
		}

		// 2. 兼容 "Bearer <token>" 格式或纯 "<token>" 格式
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		if token == "" {
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "Token 格式错误或为空")
			ctx.Abort()
			return
		}

		// 3. 核心校验：去 Redis 查询正向映射 (Token -> SessionInfo)
		tokenKey := fmt.Sprintf("auth:token:%s", token)
		sessionJSON, err := db.RDB.Get(c, tokenKey).Result()

		if err == redis.Nil {
			// 【单端挤兑/强制踢出 拦截生效】
			// 如果查不到，说明这个 Token 已经过期，或者在别处登录时被我们的 GenerateAndSaveToken 删除了！
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "登录状态已失效或已在其他设备登录，请重新登录")
			ctx.Abort()
			return
		} else if err != nil {
			hlog.CtxErrorf(c, "Redis 查询 Token 异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
			ctx.Abort()
			return
		}

		// 4. 解析 Session 数据
		var sessionInfo auth.SessionInfo
		if err := json.Unmarshal([]byte(sessionJSON), &sessionInfo); err != nil {
			hlog.CtxErrorf(c, "解析 Redis Session 数据失败: %v", err)
			response.Error(ctx, response.ErrSystemError)
			ctx.Abort()
			return
		}

		// 5. 【鉴权卸载】：将解析出的核心用户信息强行塞入当前请求的上下文 Context 中
		// 这样下游的 Controller 就可以直接拿来用了，无需再次查数据库或 Redis
		ctx.Set("user_id", sessionInfo.UserID)
		ctx.Set("role", sessionInfo.Role)
		ctx.Set("client_type", sessionInfo.ClientType)

		// 6. 校验通过，放行请求给下游
		ctx.Next(c)
	}
}
