package sfu

// Provider abstracts an SFU backend (LiveKit, SRS, Agora, MediaSoup, Daily, etc.).
type Provider interface {
	GenerateToken(room, identity string) (string, error)
	GenerateAdminToken() (string, error)
	ListRooms() ([]RoomSummary, error)
	ListParticipants(room string) ([]ParticipantSummary, error)
	MuteParticipant(room, identity, trackSid string, muted bool) error
	RemoveParticipant(room, identity string) error
	DeleteRoom(room string) error
	GetHost() string
}

// StreamProvider extends Provider for backends that use stream-based
// addressing (e.g. SRS WHIP/WHEP). Callers check via type assertion.
type StreamProvider interface {
	Provider
	StreamName(room, identity string) string
	StreamInfo(room, identity string) (stream, token string, err error)
}

// ClientInfoProvider extends Provider for backends that expose
// provider-specific connection metadata to the frontend.
type ClientInfoProvider interface {
	Provider
	ClientInfo() map[string]interface{}
}
