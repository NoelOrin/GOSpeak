package pkg

import "strings"

// RoomKey builds the domain-scoped composite room key used by signal, media
// providers and shared state. Platform rooms keep the logical name.
func RoomKey(domainUUID, roomName string) string {
	if domainUUID == "" {
		return roomName
	}
	return domainUUID + ":" + roomName
}

// SplitRoomKey reverses RoomKey.
func SplitRoomKey(key string) (domainUUID, roomName string) {
	var found bool
	domainUUID, roomName, found = strings.Cut(key, ":")
	if !found {
		domainUUID = ""
		roomName = key
	}
	return domainUUID, roomName
}
