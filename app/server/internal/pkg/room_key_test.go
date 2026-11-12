package pkg

import "testing"

func TestRoomKey(t *testing.T) {
	tests := []struct {
		name       string
		domainUUID string
		roomName   string
		want       string
	}{
		{"domain scoped", "domain-a", "lobby", "domain-a:lobby"},
		{"platform room", "", "lobby", "lobby"},
		{"empty room name", "domain-a", "", "domain-a:"},
		{"both empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoomKey(tt.domainUUID, tt.roomName); got != tt.want {
				t.Fatalf("RoomKey(%q, %q) = %q, want %q", tt.domainUUID, tt.roomName, got, tt.want)
			}
		})
	}
}

func TestSplitRoomKey(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantDomain string
		wantRoom   string
	}{
		{"domain scoped", "domain-a:lobby", "domain-a", "lobby"},
		{"platform room", "lobby", "", "lobby"},
		{"empty parts", ":", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domainUUID, roomName := SplitRoomKey(tt.key)
			if domainUUID != tt.wantDomain || roomName != tt.wantRoom {
				t.Fatalf("SplitRoomKey(%q) = (%q, %q), want (%q, %q)", tt.key, domainUUID, roomName, tt.wantDomain, tt.wantRoom)
			}
		})
	}
}
