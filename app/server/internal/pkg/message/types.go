// Package message 存放跨层共享的消息 Actor，避免 signal 反向依赖 service。
// 消息 wire DTO 已收编至 internal/protocol（单一事实源），此处保留别名。
package message

import "GOSpeak/internal/protocol"

// DTO 是消息的公开传输对象（wire 定义见 protocol.MessageDTO）。
type DTO = protocol.MessageDTO

// Actor 区分展示/作者身份与稳定用户 UUID。
// 域成员关系必须用 UserUUID 校验；AuthorID 保持为用户名。
type Actor struct {
	Identity string
	UserUUID string
}
