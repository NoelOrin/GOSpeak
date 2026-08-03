package sfu

// RoomSummary is the provider-agnostic room listing entry.
type RoomSummary struct {
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount,omitempty"`
}

// ParticipantSummary is the provider-agnostic participant entry.
type ParticipantSummary struct {
	Identity string `json:"identity"`
	JoinedAt int64  `json:"joinedAt,omitempty"`
}

// Enforcement levels for mute/kick media actions.
// hard     = native media force
// degraded = substitute media force (rule/kick-stream)
// soft     = signal/policy + client cooperation only
// none     = capability absent
const (
	EnforcementHard     = "hard"
	EnforcementDegraded = "degraded"
	EnforcementSoft     = "soft"
	EnforcementNone     = "none"
)

// PermanentMuteTTLSeconds is used for permanent mutes on providers that require a media rule TTL.
const PermanentMuteTTLSeconds = 365 * 24 * 60 * 60

// Capabilities declares media-layer enforcement for a provider.
// Signaling always runs first; *Level fields describe how media is enforced.
//
// ServerMute/ServerKick remain bool convenience flags:
// true when Level is hard or degraded.
type Capabilities struct {
	ServerMute  bool   `json:"serverMute"`
	ServerKick  bool   `json:"serverKick"`
	DeleteRoom  bool   `json:"deleteRoom"`
	AdminToken  bool   `json:"adminToken"`
	ListRooms   bool   `json:"listRooms"`
	ListMembers bool   `json:"listMembers"`
	MuteLevel   string `json:"muteLevel"`
	KickLevel   string `json:"kickLevel"`
	DeleteLevel string `json:"deleteLevel"`
	ListLevel   string `json:"listLevel"`
	AdminLevel  string `json:"adminLevel"`
}

// TimedMuteProvider optionally accepts TTL for mute enforcement.
// Hub prefers this over MuteParticipant when available.
type TimedMuteProvider interface {
	Provider
	MuteParticipantTimed(room, identity, trackSid string, muted bool, ttlSeconds int) error
}

// NormalizeLevel maps unknown values to soft.
func NormalizeLevel(level string) string {
	switch level {
	case EnforcementHard, EnforcementDegraded, EnforcementSoft, EnforcementNone:
		return level
	default:
		return EnforcementSoft
	}
}

// LevelEnabled reports whether a level is media-enforcing.
func LevelEnabled(level string) bool {
	switch NormalizeLevel(level) {
	case EnforcementHard, EnforcementDegraded:
		return true
	default:
		return false
	}
}

// EnforcementFromLevel returns the event enforcement for a successful media call.
// Failed/unsupported calls should return EnforcementSoft at the call site.
func EnforcementFromLevel(level string) string {
	switch NormalizeLevel(level) {
	case EnforcementHard:
		return EnforcementHard
	case EnforcementDegraded:
		return EnforcementDegraded
	default:
		return EnforcementSoft
	}
}
