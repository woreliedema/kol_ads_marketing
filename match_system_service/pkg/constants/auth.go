package constants

// 角色枚举 (保持与业务表结构一致)
const (
	RoleKOL   = 1  // 红人
	RoleBrand = 2  // 品牌方
	RoleAdmin = 99 // 管理员
)

// SessionInfo 对应 Redis 中存储的会话 JSON 结构
type SessionInfo struct {
	UserID     int64  `json:"user_id"`
	Role       int    `json:"role"`
	ClientType string `json:"client_type"`
}
