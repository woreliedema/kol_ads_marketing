package model

import "encoding/json"

// 定义信令枚举常量
const (
	CmdTypeAuth        = 100 // 鉴权请求 (前端 -> 后端)
	CmdTypeAuthAck     = 101 // 鉴权响应 (后端 -> 前端)
	CmdTypeHeartbeat   = 200 // 心跳 Ping (前端 -> 后端)
	CmdTypeMessage     = 300 // 聊天消息 (前端 -> 后端 -> 前端)
	CmdTypeMsgAck      = 301 // 消息送达回执 (后端 -> 前端)
	CmdTypeReadReceipt = 302 // 消息已读回执 (后端 -> 前端)
	CmdTypeError       = 400 // 系统异常错误 (后端 -> 前端)
)

// ReadReceiptPush 已读回执推送载荷
type ReadReceiptPush struct {
	SessionID string `json:"session_id"`
	ReaderID  uint64 `json:"reader_id,string"` // 谁读了消息 (用于前端校验)
	ReadTime  int64  `json:"read_time"`        // 阅读时间(毫秒)
}

// WsCmd 统一的 WebSocket 外层信令包
type WsCmd struct {
	CmdType int             `json:"cmd_type"` // 指令类型
	Data    json.RawMessage `json:"data"`     // 动态载荷（利用 json.RawMessage 延迟解析，极大地提升路由性能）
}

// ---------------- 以下为具体的 Data 载荷结构体 ----------------

// AuthReq 鉴权请求载荷
type AuthReq struct {
	Token string `json:"token"`
}

// MsgReq 客户端发送聊天消息的载荷
type MsgReq struct {
	ReceiverID uint64 `json:"receiver_id,string"` // 接收人ID (品牌发给红人，或红人发给品牌)
	MsgType    int8   `json:"msg_type"`           // 1:文本 2:图片 3:合作卡片
	Content    string `json:"content"`            // 消息内容
}

// MsgPush 服务端向客户端推送聊天消息的载荷
type MsgPush struct {
	MsgID    string `json:"msg_id"` // 同样转为 string 返回给前端
	SenderID uint64 `json:"sender_id,string"`
	MsgType  int8   `json:"msg_type"`
	Content  string `json:"content"`
	SendTime int64  `json:"send_time"` // 毫秒时间戳
}

// KafkaChatMsg 专用于后端微服务之间 Kafka 消息总线流转
type KafkaChatMsg struct {
	MsgID      int64  `json:"msg_id,string"`
	SenderID   uint64 `json:"sender_id,string"`   // 消费端要知道是谁发的
	ReceiverID uint64 `json:"receiver_id,string"` // 消费端要靠它去本地连接池找接收人
	MsgType    int8   `json:"msg_type"`
	Content    string `json:"content"`
	SendTime   int64  `json:"send_time"`

	SessionID string `json:"session_id"`
	BrandID   uint64 `json:"brand_id,string"`
	KolID     uint64 `json:"kol_id,string"`
}
