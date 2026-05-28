package signal

const (
	EventConnect    = "connection"
	EventDisconnect = "disconnect"

	// 房间管理（客户端 → 服务端）
	EventRoomCreate = "room:create"
	EventRoomJoin   = "room:join"
	EventRoomLeave  = "room:leave"
	EventRoomList   = "room:list"
	EventRoomJoinLiveKit = "room:join:livekit"

	// 服务端推送
	EventRoomCreated    = "room:created"
	EventRoomJoined     = "room:joined"
	EventRoomLeft       = "room:left"
	EventRoomUpdated    = "room:updated"
	EventMemberJoined   = "member:joined"
	EventMemberLeft     = "member:left"
	EventMemberUpdated  = "member:updated"
	EventRoomListResult = "room:list:result"
)
