package protocol

import "time"

// MessageDTO 是消息事件的 wire 载荷（message:created/updated/deleted、
// private:new、private:send ACK 共用）。原 pkg/message.DTO 收编至此。
type MessageDTO struct {
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

// MessageReactionPayload 是 message:reaction 的载荷。
type MessageReactionPayload struct {
	Action      string `json:"action"`
	MessageUUID string `json:"message_uuid"`
	UserID      string `json:"user_id"`
	Emoji       string `json:"emoji"`
}

// MessageSendPayload 是 message:send 上行载荷。
type MessageSendPayload struct {
	Room        string   `json:"room"`
	DomainUUID  string   `json:"domain_uuid,omitempty"`
	Content     string   `json:"content"`
	ReplyTo     string   `json:"reply_to,omitempty"`
	Mentions    []string `json:"mentions,omitempty"`
	ClientNonce string   `json:"client_nonce,omitempty"`
}

// MessageMutatePayload 是 message:edit/delete/react/unreact 上行载荷。
type MessageMutatePayload struct {
	Room        string `json:"room"`
	DomainUUID  string `json:"domain_uuid,omitempty"`
	MessageUUID string `json:"message_uuid"`
	Content     string `json:"content,omitempty"`
	Emoji       string `json:"emoji,omitempty"`
}

// PrivateSendPayload 是 private:send 上行载荷。
type PrivateSendPayload struct {
	TargetIdentity string `json:"target_identity"`
	Content        string `json:"content"`
	ClientNonce    string `json:"client_nonce,omitempty"`
}

// AnnouncementPayload 是 announcement:new 的载荷（原 service.AnnouncementDTO）。
type AnnouncementPayload struct {
	UUID          string `json:"uuid"`
	ScopeType     string `json:"scope_type"`
	ScopeID       string `json:"scope_id,omitempty"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         string `json:"level"`
	Pinned        bool   `json:"pinned"`
	PublishedAt   *int64 `json:"published_at,omitempty"`
	ExpiresAt     *int64 `json:"expires_at,omitempty"`
	CreatedByUUID string `json:"created_by_uuid,omitempty"`
	CreatedByName string `json:"created_by_name"`
	IsRead        bool   `json:"is_read"`
	CreatedAt     int64  `json:"created_at"`
}
