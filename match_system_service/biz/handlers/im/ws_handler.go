package im

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/websocket"

	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/pkg/auth" // Redis Session 检查逻辑
	"kol_ads_marketing/match_system_service/pkg/mq"
	"kol_ads_marketing/match_system_service/pkg/utils"
	"kol_ads_marketing/match_system_service/service/im_service"
)

// 升级器配置
var upgrader = websocket.HertzUpgrader{
	// 允许跨域请求（生产环境建议根据配置限制跨域域名，防止 CSWSH 攻击）
	CheckOrigin: func(ctx *app.RequestContext) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Connect 端点：处理客户端的 ws:// 连入请求
func Connect(c context.Context, ctx *app.RequestContext) {
	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		var currentUserID uint64
		var currentUserRole int

		// 极客防御：设置初始鉴权超时时间。
		// 如果客户端连上 WS 后 5 秒内没有完成鉴权，强制断开，防止连接泄漏
		err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			return
		}

		defer func() {
			if currentUserID > 0 {
				im_service.GlobalClientManager.RemoveClient(currentUserID)
				hlog.CtxInfof(c, "用户 [%d] 下线，已清理连接", currentUserID)
			}
			err := conn.Close()
			if err != nil {
				return
			}
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				// 包含正常断开、网络异常断开、或触犯了 ReadDeadline
				break
			}

			var cmd model.WsCmd
			if err := json.Unmarshal(message, &cmd); err != nil {
				hlog.CtxErrorf(c, "收到非法格式消息: %v", err)
				continue
			}

			switch cmd.CmdType {

			// --- A. 鉴权信令 ---
			case model.CmdTypeAuth:
				var authReq model.AuthReq
				_ = json.Unmarshal(cmd.Data, &authReq)

				// 1. 提取并清理 Token
				token := strings.TrimSpace(strings.TrimPrefix(authReq.Token, "Bearer "))
				if token == "" {
					sendWsMsg(conn, model.CmdTypeAuthAck, map[string]string{"error": "Token format invalid"})
					return // return 会触发 defer 关闭连接
				}

				// 2. 调用封装的 Session 工具包进行 Redis 校验与滑动续期
				sessionInfo, err := auth.CheckAndGetSession(c, token)
				if err != nil {
					hlog.CtxErrorf(c, "WS 鉴权失败 Token[%s]: %v", token, err)
					// 告诉前端鉴权失败（如 Token 过期），让前端引导重新登录
					sendWsMsg(conn, model.CmdTypeAuthAck, map[string]interface{}{
						"status": "fail",
						"error":  err.Error(),
					})
					return // 验证失败，断开连接
				}

				// 3. 鉴权成功，绑定当前连接上下文
				currentUserID = uint64(sessionInfo.UserID)
				currentUserRole = sessionInfo.Role

				// 4. 将其注册进本地 WebSocket 客户端管理器 (ClientManager)
				im_service.GlobalClientManager.AddClient(currentUserID, conn)

				// 5. 鉴权成功后，取消 ReadDeadline 限制，恢复长连接的无限制读取
				// (或者将其设置为心跳超时时间，如 60 秒)
				_ = conn.SetReadDeadline(time.Time{})

				// 6. 返回成功回执
				sendWsMsg(conn, model.CmdTypeAuthAck, map[string]interface{}{
					"status":  "success",
					"user_id": currentUserID,
					"role":    sessionInfo.Role,
				})
				hlog.CtxInfof(c, "用户 [%d] WS鉴权成功 (角色:%d)，加入连接池", currentUserID, sessionInfo.Role)

			// --- B. 心跳保活信令 ---
			case model.CmdTypeHeartbeat:
				// 心跳包可以用来刷新断线超时时间 (配合上面的 SetReadDeadline 使用)
				// conn.SetReadDeadline(time.Now().Add(60 * time.Second))
				sendWsMsg(conn, model.CmdTypeHeartbeat, map[string]int64{"server_time": time.Now().UnixMilli()})

			// --- C. 聊天消息信令 ---
			case model.CmdTypeMessage:
				if currentUserID == 0 {
					hlog.CtxErrorf(c, "未鉴权的用户尝试发消息，已拦截")
					continue // 忽略未鉴权的消息，或者直接踢掉
				}

				var msgReq model.MsgReq
				if err := json.Unmarshal(cmd.Data, &msgReq); err != nil {
					hlog.CtxErrorf(c, "解析消息载荷失败: %v", err)
					continue
				}

				var brandID, kolID uint64
				if currentUserRole == 1 { // 当前发送者是品牌方
					brandID = currentUserID
					kolID = msgReq.ReceiverID
				} else { // 当前发送者是红人
					brandID = msgReq.ReceiverID
					kolID = currentUserID
				}

				sessionID := GenerateSessionID(currentUserID, msgReq.ReceiverID)

				// 1. 补全安全与业务字段 (不要相信前端传的 SenderID，从鉴权上下文里取)
				// 1. 组装安全的富领域模型 (Enriched Model)
				// 永远不相信客户端传的发送者，直接取 currentUserID
				kafkaMsg := model.KafkaChatMsg{
					MsgID:      utils.GenerateSnowflakeID(), // 服务端生成全局唯一消息ID
					SenderID:   currentUserID,               // 从 Token 鉴权上下文中取
					ReceiverID: msgReq.ReceiverID,
					MsgType:    msgReq.MsgType,
					Content:    msgReq.Content,
					SendTime:   time.Now().UnixMilli(), // 服务端取绝对时间戳

					// -- 注入业务上下文，解放消费者 --
					SessionID: sessionID,
					BrandID:   brandID,
					KolID:     kolID,
				}

				hlog.CtxInfof(c, "收到 [%d] 发给 [%d] 的消息: %s", currentUserID, msgReq.ReceiverID, msgReq.Content)

				// 下一步：投递给 Kafka
				err := mq.ProduceChatMessage(c, &kafkaMsg)
				if err != nil {
					// 3. 给前端回复一个系统级发送失败的错误
					sendWsMsg(conn, model.CmdTypeError, map[string]string{
						"error": "系统繁忙，消息发送失败",
					})
					continue
				}

				// 4. (可选) 给发送方回执：你的消息已成功投递到服务器
				sendWsMsg(conn, model.CmdTypeMsgAck, map[string]string{
					"msg_id": strconv.FormatInt(kafkaMsg.MsgID, 10),
					"status": "sent",
				})
			}
		}
	})

	if err != nil {
		hlog.CtxErrorf(c, "WebSocket 升级失败: %v", err)
	}
}

// sendWsMsg 辅助函数：封装统一下发格式
func sendWsMsg(conn *websocket.Conn, cmdType int, data interface{}) {
	dataBytes, _ := json.Marshal(data)
	resp := model.WsCmd{
		CmdType: cmdType,
		Data:    dataBytes,
	}
	respBytes, _ := json.Marshal(resp)
	_ = conn.WriteMessage(websocket.TextMessage, respBytes)
}

// GenerateSessionID 生成 SessionID
func GenerateSessionID(userA, userB uint64) string {
	if userA < userB {
		return fmt.Sprintf("%d_%d", userA, userB)
	}
	return fmt.Sprintf("%d_%d", userB, userA)
}
