package service

// bus 内部事件名（非 WebSocket wire 事件）。payload 为可 JSON 化的 map。
const (
	// EventPermissionsInvalidated 平台权限缓存失效。
	EventPermissionsInvalidated = "cache:permissions-invalidated"
	// EventMuteChanged 禁言状态变更（缓存失效）。
	EventMuteChanged = "cache:mute-changed"
	// EventDomainMembershipChanged 域成员关系变更（缓存失效）。
	EventDomainMembershipChanged = "cache:domain-membership-changed"
	// EventMuteExpired 临时禁言到期，订阅方执行完整 unmute（广播 + SFU 恢复）。
	EventMuteExpired = "mute:expired"
	// EventDomainMemberAdded 新成员写入域，订阅方清理成员缓存并广播失效。
	EventDomainMemberAdded = "domain:member-added"
	// EventRoomListChanged 房间元数据经 REST 变更，订阅方重新广播房间列表。
	EventRoomListChanged = "room:list-changed"
)
