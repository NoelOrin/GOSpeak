package protocol

// bus 内部事件（NATS JSON 往返，非 WebSocket wire 事件）：名称与 payload struct
// 统一在此定义，Catalog 条目登记在 events.go（Dir: Internal，不产出 TS）。
// 发布方传 struct，订阅方经 bus.DecodePayload 还原，禁止 map[string]interface{} 拆箱。
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

type PermissionsInvalidatedPayload struct {
	Role string `json:"role"`
}

type MuteChangedPayload struct {
	UserID   string `json:"user_id"`
	Identity string `json:"identity"`
}

type MuteExpiredPayload struct {
	UserID string `json:"user_id"`
}

type DomainMembershipChangedPayload struct {
	DomainUUID string `json:"domain_uuid"`
	UserUUID   string `json:"user_uuid"`
}

type DomainMemberAddedPayload struct {
	DomainUUID string `json:"domain_uuid"`
	UserUUID   string `json:"user_uuid"`
}

type RoomListChangedPayload struct {
	DomainUUID string `json:"domain_uuid"`
}

