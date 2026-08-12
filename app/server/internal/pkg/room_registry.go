package pkg

// RoomRegistry 提供 room→streams 聚合视图，供无原生 room 维度的 SFU provider（如 SRS）使用。
// 由 signal.Hub 实现，通过 SetRoomRegistry 注入到 provider。
// 放 pkg 包以避开 sfu↔provider 的 import 循环（pkg 被三者共同 import 且不反向依赖）。
type RoomRegistry interface {
	// Rooms 返回当前有活跃成员的 room 列表。
	Rooms() []string
	// Streams 返回指定 room 下登记的 stream 名集合。
	Streams(room string) []string
	// ClearRoom 清除指定 room 的所有 stream 登记（room 维度状态重置）。
	ClearRoom(room string)
	// StreamForIdentity 返回 join 时实际登记的 stream 名（基于 identity→stream 映射），
	// 优于反算命名约定：命名函数变更后旧连接的 stream 仍可查到。未登记返 ok=false。
	StreamForIdentity(room, identity string) (stream string, ok bool)
	// IdentityForStream 返回登记该 stream 的 identity。未登记返 ok=false。
	IdentityForStream(room, stream string) (identity string, ok bool)
}

// RoomRegistrySetter 由支持 room 聚合的 provider 实现可选接口。
// DynamicProvider 在 current() 重建 provider 后转发注入。
type RoomRegistrySetter interface {
	SetRoomRegistry(r RoomRegistry)
}

// StreamRoomResolver 提供 stream→room 反查，供 SRS 等无原生 room 维度的 provider
// 在 SRS API 直查后映射回 GOSpeak room（复合键 domainUUID:roomName）。
// 由 signal.Hub 实现：本地 streamRoomCache 优先，membership KV 兜底。
type StreamRoomResolver interface {
	RoomForStream(stream string) (string, bool)
}

// StreamRoomResolverSetter 由需要 stream→room 反查的 provider 实现可选接口。
type StreamRoomResolverSetter interface {
	SetStreamRoomResolver(r StreamRoomResolver)
}
