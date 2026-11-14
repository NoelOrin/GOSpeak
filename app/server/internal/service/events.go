package service

import "GOSpeak/internal/protocol"

// bus 内部事件常量与 payload 已收编至 internal/protocol（单一事实源）。
// 本文件仅为存量 service 代码保留别名；新代码直接引用 protocol。
var (
	EventPermissionsInvalidated  = protocol.EventPermissionsInvalidated
	EventMuteChanged             = protocol.EventMuteChanged
	EventDomainMembershipChanged = protocol.EventDomainMembershipChanged
	EventMuteExpired             = protocol.EventMuteExpired
	EventDomainMemberAdded       = protocol.EventDomainMemberAdded
	EventRoomListChanged         = protocol.EventRoomListChanged
)

type (
	PermissionsInvalidatedPayload  = protocol.PermissionsInvalidatedPayload
	MuteChangedPayload             = protocol.MuteChangedPayload
	MuteExpiredPayload             = protocol.MuteExpiredPayload
	DomainMembershipChangedPayload = protocol.DomainMembershipChangedPayload
	DomainMemberAddedPayload       = protocol.DomainMemberAddedPayload
	RoomListChangedPayload         = protocol.RoomListChangedPayload
)
