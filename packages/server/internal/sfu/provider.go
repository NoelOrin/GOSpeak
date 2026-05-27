package sfu

// Provider abstracts an SFU backend (LiveKit, Janus, Pion/ion-sfu, etc.).
// Implementations must satisfy all methods to be usable via sfu.NewProvider().
type Provider interface {
	// GenerateToken creates a join token for a room/identity pair.
	GenerateToken(room, identity string) (string, error)

	// GenerateAdminToken creates an admin-scoped token (room create/list).
	GenerateAdminToken() (string, error)

	// ListRooms returns all active rooms from the SFU.
	ListRooms() (interface{}, error)

	// ListParticipants returns participants in a given room.
	ListParticipants(room string) (interface{}, error)

	// MuteParticipant mutes a participant in a room.
	MuteParticipant(room, identity string) error

	// RemoveParticipant removes a participant from a room.
	RemoveParticipant(room, identity string) error

	// DeleteRoom removes a room from the SFU.
	DeleteRoom(room string) error
}
