package protocol

type RoomRequest struct {
	Room string `json:"room"`
	// DomainUUID 命名空间隔离：同一房间名在不同 Domain 下视为不同房间。
	DomainUUID string `json:"domain_uuid,omitempty"`
	Password   string `json:"password,omitempty"`
	Stream     string `json:"stream,omitempty"`
}

type MemberInfo struct {
	ID          string `json:"id"`
	Identity    string `json:"identity"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	IsMuted     bool   `json:"is_muted"`
	IsMicMuted  bool   `json:"is_mic_muted"`
	JoinedAt    int64  `json:"joined_at"`
	Stream      string `json:"stream,omitempty"`
}

type RoomInfo struct {
	ID            uint         `json:"id"`
	UUID          string       `json:"uuid"`
	Name          string       `json:"name"`
	DomainUUID    string       `json:"domain_uuid,omitempty"`
	HasPassword   bool         `json:"has_password"`
	Description   string       `json:"description"`
	Limit         uint         `json:"limit"`
	Type          string       `json:"type"`
	AudioOnly     bool         `json:"audio_only"`
	AllowAudience bool         `json:"allow_audience"`
	Members       []MemberInfo `json:"members"`
	Count         int          `json:"count"`
	CreatedAt     int64        `json:"created_at"`
}

// MuteInfo is the DTO broadcast to clients for mute/unmute events.
type MuteInfo struct {
	UserID    uint   `json:"user_id"`
	Duration  int64  `json:"duration"`
	Permanent bool   `json:"permanent"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// ─── P2 首批：signal 层广播 payload（json tag 与既有 wire 逐字一致，行为等价迁移）───

type MemberLeftPayload struct {
	Room       string `json:"room"`
	DomainUUID string `json:"domain_uuid,omitempty"`
	Identity   string `json:"identity"`
	ID         string `json:"id"`
	// Enforcement 仅 member:kicked 后补发的 MemberLeft 事件携带。
	Enforcement string `json:"enforcement,omitempty"`
}

type MemberUpdatedPayload struct {
	Room       string `json:"room"`
	DomainUUID string `json:"domain_uuid,omitempty"`
	Identity   string `json:"identity"`
	IsMicMuted bool   `json:"is_mic_muted"`
}

type RoomActiveSpeakersPayload struct {
	Room       string   `json:"room"`
	DomainUUID string   `json:"domain_uuid,omitempty"`
	Identities []string `json:"identities"`
}

type TypingPayload struct {
	Room       string `json:"room"`
	DomainUUID string `json:"domain_uuid,omitempty"`
	Identity   string `json:"identity"`
	Typing     bool   `json:"typing"`
}

type RoomListResultPayload struct {
	Rooms []RoomInfo `json:"rooms"`
	Count int        `json:"count"`
}

type RoomKickedPayload struct {
	Room           string `json:"room"`
	DomainUUID     string `json:"domain_uuid,omitempty"`
	TargetIdentity string `json:"target_identity"`
	Enforcement    string `json:"enforcement,omitempty"`
}

type RoomLeftPayload struct {
	Room       string `json:"room"`
	DomainUUID string `json:"domain_uuid,omitempty"`
}

type MemberMutedStatePayload struct {
	Identity string `json:"identity"`
	Muted    bool   `json:"muted"`
}

type SFUProviderChangedPayload struct {
	Provider string `json:"provider"`
}
