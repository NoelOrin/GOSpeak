package bus

// SocketServer 是 go-socket.io Server 的最小广播面。
type SocketServer interface {
	BroadcastToNamespace(namespace string, event string, args ...interface{}) bool
	BroadcastToRoom(namespace string, room string, event string, args ...interface{}) bool
}

// SIODeliverer 将事件投递到本机 Socket.IO。
type SIODeliverer struct {
	Server SocketServer
}

func NewSIODeliverer(server SocketServer) *SIODeliverer {
	return &SIODeliverer{Server: server}
}

func (d *SIODeliverer) BroadcastToNamespace(event string, data interface{}) {
	if d == nil || d.Server == nil {
		return
	}
	d.Server.BroadcastToNamespace("/", event, data)
}

func (d *SIODeliverer) BroadcastToRoom(room, event string, data interface{}) {
	if d == nil || d.Server == nil {
		return
	}
	d.Server.BroadcastToRoom("/", room, event, data)
}
