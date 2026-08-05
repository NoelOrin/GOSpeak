package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"GOSpeak/internal/ws"
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

// roomSetMember writes both Members (by socketID) and ByIdentity index.
// Caller must hold h.mu write-lock.
func roomSetMember(room *Room, sid, identity string, member *MemberInfo) {
	room.Members[sid] = member
	if identity != "" && room.ByIdentity != nil {
		room.ByIdentity[identity] = member
	}
}

// roomDelMember removes from both Members and ByIdentity. Caller must hold write-lock.

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

// socketServer is replaced by ws.Broadcaster.

// eventBus is the narrow publish surface used by Hub for client fanout.
// *bus.NATSBus satisfies this without importing the bus package here.
type eventBus interface {
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

// cleanupPublisher enqueues SFU cleanup jobs (phase-3). Nil = inline goroutine.

// cleanupPublisher enqueues SFU cleanup jobs (phase-3). Nil = inline goroutine.
type cleanupPublisher interface {
	PublishSFUCleanup(ctx context.Context, room, identity string, deleteRoom bool) error
}

// roomKey generates a domain-scoped composite key for room map isolation.
// Platform-level rooms (no DomainUUID) use a "platform:" prefix for backward compatibility.

// roomKey generates a domain-scoped composite key for room map isolation.
// Platform-level rooms (no DomainUUID) use a "platform:" prefix for backward compatibility.
func roomKey(domainUUID, roomName string) string {
	return pkg.RoomKey(domainUUID, roomName)
}

// splitRoomKey reverses roomKey to extract domainUUID and roomName.

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

// roomStore abstracts DB room listing for Hub.
type roomStore interface {
	List(page, pageSize int, roomType, domainUUID string) ([]model.Room, int64, error)
	GetByName(name string) (*model.Room, error)
	GetByDomainAndName(domainUUID, name string) (*model.Room, error)
}

// muteStore abstracts mute checking for Hub.

// muteStore abstracts mute checking for Hub.
type muteStore interface {
	IsMutedByIdentity(identity string) (bool, *model.Mute, error)
}

// userStore abstracts user lookup for Hub.

// userStore abstracts user lookup for Hub.
type userStore interface {
	GetByName(name string) (*model.User, error)
	GetByID(id uint) (*model.User, error)
}

// permChecker 抽象权限校验，复用 service.PermissionService 的内存缓存。

// permChecker 抽象权限校验，复用 service.PermissionService 的内存缓存。
type permChecker interface {
	HasPermission(roleName, permCode string) bool
}

// SFUSignalHandler 注册 provider 专属的 sfu:* 媒体协商事件。

// SFUSignalHandler 注册 provider 专属的 sfu:* 媒体协商事件。
type SFUSignalHandler interface {
	RegisterWS(register func(event string, fn func(ws.ClientMessenger, string) (string, error)))
}

// broadcastFn 供 provider 模块广播房间事件，无需依赖 Hub 具体类型。

// broadcastFn 供 provider 模块广播房间事件，无需依赖 Hub 具体类型。
type BroadcastFn func(room, event string, data interface{})

// StreamNameResolver 计算给定 room+identity 的预期 stream 名，用于服务端覆写客户端提报值。

// StreamNameResolver 计算给定 room+identity 的预期 stream 名，用于服务端覆写客户端提报值。
type StreamNameResolver interface {
	StreamName(room, identity string) string
}

// ParticipantCleanupHandler 处理参与者离开时的 SFU 专属清理(如 mediasoup 广播 producer-closed + 关 transport)。
// 仅 mediasoup 实现;其它 provider 不实现此接口,Hub OnDisconnect 类型断言跳过。

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
	streamByIdentity        map[string]map[string]string
	participantCleanup      ParticipantCleanupHandler
	membershipStore         membershipStore
	instanceID              string
	cleanupPub              cleanupPublisher
	stateNotifier           stateNotifier
	connSlots               map[string]*connRoomSlots // socketID -> slots
	msgSvc                  messageSender
	convSvc                 conversationSender
	domainChecker           func(domainUUID, userUUID string) bool
	clientDomains           map[string]string // socketID -> current domain scope (empty = platform)
	membershipHeartbeatStop chan struct{}
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
		clientDomains:    make(map[string]string),
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

// claimsIdentity 从连接上下文读取 JWT 身份；未鉴权返回空。
func clientIdentity(c ws.ClientMessenger) string {
	if c == nil || c.Claims() == nil {
		return ""
	}
	return c.Claims().Username
}

// resolveIdentity 强制使用 JWT 身份，忽略客户端伪造的 identity。

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

// disconnectCleanup 记录断连后需要执行的 SFU 清理项。
type disconnectCleanup struct {
	room     string
	identity string
	deleted  bool
}

func (h *Hub) OnDisconnect(c ws.ClientMessenger) {
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
			"room":        logicalName,
			"domain_uuid": domainUUID,
			"identity":    e.identity,
			"id":          e.id,
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
					log.Printf("[Signal] enqueue sfu cleanup: %v; fallback to inline goroutine", err)
					h.runCleanupsInline([]disconnectCleanup{c})
				}
			}
		} else {
			h.runCleanupsInline(cleanups)
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

// runCleanupsInline 在后台 goroutine 中执行 SFU 清理，
// 用于 cleanup queue 不可用或入队失败时的兜底路径。
func (h *Hub) runCleanupsInline(cleanups []disconnectCleanup) {
	if len(cleanups) == 0 {
		return
	}
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

// ─── 房间创建 ───

// OnError 兜底处理 WS 层 error（含 OnConnect 返回的 panic error）。
// 仅记录日志，不做断连等副作用——连接级错误由库自行处理。

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

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
