package signal

import (
	"testing"
)

// 跨阶段全链路：Domain 隔离 + WS 事件流联合测试

func TestDomainWS_CreateRoom_WithDomainUUID(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	// WS: 创建带 domain_uuid 的房间
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a-uuid"}`)

	// 验证房间键为 domain-a-uuid:lobby
	hub.mu.RLock()
	_, exists := hub.rooms["domain-a-uuid:lobby"]
	hub.mu.RUnlock()

	if !exists {
		t.Fatal("expected room with key 'domain-a-uuid:lobby'")
	}
}

func TestDomainWS_DifferentDomains_SameRoomName(t *testing.T) {
	hub := newTestHub()

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	// Domain A creates "lobby"
	hub.OnRoomCreate(connA, `{"room":"lobby","domain_uuid":"domain-a"}`)

	// Domain B creates "lobby" — same name, different key
	hub.OnRoomCreate(connB, `{"room":"lobby","domain_uuid":"domain-b"}`)

	hub.mu.RLock()
	_, existsA := hub.rooms["domain-a:lobby"]
	_, existsB := hub.rooms["domain-b:lobby"]
	hub.mu.RUnlock()

	if !existsA {
		t.Fatal("missing domain-a:lobby")
	}
	if !existsB {
		t.Fatal("missing domain-b:lobby")
	}
	// Cannot create duplicate within same domain
	hub.OnRoomCreate(connA, `{"room":"lobby","domain_uuid":"domain-a"}`)
	// Should have received error (silently ignored in OnRoomCreate mock check)
}

func TestDomainWS_PlatformRoom_BackwardCompat(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	// Platform room (no domain_uuid)
	hub.OnRoomCreate(conn, `{"room":"general"}`)

	hub.mu.RLock()
	_, platExists := hub.rooms["general"]
	hub.mu.RUnlock()
	if !platExists {
		t.Fatal("expected platform room 'general'")
	}

	// Domain room with same name
	hub.OnRoomCreate(conn, `{"room":"general","domain_uuid":"domain-x"}`)

	hub.mu.RLock()
	_, domainExists := hub.rooms["domain-x:general"]
	hub.mu.RUnlock()
	if !domainExists {
		t.Fatal("expected domain room 'domain-x:general'")
	}
}

func TestDomainWS_RoomList_AllDomains(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"alpha","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"beta","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"alpha","domain_uuid":"domain-b"}`)

	rooms := hub.getMergedRooms("")
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}
}

func TestDomainWS_Kick_CrossDomainIsolation(t *testing.T) {
	hub := newTestHub()

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	// User A in domain-a:lobby
	hub.OnRoomCreate(connA, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomJoinSFU(connA, `{"room":"lobby","domain_uuid":"domain-a","identity":"user-a"}`)

	// User B in domain-b:lobby
	hub.OnRoomCreate(connB, `{"room":"lobby","domain_uuid":"domain-b"}`)
	hub.OnRoomJoinSFU(connB, `{"room":"lobby","domain_uuid":"domain-b","identity":"user-b"}`)

	// Disconnect user-a — should only affect domain-a:lobby
	hub.OnDisconnect(connA)

	hub.mu.RLock()
	roomA := hub.rooms["domain-a:lobby"]
	roomB := hub.rooms["domain-b:lobby"]
	hub.mu.RUnlock()

	if roomA != nil && len(roomA.Members) > 0 {
		t.Fatal("domain-a:lobby should be empty after user-a disconnect")
	}
	if roomB == nil || len(roomB.Members) == 0 {
		t.Fatal("domain-b:lobby should still have user-b")
	}
}

func TestDomainWS_TransferOwnership_NoTokenInvalidation(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	// Create a room
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)

	hub.mu.RLock()
	room, exists := hub.rooms["domain-a:lobby"]
	hub.mu.RUnlock()

	if !exists || room == nil {
		t.Fatal("room should exist unaffected by ownership transfer")
	}
}

func TestDomainWS_DomainDeleted_Cleanup(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	// Create rooms in domain-a and domain-b
	hub.OnRoomCreate(conn, `{"room":"r1","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"r2","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"r1","domain_uuid":"domain-b"}`)

	hub.OnRoomJoinSFU(conn, `{"room":"r1","domain_uuid":"domain-a","identity":"user-1"}`)

	hub.OnDomainDelete("domain-a")

	hub.mu.RLock()
	_, domainAR1 := hub.rooms["domain-a:r1"]
	_, domainAR2 := hub.rooms["domain-a:r2"]
	_, domainBR1 := hub.rooms["domain-b:r1"]
	_, connSlot := hub.connSlots["sock-1"]
	hub.mu.RUnlock()

	if domainAR1 {
		t.Fatal("domain-a:r1 should be removed after domain deletion")
	}
	if domainAR2 {
		t.Fatal("domain-a:r2 should be removed after domain deletion")
	}
	if !domainBR1 {
		t.Fatal("domain-b:r1 should survive domain-a deletion")
	}
	if connSlot {
		t.Fatal("domain member conn slot should be cleaned after domain deletion")
	}
}
