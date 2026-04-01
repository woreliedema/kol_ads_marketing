package model

// GetSessionListReq 获取会话列表请求
type GetSessionListReq struct {
	Page int `query:"page" vd:"$>0" default:"1"`
	Size int `query:"size" vd:"$>0" default:"20"`
}

// GetHistoryReq 获取历史消息请求
type GetHistoryReq struct {
	TargetUserID uint64 `query:"target_user_id" vd:"$>0"`
	Page         int    `query:"page" vd:"$>0" default:"1"`
	Size         int    `query:"size" vd:"$>0" default:"50"`
}

// ClearUnreadReq 清除未读数请求
type ClearUnreadReq struct {
	TargetUserID uint64 `json:"target_user_id" vd:"$>0"`
}
