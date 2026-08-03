package signal

import (
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/ws"
	"golang.org/x/sync/errgroup"
)

type Room struct {
	Name     string
	Password string
	Members  map[string]*MemberInfo // socketID -> MemberInfo
	// ByIdentity 身份索引，O(1) 查成员（本地 fallback 时为 O(1)，不再遍历 Members）。
	ByIdentity map[string]*MemberInfo // identity -> MemberInfo
	MicMuted   map[string]bool
	// Speaking 维护房间内各成员本地发言状态（仅 SRS/Cloudflare 等不 SFU 原生 active speaker 的 provider 上报）。
	// 与 MicMuted 类似，仅用于日志れ聚广播，不持久化。
	Speaking  map[string]bool
	CreatedAt time.Time
}

// roomSetMember writes both Members (by socketID) and ByIdentity index.
// Caller must hold h.mu write-lock.
func roomSetMember(room *Room, sid, identity string, member *MemberInfo) {
	room.Members[sid] = member
	if identity != "" && room.ByIdentity != nil {
		room.ByIdentity[identity] = member
	}
}

// roomDelMember removes from both Members and ByIdentity. Caller must hold write-lock.
func roomDelMember(room *Room, sid string) {
	if m, ok := room.Members[sid]; ok {
		if m.Identity != "" && room.ByIdentity != nil {
			delete(room.ByIdentity, m.Identity)
		}
	}
	delete(room.Members, sid)
}

// roomLookupIdentity O(1) identity lookup with index fallback.
func roomLookupIdentity(room *Room, identity string) *MemberInfo {
	if room.ByIdentity != nil {
		return room.ByIdentity[identity]
	}
	for _, m := range room.Members {
		if m.Identity == identity {
			return m
		}
	}
	return nil
}

// roomInitByIdentity lazy-inits ByIdentity from existing Members.
func roomInitByIdentity(room *Room) {
	if room.ByIdentity != nil {
		return
	}
	room.ByIdentity = make(map[string]*MemberInfo, len(room.Members))
	for _, m := range room.Members {
		if m.Identity != "" {
			room.ByIdentity[m.Identity] = m
		}
	}
}

// socketServer is replaced by ws.Broadcaster.

// eventBus is the narrow publish surface used by Hub for client fanout.
// *bus.NATSBus satisfies this without importing the bus package here.
type eventBus interface {
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

// cleanupPublisher enqueues SFU cleanup jobs (phase-3). Nil = inline goroutine.
type cleanupPublisher interface {
	PublishSFUCleanup(ctx context.Context, room, identity string, deleteRoom bool) error
}

// roomKey generates a domain-scoped composite key for room map isolation.
// Platform-level rooms (no DomainUUID) use a "platform:" prefix for backward compatibility.
func roomKey(domainUUID, roomName string) string {
	return pkg.RoomKey(domainUUID, roomName)
}

// splitRoomKey reverses roomKey to extract domainUUID and roomName.
func splitRoomKey(key string) (domainUUID, roomName string) {
	return pkg.SplitRoomKey(key)
}

func roomKeyMatchesDomain(key, domainUUID string) bool {
	if domainUUID == "" {
		return true
	}
	g, _ := splitRoomKey(key)
	return g == domainUUID
}

func domainRoomKey(domainUUID string) string {
	if domainUUID == "" {
		return "__platform"
	}
	return "__domain:" + domainUUID
}

func roomKeyIsPlatform(key string) bool {
	g, _ := splitRoomKey(key)
	return g == ""
}

// roomStore abstracts DB room listing for Hub.
type roomStore interface {
	List(page, pageSize int, roomType, domainUUID string) ([]model.Room, int64, error)
	GetByName(name string) (*model.Room, error)
	GetByDomainAndName(domainUUID, name string) (*model.Room, error)
}

// muteStore abstracts mute checking for Hub.
type muteStore interface {
	IsMutedByIdentity(identity string) (bool, *model.Mute, error)
}

// userStore abstracts user lookup for Hub.
type userStore interface {
	GetByName(name string) (*model.User, error)
	GetByID(id uint) (*model.User, error)
}

// permChecker 抽象权限校验，复用 service.PermissionService 的内存缓存。
type permChecker interface {
	HasPermission(roleName, permCode string) bool
}

// SFUSignalHandler 注册 provider 专属的 sfu:* 媒体协商事件。
type SFUSignalHandler interface {
	RegisterWS(register func(event string, fn func(ws.ClientMessenger, string) (string, error)))
}

// broadcastFn 供 provider 模块广播房间事件，无需依赖 Hub 具体类型。
type BroadcastFn func(room, event string, data interface{})

// StreamNameResolver 计算给定 room+identity 的预期 stream 名，用于服务端覆写客户端提报值。
type StreamNameResolver interface {
	StreamName(room, identity string) string
}

// ParticipantCleanupHandler 处理参与者离开时的 SFU 专属清理(如 mediasoup 广播 producer-closed + 关 transport)。
// 仅 mediasoup 实现;其它 provider 不实现此接口,Hub OnDisconnect 类型断言跳过。
type ParticipantCleanupHandler interface {
	OnParticipantLeft(room, identity string)
}

type connRoomSlots struct {
	TextRoom  string
	VoiceRoom string
}

// clearConnRoomSlot removes the matching room slot and drops the entry when empty.
func (h *Hub) clearConnRoomSlot(socketID, key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	slots := h.connSlots[socketID]
	if slots == nil {
		return
	}
	if slots.TextRoom == key {
		slots.TextRoom = ""
	}
	if slots.VoiceRoom == key {
		slots.VoiceRoom = ""
	}
	if slots.TextRoom == "" && slots.VoiceRoom == "" {
		delete(h.connSlots, socketID)
	}
}

type Hub struct {
	fanout           ws.Broadcaster
	eventBus         eventBus
	sfuProvider      sfu.Provider
	sfuProviderName  string
	streamResolver   StreamNameResolver
	rooms            map[string]*Room // roomName -> Room
	mu               sync.RWMutex
	roomStore        roomStore
	muteStore        muteStore
	userStore        userStore
	permChecker      permChecker
	sfuSignalHandler SFUSignalHandler
	activeStreams    map[string]struct{}
	// roomStreams 维护 room→stream 集合的聚合视图，供无原生 room 维度的 provider（SRS）查询。
	// 由 WS join/leave 与 SRS on_publish/on_unpublish 回调双源同步。
	roomStreams map[string]map[string]struct{}
	// streamRoomCache 维护 stream→room 反查，供 SRS callback（仅给 stream 名）反查 room 同步 roomStreams。
	streamRoomCache map[string]string
	// streamByIdentity 维护 room→identity→stream 映射，供 SRS RemoveParticipant
	// 按 identity 查实际登记的 stream，避免反算命名约定（命名函数变更后旧连接仍可查）。
	streamByIdentity   map[string]map[string]string
	participantCleanup ParticipantCleanupHandler
	membershipStore    membershipStore
	instanceID         string
	cleanupPub         cleanupPublisher
	stateNotifier      stateNotifier
	connSlots          map[string]*connRoomSlots // socketID -> slots
	msgSvc             messageSender
	convSvc            conversationSender
	domainChecker       func(domainUUID, userUUID string) bool
	clientDomains       map[string]string // socketID -> current domain scope (empty = platform)
}

func NewHub(store roomStore, mStore muteStore, uStore userStore, pChecker permChecker) *Hub {
	return &Hub{
		rooms:            make(map[string]*Room),
		roomStore:        store,
		muteStore:        mStore,
		userStore:        uStore,
		permChecker:      pChecker,
		activeStreams:    make(map[string]struct{}),
		roomStreams:      make(map[string]map[string]struct{}),
		streamRoomCache:  make(map[string]string),
		streamByIdentity: make(map[string]map[string]string),
		connSlots:        make(map[string]*connRoomSlots),
		clientDomains:     make(map[string]string),
	}
}

func (h *Hub) SetFanout(f ws.Broadcaster) {
	h.fanout = f
}

func (h *Hub) SetSFU(provider sfu.Provider) {
	h.sfuProvider = provider
}

func (h *Hub) SetSFUSignalHandler(handler SFUSignalHandler) {
	h.sfuSignalHandler = handler
	if ch, ok := handler.(ParticipantCleanupHandler); ok {
		h.participantCleanup = ch
	}
}

func (h *Hub) SetStreamResolver(r StreamNameResolver) {
	h.streamResolver = r
}

func (h *Hub) SetDomainChecker(checker func(domainUUID, userUUID string) bool) {
	h.domainChecker = checker
}

// ─── 事件注册 ───

// SetupFanout registers all signal event handlers with the WS HandlerRegistry.
func (h *Hub) SetupFanout(fanout ws.Broadcaster, handler *ws.HandlerRegistry) {
	h.SetFanout(fanout)

	handler.Handle(EventRoomCreate, h.OnRoomCreate)
	handler.HandleAck(EventRoomJoin, h.OnRoomJoin)
	handler.HandleAck(EventRoomJoinSFU, h.OnRoomJoinSFU)
	handler.HandleAck(EventRoomLeave, h.OnRoomLeave)
	handler.Handle(EventRoomList, h.OnRoomList)
	handler.Handle(EventRoomKick, h.OnRoomKick)
	handler.Handle(EventMemberMicState, h.OnMemberMicState)
	handler.Handle(EventMemberSpeaking, h.OnMemberSpeaking)
	handler.Handle(EventBotCommand, h.PublishBotCommand)
	handler.Handle(EventBotMessage, h.PublishBotMessage)
	handler.HandleAck(EventMessageSend, h.OnMessageSend)
	handler.HandleAck(EventMessageEdit, h.OnMessageEdit)
	handler.HandleAck(EventMessageDelete, h.OnMessageDelete)
	handler.HandleAck(EventMessageReact, h.OnMessageReact)
	handler.HandleAck(EventMessageUnreact, h.OnMessageUnreact)
	handler.HandleAck(EventPrivateSend, h.OnPrivateMessageSend)

	if h.sfuSignalHandler != nil {
		h.sfuSignalHandler.RegisterWS(handler.HandleAck)
	}
}

// claimsIdentity 从连接上下文读取 JWT 身份；未鉴权返回空。
func clientIdentity(c ws.ClientMessenger) string {
	if c == nil || c.Claims() == nil {
		return ""
	}
	return c.Claims().Username
}

// resolveIdentity 强制使用 JWT 身份，忽略客户端伪造的 identity。
func resolveIdentity(c ws.ClientMessenger, requested string) (string, error) {
	identity := clientIdentity(c)
	if identity == "" {
		return "", fmt.Errorf("unauthorized")
	}
	if requested != "" && requested != identity {
		return "", fmt.Errorf("identity mismatch")
	}
	return identity, nil
}

func (h *Hub) domainMemberAllowed(c ws.ClientMessenger, domainUUID string) bool {
	if domainUUID == "" {
		return true
	}
	if h.domainChecker == nil {
		return false
	}
	if c == nil || c.Claims() == nil {
		return false
	}
	userUUID := c.Claims().UserUUID
	if userUUID == "" {
		userUUID = c.Claims().Username
	}
	return h.domainChecker(domainUUID, userUUID)
}

func (h *Hub) setClientDomain(c ws.ClientMessenger, domainUUID string) {
	if c == nil {
		return
	}
	h.mu.Lock()
	prev := h.clientDomains[c.ID()]
	h.clientDomains[c.ID()] = domainUUID
	h.mu.Unlock()

	if h.fanout == nil {
		return
	}
	if prev != domainUUID {
		h.fanout.Leave(domainRoomKey(prev), c.ID())
	}
	h.fanout.Join(domainRoomKey(domainUUID), c.ID())
}

// ─── 连接/断开 ───

// OnConnect is called when a new WS client connects.
// JWT auth is handled by the Upgrader before the client reaches the Hub;
// this hook is for logging only. The connection is already authenticated
// and claims are available via c.Claims().
func (h *Hub) OnConnect(c ws.ClientMessenger) error {
	log.Printf("[Signal] client connected: %s", c.ID())
	// Join personal room for direct message delivery: __user:{identity}
	if identity := clientIdentity(c); identity != "" {
		if h.fanout != nil {
			h.fanout.Join("__user:"+identity, c.ID())
		}
	}
	return nil
}

func (h *Hub) OnDisconnect(c ws.ClientMessenger) {
	type disconnectCleanup struct {
		room        string
		identity    string
		deleted     bool
	}
	type leaveEvent struct {
		room     string
		identity string
		id       string
	}
	var cleanups []disconnectCleanup
	var updatedRooms []string
	var speakingChanged []string
	var deletedStreams []string
	var leaveEvents []leaveEvent
	// Leave personal room on disconnect
	if identity := clientIdentity(c); identity != "" {
		if h.fanout != nil {
			h.fanout.Leave("__user:"+identity, c.ID())
		}
	}
	h.mu.Lock()
	domainScope := h.clientDomains[c.ID()]
	delete(h.clientDomains, c.ID())
	for roomName, room := range h.rooms {
		if member, ok := room.Members[c.ID()]; ok {
			identity := member.Identity
			stream := member.Stream
			wasSpeaking := room.Speaking[identity]
			roomDelMember(room, c.ID())
			delete(room.Speaking, identity)
			delete(room.MicMuted, identity)
			h.unregisterStreamLocked(roomName, identity, stream)
			if stream != "" {
				deletedStreams = append(deletedStreams, stream)
			}
			if wasSpeaking {
				speakingChanged = append(speakingChanged, roomName)
			}
			leaveEvents = append(leaveEvents, leaveEvent{
				room:     roomName,
				identity: identity,
				id:       c.ID(),
			})
			updatedRooms = append(updatedRooms, roomName)
			deleted := h.deleteRoomIfEmptyLocked(roomName)
			cleanups = append(cleanups, disconnectCleanup{roomName, identity, deleted})
		}
	}
	delete(h.connSlots, c.ID())
	h.mu.Unlock()

	if h.fanout != nil {
		h.fanout.Leave(domainRoomKey(domainScope), c.ID())
	}

	// publish after unlock: avoid holding hub mu across NATS/WebSocket I/O
	for _, e := range leaveEvents {
		domainUUID, logicalName := splitRoomKey(e.room)
		h.publishRoom(e.room, EventMemberLeft, map[string]interface{}{
			"room":       logicalName,
			"domain_uuid": domainUUID,
			"identity":   e.identity,
			"id":         e.id,
		})
	}

	for _, name := range updatedRooms {
		h.syncRoomToStore(name)
	}
	for _, stream := range deletedStreams {
		h.syncStreamDelete(stream)
	}

	for _, name := range speakingChanged {
		domainUUID, logicalName := splitRoomKey(name)
		h.broadcastActiveSpeakers(domainUUID, logicalName)
	}

	// SFU 清理异步：RemoveParticipant/DeleteRoom 是 HTTP/gRPC 调用，可能慢。
	// 同步阻塞会拉长 OnDisconnect 持续时间，加剧连接 goroutine
	// 竞态（连接已 failed 时库 serveRead 状态错乱触发 gorilla panic）。
	// 丢后台 goroutine，handler 立即返回。
	if len(cleanups) > 0 {
		if h.cleanupPub != nil {
		for _, c := range cleanups {
				if err := h.cleanupPub.PublishSFUCleanup(context.Background(), c.room, c.identity, c.deleted); err != nil {
					log.Printf("[Signal] enqueue sfu cleanup: %v", err)
				}
			}
		} else {
			go func(cleanups []disconnectCleanup) {
				for _, c := range cleanups {
					h.removeParticipantSafe(c.room, c.identity)
					if c.deleted {
						h.deleteRoomSafe(c.room)
					}
					if h.participantCleanup != nil {
						h.participantCleanup.OnParticipantLeft(c.room, c.identity)
					}
				}
			}(cleanups)
		}
	}

	for _, name := range updatedRooms {
		h.mu.RLock()
		_, exists := h.rooms[name]
		h.mu.RUnlock()
		if !exists {
			// 房间已空被删除，广播房间列表更新（含 DB 持久化房间）
			domainUUID, _ := splitRoomKey(name)
			h.broadcastRoomList(domainUUID)
			continue
		}
		h.broadcastRoomUpdatedLocal(name)
	}

	log.Printf("[Signal] client disconnected: %s", c.ID())
}

// ─── 房间创建 ───

// OnError 兜底处理 WS 层 error（含 OnConnect 返回的 panic error）。
// 仅记录日志，不做断连等副作用——连接级错误由库自行处理。
func (h *Hub) OnError(c ws.ClientMessenger, err error) {
	if c == nil {
		log.Printf("[Signal] socket error: err=%v", err)
		return
	}
	log.Printf("[Signal] socket error: conn=%s err=%v", c.ID(), err)
}

func (h *Hub) OnRoomCreate(c ws.ClientMessenger, data string) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{
			"error": "room name is required",
		}})
		return
	}

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{
			"error": "not a member of this domain",
		}})
		return
	}
	h.setClientDomain(c, req.DomainUUID)

	h.mu.Lock()
	if _, exists := h.rooms[roomKey(req.DomainUUID, req.Room)]; exists {
		h.mu.Unlock()
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{
			"error": "room already exists",
		}})
		return
	}

	hashedPassword, hashErr := pkg.HashPassword(req.Password)
	if hashErr != nil {
		h.mu.Unlock()
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{"error": "invalid password"}})
		return
	}
	h.rooms[roomKey(req.DomainUUID, req.Room)] = &Room{
		Name:      req.Room,
		Password:  hashedPassword,
		Members:   make(map[string]*MemberInfo),
		MicMuted:  make(map[string]bool),
		Speaking:  make(map[string]bool),
		CreatedAt: time.Now(),
	}
	roomInfo := h.roomInfoLocked(roomKey(req.DomainUUID, req.Room))
	h.mu.Unlock()

	log.Printf("[Signal] room created: %s by %s", req.Room, c.ID())

	c.Send(map[string]interface{}{"event": EventRoomCreated, "data": roomInfo})
	h.broadcastRoomUpdatedLocal(roomKey(req.DomainUUID, req.Room))
}

// ─── 加入房间 ───
//
// 加入流程分两阶段：
//  1. OnRoomJoin     — 信令面。仅校验权限 + fanout room join，不写 h.rooms 成员。
//  2. OnRoomJoinSFU  — 媒体面。SFU 连接确认后写成员 + 广播 MemberJoined/RoomUpdated。
func (h *Hub) OnRoomJoin(c ws.ClientMessenger, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{
			"error": "room name is required",
		})
	}

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		return marshalAck(map[string]interface{}{
			"error": "not a member of this domain",
		})
	}
	h.setClientDomain(c, req.DomainUUID)

	identity, err := resolveIdentity(c, req.Identity)
	if err != nil {
		return marshalAck(map[string]interface{}{
			"error": err.Error(),
		})
	}
	req.Identity = identity

	// 密码校验（DB 为准）
	if ok, pwdErr := h.CheckRoomPassword(req.DomainUUID, req.Room, req.Password); !ok {
		if pwdErr != nil {
			return marshalAck(map[string]interface{}{
				"error": "room requires password",
			})
		}
		return marshalAck(map[string]interface{}{
			"error": "wrong room password",
		})
	}

	// 禁言检查（fail-closed）
	if h.muteStore != nil {
		muted, mute, muteErr := h.muteStore.IsMutedByIdentity(identity)
		if muteErr != nil {
			log.Printf("[signal] IsMutedByIdentity error: identity=%q err=%v", identity, muteErr)
			return marshalAck(map[string]interface{}{
				"error": "mute check failed",
			})
		}
		if muted {
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
	if full, limit, count, _ := h.CheckRoomLimit(req.DomainUUID, req.Room); full {
		return marshalAck(map[string]interface{}{
			"error": "room is full",
			"limit": limit,
			"count": count,
		})
	}

	// 提前拦截重复身份：已通过 OnRoomJoinSFU 写入 h.rooms 的在线用户，新 socket 在 OnRoomJoin 阶段即阻断。
	// 并发场景由 OnRoomJoinSFU 的 duplicate check 兜底，此处为提前拦截以节省媒体连接开销。
	h.mu.RLock()
	dup := h.duplicateIdentityLocked(roomKey(req.DomainUUID, req.Room), identity, c.ID())
	h.mu.RUnlock()
	if dup {
		return marshalAck(map[string]interface{}{
			"error":    "duplicate connection not allowed",
			"room":     req.Room,
			"identity": identity,
		})
	}

	// Dual-slot tracking: resolve room type, manage text/voice slot per conn.
	key := roomKey(req.DomainUUID, req.Room)
	h.mu.Lock()
	roomType := model.RoomTypeVoice
	if h.roomStore != nil {
		if dbRoom, dbErr := h.roomStore.GetByDomainAndName(req.DomainUUID, req.Room); dbErr == nil && dbRoom != nil {
			roomType = model.NormalizeRoomType(dbRoom.Type)
		}
	}
	slots := h.connSlots[c.ID()]
	if slots == nil {
		slots = &connRoomSlots{}
		h.connSlots[c.ID()] = slots
	}
	if roomType == model.RoomTypeText {
		// Switching text rooms: leave old slot
		if slots.TextRoom != "" && slots.TextRoom != key {
			h.fanout.Leave(slots.TextRoom, c.ID())
		}
		slots.TextRoom = key
	} else {
		// Switching voice rooms: leave old slot
		if slots.VoiceRoom != "" && slots.VoiceRoom != key {
			h.fanout.Leave(slots.VoiceRoom, c.ID())
		}
		slots.VoiceRoom = key
	}
	h.mu.Unlock()

	h.fanout.Join(key, c.ID())

	log.Printf("[Signal] %s (%s) signaling ready for room: %s (type=%s)", c.ID(), identity, req.Room, roomType)

	return marshalAck(map[string]interface{}{
		"room":     req.Room,
		"identity": identity,
	})
}

// OnRoomJoinSFU 在 SFU 媒体连接确认后写成员并广播加入。
func (h *Hub) OnRoomJoinSFU(c ws.ClientMessenger, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{"error": "room name is required"})
	}

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		return marshalAck(map[string]interface{}{
			"error": "not a member of this domain",
		})
	}
	h.setClientDomain(c, req.DomainUUID)

	identity, err := resolveIdentity(c, req.Identity)
	if err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	req.Identity = identity

	// Text room guard: text rooms have no media, reject SFU join.
	if h.roomStore != nil {
		if dbRoom, dbErr := h.roomStore.GetByDomainAndName(req.DomainUUID, req.Room); dbErr == nil && dbRoom != nil {
			if model.NormalizeRoomType(dbRoom.Type) == model.RoomTypeText {
				return marshalAck(map[string]interface{}{"error": "text room has no media"})
			}
		}
	}

	// phase2 复检 mute / limit / password，避免 phase1 通过后状态变化
	if ok, pwdErr := h.CheckRoomPassword(req.DomainUUID, req.Room, req.Password); !ok {
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		if pwdErr != nil {
			return marshalAck(map[string]interface{}{"error": "room requires password"})
		}
		return marshalAck(map[string]interface{}{"error": "wrong room password"})
	}
	if h.muteStore != nil {
		muted, _, muteErr := h.muteStore.IsMutedByIdentity(identity)
		if muteErr != nil {
			h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
			return marshalAck(map[string]interface{}{"error": "mute check failed"})
		}
		if muted {
			h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
			return marshalAck(map[string]interface{}{"error": "user is muted", "muted": true})
		}
	}
	if full, limit, count, _ := h.CheckRoomLimit(req.DomainUUID, req.Room); full {
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		return marshalAck(map[string]interface{}{"error": "room is full", "limit": limit, "count": count})
	}

	// 服务端覆写 stream：客户端提报值不可信，使用服务端基于 room+identity 计算的预期值
	if h.streamResolver != nil {
		req.Stream = h.streamResolver.StreamName(roomKey(req.DomainUUID, req.Room), identity)
	}

	member := &MemberInfo{
		ID:       c.ID(),
		Identity: identity,
		JoinedAt: time.Now().UnixMilli(),
		Stream:   req.Stream,
	}

	h.mu.Lock()
	// 重复身份检查：同一房间不允许同一 identity 用不同 socket 重复加入
	if h.duplicateIdentityLocked(roomKey(req.DomainUUID, req.Room), identity, c.ID()) {
		h.mu.Unlock()
		// 拒绝加入：回退 phase1 的 fanout.Join，避免 socket 残留在 WS room
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		return marshalAck(map[string]interface{}{
			"error":    "duplicate connection not allowed",
			"room":     req.Room,
			"identity": identity,
		})
	}
	room, exists := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !exists {
		room = &Room{
			Name:      req.Room,
			Members:   make(map[string]*MemberInfo),
			MicMuted:  make(map[string]bool),
			Speaking:  make(map[string]bool),
			CreatedAt: time.Now(),
		}
		h.rooms[roomKey(req.DomainUUID, req.Room)] = room
	}
	roomSetMember(room, c.ID(), identity, member)
	h.registerStreamLocked(roomKey(req.DomainUUID, req.Room), identity, req.Stream)
	memberList := h.memberSnapshotLocked(roomKey(req.DomainUUID, req.Room))
	h.mu.Unlock()
	memberList = h.enrichMembers(memberList)
	h.syncRoomToStore(roomKey(req.DomainUUID, req.Room))
	if req.Stream != "" {
		h.syncStreamPut(req.Stream, roomKey(req.DomainUUID, req.Room), identity)
	}

	log.Printf("[Signal] sfu confirmed: %s in %s", c.ID(), req.Room)

	h.mu.RLock()
	_, memberExists := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if memberExists {
		_, memberExists = h.rooms[roomKey(req.DomainUUID, req.Room)].Members[c.ID()]
	}
	h.mu.RUnlock()
	if !memberExists {
		log.Printf("[Signal] sfu join rejected: %s not in room %s", c.ID(), req.Room)
		// 回退 phase1 的 fanout.Join，避免 socket 残留在 WS room
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		return marshalAck(map[string]interface{}{
			"error": "not in room",
		})
	}

	h.publishRoom(roomKey(req.DomainUUID, req.Room), EventMemberJoined, map[string]interface{}{
		"room":       req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":   identity,
		"id":         c.ID(),
		"stream":     req.Stream,
	})

	h.broadcastRoomUpdatedLocal(roomKey(req.DomainUUID, req.Room))

	return marshalAck(map[string]interface{}{
		"ok":       true,
		"room":     req.Room,
		"identity": identity,
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

func (h *Hub) OnRoomLeave(c ws.ClientMessenger, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{
			"error": "room name is required",
		})
	}

	key := roomKey(req.DomainUUID, req.Room)
	h.fanout.Leave(key, c.ID())
	h.clearConnRoomSlot(c.ID(), key)

	var identity string
	var stream string
	wasSpeaking := false
	roomDeleted := false
	h.mu.Lock()
	if room, exists := h.rooms[key]; exists {
		if member, ok := room.Members[c.ID()]; ok {
			identity = member.Identity
			wasSpeaking = room.Speaking[identity]
			stream = member.Stream
			roomDelMember(room, c.ID())
			delete(room.Speaking, identity)
			delete(room.MicMuted, identity)
			h.unregisterStreamLocked(key, identity, stream)
			// 成员清空则删除房间，与 OnDisconnect 行为一致
			roomDeleted = h.deleteRoomIfEmptyLocked(key)
		}
	}
	h.mu.Unlock()
	h.syncRoomToStore(key)
	if stream != "" {
		h.syncStreamDelete(stream)
	}
	if wasSpeaking && !roomDeleted {
		h.broadcastActiveSpeakers(req.DomainUUID, req.Room)
	}

	// 同步清理 SFU 端：移除 participant，空房时删除 SFU room
	if identity != "" {
		h.removeParticipantSafe(key, identity)
		if h.participantCleanup != nil {
			h.participantCleanup.OnParticipantLeft(key, identity)
		}
	}
	if roomDeleted {
		h.deleteRoomSafe(key)
	}

	if identity == "" {
		identity = c.ID()
	}

	log.Printf("[Signal] %s (%s) left room: %s", c.ID(), identity, req.Room)

	c.Send(map[string]interface{}{"event": EventRoomLeft, "data": map[string]interface{}{
		"room":       req.Room,
		"domain_uuid": req.DomainUUID,
	}})

	h.publishRoom(key, EventMemberLeft, map[string]interface{}{
		"room":       req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":   identity,
		"id":         c.ID(),
	})

	// 房间已空被删除则广播房间列表，否则广播单房间更新（与 OnDisconnect 一致）
	if roomDeleted {
		h.broadcastRoomList(req.DomainUUID)
	} else {
		// NOTE: roomInfoLocked 在 !exists 时返回零值 RoomInfo（并发删房场景安全）
		h.broadcastRoomUpdatedLocal(key)
	}

	return marshalAck(map[string]interface{}{
		"ok":   true,
		"room": req.Room,
	})
}

// ─── 房间列表 ───

func (h *Hub) OnRoomList(c ws.ClientMessenger, data string) {
	var req struct {
		DomainUUID string `json:"domain_uuid"`
	}
	_ = parseJSON(data, &req)

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		c.Send(map[string]interface{}{"event": EventRoomListResult, "data": map[string]interface{}{
			"rooms": []RoomInfo{},
			"count": 0,
		}})
		return
	}
	h.setClientDomain(c, req.DomainUUID)

	var rooms []RoomInfo
	if req.DomainUUID == "" {
		rooms = h.getMergedPlatformRooms()
	} else {
		rooms = h.getMergedRooms(req.DomainUUID)
	}
	c.Send(map[string]interface{}{"event": EventRoomListResult, "data": map[string]interface{}{
		"rooms": rooms,
		"count": len(rooms),
	}})
}

// ─── 踢出成员 ───

func (h *Hub) OnRoomKick(c ws.ClientMessenger, data string) {
	var req struct {
		Room           string `json:"room"`
		DomainUUID      string `json:"domain_uuid,omitempty"`
		TargetIdentity string `json:"targetIdentity"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.TargetIdentity == "" {
		return
	}

	// 取踢人者身份并校验 signal:kick 权限；DB/缓存查询在锁外执行，避免持锁等 IO
	h.mu.Lock()
	room, exists := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !exists {
		h.mu.Unlock()
		return
	}
	caller, ok := room.Members[c.ID()]
	if !ok {
		h.mu.Unlock()
		return
	}
	callerIdentity := caller.Identity
	h.mu.Unlock()

	// 权限：优先用连接 JWT claims.Permissions（Bot token），否则回退 role 映射。
	claims := c.Claims()
	role := ""
	if claims != nil {
		role = claims.Role
	}
	if role == "" {
		if h.userStore == nil {
			return
		}
		callerUser, err := h.userStore.GetByName(callerIdentity)
		if err != nil || callerUser == nil {
			return
		}
		role = callerUser.Role
	}
	if !middleware.PermissionGranted(claims, role, permcode.PermSignalKick, h.permChecker) {
		return
	}

	// 保护管理员：仅非 bot 的 admin 可踢 admin。Bot 即使持有 signal:kick 也不能踢管理账号。
	if h.userStore != nil {
		targetUser, err := h.userStore.GetByName(req.TargetIdentity)
		if err == nil && targetUser != nil && targetUser.Role == "admin" {
			callerIsAdminHuman := role == "admin" && (claims == nil || len(claims.Permissions) == 0)
			if !callerIsAdminHuman {
				return
			}
		}
	}

	// 权限通过，重新加锁执行踢出
	key := roomKey(req.DomainUUID, req.Room)
	h.mu.Lock()
	room, exists = h.rooms[key]
	if !exists {
		h.mu.Unlock()
		return
	}
	var targetSocketID string
	var targetConn ws.ClientMessenger
	var targetStream string
	targetWasSpeaking := false
	roomDeleted := false
	for sid, m := range room.Members {
		if m.Identity == req.TargetIdentity {
			targetSocketID = sid
			targetStream = m.Stream
			targetWasSpeaking = room.Speaking[m.Identity]
			roomDelMember(room, sid)
			delete(room.Speaking, m.Identity)
			delete(room.MicMuted, m.Identity)
			h.unregisterStreamLocked(key, m.Identity, targetStream)
			// 踢出最后一人也删房，与 OnRoomLeave / OnDisconnect 行为一致
			roomDeleted = h.deleteRoomIfEmptyLocked(key)
			break
		}
	}
	h.mu.Unlock()

	if targetSocketID == "" {
		return
	}
	h.clearConnRoomSlot(targetSocketID, key)
	if targetStream != "" {
		h.syncStreamDelete(targetStream)
	}

	// 从 fanout room 移除目标连接，避免继续收事件
	if h.fanout != nil {
		h.fanout.ForEach(key, func(conn ws.ClientMessenger) bool {
			if conn != nil && conn.ID() == targetSocketID {
				targetConn = conn
				return false
			}
			return true
		})
		if targetConn != nil {
			h.fanout.Leave(key, targetConn.ID())
		}
	}

	log.Printf("[Signal] %s kicked %s from room: %s", c.ID(), req.TargetIdentity, req.Room)

	// 信令层踢人始终先生效；SFU 层按 Capabilities.ServerKick 尽力 hard-enforce。
	enforcement := h.removeParticipantSafe(key, req.TargetIdentity)

	// 通知被踢者；payload 带 targetIdentity，前端按 identity 过滤避免误伤同房他人
	h.publishRoom(key, EventRoomKicked, map[string]interface{}{
		"room":           req.Room,
		"domain_uuid":     req.DomainUUID,
		"targetIdentity": req.TargetIdentity,
		"enforcement":    enforcement,
	})
	if targetConn != nil {
		targetConn.Send(map[string]interface{}{"event": EventRoomKicked, "data": map[string]interface{}{
			"room":           req.Room,
			"domain_uuid":     req.DomainUUID,
			"targetIdentity": req.TargetIdentity,
			"enforcement":    enforcement,
		}})
	}

	// 通知全员成员离开
	h.publishRoom(key, EventMemberLeft, map[string]interface{}{
		"room":        req.Room,
		"domain_uuid":  req.DomainUUID,
		"identity":    req.TargetIdentity,
		"id":          targetSocketID,
		"enforcement": enforcement,
	})

	// SFU remove 已在上方 removeParticipantSafe 完成；此处同步房间状态并做 provider 清理。
	h.syncRoomToStore(key)
	if h.participantCleanup != nil {
		h.participantCleanup.OnParticipantLeft(key, req.TargetIdentity)
	}

	// 空房删除 SFU room 并广播列表，否则广播单房间更新
	if roomDeleted {
		h.deleteRoomSafe(key)
		h.broadcastRoomList(req.DomainUUID)
	} else {
		h.broadcastRoomUpdatedLocal(key)
		if targetWasSpeaking {
			h.broadcastActiveSpeakers(req.DomainUUID, req.Room)
		}
	}
}

func (h *Hub) OnMemberMicState(c ws.ClientMessenger, data string) {
	var req struct {
		Room       string `json:"room"`
		DomainUUID  string `json:"domain_uuid,omitempty"`
		Identity   string `json:"identity"`
		IsMicMuted bool   `json:"isMicMuted"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.Identity == "" {
		return
	}

	h.mu.Lock()
	room, ok := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !ok {
		h.mu.Unlock()
		return
	}
	caller := room.Members[c.ID()]
	if caller == nil || caller.Identity != req.Identity {
		h.mu.Unlock()
		return
	}
	if room.MicMuted == nil {
		room.MicMuted = make(map[string]bool)
	}
	room.MicMuted[req.Identity] = req.IsMicMuted
	h.mu.Unlock()

	h.BroadcastToRoom(roomKey(req.DomainUUID, req.Room), EventMemberUpdated, map[string]interface{}{
		"room":       req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":   req.Identity,
		"isMicMuted": req.IsMicMuted,
	})
}

// OnMemberSpeaking 接收成员本地发言态上报（SRS / Cloudflare 等无 SFU 原生
// active speaker 的 provider）。服务端按 room 聚合后广播 room:active-speakers。
// 仅持有本地麦克风的成员可上报自身状态，避免伪造他人发言态。
func (h *Hub) OnMemberSpeaking(c ws.ClientMessenger, data string) {
	var req struct {
		Room      string `json:"room"`
		DomainUUID string `json:"domain_uuid,omitempty"`
		Identity  string `json:"identity"`
		Speaking  bool   `json:"speaking"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.Identity == "" {
		return
	}

	// 先在校验锁内确认调用者是本人，再在锁外做禁言检查，避免 DB 查询阻塞信令。
	h.mu.RLock()
	room, ok := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !ok {
		h.mu.RUnlock()
		return
	}
	caller := room.Members[c.ID()]
	valid := caller != nil && caller.Identity == req.Identity
	h.mu.RUnlock()
	if !valid {
		return
	}

	// 与 join 一致做禁言检查（fail-closed）：被禁言或检查失败时忽略发言态上报。
	if h.muteStore != nil {
		muted, _, muteErr := h.muteStore.IsMutedByIdentity(req.Identity)
		if muteErr != nil {
			log.Printf("[signal] OnMemberSpeaking IsMutedByIdentity error: identity=%q err=%v", req.Identity, muteErr)
			return
		}
		if muted {
			return
		}
	}

	h.mu.Lock()
	room, ok = h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !ok {
		h.mu.Unlock()
		return
	}
	caller = room.Members[c.ID()]
	if caller == nil || caller.Identity != req.Identity {
		h.mu.Unlock()
		return
	}
	if room.Speaking == nil {
		room.Speaking = make(map[string]bool)
	}
	room.Speaking[req.Identity] = req.Speaking
	h.mu.Unlock()

	h.broadcastActiveSpeakers(req.DomainUUID, req.Room)
}

// computeActiveSpeakersLocked 在持锁状态下返回房间内正在发言的成员 identity 列表。
func (h *Hub) computeActiveSpeakersLocked(roomName string) []string {
	room, ok := h.rooms[roomName]
	if !ok || room.Speaking == nil {
		return nil
	}
	out := make([]string, 0)
	for identity, speaking := range room.Speaking {
		if speaking {
			out = append(out, identity)
		}
	}
	return out
}

// broadcastActiveSpeakers 向房间广播当前 active speakers 列表（用于无 SFU 原生
// active speaker 的 provider）。发言人清空（离开/断连）时也同样广播空列表以重置高亮。
func (h *Hub) broadcastActiveSpeakers(domainUUID, roomName string) {
	if h.fanout == nil {
		return
	}
	rk := roomKey(domainUUID, roomName)
	h.mu.RLock()
	identities := h.computeActiveSpeakersLocked(rk)
	h.mu.RUnlock()
	h.publishRoom(rk, EventRoomActiveSpeakers, map[string]interface{}{
		"room":       roomName,
		"domain_uuid": domainUUID,
		"identities": identities,
	})
}

// getMergedRooms 合并 DB 持久化房间 + 内存活跃房间（并行查询）。
func (h *Hub) getMergedRooms(domainUUID string) []RoomInfo {
	return h.getMergedRoomsScoped(domainUUID, false)
}

func (h *Hub) getMergedPlatformRooms() []RoomInfo {
	return h.getMergedRoomsScoped("", true)
}

func (h *Hub) getMergedRoomsScoped(domainUUID string, platformOnly bool) []RoomInfo {
	var dbRooms map[string]RoomInfo
	var memRooms map[string]RoomInfo

	var eg errgroup.Group

	// goroutine 1: 从 DB 获取所有房间（最多 200 条）
	eg.Go(func() error {
		dbRooms = make(map[string]RoomInfo)
		if h.roomStore != nil {
			if rooms, _, err := h.roomStore.List(1, 200, "", domainUUID); err == nil {
				for _, r := range rooms {
					if platformOnly && r.DomainUUID != "" {
						continue
					}
					key := roomKey(r.DomainUUID, r.Name)
					dbRooms[key] = RoomInfo{
						ID:            r.ID,
						UUID:          r.UUID,
						Name:          r.Name,
						DomainUUID:     r.DomainUUID,
						HasPassword:   r.Password != "",
						Description:   r.Description,
						Limit:         r.Limit,
						AudioOnly:     r.AudioOnly,
						AllowAudience: r.AllowAudience,
						Type:          r.Type,
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
			if !roomKeyMatchesDomain(name, domainUUID) {
				continue
			}
			if platformOnly && !roomKeyIsPlatform(name) {
				continue
			}
			memRooms[name] = h.roomInfoLocked(name)
		}
		h.mu.RUnlock()
		// 无 KV 合并时在锁外补全成员资料/禁言；有 KV 时后续 GetRoomMembersMerged 已含该补全。
		if h.membershipStore == nil {
			for name, info := range memRooms {
				info.Members = h.enrichMembers(info.Members)
				info.Count = len(info.Members)
				memRooms[name] = info
			}
		}
		return nil
	})

	eg.Wait()

	// 合并：内存活跃数据覆盖 DB 数据，同时保留 DB 字段
	for name, info := range memRooms {
		roomDomainUUID, _ := splitRoomKey(name)
		info.DomainUUID = roomDomainUUID
		if existing, ok := dbRooms[name]; ok {
			info.ID = existing.ID
			info.UUID = existing.UUID
			info.Description = existing.Description
			info.Limit = existing.Limit
			info.AudioOnly = existing.AudioOnly
			info.AllowAudience = existing.AllowAudience
			info.CreatedAt = existing.CreatedAt
		}
		// 有 KV 时补齐其他实例成员，修正 Count
		if h.membershipStore != nil {
			merged := h.GetRoomMembersMerged(name)
			info.Members = merged
			info.Count = len(merged)
		}
		dbRooms[name] = info
	}

	// KV 中仅远端存在的活跃房间也并入列表（跨实例可见）
	if h.membershipStore != nil {
		if names, err := h.membershipStore.ListRoomNames(context.Background()); err == nil {
			for _, name := range names {
				if name == "" {
					continue
				}
				if !roomKeyMatchesDomain(name, domainUUID) {
					continue
				}
				if platformOnly && !roomKeyIsPlatform(name) {
					continue
				}
				if _, ok := dbRooms[name]; ok {
					// already present; still refresh members/count from merge
					info := dbRooms[name]
					merged := h.GetRoomMembersMerged(name)
					info.Members = merged
					info.Count = len(merged)
					dbRooms[name] = info
					continue
				}
				dbRooms[name] = h.roomInfoMerged(name)
			}
		}
	}

	result := make([]RoomInfo, 0, len(dbRooms))
	for _, r := range dbRooms {
		if domainUUID != "" && r.DomainUUID != domainUUID {
			continue
		}
		if platformOnly && r.DomainUUID != "" {
			continue
		}
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

// HubStats 信令面实时统计，用于监控面板。
type HubStats struct {
	RoomCount        int `json:"room_count"`
	ParticipantCount int `json:"participant_count"`
	OnlineUserCount  int `json:"online_user_count"`
}

// GetStats 返回信令面房间数、参与者总数、去重在线用户数。
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	identities := make(map[string]struct{}, len(h.rooms))
	participants := 0
	for _, room := range h.rooms {
		participants += len(room.Members)
		for _, m := range room.Members {
			if m.Identity != "" {
				identities[m.Identity] = struct{}{}
			}
		}
	}
	return HubStats{
		RoomCount:        len(h.rooms),
		ParticipantCount: participants,
		OnlineUserCount:  len(identities),
	}
}

func (h *Hub) GetSFURooms() []RoomInfo {
	h.mu.RLock()
	result := make([]RoomInfo, 0, len(h.rooms))
	for key := range h.rooms {
		result = append(result, h.roomInfoLocked(key))
	}
	h.mu.RUnlock()
	for i := range result {
		result[i].Members = h.enrichMembers(result[i].Members)
		result[i].Count = len(result[i].Members)
	}
	return result
}

// GetRooms returns DB-persisted rooms + in-memory active rooms merged.
func (h *Hub) GetRooms() []RoomInfo {
	return h.getMergedRooms("")
}

// broadcastRoomList 向全员推送最新房间列表。
// 事件名必须是 room:list:result（与 OnRoomList 一致）；room:list 是客户端请求事件，前端不监听。
func (h *Hub) broadcastRoomList(domainUUID string) {
	if h.fanout == nil {
		return
	}
	var rooms []RoomInfo
	if domainUUID == "" {
		rooms = h.getMergedPlatformRooms()
	} else {
		rooms = h.getMergedRooms(domainUUID)
	}
	h.fanout.BroadcastToRoom(domainRoomKey(domainUUID), EventRoomListResult, map[string]interface{}{
		"rooms": rooms,
		"count": len(rooms),
	})
}

func (h *Hub) broadcastRoomListKnownDomains() {
	h.mu.RLock()
	seen := make(map[string]struct{}, len(h.clientDomains))
	for _, domainUUID := range h.clientDomains {
		seen[domainUUID] = struct{}{}
	}
	h.mu.RUnlock()

	for domainUUID := range seen {
		h.broadcastRoomList(domainUUID)
	}
}

// CheckRoomLimit 检查房间是否已满，返回 (已满, 限制人数, 当前人数, 错误)
func (h *Hub) CheckRoomLimit(domainUUID, roomName string) (bool, uint, int, error) {
	if h.roomStore == nil {
		return false, 0, 0, nil
	}
	dbRoom, err := h.roomStore.GetByDomainAndName(domainUUID, roomName)
	if err != nil || dbRoom.Limit == 0 {
		return false, 0, 0, err
	}
	currentCount := len(h.GetRoomMembersMerged(roomKey(domainUUID, roomName)))
	return currentCount >= int(dbRoom.Limit), dbRoom.Limit, currentCount, nil
}

// CheckRoomPassword 检查房间密码，返回 (通过, 错误)
// 密码以 DB 为准；内存房间仅作兜底。ok=true 表示通过；ok=false+err!=nil 表示需密码未提供；ok=false+err==nil 表示密码错误。
func (h *Hub) CheckRoomPassword(domainUUID, roomName, password string) (ok bool, err error) {
	expected := ""
	found := false

	if h.roomStore != nil {
		if dbRoom, dbErr := h.roomStore.GetByDomainAndName(domainUUID, roomName); dbErr == nil && dbRoom != nil {
			expected = dbRoom.Password
			found = true
		}
	}

	if !found {
		h.mu.RLock()
		room, exists := h.rooms[roomKey(domainUUID, roomName)]
		h.mu.RUnlock()
		if exists {
			expected = room.Password
			found = true
		}
	}

	// 房间尚未创建/不存在：允许后续创建流程，不因密码拦截
	if !found {
		return true, nil
	}
	if expected == "" {
		return true, nil
	}
	if password == "" {
		return false, fmt.Errorf("room requires password")
	}
	if pkg.VerifyPassword(expected, password) {
		return true, nil
	}
	return false, nil
}

func (h *Hub) GetRoomMembers(roomName string) []MemberInfo {
	return h.getMembers(roomName)
}

// IsIdentityMuted 检查指定 identity（用户名）是否被禁言
func (h *Hub) IsIdentityMuted(identity string) (bool, *model.Mute, error) {
	if h.muteStore == nil {
		return false, nil, nil
	}
	return h.muteStore.IsMutedByIdentity(identity)
}

// IsMuted 是 JoinPolicy 接口适配：仅返 bool/error，剥离 *model.Mute（pkg.JoinPolicy 不依赖 model）。
func (h *Hub) IsMuted(identity string) (bool, error) {
	muted, _, err := h.IsIdentityMuted(identity)
	return muted, err
}

// 编译期断言：Hub 实现 pkg.JoinPolicy，供 SFUService 经接口注入。
var _ pkg.JoinPolicy = (*Hub)(nil)

func (h *Hub) RegisterStream(stream string) {
	var room, identity string
	h.mu.Lock()
	h.activeStreams[stream] = struct{}{}
	// SRS callback 仅给 stream，反查 streamRoomCache 同步 roomStreams 聚合视图。
	if r, ok := h.streamRoomCache[stream]; ok {
		room = r
		if h.roomStreams[room] == nil {
			h.roomStreams[room] = make(map[string]struct{})
		}
		h.roomStreams[room][stream] = struct{}{}
		if byID := h.streamByIdentity[room]; byID != nil {
			for id, st := range byID {
				if st == stream {
					identity = id
					break
				}
			}
		}
	}
	h.mu.Unlock()
	if room != "" {
		h.syncStreamPut(stream, room, identity)
	}
}

func (h *Hub) UnregisterStream(stream string) {
	h.mu.Lock()
	delete(h.activeStreams, stream)
	if room, ok := h.streamRoomCache[stream]; ok {
		if streams, ok := h.roomStreams[room]; ok {
			delete(streams, stream)
			if len(streams) == 0 {
				delete(h.roomStreams, room)
			}
		}
	}
	h.mu.Unlock()
	h.syncStreamDelete(stream)
}

func (h *Hub) IsStreamActive(stream string) bool {
	h.mu.RLock()
	_, ok := h.activeStreams[stream]
	h.mu.RUnlock()
	if ok {
		return true
	}
	if stream == "" || h.membershipStore == nil {
		return false
	}
	room, _, err := h.membershipStore.GetStream(context.Background(), stream)
	return err == nil && room != ""
}

func (h *Hub) RoomForStream(stream string) (string, bool) {
	h.mu.RLock()
	room, ok := h.streamRoomCache[stream]
	h.mu.RUnlock()
	if ok {
		return room, true
	}
	if stream == "" || h.membershipStore == nil {
		return "", false
	}
	room, _, err := h.membershipStore.GetStream(context.Background(), stream)
	if err != nil || room == "" {
		return "", false
	}
	return room, true
}

func (h *Hub) SetEventBus(b eventBus) {
	h.eventBus = b
}

func (h *Hub) SetCleanupPublisher(p cleanupPublisher) {
	h.cleanupPub = p
}

func (h *Hub) SetMessageService(svc messageSender) {
	h.msgSvc = svc
}

func (h *Hub) SetConversationService(svc conversationSender) {
	h.convSvc = svc
}

// IsRoomMember checks if identity is currently in room's member list.
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描同后缀 domain 房。
func (h *Hub) IsRoomMember(room, identity string) bool {
	// 1. 优先读 KV（无锁，跨实例可见）
	if h.membershipStore != nil {
		if snap, err := h.membershipStore.GetRoomMembers(context.Background(), room); err == nil {
			for _, m := range snap.Members {
				if m.Identity == identity {
					return true
				}
			}
		}
	}

	// 2. KV 未命中 → fallback 本地 map
	h.mu.RLock()
	defer h.mu.RUnlock()
	if r, ok := h.rooms[room]; ok {
		roomInitByIdentity(r)
		return roomLookupIdentity(r, identity) != nil
	}
	// 纯逻辑名回退扫描
	if !strings.Contains(room, ":") {
		for rk, r := range h.rooms {
			if _, rn := splitRoomKey(rk); rn == room {
				roomInitByIdentity(r)
				return roomLookupIdentity(r, identity) != nil
			}
		}
	}
	return false
}

func (h *Hub) publishRoom(room, event string, data interface{}) {
	// eventBus 由启动路径初始化（embedded NATS / external NATS），生产环境始终在线。
	// 下面的 BroadcastToRoom 直调仅在 eventBus 为 nil 时使用（仅测试机的裸 WS 路径）。
	if h.eventBus != nil {
		if err := h.eventBus.PublishRoom(context.Background(), room, event, data); err != nil {
			log.Printf("[Signal] eventbus publish room %s %s: %v", room, event, err)
		}
		return
	}
	if h.fanout != nil {
		h.fanout.BroadcastToRoom(room, event, data)
	}
}

func (h *Hub) publishNamespace(event string, data interface{}) {
	if h.eventBus != nil {
		if err := h.eventBus.PublishNamespace(context.Background(), event, data); err != nil {
			log.Printf("[Signal] eventbus publish ns %s: %v", event, err)
		}
		return
	}
	if h.fanout != nil {
		h.fanout.BroadcastToNamespace(event, data)
	}
}

func (h *Hub) BroadcastToRoom(room string, event string, data interface{}) {
	h.publishRoom(room, event, data)
}

// removeParticipantSafe 从 SFU 移除 participant。
// 信令层踢人始终先生效；这里只负责媒体层 enforcement。
// 返回 hard|degraded|soft，供事件 payload 透出。
func (h *Hub) removeParticipantSafe(room, identity string) string {
	if h.sfuProvider == nil || identity == "" {
		return sfu.EnforcementSoft
	}
	caps := h.sfuProvider.Capabilities()
	if !sfu.LevelEnabled(caps.KickLevel) {
		return sfu.EnforcementSoft
	}
	if err := h.sfuProvider.RemoveParticipant(room, identity); err != nil {
		if errors.Is(err, pkg.ErrSFUNotSupported) {
			log.Printf("[Signal] SFU kick unsupported, soft fallback room=%s identity=%s", room, identity)
			return sfu.EnforcementSoft
		}
		log.Printf("[Signal] failed to remove participant from SFU (soft fallback): %v", err)
		return sfu.EnforcementSoft
	}
	return sfu.EnforcementFromLevel(caps.KickLevel)
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

// CleanupParticipant is used by async job consumers for SFU-side cleanup.
// identity empty + deleteRoom cleans the SFU room only.
func (h *Hub) CleanupParticipant(room, identity string, deleteRoom bool) {
	if identity != "" {
		h.removeParticipantSafe(room, identity)
		if h.participantCleanup != nil {
			h.participantCleanup.OnParticipantLeft(room, identity)
		}
	}
	if deleteRoom {
		h.deleteRoomSafe(room)
	}
}

// ForceSFUProviderSwitch SFU 热切换：广播事件 → 清信令房间 → 尽力清理 SFU 侧。
// 客户端收到后强制断连并刷新页面。对端实例通过 HandleRemoteEvent 执行同等清理。
func (h *Hub) ForceSFUProviderSwitch(provider string) {
	h.publishNamespace(EventSFUProviderChanged, map[string]interface{}{
		"provider": provider,
	})
	h.clearLocalRoomsForSFUSwitch()
	log.Printf("[Signal] SFU provider switched to %s, forced all sessions offline", provider)
}

// OnDomainDelete 清理指定 Domain 下所有信令房间（房间键前缀匹配）。
// - 断开所有在线成员连接
// - 删除本地 rooms map 中的条目
// - 调用 SFU Provider 清理实际房间
func (h *Hub) OnDomainDelete(domainUUID string) {
	if domainUUID == "" {
		return
	}
	prefix := domainUUID + ":"

	var sfuRooms []string
	var roomKeys []string
	var deletedStreams []string
	var closedClients []ws.ClientMessenger
	h.mu.Lock()
	for key, room := range h.rooms {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		// 收集 SFU 房间名（复合 key 与 token/媒体层一致）
		sfuRooms = append(sfuRooms, key)
		// 断开所有成员
		for sid := range room.Members {
			if m := room.Members[sid]; m != nil {
				h.unregisterStreamLocked(key, m.Identity, m.Stream)
				if m.Stream != "" {
					deletedStreams = append(deletedStreams, m.Stream)
				}
			}
			delete(h.connSlots, sid)
			if client := h.fanout.GetClient(sid); client != nil {
				// Fanout.GetClient 对已删除 key 会返回带类型的 nil 指针；该 socket 已断开，跳过避免误 Close。
				if typed, ok := client.(*ws.Client); ok && typed == nil {
					continue
				}
				closedClients = append(closedClients, client)
			}
			h.fanout.Remove(sid)
		}
		delete(h.rooms, key)
		roomKeys = append(roomKeys, key)
	}
	h.mu.Unlock()

	// 在锁外关闭成员连接，避免 Close 阻塞拖住所有信令。
	closed := make(map[string]struct{}, len(closedClients))
	for _, client := range closedClients {
		if _, ok := closed[client.ID()]; ok {
			continue
		}
		closed[client.ID()] = struct{}{}
		client.Close()
	}

	// 清理 SFU 侧
	for _, room := range sfuRooms {
		h.deleteRoomSafe(room)
	}
	// 清理共享状态和 stream 映射
	for _, key := range roomKeys {
		if h.membershipStore != nil {
			_ = h.membershipStore.DeleteRoomMembers(context.Background(), key)
			h.notifyRoomStateChanged(key)
		}
	}
	for _, stream := range deletedStreams {
		if stream != "" {
			h.syncStreamDelete(stream)
		}
	}
}

// HandleRemoteEvent 处理来自其他实例的控制面事件。
// 当前仅对 sfu:provider-changed 做本机房间清理；其余事件已由 EventBus 投递到本地 WebSocket。
func (h *Hub) HandleRemoteEvent(event string, payload interface{}) {
	switch event {
	case EventSFUProviderChanged:
		provider := ""
		switch p := payload.(type) {
		case map[string]interface{}:
			if v, ok := p["provider"].(string); ok {
				provider = v
			}
		}
		log.Printf("[Signal] remote SFU provider switch received: %s", provider)
		h.clearLocalRoomsForSFUSwitch()
	case EventStateRoomChanged:
		room := ""
		switch p := payload.(type) {
		case map[string]interface{}:
			if v, ok := p["room"].(string); ok {
				room = v
			}
		}
		h.ApplyRemoteRoomState(room)
	default:
		// other events already delivered to local WebSocket by EventBus
	}
}

// clearLocalRoomsForSFUSwitch 清理本机信令房间与 stream 视图（不重复广播 provider-changed）。
func (h *Hub) clearLocalRoomsForSFUSwitch() {
	type cleanupItem struct {
		room        string
		identity    string
	}
	var cleanups []cleanupItem
	var roomsToDelete []string
	var streamsToDelete []string

	h.mu.Lock()
	for roomName, room := range h.rooms {
		for sid, m := range room.Members {
			identity := m.Identity
			stream := m.Stream
			cleanups = append(cleanups, cleanupItem{room: roomName, identity: identity})
			if stream != "" {
				streamsToDelete = append(streamsToDelete, stream)
			}
			h.unregisterStreamLocked(roomName, identity, stream)
			roomDelMember(room, sid)
			delete(room.Speaking, identity)
			delete(room.MicMuted, identity)
			if h.fanout != nil {
				h.fanout.ForEach(roomName, func(conn ws.ClientMessenger) bool {
					if conn != nil && conn.ID() == sid {
						h.fanout.Leave(roomName, conn.ID())
					}
					return true
				})
			}
		}
		roomsToDelete = append(roomsToDelete, roomName)
	}
	for _, roomName := range roomsToDelete {
		delete(h.rooms, roomName)
	}
	h.activeStreams = make(map[string]struct{})
	h.roomStreams = make(map[string]map[string]struct{})
	h.streamRoomCache = make(map[string]string)
	h.streamByIdentity = make(map[string]map[string]string)
	h.mu.Unlock()

	for _, stream := range streamsToDelete {
		h.syncStreamDelete(stream)
	}
	for _, roomName := range roomsToDelete {
		h.syncRoomToStore(roomName)
	}
	for _, c := range cleanups {
		h.removeParticipantSafe(c.room, c.identity)
		if h.participantCleanup != nil {
			h.participantCleanup.OnParticipantLeft(c.room, c.identity)
		}
	}
	for _, roomName := range roomsToDelete {
		h.deleteRoomSafe(roomName)
	}
	h.broadcastRoomListKnownDomains()
}

// BroadcastMute 广播禁言事件到所有客户端。
// 禁言策略始终以信令/业务层为准（始终 soft 可达）；若当前 SFU 支持 ServerMute，
// 再尽力对在线房间做媒体 hard/degraded mute，并在事件里标注 enforcement。
func (h *Hub) BroadcastMute(userID uint, info *MuteInfo) {
	ttlSeconds := 0
	if info != nil {
		if info.Permanent {
			ttlSeconds = sfu.PermanentMuteTTLSeconds
		} else if info.Duration > 0 {
			ttlSeconds = int(info.Duration)
		}
	}
	enforcement := h.enforceUserMediaMute(userID, true, ttlSeconds)
	data := map[string]interface{}{
		"user_id":     userID,
		"permanent":   info.Permanent,
		"reason":      info.Reason,
		"enforcement": enforcement,
	}
	if info != nil {
		if !info.Permanent && info.ExpiresAt != "" {
			data["expires_at"] = info.ExpiresAt
		}
		if info.Duration > 0 {
			data["duration"] = info.Duration
		}
	}
	h.publishNamespace(EventUserMuted, data)
}

// BroadcastUnmute 广播取消禁言事件到所有客户端。
func (h *Hub) BroadcastUnmute(userID uint) {
	enforcement := h.enforceUserMediaMute(userID, false, 0)
	h.publishNamespace(EventUserUnmuted, map[string]interface{}{
		"user_id":     userID,
		"enforcement": enforcement,
	})
}

// enforceUserMediaMute 在支持 ServerMute 的 provider 上，对 userID 当前所在房间做媒体层 mute/unmute。
// 返回 hard|degraded|soft。多房间必须全部成功才返回非 soft。
func (h *Hub) enforceUserMediaMute(userID uint, muted bool, ttlSeconds int) string {
	if h.sfuProvider == nil {
		return sfu.EnforcementSoft
	}
	caps := h.sfuProvider.Capabilities()
	if !sfu.LevelEnabled(caps.MuteLevel) {
		return sfu.EnforcementSoft
	}

	identity := h.identityForUserID(userID)
	if identity == "" {
		log.Printf("[Signal] mute media skip: cannot resolve identity for userID=%d", userID)
		return sfu.EnforcementSoft
	}

	type target struct {
		room        string // composite key (domainUUID:roomName)
		identity    string
	}
	seen := make(map[string]struct{})
	targets := make([]target, 0)
	h.mu.RLock()
	for roomName, room := range h.rooms {
		for _, member := range room.Members {
			if member.Identity == identity {
				if _, ok := seen[roomName]; !ok {
					seen[roomName] = struct{}{}
					targets = append(targets, target{room: roomName, identity: identity})
				}
				break
			}
		}
	}
	h.mu.RUnlock()
	// Multi-instance: user may only be online on another process; still media-enforce via SFU APIs.
	if h.membershipStore != nil {
		if names, err := h.membershipStore.ListRoomNames(context.Background()); err == nil {
			for _, roomName := range names {
				if roomName == "" {
					continue
				}
				if _, ok := seen[roomName]; ok {
					continue
				}
				snap, err := h.membershipStore.GetRoomMembers(context.Background(), roomName)
				if err != nil {
					continue
				}
				for _, m := range snap.Members {
					if m.Identity == identity {
						seen[roomName] = struct{}{}
						targets = append(targets, target{room: roomName, identity: identity})
						break
					}
				}
			}
		}
	}
	if len(targets) == 0 {
		// No online media target: policy still soft-enforced; not a media hard success.
		return sfu.EnforcementSoft
	}

	success := 0
	for _, t := range targets {
		var err error
		if tp, ok := h.sfuProvider.(sfu.TimedMuteProvider); ok {
			err = tp.MuteParticipantTimed(t.room, t.identity, "", muted, ttlSeconds)
		} else {
			err = h.sfuProvider.MuteParticipant(t.room, t.identity, "", muted)
		}
		if err != nil {
			if errors.Is(err, pkg.ErrSFUNotSupported) {
				log.Printf("[Signal] SFU mute unsupported, soft fallback room=%s identity=%s", t.room, t.identity)
			} else {
				log.Printf("[Signal] failed SFU mute (soft fallback) room=%s identity=%s err=%v", t.room, t.identity, err)
			}
			continue
		}
		success++
	}
	if success == 0 {
		return sfu.EnforcementSoft
	}
	if success < len(targets) {
		// Partial multi-room success: do not overclaim full hard/degraded.
		log.Printf("[Signal] partial SFU mute success %d/%d identity=%s", success, len(targets), identity)
		return sfu.EnforcementSoft
	}
	return sfu.EnforcementFromLevel(caps.MuteLevel)
}

// identityForUserID resolves username from userStore by numeric user id.
func (h *Hub) identityForUserID(userID uint) string {
	if h.userStore == nil {
		return ""
	}
	user, err := h.userStore.GetByID(userID)
	if err != nil || user == nil {
		log.Printf("[Signal] identityForUserID failed userID=%d err=%v", userID, err)
		return ""
	}
	return user.Name
}

// ─── 内部辅助 ───

// memberSnapshotLocked 仅拷贝房间成员快照与本地麦克风状态，不做任何 IO。
// 调用方须持有 h.mu（读或写）；用户资料与禁言状态由 enrichMembers 在锁外补齐。
func (h *Hub) memberSnapshotLocked(roomName string) []MemberInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		return nil
	}
	members := make([]MemberInfo, 0, len(room.Members))
	for _, m := range room.Members {
		info := *m
		info.IsMicMuted = room.MicMuted[m.Identity]
		members = append(members, info)
	}
	return members
}

// enrichMembers 在 h.mu 锁外为成员快照补全用户资料与禁言状态（DB/缓存 IO）。
// 失败按原逻辑降级：查不到用户时保留成员自身字段，禁言查询失败按未禁言处理。
func (h *Hub) enrichMembers(members []MemberInfo) []MemberInfo {
	if members == nil {
		return nil
	}
	out := make([]MemberInfo, len(members))
	copy(out, members)
	for i := range out {
		m := &out[i]
		if h.userStore != nil {
			if u, err := h.userStore.GetByName(m.Identity); err == nil && u != nil {
				m.Name = u.Name
				m.DisplayName = u.DisplayName
				m.Avatar = u.Avatar
			}
		}
		if h.muteStore != nil {
			if muted, _, _ := h.muteStore.IsMutedByIdentity(m.Identity); muted {
				m.IsMuted = true
			}
		}
	}
	return out
}

// getMembers 返回房间成员完整信息；锁内只做快照，锁外补全 IO，避免大房间阻塞全部信令。
func (h *Hub) getMembers(roomName string) []MemberInfo {
	h.mu.RLock()
	members := h.memberSnapshotLocked(roomName)
	h.mu.RUnlock()
	return h.enrichMembers(members)
}

func (h *Hub) roomInfoLocked(roomName string) RoomInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		domainUUID, _ := splitRoomKey(roomName)
		return RoomInfo{Name: roomName, DomainUUID: domainUUID, Members: []MemberInfo{}, Count: 0}
	}
	return RoomInfo{
		Name:        room.Name,
		DomainUUID:   func() string { g, _ := splitRoomKey(roomName); return g }(),
		HasPassword: room.Password != "",
		Members:     h.memberSnapshotLocked(roomName),
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

// deleteRoomIfEmptyLocked 在房间无成员时删除并返回 true。调用方须持有 h.mu（写）。
func (h *Hub) deleteRoomIfEmptyLocked(roomName string) bool {
	room, exists := h.rooms[roomName]
	if !exists || len(room.Members) != 0 {
		return false
	}
	delete(h.rooms, roomName)
	return true
}

// registerStreamLocked 在 WS join 时登记 stream→room 反查 + room→streams 聚合 + identity→stream 映射。
// 调用方须持有 h.mu（写）。stream 为空则跳过（provider 无 stream 概念）。
func (h *Hub) registerStreamLocked(room, identity, stream string) {
	if stream == "" {
		return
	}
	h.streamRoomCache[stream] = room
	if h.roomStreams[room] == nil {
		h.roomStreams[room] = make(map[string]struct{})
	}
	h.roomStreams[room][stream] = struct{}{}
	if identity != "" {
		if h.streamByIdentity[room] == nil {
			h.streamByIdentity[room] = make(map[string]string)
		}
		h.streamByIdentity[room][identity] = stream
	}
}

// unregisterStreamLocked 在 WS leave/disconnect 时清除 stream 映射 + identity→stream 映射。
// 调用方须持有 h.mu（写）。stream 为空则跳过。
func (h *Hub) unregisterStreamLocked(room, identity, stream string) {
	if stream == "" {
		return
	}
	delete(h.streamRoomCache, stream)
	if streams, ok := h.roomStreams[room]; ok {
		delete(streams, stream)
		if len(streams) == 0 {
			delete(h.roomStreams, room)
		}
	}
	if identity != "" {
		if identities, ok := h.streamByIdentity[room]; ok {
			delete(identities, identity)
			if len(identities) == 0 {
				delete(h.streamByIdentity, room)
			}
		}
	}
}

// Rooms 返回当前有活跃 stream 的 room 列表（RoomRegistry 实现）。
// 本地 roomStreams 优先；membership KV 中带 Stream 的远端房间一并并入。
func (h *Hub) Rooms() []string {
	h.mu.RLock()
	seen := make(map[string]struct{}, len(h.roomStreams))
	out := make([]string, 0, len(h.roomStreams))
	for room := range h.roomStreams {
		if room == "" {
			continue
		}
		seen[room] = struct{}{}
		out = append(out, room)
	}
	h.mu.RUnlock()
	if h.membershipStore != nil {
		if names, err := h.membershipStore.ListRoomNames(context.Background()); err == nil {
			for _, room := range names {
				if room == "" {
					continue
				}
				if _, ok := seen[room]; ok {
					continue
				}
				snap, err := h.membershipStore.GetRoomMembers(context.Background(), room)
				if err != nil {
					continue
				}
				hasStream := false
				for _, m := range snap.Members {
					if m.Stream != "" {
						hasStream = true
						break
					}
				}
				if !hasStream {
					continue
				}
				seen[room] = struct{}{}
				out = append(out, room)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Streams 返回指定 room 下登记的 stream 名集合（RoomRegistry 实现）。
// 本地 cache 优先，再并入 membership KV 中远端成员的 stream。
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描同后缀 domain 房。
func (h *Hub) Streams(room string) []string {
	h.mu.RLock()
	seen := make(map[string]struct{})
	out := make([]string, 0)
	if !strings.Contains(room, ":") {
		// 纯逻辑名：扫描所有同后缀的 domain 房 + 平台房
		for rk, streams := range h.roomStreams {
			if _, r := splitRoomKey(rk); r == room || rk == room {
				for s := range streams {
					if s == "" {
						continue
					}
					if _, exists := seen[s]; !exists {
						seen[s] = struct{}{}
						out = append(out, s)
					}
				}
			}
		}
	} else {
		// 复合键：精确匹配
		if rstreams, ok := h.roomStreams[room]; ok {
			for s := range rstreams {
				if s == "" {
					continue
				}
				if _, exists := seen[s]; !exists {
					seen[s] = struct{}{}
					out = append(out, s)
				}
			}
		}
	}
	h.mu.RUnlock()
	if h.membershipStore != nil {
		if snap, err := h.membershipStore.GetRoomMembers(context.Background(), room); err == nil {
			for _, m := range snap.Members {
				if m.Stream == "" {
					continue
				}
				if _, exists := seen[m.Stream]; !exists {
					seen[m.Stream] = struct{}{}
					out = append(out, m.Stream)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ClearRoom 清除指定 room 的 stream 聚合登记（RoomRegistry 实现）。
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描匹配所有同后缀的 domain 房。
func (h *Hub) ClearRoom(room string) {
	streams := make([]string, 0)
	h.mu.Lock()
	if !strings.Contains(room, ":") {
		// 纯逻辑名：清除所有同后缀的 domain 房 stream 登记
		for rk, local := range h.roomStreams {
			if _, rn := splitRoomKey(rk); rn == room || rk == room {
				for s := range local {
					streams = append(streams, s)
					delete(h.streamRoomCache, s)
				}
				delete(h.roomStreams, rk)
			}
		}
		for rk := range h.streamByIdentity {
			if _, r := splitRoomKey(rk); r == room || rk == room {
				delete(h.streamByIdentity, rk)
			}
		}
	} else {
		// 复合键：精确匹配
		if local, ok := h.roomStreams[room]; ok {
			for s := range local {
				streams = append(streams, s)
				delete(h.streamRoomCache, s)
			}
			delete(h.roomStreams, room)
		}
		delete(h.streamByIdentity, room)
	}
	h.mu.Unlock()
	for _, s := range streams {
		if s != "" {
			h.syncStreamDelete(s)
		}
	}
	h.syncRoomToStore(room)
}

// StreamForIdentity 返回 join 时按 identity 实际登记的 stream 名（RoomRegistry 实现）。
// 未登记返 ok=false，调用方（SRS RemoveParticipant）降级反算命名约定保持兼容。
func (h *Hub) StreamForIdentity(room, identity string) (string, bool) {
	h.mu.RLock()
	if identities, ok := h.streamByIdentity[room]; ok {
		if s, ok := identities[identity]; ok && s != "" {
			h.mu.RUnlock()
			return s, true
		}
	}
	h.mu.RUnlock()
	if room == "" || identity == "" || h.membershipStore == nil {
		return "", false
	}
	snap, err := h.membershipStore.GetRoomMembers(context.Background(), room)
	if err != nil {
		return "", false
	}
	for _, m := range snap.Members {
		if m.Identity == identity && m.Stream != "" {
			return m.Stream, true
		}
	}
	return "", false
}

// IdentityForStream 返回登记该 stream 的 identity（RoomRegistry 实现）。
// 未登记返 ok=false，调用方（SRS ListParticipants）可降级返回 client id。
func (h *Hub) IdentityForStream(room, stream string) (string, bool) {
	h.mu.RLock()
	if identities, ok := h.streamByIdentity[room]; ok {
		for identity, st := range identities {
			if st == stream && identity != "" {
				h.mu.RUnlock()
				return identity, true
			}
		}
	}
	h.mu.RUnlock()
	if stream == "" || h.membershipStore == nil {
		return "", false
	}
	// Prefer stream KV (authoritative stream→identity).
	r, identity, err := h.membershipStore.GetStream(context.Background(), stream)
	if err == nil && identity != "" && (room == "" || r == room || r == "") {
		return identity, true
	}
	if room == "" {
		return "", false
	}
	snap, err := h.membershipStore.GetRoomMembers(context.Background(), room)
	if err != nil {
		return "", false
	}
	for _, m := range snap.Members {
		if m.Stream == stream && m.Identity != "" {
			return m.Identity, true
		}
	}
	return "", false
}

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
