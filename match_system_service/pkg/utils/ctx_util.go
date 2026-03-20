package utils

import (
	"github.com/cloudwego/hertz/pkg/app"
)

// GetUserIDFromCtx 安全地从请求上下文中获取 UserID
func GetUserIDFromCtx(ctx *app.RequestContext) int64 {
	if val, exists := ctx.Get("user_id"); exists {
		if uid, ok := val.(int64); ok {
			return uid
		}
		// 根据 JSON 序列化的情况，有时候数字会被解析为 float64
		if uidFloat, ok := val.(float64); ok {
			return int64(uidFloat)
		}
	}
	return 0
}

// GetUserRoleFromCtx 获取角色
func GetUserRoleFromCtx(ctx *app.RequestContext) int {
	if val, exists := ctx.Get("role"); exists {
		if role, ok := val.(int); ok {
			return role
		}
		if roleFloat, ok := val.(float64); ok {
			return int(roleFloat)
		}
	}
	return 0
}
