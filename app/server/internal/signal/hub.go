package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/sfu"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"GOSpeak/internal/ws"
)

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
	IsMutedBatch(identities []string) (map[string]bool, error)
}

// userStore abstracts user lookup for Hub.

// userStore abstracts user lookup for Hub.
type userStore interface {
	GetByName(name string) (*model.User, error)
	GetByNames(names []string) (map[string]*model.User, error)
	GetByID(id uint) (*model.User, error)
	GetByUUID(uuid string) (*model.User, error)
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
	membershipHeartbeatDone chan struct{}
	// kickBans 记录 room+identity 的短时踢出冷却，阻止被踢者立即重连同一房间。
	kickBans map[string]time.Time
	// muteCache 缓存 OnMemberSpeaking 的禁言查询结果，避免高频发言态上报打爆 DB。
	// TTL 5s；禁言/解禁不主动失效缓存，最坏 5s 陈旧窗口（发言指示非安全边界，SFU 层仍强制执行静音）。
	muteCache map[string]muteCacheEntry
}

// HubOptions 聚合 Hub 的协作依赖，由组合根在构造时一次传入。
type HubOptions struct {
	Fanout             ws.Broadcaster
	EventBus           eventBus
	SFUProvider        sfu.Provider
	StreamResolver     StreamNameResolver
	MessageSender      messageSender
	ConversationSender conversationSender
	DomainChecker      func(domainUUID, userUUID string) bool
	MembershipStore    membershipStore
	InstanceID         string
	StateNotifier      stateNotifier
}

// NewHub 保留测试友好的四参构造，生产路径请使用 NewHubWithOptions。
func NewHub(store roomStore, mStore muteStore, uStore userStore, pChecker permChecker) *Hub {
	return NewHubWithOptions(store, mStore, uStore, pChecker, HubOptions{})
}

// NewHubWithOptions 在构造时注入协作依赖，避免启动期 Set* 后置装配。
func NewHubWithOptions(store roomStore, mStore muteStore, uStore userStore, pChecker permChecker, opts HubOptions) *Hub {
	h := &Hub{
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
		kickBans:         make(map[string]time.Time),
		muteCache:        make(map[string]muteCacheEntry),
	}
	h.fanout = opts.Fanout
	h.eventBus = opts.EventBus
	h.sfuProvider = opts.SFUProvider
	h.streamResolver = opts.StreamResolver
	h.msgSvc = opts.MessageSender
	h.convSvc = opts.ConversationSender
	h.domainChecker = opts.DomainChecker
	h.membershipStore = opts.MembershipStore
	h.instanceID = opts.InstanceID
	h.stateNotifier = opts.StateNotifier
	return h
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
