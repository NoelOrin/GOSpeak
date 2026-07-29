package ws

// Broadcaster 是 Hub 等上层组件依赖的广播/房间管理最小面。
// 隔离 Fanout 实现细节，支持 mock 测试。
type Broadcaster interface {
	// Add 注册一个客户端到扇出（不加入任何房间）。
	Add(c *Client)
	// Remove 从扇出和所有房间中移除客户端，返回它所在的房间列表。
	Remove(clientID string) []string
	// Join 将客户端加入指定房间（房间名使用复合键 roomKey）。
	Join(room, clientID string)
	// Leave 将客户端从指定房间移除。
	Leave(room, clientID string)
	// BroadcastToRoom 向房间内所有客户端广播。
	BroadcastToRoom(room, event string, data interface{})
	// BroadcastToNamespace 向所有连接客户端广播。
	BroadcastToNamespace(event string, data interface{})
	// ForEach 遍历房间内客户端，fn 返回 false 时停止。
	ForEach(room string, fn func(ClientMessenger) bool)
	// RoomExists 检查房间是否存在（有客户端连接）。
	RoomExists(room string) bool
	// GetClient 通过 ID 查找客户端。
	GetClient(clientID string) ClientMessenger
}
