package signal

type RoomRequest struct {
	Room     string `json:"room"`
	Identity string `json:"identity,omitempty"`
}

type MemberInfo struct {
	ID       string `json:"id"`
	Identity string `json:"identity"`
	JoinedAt int64  `json:"joinedAt"`
}

type RoomInfo struct {
	ID        uint         `json:"id"`
	UUID      string       `json:"uuid"`
	Name      string       `json:"name"`
	Limit     uint         `json:"limit"`
	Members   []MemberInfo `json:"members"`
	Count     int          `json:"count"`
	CreatedAt int64        `json:"createdAt"`
}
