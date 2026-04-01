package im

import (
	"context"
	"encoding/json"
	"github.com/hertz-contrib/websocket"
	"kol_ads_marketing/match_system_service/biz/model"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"

	//"kol_ads_marketing/match_system_service/dal/db"
	"kol_ads_marketing/match_system_service/pkg/response"
	"kol_ads_marketing/match_system_service/pkg/utils"
	"kol_ads_marketing/match_system_service/service/im_service"
)

var persistSvc *im_service.IMPersistenceService

// InitIMHTTPHandler 依赖注入初始化
func InitIMHTTPHandler(svc *im_service.IMPersistenceService) {
	persistSvc = svc
}

// SessionItem 单个会话对象
type SessionItem struct {
	SessionID      string `json:"session_id" example:"sess_123_456" description:"全局唯一会话ID"`
	TargetUserID   uint64 `json:"target_user_id,string" example:"456" description:"聊天对象的用户ID"`
	TargetUserName string `json:"target_user_name" example:"机智的何同学" description:"聊天对象昵称"`
	TargetAvatar   string `json:"target_avatar" example:"https://cdn.example.com/avatar.png" description:"聊天对象头像URL"`
	LatestMsg      string `json:"latest_msg" example:"期待与您的合作！" description:"该会话的最新一条消息内容"`
	UnreadCount    int    `json:"unread_count" example:"2" description:"当前用户在该会话下的未读消息数"`
	UpdatedAt      int64  `json:"updated_at" example:"1672502400000" description:"最新消息的时间戳(毫秒)"`
}

// SessionListResponseData 获取会话列表的出参载荷
type SessionListResponseData struct {
	Sessions []SessionItem `json:"sessions" description:"会话列表数组（按最新消息时间降序排列）"`
}

// MessageItemDTO 专门用于 HTTP 响应的消息 DTO，实现与数据库模型 db.IMMessage 解耦
type MessageItemDTO struct {
	MsgID      int64  `json:"msg_id,string"`      // 雪花ID，必须 string
	SenderID   uint64 `json:"sender_id,string"`   // 用户ID，必须 string
	ReceiverID uint64 `json:"receiver_id,string"` // 用户ID，必须 string
	MsgType    int8   `json:"msg_type"`
	Content    string `json:"content"`
	SendTime   int64  `json:"send_time"` // 毫秒时间戳
	Status     int8   `json:"status"`
}

type HistoryMessagesResponseData struct {
	Messages   []MessageItemDTO `json:"messages" description:"消息对象数组(按时间正序排列：从旧到新，适配前端直渲)"`
	NextCursor int64            `json:"next_cursor,string" example:"10086" description:"下一页分页游标(最老一条消息的msg_id)，为0表示无更多数据"`
	HasMore    bool             `json:"has_more" example:"true" description:"是否还有更多历史记录可以上拉加载"`
}

// GetSessionList 获取当前用户的会话列表
// @Summary 获取当前用户的会话列表（左侧联系人列表）
// @Description 提取当前登录用户的鉴权上下文，拉取其参与的所有IM会话列表。内置处理了目标用户的头像昵称聚合查询。
// @Tags IM消息模块
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Success 200 {object} response.Response{data=SessionListResponseData} "成功获取会话列表"
// @Failure 401 {object} response.Response "未授权或Token失效"
// @Failure 500 {object} response.Response "服务器内部错误"
// @Router /api/v1/match/im/sessions [get]
func GetSessionList(c context.Context, ctx *app.RequestContext) {
	// 1. 从 AuthMiddleware 提取鉴权上下文
	userIDVal, _ := ctx.Get("user_id")
	roleVal, _ := ctx.Get("role")

	currentUserID := uint64(userIDVal.(int64))
	currentUserRole := roleVal.(int)

	// 2. 调用 Service 层查询 MySQL/Redis 会话列表
	sessions, err := persistSvc.GetUserSessions(c, currentUserID, currentUserRole)
	if err != nil {
		hlog.CtxErrorf(c, "获取会话列表失败 UID:%d Err:%v", currentUserID, err)
		response.ErrorWithMsg(ctx, response.ErrSystemError, "加载会话列表失败")
		return
	}

	// 3. 响应给前端
	response.Success(ctx, map[string]interface{}{
		"sessions": sessions,
	})
}

// GetHistoryMessages 获取历史聊天记录
// @Summary 获取指定会话的历史聊天记录
// @Description 传入目标用户ID(target_id)获取历史聊天记录。采用游标(cursor)分页机制解决深度分页性能问题及数据漂移问题；返回的数据已针对UI渲染做了倒序反转；并在后台异步清理未读红点。
// @Tags IM消息模块
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param target_id query string true "目标用户ID (被查询的KOL或品牌方UID)"
// @Param cursor query string false "分页游标：传入上一页返回的 next_cursor 字段值，首页传0即可" default(0)
// @Param limit query int false "分页大小：防爆内存最大限制为100" default(20) minimum(1) maximum(100)
// @Success 200 {object} response.Response{data=HistoryMessagesResponseData} "成功获取历史聊天记录及游标状态"
// @Failure 400 {object} response.Response "参数异常（如 target_id 缺失或非法）"
// @Failure 401 {object} response.Response "未授权或Token失效"
// @Failure 500 {object} response.Response "服务器内部错误"
// @Router /api/v1/match/im/history [get]
// 路由：GET /api/v1/match/im/history?target_id=xxx&cursor=0&limit=20
func GetHistoryMessages(c context.Context, ctx *app.RequestContext) {
	// 1. 提取鉴权上下文
	userIDVal, _ := ctx.Get("user_id")
	roleVal, _ := ctx.Get("role")
	currentUserID := uint64(userIDVal.(int64))
	currentUserRole := roleVal.(int)

	// 2. 解析参数
	targetIDStr := ctx.Query("target_id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 64)
	if err != nil || targetID == 0 {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "目标用户ID非法")
		return
	}

	cursor, _ := strconv.ParseInt(ctx.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100 // 极客防御：防止恶意拉爆内存
	}

	// 3. 计算全局唯一的 SessionID (复用你发消息时的逻辑)
	sessionID := utils.GenerateSessionID(currentUserID, targetID)

	// 4. 调用 Service 查询历史记录
	dbMessages, nextCursor, err := persistSvc.GetChatHistory(c, sessionID, cursor, limit)
	if err != nil {
		hlog.CtxErrorf(c, "获取历史消息失败 SID:%s Err:%v", sessionID, err)
		response.ErrorWithMsg(ctx, response.ErrSystemError, "加载历史记录失败")
		return
	}

	dtoMessages := make([]MessageItemDTO, 0, len(dbMessages))
	for _, dbMsg := range dbMessages {
		dtoMessages = append(dtoMessages, MessageItemDTO{
			MsgID:      dbMsg.MsgID,
			SenderID:   dbMsg.SenderID,
			ReceiverID: dbMsg.ReceiverID,
			MsgType:    dbMsg.MsgType,
			Content:    dbMsg.Content,
			SendTime:   dbMsg.CreatedAt.UnixMilli(),
			Status:     dbMsg.Status,
		})
	}

	// 5. 异步清空该会话下属己方的未读红点数
	go func(sid string, role int, uid uint64) {
		bgCtx := context.Background()
		// 极客防御：捕获野生 Goroutine 的 Panic，防止整个微服务雪崩
		defer func() {
			if r := recover(); r != nil {
				hlog.CtxErrorf(bgCtx, "异步清理未读数触发 Panic 拦截 SID:%s, Err: %v", sid, r)
			}
		}()

		// 【修改点】：传入 uid (即 currentUserID)
		if err := persistSvc.ClearUnreadCount(bgCtx, sid, role, uid); err != nil {
			hlog.CtxErrorf(bgCtx, "清理未读数失败 SID:%s Role:%d Err:%v", sid, role, err)
		}
	}(sessionID, currentUserRole, currentUserID)

	// 6. 统一返回
	response.Success(ctx, map[string]interface{}{
		"messages":    dtoMessages,
		"next_cursor": nextCursor,
		"has_more":    len(dtoMessages) >= limit,
	})
}

// ClearUnreadReq 清空未读数请求体
type ClearUnreadReq struct {
	// 依然加上 ,string 防止前端精度丢失
	TargetID uint64 `json:"target_id,string" binding:"required"`
}

// ClearSessionUnread 显式清空指定会话的未读数
// @Summary 清空指定会话的未读红点
// @Description 当用户正处于聊天窗口且收到新消息时，前端静默调用此接口对齐底表未读状态
// @Tags IM消息模块
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Router /api/v1/match/im/session/read [post]
func ClearSessionUnread(c context.Context, ctx *app.RequestContext) {
	// 1. 提取鉴权上下文
	userIDVal, _ := ctx.Get("user_id")
	roleVal, _ := ctx.Get("role")
	currentUserID := uint64(userIDVal.(int64))
	currentUserRole := roleVal.(int)

	// 2. 解析请求体
	var req ClearUnreadReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "参数非法")
		return
	}

	// 3. 计算确定的 SessionID
	sessionID := utils.GenerateSessionID(currentUserID, req.TargetID)

	// 4. 执行持久化层的清零操作 (复用现有 Service 逻辑)
	if err := persistSvc.ClearUnreadCount(c, sessionID, currentUserRole, currentUserID); err != nil {
		response.ErrorWithMsg(ctx, response.ErrSystemError, "清除未读状态失败")
		return
	}

	// 5. 触发 WebSocket 反向已读回执推送！
	// currentUserID 是正在看屏幕的接收方，req.TargetID 是在另一头苦苦等待回复的发送方
	senderConn, _ := im_service.GlobalClientManager.GetClient(req.TargetID)
	if senderConn != nil { // 如果发送方刚好在线
		receiptData := model.ReadReceiptPush{
			SessionID: sessionID,
			ReaderID:  currentUserID,
			ReadTime:  time.Now().UnixMilli(),
		}
		receiptBytes, _ := json.Marshal(receiptData)

		wsCmd := model.WsCmd{
			CmdType: model.CmdTypeReadReceipt, // 302
			Data:    receiptBytes,
		}
		wsCmdBytes, _ := json.Marshal(wsCmd)

		// 极客操作：物理推送给发送方
		_ = senderConn.WriteMessage(websocket.TextMessage, wsCmdBytes)
		hlog.CtxInfof(c, "已成功向用户 [%d] 推送会话 [%s] 的已读回执", req.TargetID, sessionID)
	}

	response.Success(ctx, map[string]string{"status": "ok"})
}
