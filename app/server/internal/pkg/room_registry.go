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
}

// RoomRegistrySetter 由支持 room 聚合的 provider 实现可选接口。
// DynamicProvider 在 current() 重建 provider 后转发注入。
type RoomRegistrySetter interface {
	SetRoomRegistry(r RoomRegistry)
}
