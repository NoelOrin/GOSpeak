package signal

import (
	"testing"
	"time"
)

// ─── Integration: Full Room Lifecycle ───

func TestRoomLifecycle_CreateJoinLeave(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	// Step 1: Create room
	creator := newMockConn("creator-socket")
	hub.OnRoomCreate(creator, `{"room":"meeting-room"}`)

	if _, exists := hub.rooms["meeting-room"]; !exists {
		t.Fatal("Step 1: room not created")
	}
	if creator.emitted[EventRoomCreated] == nil {
		t.Fatal("Step 1: room:created not emitted")
	}

	// Step 2: User 1 joins
	user1 := newMockConn("socket-user1")
	hub.OnRoomJoin(user1, `{"room":"meeting-room","identity":"alice"}`)

	if !user1.joined["meeting-room"] {
		t.Fatal("Step 2: user1 did not join room")
	}
	emitted1, ok := user1.emitted[EventRoomJoined].([]interface{})
	if !ok || len(emitted1) == 0 {
		t.Fatal("Step 2: room:joined not emitted")
	}
	data1, _ := emitted1[0].(map[string]interface{})
	members1, _ := data1["members"].([]interface{})
	if len(members1) != 1 {
		t.Errorf("Step 2: expected 1 member, got %d", len(members1))
	}

	// Step 3: User 2 joins
	user2 := newMockConn("socket-user2")
	hub.OnRoomJoin(user2, `{"room":"meeting-room","identity":"bob"}`)

	if !user2.joined["meeting-room"] {
		t.Fatal("Step 3: user2 did not join room")
	}
	emitted2, ok := user2.emitted[EventRoomJoined].([]interface{})
	if !ok || len(emitted2) == 0 {
		t.Fatal("Step 3: room:joined not emitted to user2")
	}
	data2, _ := emitted2[0].(map[string]interface{})
	members2, _ := data2["members"].([]interface{})
	if len(members2) != 2 {
		t.Errorf("Step 3: expected 2 members, got %d", len(members2))
	}

	// Step 4: List rooms
	lister := newMockConn("socket-lister")
	hub.OnRoomList(lister)

	emittedList, ok := lister.emitted[EventRoomListResult].([]interface{})
	if !ok || len(emittedList) == 0 {
		t.Fatal("Step 4: room:list:result not emitted")
	}
	dataList, _ := emittedList[0].(map[string]interface{})
	if count, _ := dataList["count"].(int); count != 1 {
		t.Errorf("Step 4: expected 1 room, got %d", count)
	}

	// Step 5: User 1 leaves
	hub.OnRoomLeave(user1, `{"room":"meeting-room"}`)

	if !user1.left["meeting-room"] {
		t.Fatal("Step 5: user1 did not leave room")
	}
	if len(hub.rooms["meeting-room"].Members) != 1 {
		t.Errorf("Step 5: expected 1 member remaining, got %d", len(hub.rooms["meeting-room"].Members))
	}

	// Step 6: User 2 leaves — room should be deleted
	hub.OnRoomLeave(user2, `{"room":"meeting-room"}`)

	if !user2.left["meeting-room"] {
		t.Fatal("Step 6: user2 did not leave room")
	}
	if _, exists := hub.rooms["meeting-room"]; exists {
		t.Fatal("Step 6: empty room not deleted")
	}

	// Step 7: Verify no rooms remain
	finalList := newMockConn("socket-final")
	hub.OnRoomList(finalList)

	emittedFinal, _ := finalList.emitted[EventRoomListResult].([]interface{})
	dataFinal, _ := emittedFinal[0].(map[string]interface{})
	if finalCount, _ := dataFinal["count"].(int); finalCount != 0 {
		t.Errorf("Step 7: expected 0 rooms, got %d", finalCount)
	}
}

// ─── Integration: Multiple Rooms with Concurrent Users ───

func TestRoomLifecycle_MultipleRooms(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	for _, roomName := range []string{"room-1", "room-2", "room-3"} {
		creator := newMockConn("creator-" + roomName)
		hub.OnRoomCreate(creator, `{"room":"`+roomName+`"}`)
	}

	user1 := newMockConn("socket-1")
	hub.OnRoomJoin(user1, `{"room":"room-1","identity":"user1"}`)

	user2 := newMockConn("socket-2")
	hub.OnRoomJoin(user2, `{"room":"room-2","identity":"user2"}`)

	user3 := newMockConn("socket-3")
	hub.OnRoomJoin(user3, `{"room":"room-1","identity":"user3"}`)

	user4 := newMockConn("socket-4")
	hub.OnRoomJoin(user4, `{"room":"room-3","identity":"user4"}`)

	if len(hub.rooms) != 3 {
		t.Errorf("expected 3 rooms, got %d", len(hub.rooms))
	}
	if len(hub.rooms["room-1"].Members) != 2 {
		t.Errorf("room-1: expected 2 members, got %d", len(hub.rooms["room-1"].Members))
	}
	if len(hub.rooms["room-2"].Members) != 1 {
		t.Errorf("room-2: expected 1 member, got %d", len(hub.rooms["room-2"].Members))
	}

	// user1 disconnects — room-1 still has user3
	hub.OnDisconnect(user1, "disconnect")
	if len(hub.rooms["room-1"].Members) != 1 {
		t.Errorf("after user1 disconnect: room-1 should have 1 member, got %d", len(hub.rooms["room-1"].Members))
	}

	// user3 disconnects — room-1 should be deleted
	hub.OnDisconnect(user3, "disconnect")
	if _, exists := hub.rooms["room-1"]; exists {
		t.Fatal("room-1 should be deleted after last member disconnects")
	}

	if len(hub.rooms) != 2 {
		t.Errorf("expected 2 rooms remaining, got %d", len(hub.rooms))
	}
}

// ─── Integration: Rejoin After Leave ───

func TestRoomLifecycle_RejoinAfterLeave(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	user := newMockConn("socket-1")

	// First join
	hub.OnRoomJoin(user, `{"room":"test-room","identity":"alice"}`)
	room1 := hub.rooms["test-room"]
	joinedAt1 := room1.Members["socket-1"].JoinedAt

	// Leave
	hub.OnRoomLeave(user, `{"room":"test-room"}`)
	if _, exists := hub.rooms["test-room"]; exists {
		t.Fatal("room should be deleted after last member leaves")
	}

	// Brief sleep to ensure different timestamp
	time.Sleep(time.Millisecond)

	// Rejoin — creates new room instance
	hub.OnRoomJoin(user, `{"room":"test-room","identity":"alice"}`)
	room2 := hub.rooms["test-room"]

	if room1 == room2 {
		t.Fatal("expected new room instance on rejoin")
	}
	if _, exists := room2.Members["socket-1"]; !exists {
		t.Fatal("user should be back in room")
	}
	if joinedAt1 == room2.Members["socket-1"].JoinedAt {
		t.Fatal("expected new JoinedAt timestamp on rejoin")
	}
}

// ─── Integration: Same User in Multiple Rooms ───

func TestRoomLifecycle_SameUserMultipleRooms(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	user := newMockConn("socket-1")

	hub.OnRoomJoin(user, `{"room":"room-1","identity":"alice"}`)
	hub.OnRoomJoin(user, `{"room":"room-2","identity":"alice"}`)

	if len(hub.rooms["room-1"].Members) != 1 {
		t.Errorf("room-1: expected 1 member, got %d", len(hub.rooms["room-1"].Members))
	}
	if len(hub.rooms["room-2"].Members) != 1 {
		t.Errorf("room-2: expected 1 member, got %d", len(hub.rooms["room-2"].Members))
	}

	// Leave room-1
	hub.OnRoomLeave(user, `{"room":"room-1"}`)
	if _, exists := hub.rooms["room-1"]; exists {
		t.Fatal("room-1 should be deleted")
	}
	if _, exists := hub.rooms["room-2"]; !exists {
		t.Fatal("room-2 should still exist")
	}
	if len(hub.rooms["room-2"].Members) != 1 {
		t.Errorf("room-2: expected 1 member, got %d", len(hub.rooms["room-2"].Members))
	}
}

// ─── Integration: CreatedAt Timestamp ───

func TestRoomLifecycle_CreatedAtTimestamp(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	creator := newMockConn("creator")
	before := time.Now()
	hub.OnRoomCreate(creator, `{"room":"test-room"}`)
	after := time.Now()

	room := hub.rooms["test-room"]
	if room.CreatedAt.Before(before) || room.CreatedAt.After(after) {
		t.Errorf("room CreatedAt should be between %v and %v, got %v", before, after, room.CreatedAt)
	}
}
