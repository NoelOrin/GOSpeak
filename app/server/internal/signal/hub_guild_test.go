package signal

import (
	"testing"
)

func TestHub_RoomKey_Format(t *testing.T) {
	tests := []struct {
		guildUUID string
		roomName  string
		want      string
	}{
		{"guild-uuid-1", "lobby", "guild-uuid-1:lobby"},
		{"", "lobby", "lobby"},
		{"guild-uuid-1", "", "guild-uuid-1:"},
		{"", "", ""},
	}
	for _, tt := range tests {
		got := roomKey(tt.guildUUID, tt.roomName)
		if got != tt.want {
			t.Errorf("roomKey(%q, %q) = %q, want %q", tt.guildUUID, tt.roomName, got, tt.want)
		}
	}
}

func TestHub_GuildRoomIsolation(t *testing.T) {
	hub := newTestHub()

	guildA := newAuthedMockClient("sock-a", "user-a")
	guildB := newAuthedMockClient("sock-b", "user-b")

	hub.OnRoomCreate(guildA, `{"room":"lobby","guild_uuid":"guild-a-uuid"}`)
	hub.OnRoomCreate(guildB, `{"room":"lobby","guild_uuid":"guild-b-uuid"}`)

	hub.mu.RLock()
	_, existsA := hub.rooms["guild-a-uuid:lobby"]
	_, existsB := hub.rooms["guild-b-uuid:lobby"]
	hub.mu.RUnlock()

	if !existsA {
		t.Fatal("expected room 'guild-a-uuid:lobby' to exist")
	}
	if !existsB {
		t.Fatal("expected room 'guild-b-uuid:lobby' to exist")
	}
	if len(hub.rooms) != 2 {
		t.Fatalf("expected 2 rooms in hub, got %d", len(hub.rooms))
	}
}

func TestHub_GuildRoomCreate(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"game-room","guild_uuid":"guild-x","password":"secret"}`)

	expectedKey := "guild-x:game-room"
	hub.mu.RLock()
	room, exists := hub.rooms[expectedKey]
	hub.mu.RUnlock()

	if !exists {
		t.Fatalf("expected room with key %q to exist", expectedKey)
	}
	if room.Name != "game-room" {
		t.Fatalf("expected room name 'game-room', got %q", room.Name)
	}
	if room.Password != "secret" {
		t.Fatalf("expected password 'secret', got %q", room.Password)
	}
}

func TestHub_PlatformRoomCompat(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"general"}`)

	hub.mu.RLock()
	_, platformExists := hub.rooms["general"]
	hub.mu.RUnlock()
	if !platformExists {
		t.Fatal("expected platform room 'general' to exist")
	}

	hub.OnRoomCreate(conn, `{"room":"general","guild_uuid":"guild-y"}`)
	hub.mu.RLock()
	_, guildRoomExists := hub.rooms["guild-y:general"]
	hub.mu.RUnlock()

	if !guildRoomExists {
		t.Fatal("expected guild-scoped room 'guild-y:general' to exist separately")
	}
}

func TestHub_GuildDisconnect_CrossGuildIsolation(t *testing.T) {
	hub := newTestHub()

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	hub.OnRoomCreate(connA, `{"room":"lobby","guild_uuid":"guild-a"}`)
	hub.OnRoomJoinSFU(connA, `{"room":"lobby","guild_uuid":"guild-a","identity":"user-a"}`)

	hub.OnRoomCreate(connB, `{"room":"lobby","guild_uuid":"guild-b"}`)
	hub.OnRoomJoinSFU(connB, `{"room":"lobby","guild_uuid":"guild-b","identity":"user-b"}`)

	hub.OnDisconnect(connA)

	hub.mu.RLock()
	roomA := hub.rooms["guild-a:lobby"]
	roomB := hub.rooms["guild-b:lobby"]
	hub.mu.RUnlock()

	if roomA != nil && len(roomA.Members) > 0 {
		t.Fatal("expected guild-a:lobby to be empty after user-a disconnect")
	}

	if roomB == nil {
		t.Fatal("expected guild-b:lobby to still exist")
	}
	if len(roomB.Members) == 0 {
		t.Fatal("expected guild-b:lobby to have members")
	}
}

func TestHub_GuildRoomBroadcast(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-a"}`)

	hub.BroadcastToRoom("guild-a:lobby", "event:test", map[string]string{"msg": "hello"})

	calls := fanout.roomCasts["guild-a:lobby"]["event:test"]
	if len(calls) == 0 {
		t.Fatal("expected BroadcastToRoom for guild-a:lobby event:test")
	}
}

func TestHub_GuildMemberVisibility(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-a"}`)
	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-b"}`)
	hub.OnRoomCreate(conn, `{"room":"general","guild_uuid":"guild-a"}`)

	rooms := hub.getMergedRooms()
	if len(rooms) != 3 {
		t.Fatalf("expected 3 merged rooms, got %d", len(rooms))
	}
}
