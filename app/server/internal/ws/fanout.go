package ws

import "sync"

// Fanout 实现 Broadcaster 接口。
// 房间名使用复合键（由 signal.roomKey 生成）以支持 Guild 命名空间隔离。
// 线程安全，读写分离（广播时只加 RLock）。
type Fanout struct {
	mu      sync.RWMutex
	rooms   map[string]map[string]*Client // roomKey -> clientID -> *Client
	clients map[string]*Client            // clientID -> *Client
}

// compile-time interface check
var _ Broadcaster = (*Fanout)(nil)

// NewFanout 创建一个空的扇出。
func NewFanout() *Fanout {
	return &Fanout{
		rooms:   make(map[string]map[string]*Client),
		clients: make(map[string]*Client),
	}
}

// Add 实现 Broadcaster.Add。
func (f *Fanout) Add(c *Client) {
	f.mu.Lock()
	f.clients[c.ID()] = c
	f.mu.Unlock()
}

// Remove 实现 Broadcaster.Remove — 注销客户端并清理空房间。
func (f *Fanout) Remove(clientID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.clients, clientID)

	var rooms []string
	for room, members := range f.rooms {
		if _, ok := members[clientID]; ok {
			delete(members, clientID)
			rooms = append(rooms, room)
		}
		if len(members) == 0 {
			delete(f.rooms, room)
		}
	}
	return rooms
}

// Join 实现 Broadcaster.Join。
func (f *Fanout) Join(room, clientID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rooms[room] == nil {
		f.rooms[room] = make(map[string]*Client)
	}
	if c, ok := f.clients[clientID]; ok {
		f.rooms[room][clientID] = c
	}
}

// Leave 实现 Broadcaster.Leave。
func (f *Fanout) Leave(room, clientID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if members, ok := f.rooms[room]; ok {
		delete(members, clientID)
		if len(members) == 0 {
			delete(f.rooms, room)
		}
	}
}

// BroadcastToRoom 实现 Broadcaster.BroadcastToRoom。
// 在 RLock 下拷贝成员指针切片后释放锁，避免持锁调用 Send。
func (f *Fanout) BroadcastToRoom(room, event string, data interface{}) {
	payload := map[string]interface{}{"event": event, "data": data}
	f.mu.RLock()
	members := f.rooms[room]
	// 拷贝指针，避免在 Send 阻塞时持锁
	targets := make([]*Client, 0, len(members))
	for _, c := range members {
		targets = append(targets, c)
	}
	f.mu.RUnlock()
	for _, c := range targets {
		c.Send(payload)
	}
}

// BroadcastToNamespace 实现 Broadcaster.BroadcastToNamespace。
func (f *Fanout) BroadcastToNamespace(event string, data interface{}) {
	payload := map[string]interface{}{"event": event, "data": data}
	f.mu.RLock()
	all := make([]*Client, 0, len(f.clients))
	for _, c := range f.clients {
		all = append(all, c)
	}
	f.mu.RUnlock()
	for _, c := range all {
		c.Send(payload)
	}
}

// ForEach 实现 Broadcaster.ForEach。
func (f *Fanout) ForEach(room string, fn func(ClientMessenger) bool) {
	f.mu.RLock()
	members := f.rooms[room]
	// 拷贝指针，避免在 fn 阻塞时持锁
	targets := make([]*Client, 0, len(members))
	for _, c := range members {
		targets = append(targets, c)
	}
	f.mu.RUnlock()
	for _, c := range targets {
		if !fn(c) {
			return
		}
	}
}

// RoomExists 实现 Broadcaster.RoomExists。
func (f *Fanout) RoomExists(room string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.rooms[room]) > 0
}

// GetClient 实现 Broadcaster.GetClient。
func (f *Fanout) GetClient(clientID string) ClientMessenger {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.clients[clientID]
}
