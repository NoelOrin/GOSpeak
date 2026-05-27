package signal

import (
	"testing"
	"time"
)

// ─── Integration: Full Room Lifecycle ───

func TestRoomLifecycle_CreateJoinLeave(t *testing.T) {
	// Setup
	hub := NewHub()
	server := newMockServer()
	hub.SetServer(server)

	// === Step 1: Create room ===
	creator := newMockConn("creator-socket")
	createData := `{"room":"meeting-room"}`
	hub.OnRoomCreate(creator, createData)

	// Verify room created
	if _, exists := hub.rooms["meeting-room"]; !exists {
		t.Fatal("Step 1 Failed: room not created")
	}

	if creator.emitted[EventRoomCreated] == nil {
		t.Fatal("Step 1 Failed: room:created not emitted")
	}

	// === Step 2: User 1 joins ===
	user1 := newMockConn("socket-user1")
	joinData1 := `{"room":"meeting-room","identity":"alice"}`
	hub.OnRoomJoin(user1, joinData1)

	// Verify user1 joined
	if !user1.joined["meeting-room"] {
		t.Fatal("Step 2 Failed: user1 did not join room")
	}

	emitted1, ok := user1.emitted[EventRoomJoined].([]interface{})
	if !ok || len(emitted1) == 0 {
		t.Fatal("Step 2 Failed: room:joined not emitted")
	}

	data1, _ := emitted1[0].(map[string]interface{})
	members1, _ := data1["members"].([]interface{})
	if len(members1) != 1 {
		t.Errorf("Step 2 Failed: expected 1 member, got %d", len(members1))
	}

	// === Step 3: User 2 joins ===
	user2 := newMockConn("socket-user2")
	joinData2 := `{"room":"meeting-room","identity":"bob"}`
	hub.OnRoomJoin(user2, joinData2)

	// Verify user2 joined
	if !user2.joined["meeting-room"] {
		t.Fatal("Step 3 Failed: user2 did not join room")
	}

	emitted2, ok := user2.emitted[EventRoomJoined].([]interface{})
	if !ok || len(emitted2) == 0 {
		t.Fatal("Step 3 Failed: room:joined not emitted to user2")
	}

	data2, _ := emitted2[0].(map[string]interface{})
	members2, _ := data2["members"].([]interface{})
	if len(members2) != 2 {
		t.Errorf("Step 3 Failed: expected 2 members, got %d", len(members2))
	}

	// === Step 4: List rooms ===
	lister := newMockConn("socket-lister")
	hub.OnRoomList(lister)

	emittedList, ok := lister.emitted[EventRoomListResult].([]interface{})
	if !ok || len(emittedList) == 0 {
		t.Fatal("Step 4 Failed: room:list:result not emitted")
	}

	dataList, _ := emittedList[0].(map[string]interface{})
	roomCount, _ := dataList["count"].(int)
	if roomCount != 1 {
		t.Errorf("Step 4 Failed: expected 1 room, got %d", roomCount)
	}

	// === Step 5: User 1 leaves ===
	leaveData1 := `{"room":"meeting-room"}`
	hub.OnRoomLeave(user1, leaveData1)

	// Verify user1 left
	if !user1.left["meeting-room"] {
		t.Fatal("Step 5 Failed: user1 did not leave room")
	}

	room := hub.rooms["meeting-room"]
	if len(room.Members) != 1 {
		t.Errorf("Step 5 Failed: expected 1 member remaining, got %d", len(room.Members))
	}

	// === Step 6: User 2 leaves ===
	leaveData2 := `{"room":"meeting-room"}`
	hub.OnRoomLeave(user2, leaveData2)

	// Verify user2 left and room is empty
	if !user2.left["meeting-room"] {
		t.Fatal("Step 6 Failed: user2 did not leave room")
	}

	if _, exists := hub.rooms["meeting-room"]; exists {
		t.Fatal("Step 6 Failed: empty room not deleted")
	}

	// === Step 7: Verify no rooms remain ===
	finalList := newMockConn("socket-final")
	hub.OnRoomList(finalList)

	emittedFinal, _ := finalList.emitted[EventRoomListResult].([]interface{})
	dataFinal, _ := emittedFinal[0].(map[string]interface{})
	finalCount, _ := dataFinal["count"].(int)

	if finalCount != 0 {
		t.Errorf("Step 7 Failed: expected 0 rooms, got %d", finalCount)
	}
}

// ─── Integration: Concurrent Users in Multiple Rooms ───

func TestRoomLifecycle_MultipleRooms(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.SetServer(server)

	// Create rooms
	for _, roomName := range []string{"room-1", "room-2", "room-3"} {
		creator := newMockConn("creator-" + roomName)
		data := `{"room":"` + roomName + `"}`
		hub.OnRoomCreate(creator, data)
	}

	// Join users to rooms
	user1 := newMockConn("socket-1")
	hub.OnRoomJoin(user1, `{"room":"room-1","identity":"user1"}`)

	user2 := newMockConn("socket-2")
	hub.OnRoomJoin(user2, `{"room":"room-2","identity":"user2"}`)

	user3 := newMockConn("socket-3")
	hub.OnRoomJoin(user3, `{"room":"room-1","identity":"user3"}`)

	user4 := newMockConn("socket-4")
	hub.OnRoomJoin(user4, `{"room":"room-3","identity":"user4"}`)

	// Verify state
	if len(hub.rooms) != 3 {
		t.Errorf("expected 3 rooms, got %d", len(hub.rooms))
	}

	if len(hub.rooms["room-1"].Members) != 2 {
		t.Errorf("room-1: expected 2 members, got %d", len(hub.rooms["room-1"].Members))
	}

	if len(hub.rooms["room-2"].Members) != 1 {
		t.Errorf("room-2: expected 1 member, got %d", len(hub.rooms["room-2"].Members))
	}

	if len(hub.rooms["room-3"].Members) != 1 {
		t.Errorf("room-3: expected 1 member, got %d", len(hub.rooms["room-3"].Members))
	}

	// Simulate user1 disconnect
	hub.OnDisconnect(user1, "disconnect")

	// room-1 should still exist with user3
	if len(hub.rooms["room-1"].Members) != 1 {
		t.Errorf("after user1 disconnect: room-1 should have 1 member, got %d", len(hub.rooms["room-1"].Members))
	}

	// Simulate user3 disconnect (room-1 should be cleaned up)
	hub.OnDisconnect(user3, "disconnect")

	if _, exists := hub.rooms["room-1"]; exists {
		t.Fatal("room-1 should be deleted after last member disconnect")
	}

	// room-2 and room-3 should still exist
	if len(hub.rooms) != 2 {
		t.Errorf("expected 2 rooms remaining, got %d", len(hub.rooms))
	}
}

// ─── Integration: Edge Cases ───

func TestRoomLifecycle_RejoinAfterLeave(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.SetServer(server)

	user := newMockConn("socket-1")

	// First join
	hub.OnRoomJoin(user, `{"room":"test-room","identity":"alice"}`)
	room1 := hub.rooms["test-room"]
	member1 := room1.Members["socket-1"]

	// Leave
	hub.OnRoomLeave(user, `{"room":"test-room"}`)
	if _, exists := hub.rooms["test-room"]; exists {
		t.Fatal("room should be deleted after leaving")
	}

	// Rejoin (should create new room instance)
	hub.OnRoomJoin(user, `{"room":"test-room","identity":"alice"}`)
	room2 := hub.rooms["test-room"]

	if room1 == room2 {
		t.Fatal("expected new room instance on rejoin")
	}

	if _, exists := room2.Members["socket-1"]; !exists {
		t.Fatal("user should be back in room")
	}

	if member1.JoinedAt == room2.Members["socket-1"].JoinedAt {
		t.Fatal("expected new JoinedAt timestamp")
	}
}

func TestRoomLifecycle_SameUserMultipleRooms(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.SetServer(server)

	user := newMockConn("socket-1")

	// Join room-1
	hub.OnRoomJoin(user, `{"room":"room-1","identity":"alice"}`)
	if !user.joined["room-1"] {
		t.Fatal("expected join room-1")
	}

	// Join room-2
	hub.OnRoomJoin(user, `{"room":"room-2","identity":"alice"}`)
	if !user.joined["room-2"] {
		t.Fatal("expected join room-2")
	}

	// Both rooms should have the user
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

	// room-2 should still exist
	if _, exists := hub.rooms["room-2"]; !exists {
		t.Fatal("room-2 should still exist")
	}

	if len(hub.rooms["room-2"].Members) != 1 {
		t.Errorf("room-2: expected 1 member, got %d", len(hub.rooms["room-2"].Members))
	}
}

func TestRoomLifecycle_CreatedAtTimestamp(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.SetServer(server)

	creator := newMockConn("creator")
	before := time.Now()
	hub.OnRoomCreate(creator, `{"room":"test-room"}`)
	after := time.Now()

	room := hub.rooms["test-room"]
	if room.CreatedAt.Before(before) || room.CreatedAt.After(after) {
		t.Errorf("room CreatedAt should be between %v and %v, got %v", before, after, room.CreatedAt)
	}
}
