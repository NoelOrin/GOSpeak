package signal

const (
	EventConnect    = "connection"
	EventDisconnect = "disconnect"

	// 房间管理（客户端 → 服务端）
	EventRoomCreate  = "room:create"
	EventRoomJoin    = "room:join"
	EventRoomLeave   = "room:leave"
	EventRoomList    = "room:list"
	EventRoomJoinSFU = "room:join:sfu"

	// 服务端推送
	EventRoomCreated    = "room:created"
	EventRoomLeft       = "room:left"
	EventRoomUpdated    = "room:updated"
	EventMemberJoined   = "member:joined"
	EventMemberLeft     = "member:left"
	EventMemberUpdated  = "member:updated"
	EventRoomListResult = "room:list:result"

	// Bot 消息桥（客户端 → 服务端）

	// 管理操作（客户端 → 服务端）
	EventMemberMicState = "member:mic-state"
	EventRoomKick       = "room:kick"

	// 管理操作推送
	EventRoomKicked = "room:kicked"

	// 发言检测（无 SFU 原生 active speaker 的 provider）
	EventMemberSpeaking     = "member:speaking"
	EventRoomActiveSpeakers = "room:active-speakers"

	// 文字房「正在输入」：客户端上报 typing，服务端按房间广播（含上报者 identity，
	// 前端自行过滤自己）。不落库、不保证送达，纯 UI 提示。
	EventTyping = "typing"

	// 禁言事件
	EventUserMuted   = "user:muted"
	EventUserUnmuted = "user:unmuted"

	// 成员级禁言事件（订阅端静音用）：所有客户端按 identity 静音/恢复远端 track。
	EventMemberMuted   = "member:muted"
	EventMemberUnmuted = "member:unmuted"

	// SFU 热切换：通知所有客户端断连并刷新
	EventSFUProviderChanged = "sfu:provider-changed"

	// 消息事件（客户端 → 服务端）
	EventMessageSend    = "message:send"
	EventMessageEdit    = "message:edit"
	EventMessageDelete  = "message:delete"
	EventMessageReact   = "message:react"
	EventMessageUnreact = "message:unreact"

	// 消息事件（服务端推送）
	EventMessageCreated  = "message:created"
	EventMessageUpdated  = "message:updated"
	EventMessageDeleted  = "message:deleted"
	EventMessageReaction = "message:reaction"
	EventMessageAck      = "message:ack"
	EventMessageError    = "message:error"

	// 私聊消息事件（客户端 → 服务端）
	EventPrivateSend = "private:send"

	// 私聊消息事件（服务端推送）
	EventPrivateNew = "private:new"
)
