// Package protocol 是 WS 事件协议的单一事实源：事件名、方向、ACK 需求、
// payload struct 全部在此定义。前端 TS 产物由 cmd/gen-protocol 从本包生成，
// 禁止在任何其他包手工书写 wire 事件名字符串。
package protocol

type Direction string

const (
	DirClientToServer Direction = "c2s"
	DirServerToClient Direction = "s2c"
	DirBoth           Direction = "both"
	DirInternal       Direction = "internal"
)

const (
	EventConnect    = "connection"
	EventDisconnect = "disconnect"

	EventRoomCreate  = "room:create"
	EventRoomJoin    = "room:join"
	EventRoomLeave   = "room:leave"
	EventRoomList    = "room:list"
	EventRoomJoinSFU = "room:join:sfu"

	EventRoomCreated    = "room:created"
	EventRoomLeft       = "room:left"
	EventRoomUpdated    = "room:updated"
	EventMemberJoined   = "member:joined"
	EventMemberLeft     = "member:left"
	EventMemberUpdated  = "member:updated"
	EventRoomListResult = "room:list:result"

	EventMemberMicState = "member:mic-state"
	EventRoomKick       = "room:kick"
	EventRoomKicked     = "room:kicked"

	EventMemberSpeaking     = "member:speaking"
	EventRoomActiveSpeakers = "room:active-speakers"

	// 文字房「正在输入」：客户端上报 typing，服务端按房间广播（含上报者
	// identity，前端自行过滤自己）。不落库、不保证送达，纯 UI 提示。
	EventTyping = "typing"

	EventUserMuted   = "user:muted"
	EventUserUnmuted = "user:unmuted"

	// 成员级禁言事件（订阅端静音用）：所有客户端按 identity 静音/恢复远端 track。
	EventMemberMuted   = "member:muted"
	EventMemberUnmuted = "member:unmuted"

	EventSFUProviderChanged = "sfu:provider-changed"

	EventMessageSend    = "message:send"
	EventMessageEdit    = "message:edit"
	EventMessageDelete  = "message:delete"
	EventMessageReact   = "message:react"
	EventMessageUnreact = "message:unreact"

	EventMessageCreated  = "message:created"
	EventMessageUpdated  = "message:updated"
	EventMessageDeleted  = "message:deleted"
	EventMessageReaction = "message:reaction"
	EventMessageAck      = "message:ack"
	EventMessageError    = "message:error"

	EventPrivateSend = "private:send"
	EventPrivateNew  = "private:new"

	// 系统公告（服务端推送）：平台公告走 namespace 广播，域公告定向投递到 __user 房间。
	EventAnnouncementNew = "announcement:new"
)

// Event 是目录中的一个事件条目。
type Event struct {
	Name    string    // wire 事件名
	Dir     Direction // 方向；DirInternal 不产出前端 TS
	Ack     bool      // 上行是否需要 ACK（仅 c2s/both 有意义）
	Payload string    // payloads.go 中的 struct 名；"" 表示 P1 未挂 payload（P2 补齐）
}

// payloadStructs 供测试校验 Payload 引用；每新增 payload struct 同步登记。
var payloadStructs = map[string]bool{
	"RoomRequest": true,
	"MemberInfo":  true,
	"RoomInfo":    true,
	"MuteInfo":    true,
}

// eventConsts 供测试校验 Catalog 与常量一致；登记全部事件名常量。
var eventConsts = map[string]string{
	EventConnect:    EventConnect,
	EventDisconnect: EventDisconnect,

	EventRoomCreate:  EventRoomCreate,
	EventRoomJoin:    EventRoomJoin,
	EventRoomLeave:   EventRoomLeave,
	EventRoomList:    EventRoomList,
	EventRoomJoinSFU: EventRoomJoinSFU,

	EventRoomCreated:    EventRoomCreated,
	EventRoomLeft:       EventRoomLeft,
	EventRoomUpdated:    EventRoomUpdated,
	EventMemberJoined:   EventMemberJoined,
	EventMemberLeft:     EventMemberLeft,
	EventMemberUpdated:  EventMemberUpdated,
	EventRoomListResult: EventRoomListResult,

	EventMemberMicState: EventMemberMicState,
	EventRoomKick:       EventRoomKick,
	EventRoomKicked:     EventRoomKicked,

	EventMemberSpeaking:     EventMemberSpeaking,
	EventRoomActiveSpeakers: EventRoomActiveSpeakers,
	EventTyping:             EventTyping,

	EventUserMuted:   EventUserMuted,
	EventUserUnmuted: EventUserUnmuted,
	EventMemberMuted:   EventMemberMuted,
	EventMemberUnmuted: EventMemberUnmuted,

	EventSFUProviderChanged: EventSFUProviderChanged,

	EventMessageSend:    EventMessageSend,
	EventMessageEdit:    EventMessageEdit,
	EventMessageDelete:  EventMessageDelete,
	EventMessageReact:   EventMessageReact,
	EventMessageUnreact: EventMessageUnreact,

	EventMessageCreated:  EventMessageCreated,
	EventMessageUpdated:  EventMessageUpdated,
	EventMessageDeleted:  EventMessageDeleted,
	EventMessageReaction: EventMessageReaction,
	EventMessageAck:      EventMessageAck,
	EventMessageError:    EventMessageError,

	EventPrivateSend: EventPrivateSend,
	EventPrivateNew:  EventPrivateNew,

	EventAnnouncementNew: EventAnnouncementNew,
}

// Catalog 是全部事件的权威目录。Ack 标志与 signal/hub.go SetupFanout 的
// Handle/HandleAck 注册一一对应；P2 逐事件补 Payload。
var Catalog = []Event{
	{Name: EventConnect, Dir: DirInternal},
	{Name: EventDisconnect, Dir: DirInternal},

	{Name: EventRoomCreate, Dir: DirClientToServer},
	{Name: EventRoomJoin, Dir: DirClientToServer, Ack: true, Payload: "RoomRequest"},
	{Name: EventRoomLeave, Dir: DirClientToServer, Ack: true, Payload: "RoomRequest"},
	{Name: EventRoomList, Dir: DirClientToServer},
	{Name: EventRoomJoinSFU, Dir: DirClientToServer, Ack: true},

	{Name: EventRoomCreated, Dir: DirServerToClient},
	{Name: EventRoomLeft, Dir: DirServerToClient},
	{Name: EventRoomUpdated, Dir: DirServerToClient, Payload: "RoomInfo"},
	{Name: EventMemberJoined, Dir: DirServerToClient, Payload: "MemberInfo"},
	{Name: EventMemberLeft, Dir: DirServerToClient},
	{Name: EventMemberUpdated, Dir: DirServerToClient},
	{Name: EventRoomListResult, Dir: DirServerToClient},

	{Name: EventMemberMicState, Dir: DirClientToServer},
	{Name: EventRoomKick, Dir: DirClientToServer},
	{Name: EventRoomKicked, Dir: DirServerToClient},

	{Name: EventMemberSpeaking, Dir: DirClientToServer},
	{Name: EventRoomActiveSpeakers, Dir: DirServerToClient},
	{Name: EventTyping, Dir: DirBoth},

	{Name: EventUserMuted, Dir: DirServerToClient, Payload: "MuteInfo"},
	{Name: EventUserUnmuted, Dir: DirServerToClient, Payload: "MuteInfo"},
	{Name: EventMemberMuted, Dir: DirServerToClient},
	{Name: EventMemberUnmuted, Dir: DirServerToClient},

	{Name: EventSFUProviderChanged, Dir: DirServerToClient},

	{Name: EventMessageSend, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageEdit, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageDelete, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageReact, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageUnreact, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageCreated, Dir: DirServerToClient},
	{Name: EventMessageUpdated, Dir: DirServerToClient},
	{Name: EventMessageDeleted, Dir: DirServerToClient},
	{Name: EventMessageReaction, Dir: DirServerToClient},
	{Name: EventMessageAck, Dir: DirServerToClient},
	{Name: EventMessageError, Dir: DirServerToClient},

	{Name: EventPrivateSend, Dir: DirClientToServer, Ack: true},
	{Name: EventPrivateNew, Dir: DirServerToClient},

	{Name: EventAnnouncementNew, Dir: DirServerToClient},
}
