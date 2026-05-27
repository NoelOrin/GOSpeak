package signal

import (
	"encoding/json"
	"go_rtc/internal/model"
	"log"
	"sync"
	"time"

	socketio "github.com/googollee/go-socket.io"
)

type Room struct {
	Name      string
	Members   map[string]*MemberInfo // socketID -> MemberInfo
	CreatedAt time.Time
}

// socketServer abstracts the socketio.Server methods used by Hub, enabling testing.
type socketServer interface {
	BroadcastToNamespace(namespace string, event string, args ...interface{}) bool
	BroadcastToRoom(namespace string, room string, event string, args ...interface{}) bool
	ForEach(namespace string, room string, f socketio.EachFunc) bool
}
// roomStore abstracts DB room listing for Hub.
type roomStore interface {
	List(page, pageSize int) ([]model.Room, int64, error)
}

type Hub struct {
	server    socketServer
	rooms     map[string]*Room // roomName -> Room
	mu        sync.RWMutex
	roomStore roomStore
}

func NewHub(store roomStore) *Hub {
	return &Hub{
		rooms:     make(map[string]*Room),
		roomStore: store,
	}
}

func (h *Hub) SetServer(server *socketio.Server) {
	h.server = server
}

// ─── 事件注册 ───

func (h *Hub) SetupRoutes(server *socketio.Server) {
	h.SetServer(server)

	server.OnConnect("/", h.OnConnect)
	server.OnDisconnect("/", h.OnDisconnect)
	server.OnEvent("/", EventRoomCreate, h.OnRoomCreate)
	server.OnEvent("/", EventRoomJoin, h.OnRoomJoin)
	server.OnEvent("/", EventRoomLeave, h.OnRoomLeave)
	server.OnEvent("/", EventRoomList, h.OnRoomList)
}

// ─── 连接/断开 ───

func (h *Hub) OnConnect(s socketio.Conn) error {
	s.SetContext("")
	log.Printf("[Signal] client connected: %s", s.ID())
	return nil
}

func (h *Hub) OnDisconnect(s socketio.Conn, reason string) {
	h.mu.Lock()
	for roomName, room := range h.rooms {
		if member, ok := room.Members[s.ID()]; ok {
			delete(room.Members, s.ID())
			h.broadcastToRoomLocked(roomName, EventMemberLeft, map[string]interface{}{
				"room":     roomName,
				"identity": member.Identity,
				"id":       s.ID(),
			})
			if len(room.Members) == 0 {
				delete(h.rooms, roomName)
			}
		}
	}
	h.mu.Unlock()
	log.Printf("[Signal] client disconnected: %s, reason: %s", s.ID(), reason)
}

// ─── 房间创建 ───

func (h *Hub) OnRoomCreate(s socketio.Conn, data string) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		s.Emit(EventRoomCreated, map[string]interface{}{
			"error": "room name is required",
		})
		return
	}

	h.mu.Lock()
	if _, exists := h.rooms[req.Room]; exists {
		h.mu.Unlock()
		s.Emit(EventRoomCreated, map[string]interface{}{
			"error": "room already exists",
		})
		return
	}

	h.rooms[req.Room] = &Room{
		Name:      req.Room,
		Members:   make(map[string]*MemberInfo),
		CreatedAt: time.Now(),
	}
	roomInfo := h.roomInfoLocked(req.Room)
	h.mu.Unlock()

	log.Printf("[Signal] room created: %s by %s", req.Room, s.ID())

	s.Emit(EventRoomCreated, roomInfo)
	h.server.BroadcastToNamespace("/", EventRoomUpdated, roomInfo)
}

// ─── 加入房间 ───

func (h *Hub) OnRoomJoin(s socketio.Conn, data string) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		s.Emit(EventRoomJoined, map[string]interface{}{
			"error": "room name is required",
		})
		return
	}

	s.Join(req.Room)

	identity := req.Identity
	if identity == "" {
		identity = s.ID()
	}

	member := &MemberInfo{
		ID:       s.ID(),
		Identity: identity,
		JoinedAt: time.Now().UnixMilli(),
	}

	h.mu.Lock()
	room, exists := h.rooms[req.Room]
	if !exists {
		room = &Room{
			Name:      req.Room,
			Members:   make(map[string]*MemberInfo),
			CreatedAt: time.Now(),
		}
		h.rooms[req.Room] = room
	}
	room.Members[s.ID()] = member
	members := h.getMembersLocked(req.Room)
	h.mu.Unlock()

	log.Printf("[Signal] %s (%s) joined room: %s", s.ID(), identity, req.Room)

	// 给发送者回完整的成员列表
	s.Emit(EventRoomJoined, map[string]interface{}{
		"room":    req.Room,
		"members": members,
		"count":   len(members),
	})

	// 通知房间内其他人（排除发送者）
	h.BroadcastToRoomExcept(req.Room, s.ID(), EventMemberJoined, map[string]interface{}{
		"room":     req.Room,
		"identity": identity,
		"id":       s.ID(),
	})
}

// ─── 离开房间 ───

func (h *Hub) OnRoomLeave(s socketio.Conn, data string) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		s.Emit(EventRoomLeft, map[string]interface{}{
			"error": "room name is required",
		})
		return
	}

	s.Leave(req.Room)

	var identity string
	h.mu.Lock()
	if room, exists := h.rooms[req.Room]; exists {
		if member, ok := room.Members[s.ID()]; ok {
			identity = member.Identity
			delete(room.Members, s.ID())
			if len(room.Members) == 0 {
				delete(h.rooms, req.Room)
			}
		}
	}
	h.mu.Unlock()

	if identity == "" {
		identity = s.ID()
	}

	log.Printf("[Signal] %s (%s) left room: %s", s.ID(), identity, req.Room)

	s.Emit(EventRoomLeft, map[string]interface{}{
		"room": req.Room,
	})

	h.BroadcastToRoom(req.Room, EventMemberLeft, map[string]interface{}{
		"room":     req.Room,
		"identity": identity,
		"id":       s.ID(),
	})
}

// ─── 房间列表 ───

func (h *Hub) OnRoomList(s socketio.Conn) {
	rooms := h.getMergedRooms()
	s.Emit(EventRoomListResult, map[string]interface{}{
		"rooms": rooms,
		"count": len(rooms),
	})
}

// getMergedRooms 合并 DB 持久化房间 + 内存活跃房间。
func (h *Hub) getMergedRooms() []RoomInfo {
	// 从 DB 获取所有房间（最多 200 条）
	dbRooms := make(map[string]RoomInfo)
	if h.roomStore != nil {
		if rooms, _, err := h.roomStore.List(1, 200); err == nil {
			for _, r := range rooms {
				dbRooms[r.Name] = RoomInfo{
					ID:        r.ID,
					UUID:      r.UUID,
					Name:      r.Name,
					Limit:     r.Limit,
					Members:   []MemberInfo{},
					Count:     0,
					CreatedAt: r.CreatedAt.UnixMilli(),
				}
			}
		}
	}

	// 用内存活跃数据覆盖（有实时成员信息），同时保留 DB 字段
	h.mu.RLock()
	for name, room := range h.rooms {
		info := h.roomInfoLocked(room.Name)
		if existing, ok := dbRooms[name]; ok {
			info.ID = existing.ID
			info.UUID = existing.UUID
			info.Limit = existing.Limit
		}
		dbRooms[name] = info
	}
	h.mu.RUnlock()

	result := make([]RoomInfo, 0, len(dbRooms))
	for _, r := range dbRooms {
		result = append(result, r)
	}
	return result
}

// ─── 公共方法 ───

func (h *Hub) GetRooms() []RoomInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]RoomInfo, 0, len(h.rooms))
	for _, room := range h.rooms {
		result = append(result, h.roomInfoLocked(room.Name))
	}
	return result
}

func (h *Hub) GetRoomMembers(roomName string) []MemberInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.getMembersLocked(roomName)
}

func (h *Hub) BroadcastToRoom(room string, event string, data interface{}) {
	if h.server != nil {
		h.server.BroadcastToRoom("/", room, event, data)
	}
}

func (h *Hub) BroadcastToRoomExcept(room string, excludeSocketID string, event string, data interface{}) {
	if h.server == nil {
		return
	}
	h.server.ForEach("/", room, func(conn socketio.Conn) {
		if conn.ID() != excludeSocketID {
			conn.Emit(event, data)
		}
	})
}

// ─── 内部辅助 ───

func (h *Hub) getMembersLocked(roomName string) []MemberInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		return nil
	}
	members := make([]MemberInfo, 0, len(room.Members))
	for _, m := range room.Members {
		members = append(members, *m)
	}
	return members
}

func (h *Hub) roomInfoLocked(roomName string) RoomInfo {
	room := h.rooms[roomName]
	return RoomInfo{
		Name:      room.Name,
		Members:   h.getMembersLocked(roomName),
		Count:     len(room.Members),
		CreatedAt: room.CreatedAt.UnixMilli(),
	}
}

func (h *Hub) broadcastToRoomLocked(room string, event string, data interface{}) {
	if h.server == nil {
		return
	}
	h.server.BroadcastToRoom("/", room, event, data)
}

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
