package signal

import (
	"testing"
)

func TestHub_WithBroadcaster_RoomCreate(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"test-room"}`)

	hub.mu.RLock()
	_, exists := hub.rooms["test-room"]
	hub.mu.RUnlock()
	if !exists {
		t.Fatal("expected room to be created in hub rooms map")
	}
}

func TestHub_WithBroadcaster_RoomJoin(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"test-room"}`)

	if _, err := hub.OnRoomJoin(conn, `{"room":"test-room","identity":"user-1"}`); err != nil {
		t.Fatalf("OnRoomJoin error: %v", err)
	}

	if !fanout.didJoin("sock-1", "test-room") {
		t.Fatal("expected client to have joined the room in fanout")
	}
}

func TestHub_WithBroadcaster_ClientDisconnect(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnConnect(conn)
	hub.OnRoomCreate(conn, `{"room":"test-room"}`)
	if _, err := hub.OnRoomJoin(conn, `{"room":"test-room","identity":"user-1"}`); err != nil {
		t.Fatalf("OnRoomJoin error: %v", err)
	}

	if !fanout.didJoin("sock-1", "test-room") {
		t.Fatal("expected client to have joined room in fanout")
	}

	hub.OnDisconnect(conn)

	hub.mu.RLock()
	room, exists := hub.rooms["test-room"]
	hub.mu.RUnlock()

	if exists && len(room.Members) > 0 {
		t.Fatal("expected hub room to be empty after disconnect")
	}
}

func TestHub_WithBroadcaster_NamespaceBroadcast(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnConnect(conn)

	hub.publishNamespace("event:global", map[string]string{"msg": "hello"})

	events := fanout.broadcasts["event:global"]
	if len(events) == 0 {
		t.Fatal("expected namespace broadcast to be forwarded to fanout")
	}
}

func TestHub_Fanout_ACL_Isolation(t *testing.T) {
	hub := newTestHub()

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	hub.OnRoomCreate(connA, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomJoinSFU(connA, `{"room":"lobby","domain_uuid":"domain-a","identity":"user-a"}`)

	hub.OnRoomCreate(connB, `{"room":"lobby","domain_uuid":"domain-b"}`)

	hub.mu.RLock()
	_, existsA := hub.rooms["domain-a:lobby"]
	_, existsB := hub.rooms["domain-b:lobby"]
	hub.mu.RUnlock()

	if !existsA || !existsB {
		t.Fatal("expected both domain rooms to exist with different keys")
	}
}

func TestHub_Rooms_DisplayNames(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"user-1"}`)

	rooms := hub.Rooms()
	if len(rooms) == 0 {
		t.Fatal("expected at least 1 room after join")
	}
}
