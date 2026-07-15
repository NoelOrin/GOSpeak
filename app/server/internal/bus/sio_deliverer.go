package bus

import socketio "github.com/googollee/go-socket.io"

// SIODeliverer 将事件投递到 Socket.IO server。
// 实现 bus.Deliverer 接口。
type SIODeliverer struct {
	server *socketio.Server
}

// NewSIODeliverer 创建基于 Socket.IO server 的投递器。
func NewSIODeliverer(s *socketio.Server) *SIODeliverer {
	return &SIODeliverer{server: s}
}

// BroadcastToNamespace 向整个命名空间广播事件。
func (d *SIODeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.server == nil {
		return
	}
	d.server.BroadcastToNamespace("/", event, data)
}

// BroadcastToRoom 向指定房间广播事件。
func (d *SIODeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.server == nil {
		return
	}
	d.server.BroadcastToRoom("/", room, event, data)
}
