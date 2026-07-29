package ws

import (
	"testing"
)

func TestFanout_Add_Remove(t *testing.T) {
	f := NewFanout()
	c := NewTestClient("client-1", nil)

	f.Add(c)
	// Check via internal map (same package)
	if _, ok := f.clients["client-1"]; !ok {
		t.Fatal("expected client to be registered")
	}

	f.Remove("client-1")
	if _, ok := f.clients["client-1"]; ok {
		t.Fatal("expected client to be removed")
	}
}

func TestFanout_Join_Leave(t *testing.T) {
	f := NewFanout()
	c := NewTestClient("client-1", nil)
	f.Add(c)

	f.Join("room-a", "client-1")
	if !f.RoomExists("room-a") {
		t.Fatal("expected room-a to exist after join")
	}

	f.Leave("room-a", "client-1")
	if f.RoomExists("room-a") {
		t.Fatal("expected room-a to be removed after leave")
	}
}

func TestFanout_BroadcastToRoom(t *testing.T) {
	f := NewFanout()
	c1 := NewTestClient("c1", nil)
	c2 := NewTestClient("c2", nil)
	f.Add(c1)
	f.Add(c2)
	f.Join("room-x", "c1")
	f.Join("room-x", "c2")

	f.BroadcastToRoom("room-x", "event:test", "hello")

	// c1 and c2 should have received the message in their writeCh
	select {
	case <-c1.writeCh:
	default:
		t.Fatal("expected c1 to receive broadcast")
	}
	select {
	case <-c2.writeCh:
	default:
		t.Fatal("expected c2 to receive broadcast")
	}
}

func TestFanout_BroadcastToNamespace(t *testing.T) {
	f := NewFanout()
	c1 := NewTestClient("c1", nil)
	c2 := NewTestClient("c2", nil)
	f.Add(c1)
	f.Add(c2)

	f.BroadcastToNamespace("event:global", "payload")

	select {
	case <-c1.writeCh:
	default:
		t.Fatal("expected c1 to receive namespace broadcast")
	}
	select {
	case <-c2.writeCh:
	default:
		t.Fatal("expected c2 to receive namespace broadcast")
	}
}

func TestFanout_ForEach(t *testing.T) {
	f := NewFanout()
	for i := 0; i < 3; i++ {
		c := NewTestClient(string(rune('a'+i)), nil)
		f.Add(c)
		f.Join("room-y", c.ID())
	}

	var count int
	f.ForEach("room-y", func(cm ClientMessenger) bool {
		count++
		return true
	})
	if count != 3 {
		t.Fatalf("expected 3 iterations, got %d", count)
	}
}

func TestFanout_ForEach_StopEarly(t *testing.T) {
	f := NewFanout()
	c1 := NewTestClient("c1", nil)
	c2 := NewTestClient("c2", nil)
	f.Add(c1)
	f.Add(c2)
	f.Join("room-z", "c1")
	f.Join("room-z", "c2")

	var count int
	f.ForEach("room-z", func(cm ClientMessenger) bool {
		count++
		return false // stop after first
	})
	if count != 1 {
		t.Fatalf("expected 1 iteration, got %d", count)
	}
}

func TestFanout_RoomCount(t *testing.T) {
	f := NewFanout()
	c := NewTestClient("c1", nil)
	f.Add(c)
	f.Join("room-a", "c1")

	if !f.RoomExists("room-a") {
		t.Fatal("expected room-a to exist")
	}

	// Remove client should clean up the room
	f.Remove("c1")
	if f.RoomExists("room-a") {
		t.Fatal("expected room-a to be cleaned up after client removal")
	}
}

func TestFanout_EmptyRoom_AutoDelete(t *testing.T) {
	f := NewFanout()
	c := NewTestClient("c1", nil)
	f.Add(c)
	f.Join("room-empty", "c1")

	f.Leave("room-empty", "c1")
	if f.RoomExists("room-empty") {
		t.Fatal("expected room to be deleted after last member leaves")
	}
}

func TestFanout_Remove_CleansRooms(t *testing.T) {
	f := NewFanout()
	c := NewTestClient("c1", nil)
	f.Add(c)
	f.Join("room-1", "c1")
	f.Join("room-2", "c1")

	rooms := f.Remove("c1")
	if len(rooms) == 0 {
		t.Fatal("expected rooms to be returned after remove")
	}
	if f.RoomExists("room-1") || f.RoomExists("room-2") {
		t.Fatal("expected both rooms to be cleaned up")
	}
}

func TestFanout_ConcurrentAccess(t *testing.T) {
	f := NewFanout()

	// Add clients concurrently
	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		c := NewTestClient(id, nil)
		f.Add(c)
		f.Join("room-concurrent", id)
	}

	// Broadcast concurrently (should not race)
	done := make(chan struct{})
	go func() {
		f.BroadcastToRoom("room-concurrent", "event:test", "data")
		close(done)
	}()
	go func() {
		f.BroadcastToNamespace("event:other", "data")
	}()
	go func() {
		f.ForEach("room-concurrent", func(cm ClientMessenger) bool { return true })
	}()

	<-done
	// Test passes if no data race (use -race flag)
}
