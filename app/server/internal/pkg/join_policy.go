package pkg

// JoinPolicy 封装加入房间前的业务规则校验（禁言 / 限流 / 密码）。
// 由 signal.Hub 实现，注入 SFUService，使 handler 不再直连 signal 层做规则校验，
// SFU 调用与业务规则在 service 层聚合。方法签名仅用 bool/error，避免 pkg→model 依赖。
type JoinPolicy interface {
	// IsMuted 返回 identity 是否被禁言。
	IsMuted(identity string) (bool, error)
	// CheckRoomLimit 返回房间是否已满，附带 limit 与当前 count。
	CheckRoomLimit(domainUUID, roomName string) (full bool, limit uint, count int, err error)
	// CheckRoomPassword 校验房间密码：ok=false 且 err!=nil 表示需密码（未提供），
	// ok=false 且 err==nil 表示密码错误，ok=true 表示通过（无密码或密码正确）。
	CheckRoomPassword(domainUUID, roomName, password string) (ok bool, err error)
}
