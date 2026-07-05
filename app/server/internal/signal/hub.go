package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	socketio "github.com/googollee/go-socket.io"
	"golang.org/x/sync/errgroup"
)

type Room struct {
	Name      string
	Password  string
	Members   map[string]*MemberInfo // socketID -> MemberInfo
	MicMuted  map[string]bool
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
	GetByName(name string) (*model.Room, error)
}

// muteStore abstracts mute checking for Hub.
type muteStore interface {
	IsMutedByIdentity(identity string) (bool, *model.Mute, error)
}

// userStore abstracts user lookup for Hub.
type userStore interface {
	GetByName(name string) (*model.User, error)
}

// permChecker 抽象权限校验，复用 service.PermissionService 的内存缓存。
type permChecker interface {
	HasPermission(roleName, permCode string) bool
}

// SFUSignalHandler 注册 provider 专属的 sfu:* 媒体协商事件。
type SFUSignalHandler interface {
	RegisterRoutes(server *socketio.Server)
}

// broadcastFn 供 provider 模块广播房间事件，无需依赖 Hub 具体类型。
type BroadcastFn func(room, event string, data interface{})

type Hub struct {
	server           socketServer
	sfuProvider      sfu.Provider
	sfuProviderName  string
	rooms            map[string]*Room // roomName -> Room
	mu               sync.RWMutex
	roomStore        roomStore
	muteStore        muteStore
	userStore        userStore
	permChecker      permChecker
	sfuSignalHandler SFUSignalHandler
	activeStreams    map[string]struct{}
}

func NewHub(store roomStore, mStore muteStore, uStore userStore, pChecker permChecker) *Hub {
	return &Hub{
		rooms:         make(map[string]*Room),
		roomStore:     store,
		muteStore:     mStore,
		userStore:     uStore,
		permChecker:   pChecker,
		activeStreams: make(map[string]struct{}),
	}
}

func (h *Hub) SetServer(server *socketio.Server) {
	h.server = server
}

func (h *Hub) SetSFU(provider sfu.Provider) {
	h.sfuProvider = provider
	if pn, ok := provider.(interface{ ProviderName() string }); ok {
		h.sfuProviderName = pn.ProviderName()
	}
}

func (h *Hub) SetSFUSignalHandler(handler SFUSignalHandler) {
	h.sfuSignalHandler = handler
}

// ─── 事件注册 ───

func (h *Hub) SetupRoutes(server *socketio.Server) {
	h.SetServer(server)

	server.OnConnect("/", safeOnConnect(h.OnConnect))
	server.OnDisconnect("/", safeOnDisconnect(h.OnDisconnect))
	server.OnError("/", safeOnError(h.OnError))
	server.OnEvent("/", EventRoomCreate, safeOnEventData(h.OnRoomCreate))
	server.OnEvent("/", EventRoomJoin, safeOnEventDataAck(h.OnRoomJoin))
	server.OnEvent("/", EventRoomJoinSFU, safeOnEventDataAck(h.OnRoomJoinSFU))
	server.OnEvent("/", EventRoomLeave, safeOnEventDataAck(h.OnRoomLeave))
	server.OnEvent("/", EventRoomList, safeOnEventNoData(h.OnRoomList))
	server.OnEvent("/", EventRoomKick, safeOnEventData(h.OnRoomKick))
	server.OnEvent("/", EventMemberMicState, safeOnEventData(h.OnMemberMicState))

	if h.sfuSignalHandler != nil {
		h.sfuSignalHandler.RegisterRoutes(server)
	}
}

// ─── 连接/断开 ───

func (h *Hub) OnConnect(s socketio.Conn) error {
	s.SetContext("")
	log.Printf("[Signal] client connected: %s", s.ID())
	return nil
}

func (h *Hub) OnDisconnect(s socketio.Conn, reason string) {
	type disconnectCleanup struct {
		room     string
		identity string
		deleted  bool
	}
	var cleanups []disconnectCleanup
	var updatedRooms []string
	h.mu.Lock()
	for roomName, room := range h.rooms {
		if member, ok := room.Members[s.ID()]; ok {
			identity := member.Identity
			delete(room.Members, s.ID())
			h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
				"room":     roomName,
				"identity": identity,
				"id":       s.ID(),
			})
			updatedRooms = append(updatedRooms, roomName)
			deleted := h.deleteRoomIfEmpty(roomName)
			cleanups = append(cleanups, disconnectCleanup{roomName, identity, deleted})
		}
	}
	h.mu.Unlock()

	// SFU 清理异步：RemoveParticipant/DeleteRoom 是 HTTP/gRPC 调用，可能慢。
	// 同步阻塞会拉长 OnDisconnect 持续时间，加剧 go-socket.io 内部 goroutine
	// 竞态（连接已 failed 时库 serveRead 状态错乱触发 gorilla panic）。
	// 丢后台 goroutine，handler 立即返回。
	if len(cleanups) > 0 {
		go func(cleanups []disconnectCleanup) {
			for _, c := range cleanups {
				h.removeParticipantSafe(c.room, c.identity)
				if c.deleted {
					h.deleteRoomSafe(c.room)
				}
			}
		}(cleanups)
	}

	for _, name := range updatedRooms {
		h.mu.RLock()
		_, exists := h.rooms[name]
		h.mu.RUnlock()
		if !exists {
			// 房间已空被删除，广播房间列表更新（含 DB 持久化房间）
			h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
			continue
		}
		h.mu.RLock()
		info := h.roomInfoLocked(name)
		h.mu.RUnlock()
		h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
	}

	log.Printf("[Signal] client disconnected: %s, reason: %s", s.ID(), reason)
}

// ─── 房间创建 ───

// OnError 兜底处理 socket.io 层 error（含 OnConnect 返回的 panic error）。
// 仅记录日志，不做断连等副作用——连接级错误由库自行处理。
func (h *Hub) OnError(s socketio.Conn, err error) {
	log.Printf("[Signal] socket error: conn=%s err=%v", s.ID(), err)
}

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
		Password:  req.Password,
		Members:   make(map[string]*MemberInfo),
		MicMuted:  make(map[string]bool),
		CreatedAt: time.Now(),
	}
	roomInfo := h.roomInfoLocked(req.Room)
	h.mu.Unlock()

	log.Printf("[Signal] room created: %s by %s", req.Room, s.ID())

	s.Emit(EventRoomCreated, roomInfo)
	h.server.BroadcastToNamespace("/", EventRoomUpdated, roomInfo)
}

// ─── 加入房间 ───
//
// 加入流程分两阶段：
//  1. OnRoomJoin     — 信令面。仅校验权限 + s.Join() socketio room，不写 h.rooms 成员。
//  2. OnRoomJoinSFU  — 媒体面。SFU 连接确认后写成员 + 广播 MemberJoined/RoomUpdated。
func (h *Hub) OnRoomJoin(s socketio.Conn, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{
			"error": "room name is required",
		})
	}

	// 禁言检查
	if h.muteStore != nil && req.Identity != "" {
		if muted, mute, _ := h.muteStore.IsMutedByIdentity(req.Identity); muted {
			remaining := int64(0)
			if !mute.Permanent && mute.ExpiresAt != nil {
				remaining = int64(time.Until(*mute.ExpiresAt).Seconds())
				if remaining < 0 {
					remaining = 0
				}
			}
			return marshalAck(map[string]interface{}{
				"error":     "user is muted",
				"muted":     true,
				"permanent": mute.Permanent,
				"remaining": remaining,
			})
		}
	}

	// 服务端校验房间人数上限
	if h.roomStore != nil {
		if dbRoom, err := h.roomStore.GetByName(req.Room); err == nil && dbRoom.Limit > 0 {
			h.mu.RLock()
			var currentCount int
			if room, ok := h.rooms[req.Room]; ok {
				currentCount = len(room.Members)
			}
			h.mu.RUnlock()
			if currentCount >= int(dbRoom.Limit) {
				return marshalAck(map[string]interface{}{
					"error": "room is full",
					"limit": dbRoom.Limit,
					"count": currentCount,
				})
			}
		}
	}

	identity := req.Identity
	if identity == "" {
		identity = s.ID()
	}

	// 提前拦截重复身份：已通过 OnRoomJoinSFU 写入 h.rooms 的在线用户，新 socket 在 OnRoomJoin 阶段即阻断。
	// 并发场景由 OnRoomJoinSFU 的 duplicate check 兜底，此处为提前拦截以节省媒体连接开销。
	h.mu.RLock()
	dup := h.duplicateIdentityLocked(req.Room, identity, s.ID())
	h.mu.RUnlock()
	if dup {
		return marshalAck(map[string]interface{}{
			"error":    "duplicate connection not allowed",
			"room":     req.Room,
			"identity": identity,
		})
	}

	s.Join(req.Room)

	log.Printf("[Signal] %s (%s) signaling ready for room: %s", s.ID(), identity, req.Room)

	return marshalAck(map[string]interface{}{
		"room":     req.Room,
		"identity": identity,
	})
}

// OnRoomJoinSFU 在 SFU 媒体连接确认后写成员并广播加入。
func (h *Hub) OnRoomJoinSFU(s socketio.Conn, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{"error": "room name is required"})
	}

	identity := req.Identity
	if identity == "" {
		identity = s.ID()
	}

	member := &MemberInfo{
		ID:       s.ID(),
		Identity: identity,
		JoinedAt: time.Now().UnixMilli(),
		Stream:   req.Stream,
	}

	h.mu.Lock()
	// 重复身份检查：同一房间不允许同一 identity 用不同 socket 重复加入
	if h.duplicateIdentityLocked(req.Room, identity, s.ID()) {
		h.mu.Unlock()
		// 拒绝加入：回退 phase1 的 s.Join，避免 socket 残留在 socket.io room
		s.Leave(req.Room)
		return marshalAck(map[string]interface{}{
			"error":    "duplicate connection not allowed",
			"room":     req.Room,
			"identity": identity,
		})
	}
	room, exists := h.rooms[req.Room]
	if !exists {
		room = &Room{
			Name:      req.Room,
			Members:   make(map[string]*MemberInfo),
			MicMuted:  make(map[string]bool),
			CreatedAt: time.Now(),
		}
		h.rooms[req.Room] = room
	}
	room.Members[s.ID()] = member
	memberList := h.getMembersLocked(req.Room)
	h.mu.Unlock()

	log.Printf("[Signal] sfu confirmed: %s in %s", s.ID(), req.Room)

	h.mu.RLock()
	_, memberExists := h.rooms[req.Room]
	if memberExists {
		_, memberExists = h.rooms[req.Room].Members[s.ID()]
	}
	h.mu.RUnlock()
	if !memberExists {
		log.Printf("[Signal] sfu join rejected: %s not in room %s", s.ID(), req.Room)
		// 回退 phase1 的 s.Join，避免 socket 残留在 socket.io room
		s.Leave(req.Room)
		return marshalAck(map[string]interface{}{
			"error": "not in room",
		})
	}

	h.server.BroadcastToNamespace("/", EventMemberJoined, map[string]interface{}{
		"room":     req.Room,
		"identity": req.Identity,
		"id":       s.ID(),
		"stream":   req.Stream,
	})

	h.mu.RLock()
	info := h.roomInfoLocked(req.Room)
	h.mu.RUnlock()
	h.server.BroadcastToNamespace("/", EventRoomUpdated, info)

	return marshalAck(map[string]interface{}{
		"ok":       true,
		"room":     req.Room,
		"identity": req.Identity,
		"members":  memberList,
	})
}

func marshalAck(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"serialization failed"}`, nil
	}
	return string(data), nil
}

// ─── 离开房间 ───

func (h *Hub) OnRoomLeave(s socketio.Conn, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{
			"error": "room name is required",
		})
	}

	s.Leave(req.Room)

	var identity string
	roomDeleted := false
	h.mu.Lock()
	if room, exists := h.rooms[req.Room]; exists {
		if member, ok := room.Members[s.ID()]; ok {
			identity = member.Identity
			delete(room.Members, s.ID())
			// 成员清空则删除房间，与 OnDisconnect 行为一致
			roomDeleted = h.deleteRoomIfEmpty(req.Room)
		}
	}
	h.mu.Unlock()

	// 同步清理 SFU 端：移除 participant，空房时删除 SFU room
	if identity != "" {
		h.removeParticipantSafe(req.Room, identity)
	}
	if roomDeleted {
		h.deleteRoomSafe(req.Room)
	}

	if identity == "" {
		identity = s.ID()
	}

	log.Printf("[Signal] %s (%s) left room: %s", s.ID(), identity, req.Room)

	s.Emit(EventRoomLeft, map[string]interface{}{
		"room": req.Room,
	})

	h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
		"room":     req.Room,
		"identity": identity,
		"id":       s.ID(),
	})

	// 房间已空被删除则广播房间列表，否则广播单房间更新（与 OnDisconnect 一致）
	if roomDeleted {
		h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
	} else {
		// NOTE: roomInfoLocked 在 !exists 时返回零值 RoomInfo（并发删房场景安全）
		h.mu.RLock()
		info := h.roomInfoLocked(req.Room)
		h.mu.RUnlock()
		h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
	}

	return marshalAck(map[string]interface{}{
		"ok":   true,
		"room": req.Room,
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

// ─── 踢出成员 ───

func (h *Hub) OnRoomKick(s socketio.Conn, data string) {
	var req struct {
		Room           string `json:"room"`
		TargetIdentity string `json:"targetIdentity"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.TargetIdentity == "" {
		return
	}

	// 取踢人者身份并校验 signal:kick 权限；DB/缓存查询在锁外执行，避免持锁等 IO
	h.mu.Lock()
	room, exists := h.rooms[req.Room]
	if !exists {
		h.mu.Unlock()
		return
	}
	caller, ok := room.Members[s.ID()]
	if !ok {
		h.mu.Unlock()
		return
	}
	callerIdentity := caller.Identity
	h.mu.Unlock()

	callerUser, err := h.userStore.GetByName(callerIdentity)
	if err != nil || callerUser == nil {
		return
	}
	if h.permChecker == nil || !h.permChecker.HasPermission(callerUser.Role, permcode.PermSignalKick) {
		return
	}

	// 权限通过，重新加锁执行踢出
	h.mu.Lock()
	room, exists = h.rooms[req.Room]
	if !exists {
		h.mu.Unlock()
		return
	}
	var targetSocketID string
	roomDeleted := false
	for sid, m := range room.Members {
		if m.Identity == req.TargetIdentity {
			targetSocketID = sid
			delete(room.Members, sid)
			// 踢出最后一人也删房，与 OnRoomLeave / OnDisconnect 行为一致
			roomDeleted = h.deleteRoomIfEmpty(req.Room)
			break
		}
	}
	h.mu.Unlock()

	if targetSocketID == "" {
		return
	}

	log.Printf("[Signal] %s kicked %s from room: %s", s.ID(), req.TargetIdentity, req.Room)

	// 通知被踢者；payload 带 targetIdentity，前端按 identity 过滤避免误伤同房他人
	h.server.BroadcastToRoom("/", req.Room, EventRoomKicked, map[string]interface{}{
		"room":           req.Room,
		"targetIdentity": req.TargetIdentity,
	})

	// 通知全员成员离开
	h.server.BroadcastToNamespace("/", EventMemberLeft, map[string]interface{}{
		"room":     req.Room,
		"identity": req.TargetIdentity,
		"id":       targetSocketID,
	})

	// 从 SFU 移除 participant（不支持时优雅降级）
	h.removeParticipantSafe(req.Room, req.TargetIdentity)

	// 空房删除 SFU room 并广播列表，否则广播单房间更新
	if roomDeleted {
		h.deleteRoomSafe(req.Room)
		h.server.BroadcastToNamespace("/", EventRoomList, h.GetRooms())
	} else {
		h.mu.RLock()
		info := h.roomInfoLocked(req.Room)
		h.mu.RUnlock()
		h.server.BroadcastToNamespace("/", EventRoomUpdated, info)
	}
}

func (h *Hub) OnMemberMicState(s socketio.Conn, data string) {
	var req struct {
		Room       string `json:"room"`
		Identity   string `json:"identity"`
		IsMicMuted bool   `json:"isMicMuted"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.Identity == "" {
		return
	}

	h.mu.Lock()
	room, ok := h.rooms[req.Room]
	if !ok {
		h.mu.Unlock()
		return
	}
	caller := room.Members[s.ID()]
	if caller == nil || caller.Identity != req.Identity {
		h.mu.Unlock()
		return
	}
	if room.MicMuted == nil {
		room.MicMuted = make(map[string]bool)
	}
	room.MicMuted[req.Identity] = req.IsMicMuted
	h.mu.Unlock()

	h.BroadcastToRoom(req.Room, EventMemberUpdated, map[string]interface{}{
		"room":       req.Room,
		"identity":   req.Identity,
		"isMicMuted": req.IsMicMuted,
	})
}

// getMergedRooms 合并 DB 持久化房间 + 内存活跃房间（并行查询）。
func (h *Hub) getMergedRooms() []RoomInfo {
	var dbRooms map[string]RoomInfo
	var memRooms map[string]RoomInfo

	var eg errgroup.Group

	// goroutine 1: 从 DB 获取所有房间（最多 200 条）
	eg.Go(func() error {
		dbRooms = make(map[string]RoomInfo)
		if h.roomStore != nil {
			if rooms, _, err := h.roomStore.List(1, 200); err == nil {
				for _, r := range rooms {
					dbRooms[r.Name] = RoomInfo{
						ID:            r.ID,
						UUID:          r.UUID,
						Name:          r.Name,
						HasPassword:   r.Password != "",
						Description:   r.Description,
						Limit:         r.Limit,
						AudioOnly:     r.AudioOnly,
						AllowAudience: r.AllowAudience,
						Members:       []MemberInfo{},
						Count:         0,
						CreatedAt:     r.CreatedAt.UnixMilli(),
					}
				}
			}
		}
		return nil
	})

	// goroutine 2: 读取内存活跃房间
	eg.Go(func() error {
		memRooms = make(map[string]RoomInfo)
		h.mu.RLock()
		for name := range h.rooms {
			memRooms[name] = h.roomInfoLocked(name)
		}
		h.mu.RUnlock()
		return nil
	})

	eg.Wait()

	// 合并：内存活跃数据覆盖 DB 数据，同时保留 DB 字段
	for name, info := range memRooms {
		if existing, ok := dbRooms[name]; ok {
			info.ID = existing.ID
			info.UUID = existing.UUID
			info.Description = existing.Description
			info.Limit = existing.Limit
			info.AudioOnly = existing.AudioOnly
			info.AllowAudience = existing.AllowAudience
			info.CreatedAt = existing.CreatedAt
		}
		dbRooms[name] = info
	}

	result := make([]RoomInfo, 0, len(dbRooms))
	for _, r := range dbRooms {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// ─── 公共方法 ───

func (h *Hub) GetSFURooms() []RoomInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]RoomInfo, 0, len(h.rooms))
	for _, room := range h.rooms {
		result = append(result, h.roomInfoLocked(room.Name))
	}
	return result
}

// GetRooms returns DB-persisted rooms + in-memory active rooms merged.
func (h *Hub) GetRooms() []RoomInfo {
	return h.getMergedRooms()
}

// CheckRoomLimit 检查房间是否已满，返回 (已满, 限制人数, 当前人数, 错误)
func (h *Hub) CheckRoomLimit(roomName string) (bool, uint, int, error) {
	if h.roomStore == nil {
		return false, 0, 0, nil
	}
	dbRoom, err := h.roomStore.GetByName(roomName)
	if err != nil || dbRoom.Limit == 0 {
		return false, 0, 0, err
	}
	h.mu.RLock()
	var currentCount int
	if room, ok := h.rooms[roomName]; ok {
		currentCount = len(room.Members)
	}
	h.mu.RUnlock()
	return currentCount >= int(dbRoom.Limit), dbRoom.Limit, currentCount, nil
}

// CheckRoomPassword 检查房间密码，返回 (通过, 错误)
// ok=true 表示密码正确或房间无密码；err!=nil 表示房间不存在（视为无密码，允许创建）
func (h *Hub) CheckRoomPassword(roomName, password string) (ok bool, err error) {
	h.mu.RLock()
	room, exists := h.rooms[roomName]
	h.mu.RUnlock()
	if !exists {
		// 房间不在内存中（尚未创建或已销毁），不阻止
		return true, fmt.Errorf("room not found")
	}
	if room.Password == "" {
		return true, nil
	}
	if room.Password == password {
		return true, nil
	}
	return false, nil
}

func (h *Hub) GetRoomMembers(roomName string) []MemberInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.getMembersLocked(roomName)
}

// IsIdentityMuted 检查指定 identity（用户名）是否被禁言
func (h *Hub) IsIdentityMuted(identity string) (bool, *model.Mute, error) {
	if h.muteStore == nil {
		return false, nil, nil
	}
	return h.muteStore.IsMutedByIdentity(identity)
}

func (h *Hub) RegisterStream(stream string) {
	h.mu.Lock()
	h.activeStreams[stream] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) UnregisterStream(stream string) {
	h.mu.Lock()
	delete(h.activeStreams, stream)
	h.mu.Unlock()
}

func (h *Hub) IsStreamActive(stream string) bool {
	h.mu.RLock()
	_, ok := h.activeStreams[stream]
	h.mu.RUnlock()
	return ok
}

func (h *Hub) BroadcastToRoom(room string, event string, data interface{}) {
	if h.server != nil {
		h.server.BroadcastToRoom("/", room, event, data)
	}
}

// removeParticipantSafe 从 SFU 移除 participant。
// provider 不支持时（sfu.ErrNotSupported）静默降级，避免硬编码 provider 名判断。
func (h *Hub) removeParticipantSafe(room, identity string) {
	if h.sfuProvider == nil || identity == "" {
		return
	}
	if err := h.sfuProvider.RemoveParticipant(room, identity); err != nil {
		if errors.Is(err, pkg.ErrSFUNotSupported) {
			return
		}
		log.Printf("[Signal] failed to remove participant from SFU: %v", err)
	}
}

// deleteRoomSafe 删除 SFU room。provider 不支持时静默降级。
func (h *Hub) deleteRoomSafe(room string) {
	if h.sfuProvider == nil {
		return
	}
	if err := h.sfuProvider.DeleteRoom(room); err != nil {
		if errors.Is(err, pkg.ErrSFUNotSupported) {
			return
		}
		log.Printf("[Signal] failed to delete SFU room: %v", err)
	}
}

// BroadcastMute 广播禁言事件到所有客户端
func (h *Hub) BroadcastMute(userID uint, info *MuteInfo) {
	if h.server != nil {
		data := map[string]interface{}{
			"user_id":   userID,
			"permanent": info.Permanent,
			"reason":    info.Reason,
		}
		if !info.Permanent && info.ExpiresAt != "" {
			data["expires_at"] = info.ExpiresAt
		}
		h.server.BroadcastToNamespace("/", EventUserMuted, data)
	}
}

// BroadcastUnmute 广播取消禁言事件到所有客户端
func (h *Hub) BroadcastUnmute(userID uint) {
	if h.server != nil {
		h.server.BroadcastToNamespace("/", EventUserUnmuted, map[string]interface{}{
			"user_id": userID,
		})
	}
}

// ─── 内部辅助 ───

func (h *Hub) getMembersLocked(roomName string) []MemberInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		return nil
	}
	members := make([]MemberInfo, 0, len(room.Members))
	for _, m := range room.Members {
		info := *m
		if h.userStore != nil {
			if u, err := h.userStore.GetByName(m.Identity); err == nil && u != nil {
				info.Name = u.Name
				info.DisplayName = u.DisplayName
				info.Avatar = u.Avatar
			}
		}
		if h.muteStore != nil {
			if muted, _, _ := h.muteStore.IsMutedByIdentity(m.Identity); muted {
				info.IsMuted = true
			}
		}
		info.IsMicMuted = room.MicMuted[m.Identity]
		members = append(members, info)
	}
	return members
}

func (h *Hub) roomInfoLocked(roomName string) RoomInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		return RoomInfo{}
	}
	return RoomInfo{
		Name:        room.Name,
		HasPassword: room.Password != "",
		Members:     h.getMembersLocked(roomName),
		Count:       len(room.Members),
		CreatedAt:   room.CreatedAt.UnixMilli(),
	}
}

// duplicateIdentityLocked 判断同一房间是否已有其他 socket 占用 identity。调用方须持有 h.mu（读或写）。
func (h *Hub) duplicateIdentityLocked(roomName, identity, socketID string) bool {
	room, exists := h.rooms[roomName]
	if !exists {
		return false
	}
	for sid, m := range room.Members {
		if sid != socketID && m.Identity == identity {
			return true
		}
	}
	return false
}

// deleteRoomIfEmpty 在房间无成员时删除并返回 true。调用方须持有 h.mu（写）。
func (h *Hub) deleteRoomIfEmpty(roomName string) bool {
	room, exists := h.rooms[roomName]
	if !exists || len(room.Members) != 0 {
		return false
	}
	delete(h.rooms, roomName)
	return true
}

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
