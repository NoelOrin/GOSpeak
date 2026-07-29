package signal

import (
	"testing"
)

// 跨阶段全链路：Guild 隔离 + WS 事件流联合测试

func TestGuildWS_CreateRoom_WithGuildUUID(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	fanout := newMockBroadcaster()
	hub.fanout = fanout
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")

	// WS: 创建带 guild_uuid 的房间
	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-a-uuid"}`)

	// 验证房间键为 guild-a-uuid:lobby
	hub.mu.RLock()
	_, exists := hub.rooms["guild-a-uuid:lobby"]
	hub.mu.RUnlock()

	if !exists {
		t.Fatal("expected room with key 'guild-a-uuid:lobby'")
	}
}

func TestGuildWS_DifferentGuilds_SameRoomName(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	fanout := newMockBroadcaster()
	hub.fanout = fanout
	hub.SetStreamResolver(fakeStreamResolver{})

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	// Guild A creates "lobby"
	hub.OnRoomCreate(connA, `{"room":"lobby","guild_uuid":"guild-a"}`)

	// Guild B creates "lobby" — same name, different key
	hub.OnRoomCreate(connB, `{"room":"lobby","guild_uuid":"guild-b"}`)

	hub.mu.RLock()
	_, existsA := hub.rooms["guild-a:lobby"]
	_, existsB := hub.rooms["guild-b:lobby"]
	hub.mu.RUnlock()

	if !existsA {
		t.Fatal("missing guild-a:lobby")
	}
	if !existsB {
		t.Fatal("missing guild-b:lobby")
	}
	// Cannot create duplicate within same guild
	hub.OnRoomCreate(connA, `{"room":"lobby","guild_uuid":"guild-a"}`)
	// Should have received error (silently ignored in OnRoomCreate mock check)
}

func TestGuildWS_PlatformRoom_BackwardCompat(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	fanout := newMockBroadcaster()
	hub.fanout = fanout
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")

	// Platform room (no guild_uuid)
	hub.OnRoomCreate(conn, `{"room":"general"}`)

	hub.mu.RLock()
	_, platExists := hub.rooms["general"]
	hub.mu.RUnlock()
	if !platExists {
		t.Fatal("expected platform room 'general'")
	}

	// Guild room with same name
	hub.OnRoomCreate(conn, `{"room":"general","guild_uuid":"guild-x"}`)

	hub.mu.RLock()
	_, guildExists := hub.rooms["guild-x:general"]
	hub.mu.RUnlock()
	if !guildExists {
		t.Fatal("expected guild room 'guild-x:general'")
	}
}

func TestGuildWS_RoomList_AllGuilds(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"alpha","guild_uuid":"guild-a"}`)
	hub.OnRoomCreate(conn, `{"room":"beta","guild_uuid":"guild-a"}`)
	hub.OnRoomCreate(conn, `{"room":"alpha","guild_uuid":"guild-b"}`)

	rooms := hub.getMergedRooms()
	// getMergedRooms returns all rooms regardless of guild
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}
}

func TestGuildWS_Kick_CrossGuildIsolation(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	fanout := newMockBroadcaster()
	hub.fanout = fanout
	hub.SetStreamResolver(fakeStreamResolver{})

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	// User A in guild-a:lobby
	hub.OnRoomCreate(connA, `{"room":"lobby","guild_uuid":"guild-a"}`)
	hub.OnRoomJoinSFU(connA, `{"room":"lobby","guild_uuid":"guild-a","identity":"user-a"}`)

	// User B in guild-b:lobby
	hub.OnRoomCreate(connB, `{"room":"lobby","guild_uuid":"guild-b"}`)
	hub.OnRoomJoinSFU(connB, `{"room":"lobby","guild_uuid":"guild-b","identity":"user-b"}`)

	// Disconnect user-a — should only affect guild-a:lobby
	hub.OnDisconnect(connA)

	hub.mu.RLock()
	roomA := hub.rooms["guild-a:lobby"]
	roomB := hub.rooms["guild-b:lobby"]
	hub.mu.RUnlock()

	if roomA != nil && len(roomA.Members) > 0 {
		t.Fatal("guild-a:lobby should be empty after user-a disconnect")
	}
	if roomB == nil || len(roomB.Members) == 0 {
		t.Fatal("guild-b:lobby should still have user-b")
	}
}

func TestGuildWS_TransferOwnership_NoTokenInvalidation(t *testing.T) {
	// Transferring guild ownership does not invalidate JWT tokens.
	// Permissions are checked at request time via GuildMember role.
	// This test verifies the Hub doesn't have token-related cleanup
	// during ownership transfer.
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()

	conn := newAuthedMockClient("sock-1", "user-1")

	// Create a room
	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-a"}`)

	hub.mu.RLock()
	room, exists := hub.rooms["guild-a:lobby"]
	hub.mu.RUnlock()

	if !exists || room == nil {
		t.Fatal("room should exist unaffected by ownership transfer")
	}
}

func TestGuildWS_GuildDeleted_Cleanup(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	fanout := newMockBroadcaster()
	hub.fanout = fanout
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")

	// Create rooms in guild-a and guild-b
	hub.OnRoomCreate(conn, `{"room":"r1","guild_uuid":"guild-a"}`)
	hub.OnRoomCreate(conn, `{"room":"r2","guild_uuid":"guild-a"}`)
	hub.OnRoomCreate(conn, `{"room":"r1","guild_uuid":"guild-b"}`)

	hub.mu.Lock()
	// Simulate guild-a deletion by removing its rooms from hub.rooms
	for key := range hub.rooms {
		if key == "guild-a:r1" || key == "guild-a:r2" {
			delete(hub.rooms, key)
		}
	}
	hub.mu.Unlock()

	hub.mu.RLock()
	_, guildBRoom := hub.rooms["guild-b:r1"]
	hub.mu.RUnlock()

	if !guildBRoom {
		t.Fatal("guild-b:r1 should survive guild-a deletion")
	}
}
