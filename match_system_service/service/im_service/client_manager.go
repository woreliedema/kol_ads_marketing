package im_service

import (
	"github.com/hertz-contrib/websocket"
	"sync"
)

// ClientManager WebSocket 客户端连接管理器
type ClientManager struct {
	// 存储结构: UserID (uint64) -> *websocket.Conn
	clients sync.Map
}

// GlobalClientManager 全局单例的连接管理器
var GlobalClientManager = &ClientManager{}

// AddClient 用户鉴权成功后，将其加入本地连接池
func (m *ClientManager) AddClient(userID uint64, conn *websocket.Conn) {
	m.clients.Store(userID, conn)
}

// RemoveClient 用户断开连接时，将其移出连接池
func (m *ClientManager) RemoveClient(userID uint64) {
	m.clients.Delete(userID)
}

// GetClient 获取指定用户的 WebSocket 连接 (用于后续发消息)
func (m *ClientManager) GetClient(userID uint64) (*websocket.Conn, bool) {
	val, ok := m.clients.Load(userID)
	if !ok {
		return nil, false
	}
	return val.(*websocket.Conn), true
}
