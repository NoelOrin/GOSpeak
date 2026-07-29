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

	// SFU 媒体协商（当前由 mediasoup 使用）
	EventSFUGetRouterCapabilities = "sfu:get-router-capabilities"
	EventSFUCreateTransport       = "sfu:create-transport"
	EventSFUConnectTransport      = "sfu:connect-transport"
	EventSFUProduce               = "sfu:produce"
	EventSFUConsume               = "sfu:consume"
	EventSFUProducerReady         = "sfu:producer-ready"
	EventSFUProducerClosed        = "sfu:producer-closed"

	// 服务端推送
	EventRoomCreated    = "room:created"
	EventRoomLeft       = "room:left"
	EventRoomUpdated    = "room:updated"
	EventMemberJoined   = "member:joined"
	EventMemberLeft     = "member:left"
	EventMemberUpdated  = "member:updated"
	EventRoomListResult = "room:list:result"

	// Bot 消息桥（客户端 → 服务端）
	EventBotCommand = "bot:command"
	EventBotMessage = "bot:message"

	// 管理操作（客户端 → 服务端）
	EventMemberMicState = "member:mic-state"
	EventRoomKick = "room:kick"

	// 管理操作推送
	EventRoomKicked = "room:kicked"

	// 发言检测（无 SFU 原生 active speaker 的 provider：SRS / Cloudflare）
	EventMemberSpeaking = "member:speaking"
	EventRoomActiveSpeakers = "room:active-speakers"

	// 禁言事件
	EventUserMuted   = "user:muted"
	EventUserUnmuted = "user:unmuted"

	// SFU 热切换：通知所有客户端断连并刷新
	EventSFUProviderChanged = "sfu:provider-changed"

	// 消息（客户端 → 服务端）
	EventMessageSend = "message:send"

	// 消息推送（服务端 → 客户端）
	EventMessageNew = "message:new"

	// 私聊消息（客户端 → 服务端）
	EventPrivateSend = "private:send"
)
