package signal

import (
	"testing"
	"time"
)

func newTestHubWithKV(t *testing.T) *Hub {
	t.Helper()
	h := NewHub(nil, nil, nil, nil)
	h.SetMembershipStore(newMemStateStore(), "test-instance")
	return h
}

func (h *Hub) createLocalRoom(domainUUID, roomName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rooms[roomKey(domainUUID, roomName)] = &Room{
		Name:       roomName,
		Members:    map[string]*MemberInfo{},
		ByIdentity: map[string]*MemberInfo{},
		MicMuted:   map[string]bool{},
		Speaking:   map[string]bool{},
		CreatedAt:  time.Now(),
	}
}

func testHubKVCallCount(t *testing.T, h *Hub) struct{ GetRoomMembers int } {
	t.Helper()
	store, ok := h.membershipStore.(*memStateStore)
	if !ok {
		t.Fatalf("expected *memStateStore, got %T", h.membershipStore)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return struct{ GetRoomMembers int }{GetRoomMembers: store.getRoomMembersCalls}
}

func TestRoomList_BatchLocalKV(t *testing.T) {
	h := newTestHubWithKV(t)
	h.createLocalRoom("dom1", "r1")
	h.createLocalRoom("dom1", "r2")
	got := h.getMergedRooms("")
	if len(got) < 2 {
		t.Fatalf("expected >=2 rooms, got %d", len(got))
	}
	if calls := testHubKVCallCount(t, h); calls.GetRoomMembers != 0 {
		t.Fatalf("expected batched KV gets (0 individual calls), got %d", calls.GetRoomMembers)
	}
}
