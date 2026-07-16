package sfu

// Provider abstracts an SFU backend (LiveKit, SRS, Agora, MediaSoup, Daily, etc.).
//
// Capability contract:
//   - Supported operations return concrete results / nil error.
//   - Unsupported operations MUST return pkg.NewErrSFUNotSupported()
//     so callers can degrade with errors.Is(err, pkg.ErrSFUNotSupported).
//   - Missing configuration should return SFU_NOT_CONFIGURED, not "not supported".
//   - Capabilities() is declarative and must match the real method behavior.
type Provider interface {
	// ProviderName returns the stable provider id (livekit/agora/...).
	ProviderName() string
	// Capabilities returns media-layer hard-enforcement support for this provider.
	Capabilities() Capabilities
	GenerateToken(room, identity string) (string, error)
	// GenerateAdminToken returns a management token when the backend has that concept.
	// Providers without admin-token semantics return pkg.NewErrSFUNotSupported().
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
