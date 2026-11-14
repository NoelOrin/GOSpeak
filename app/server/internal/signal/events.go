package signal

import "GOSpeak/internal/protocol"

// 事件名常量已收编至 internal/protocol（单一事实源）。
// 本文件仅为存量 signal 代码保留别名，勿新增；新代码直接引用 protocol。
const (
	EventConnect    = protocol.EventConnect
	EventDisconnect = protocol.EventDisconnect

	EventRoomCreate  = protocol.EventRoomCreate
	EventRoomJoin    = protocol.EventRoomJoin
	EventRoomLeave   = protocol.EventRoomLeave
	EventRoomList    = protocol.EventRoomList
	EventRoomJoinSFU = protocol.EventRoomJoinSFU

	EventRoomCreated    = protocol.EventRoomCreated
	EventRoomLeft       = protocol.EventRoomLeft
	EventRoomUpdated    = protocol.EventRoomUpdated
	EventMemberJoined   = protocol.EventMemberJoined
	EventMemberLeft     = protocol.EventMemberLeft
	EventMemberUpdated  = protocol.EventMemberUpdated
	EventRoomListResult = protocol.EventRoomListResult

	EventMemberMicState = protocol.EventMemberMicState
	EventRoomKick       = protocol.EventRoomKick
	EventRoomKicked     = protocol.EventRoomKicked

	EventMemberSpeaking     = protocol.EventMemberSpeaking
	EventRoomActiveSpeakers = protocol.EventRoomActiveSpeakers
	EventTyping             = protocol.EventTyping

	EventUserMuted     = protocol.EventUserMuted
	EventUserUnmuted   = protocol.EventUserUnmuted
	EventMemberMuted   = protocol.EventMemberMuted
	EventMemberUnmuted = protocol.EventMemberUnmuted

	EventSFUProviderChanged = protocol.EventSFUProviderChanged

	EventMessageSend    = protocol.EventMessageSend
	EventMessageEdit    = protocol.EventMessageEdit
	EventMessageDelete  = protocol.EventMessageDelete
	EventMessageReact   = protocol.EventMessageReact
	EventMessageUnreact = protocol.EventMessageUnreact

	EventMessageCreated  = protocol.EventMessageCreated
	EventMessageUpdated  = protocol.EventMessageUpdated
	EventMessageDeleted  = protocol.EventMessageDeleted
	EventMessageReaction = protocol.EventMessageReaction
	EventMessageAck      = protocol.EventMessageAck
	EventMessageError    = protocol.EventMessageError

	EventPrivateSend = protocol.EventPrivateSend
	EventPrivateNew  = protocol.EventPrivateNew
)
