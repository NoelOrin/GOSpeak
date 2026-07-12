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
