package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/redis/go-redis/v9"

	"kol_ads_marketing/match_system_service/dal/cache"
	"kol_ads_marketing/match_system_service/pkg/constants"
	"kol_ads_marketing/match_system_service/pkg/response"
)

// AuthMiddleware 鉴权与角色拦截中间件
// allowedRoles: 变长参数，允许访问的角色列表。如果不传，则只校验是否登录。
func AuthMiddleware(allowedRoles ...int) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		// 1. 提取 Token
		authHeader := string(ctx.Request.Header.Peek("Authorization"))
		if authHeader == "" {
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "缺失 Authorization 请求头")
			return
		}

		// 2. 兼容 "Bearer <token>" 格式
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		if token == "" {
			response.ErrorWithMsg(ctx, response.ErrUnauthorized, "Token 格式错误或为空")
			ctx.Abort()
			return
		}

		// 2. 查 Redis 获取 Session (注意：这里的 Redis 必须与 user_center 连同一个集群/库)
		tokenKey := fmt.Sprintf("auth:token:%s", token)
		sessionJSON, err := cache.RDB.Get(c, tokenKey).Result()

		if errors.Is(err, redis.Nil) {
			response.Error(ctx, response.ErrUnauthorized)
			ctx.Abort()
			return
		} else if err != nil {
			hlog.CtxErrorf(c, "Redis 查询 Token 异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
			ctx.Abort()
			return
		}

		// 4. 解析 Session 数据
		var sessionInfo constants.SessionInfo
		if err := json.Unmarshal([]byte(sessionJSON), &sessionInfo); err != nil {
			hlog.CtxErrorf(c, "解析 Redis Session 数据失败: %v", err)
			response.ErrorWithMsg(ctx, response.ErrSystemError, "会话数据解析异常")
			ctx.Abort()
			return
		}

		// 5. RBAC 角色权限校验 (如果路由组要求了特定角色)
		if len(allowedRoles) > 0 {
			hasPermission := false
			for _, role := range allowedRoles {
				// 匹配所需角色，或者当前用户是超级管理员(RoleAdmin)则直接放行
				if sessionInfo.Role == role || sessionInfo.Role == constants.RoleAdmin {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				// 角色不匹配，抛出 403 Forbidden 业务错误
				response.Error(ctx, response.ErrPermission)
				ctx.Abort()
				return
			}
		}

		// 6. 鉴权卸载：注入上下文
		ctx.Set("user_id", sessionInfo.UserID)
		ctx.Set("role", sessionInfo.Role)

		ctx.Next(c)
	}
}
