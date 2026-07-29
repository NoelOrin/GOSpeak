package bus

import "GOSpeak/internal/ws"

// WSDeliverer 实现 event bus 的 local deliverer，将事件投递到本地 WS Fanout。
// 替代旧的 SIODeliverer（go-socket.io）。
type WSDeliverer struct {
	fanout ws.Broadcaster
}

// NewWSDeliverer 创建一个 WSDeliverer。
func NewWSDeliverer(f ws.Broadcaster) *WSDeliverer {
	return &WSDeliverer{fanout: f}
}

// BroadcastToNamespace 实现 Deliverer 接口 — 向所有连接客户端广播。
func (d *WSDeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.fanout == nil {
		return
	}
	d.fanout.BroadcastToNamespace(event, data)
}

// BroadcastToRoom 实现 Deliverer 接口 — 向指定房间内客户端广播。
func (d *WSDeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.fanout == nil {
		return
	}
	d.fanout.BroadcastToRoom(room, event, data)
}
