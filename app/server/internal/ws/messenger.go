package ws

import "GOSpeak/internal/pkg"

// ClientMessenger 是 Handler 层依赖的客户端最小面。
// 所有信号处理器只依赖此接口，不依赖具体 Client 实现。
// Hub 不需要知道底层用的是 nhooyr、gorilla 还是 mock。
type ClientMessenger interface {
	// ID 返回客户端唯一标识（通常是 UserUUID）。
	ID() string
	// Claims 返回 JWT 认证声明。
	Claims() *pkg.Claims
	// Send 发送任意 JSON 可序列化的数据。
	Send(v interface{}) bool
	// SendACK 发送带关联 id 的应答。
	SendACK(id, event string, data interface{})
	// SendErrorACK 发送带关联 id 的错误应答。
	SendErrorACK(id, event string, code int, message string)
	// Close 关闭连接。
	Close()
}

// StatefulMessenger 暴露连接状态，供需要观测 WS 生命周期的调用方使用。
type StatefulMessenger interface {
	ClientMessenger
	State() ConnState
}

// PolicyClosingMessenger 是命中封禁时的可选能力：先把错误 ACK 真正写到对端，
// 再以 policy violation 状态码关闭连接。
// 只入队 ACK 后立即 Close 会让 writeLoop 在 writeCh 与 ctx.Done() 之间选中后者，
// 客户端永远收不到封禁原因。Client 实现该接口；未实现时回退为
// SendErrorACK + Close（测试替身等场景）。
type PolicyClosingMessenger interface {
	SendErrorACKAndClose(id, event string, code int, message string)
}
