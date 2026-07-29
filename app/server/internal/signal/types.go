package signal

type RoomRequest struct {
	Room     string `json:"room"`
	// GuildUUID 命名空间隔离：同一房间名在不同 Guild 下视为不同房间。
	GuildUUID string `json:"guild_uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Identity string `json:"identity,omitempty"`
	Stream   string `json:"stream,omitempty"`
}

type MemberInfo struct {
	ID          string `json:"id"`
	Identity    string `json:"identity"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Avatar      string `json:"avatar"`
	IsMuted     bool   `json:"isMuted"`
	IsMicMuted  bool   `json:"isMicMuted"`
	JoinedAt    int64  `json:"joinedAt"`
	Stream      string `json:"stream,omitempty"`
}

type RoomInfo struct {
	ID            uint         `json:"id"`
	UUID          string       `json:"uuid"`
	Name          string       `json:"name"`
	HasPassword   bool         `json:"hasPassword"`
	Description   string       `json:"description"`
	Limit         uint         `json:"limit"`
	Type          string       `json:"type"`
	AudioOnly     bool         `json:"audioOnly"`
	AllowAudience bool         `json:"allowAudience"`
	Members       []MemberInfo `json:"members"`
	Count         int          `json:"count"`
	CreatedAt     int64        `json:"createdAt"`
}

// MuteInfo is the DTO broadcast to clients for mute/unmute events.
// Lives in the signal package so handler layers reference signal types
// instead of model types for mute event broadcasting.
type MuteInfo struct {
	UserID    uint   `json:"user_id"`
	Duration  int64  `json:"duration"`
	Permanent bool   `json:"permanent"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at,omitempty"`
}
