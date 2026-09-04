// Package message 存放跨层共享的消息 DTO 与 Actor，避免 signal 反向依赖 service。
package message

import "time"

// DTO 是消息的公开传输对象，用于广播载荷与 API 响应。
type DTO struct {
	UUID           string     `json:"uuid"`
	RoomUUID       string     `json:"room_uuid"`
	AuthorID       string     `json:"author_id"`
	AuthorUUID     string     `json:"author_uuid"`
	AuthorName     string     `json:"author_name"`
	AuthorAvatar   string     `json:"author_avatar"`
	Content        string     `json:"content"`
	ReplyTo        string     `json:"reply_to,omitempty"`
	Mentions       []string   `json:"mentions,omitempty"`
	EditedAt       *time.Time `json:"edited_at,omitempty"`
	Deleted        bool       `json:"deleted"`
	CreatedAt      time.Time  `json:"created_at"`
	ClientNonce    string     `json:"client_nonce,omitempty"`
	ConversationID string     `json:"conversation_id,omitempty"`
	TargetIdentity string     `json:"target_identity,omitempty"`
}

// Actor 区分展示/作者身份与稳定用户 UUID。
// 域成员关系必须用 UserUUID 校验；AuthorID 保持为用户名。
type Actor struct {
	Identity string
	UserUUID string
}
