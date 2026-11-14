package signal

import "GOSpeak/internal/protocol"

// 协议 DTO 已收编至 internal/protocol（单一事实源，json tag 保持 wire 等价）。
// 本文件仅为存量 signal 代码保留别名，勿新增；新代码直接引用 protocol。
type (
	RoomRequest = protocol.RoomRequest
	MemberInfo  = protocol.MemberInfo
	RoomInfo    = protocol.RoomInfo
	MuteInfo    = protocol.MuteInfo
)
