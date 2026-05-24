package sign

type RoomClientStruct struct {
	HostURL   string
	ApiKey    string
	ApiSecret string
	RoomName  string
	// 可能没用
	// ParticipantIdentity string
}

var RoomClient RoomClientStruct

func init() {
	RoomClient = RoomClientStruct{
		HostURL:   "",
		ApiKey:    "",
		ApiSecret: "",
	}
}

func ListRooms(RC *RoomClientStruct) {

}

func ListRoomParticipants(RC *RoomClientStruct) {

}

func DisconnectParticipant(RC *RoomClientStruct) {

}

// MuteParticipantInRoom 房间级禁言
func MuteParticipantInRoom(RC *RoomClientStruct) {

}
func DeleteRoom(RC *RoomClientStruct) {

}
