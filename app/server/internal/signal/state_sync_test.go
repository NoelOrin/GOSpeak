package signal

import (
	"context"
	"errors"
	"sync"
	"testing"

	"GOSpeak/internal/bus"
)

var errNotFound = errors.New("not found")

type memStateStore struct {
	mu    sync.Mutex
	rooms map[string]bus.RoomMembersSnapshot
	strm  map[string][2]string // stream -> [room, identity]
}

func newMemStateStore() *memStateStore {
	return &memStateStore{
		rooms: make(map[string]bus.RoomMembersSnapshot),
		strm:  make(map[string][2]string),
	}
}

func (m *memStateStore) PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := snap
	cp.Members = append([]bus.MemberRecord(nil), snap.Members...)
	m.rooms[snap.Room] = cp
	return nil
}

func (m *memStateStore) GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap, ok := m.rooms[room]
	if !ok {
		return bus.RoomMembersSnapshot{}, errNotFound
	}
	return snap, nil
}

func (m *memStateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, room)
	return nil
}

func (m *memStateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strm[stream] = [2]string{room, identity}
	return nil
}

func (m *memStateStore) DeleteStream(ctx context.Context, stream string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.strm, stream)
	return nil
}

func (m *memStateStore) ListRoomNames(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.rooms))
	for k := range m.rooms {
		out = append(out, k)
	}
	return out, nil
}

func TestHub_JoinSFU_WritesMembershipKV(t *testing.T) {
	store := newMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	// seed room via join path pieces
	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name:     "r1",
		Members:  map[string]*MemberInfo{"sock-1": {ID: "sock-1", Identity: "alice", Stream: "gs-a"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()
	hub.syncRoomToStore("r1")
	hub.syncStreamPut("gs-a", "r1", "alice")

	store.mu.Lock()
	snap, ok := store.rooms["r1"]
	store.mu.Unlock()
	if !ok || len(snap.Members) != 1 || snap.Members[0].Identity != "alice" {
		t.Fatalf("membership not written: %+v ok=%v", snap, ok)
	}
	store.mu.Lock()
	pair, ok := store.strm["gs-a"]
	store.mu.Unlock()
	if !ok || pair[0] != "r1" || pair[1] != "alice" {
		t.Fatalf("stream not written: %+v ok=%v", pair, ok)
	}
}

func TestHub_GetRoomMembers_MergesKV(t *testing.T) {
	store := newMemStateStore()
	store.rooms["r1"] = bus.RoomMembersSnapshot{
		Room: "r1",
		Members: []bus.MemberRecord{
			{Identity: "bob", InstanceID: "other", Stream: "gs-b"},
		},
	}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")

	// local only alice
	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name:    "r1",
		Members: map[string]*MemberInfo{"s1": {ID: "s1", Identity: "alice"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()

	members := hub.GetRoomMembersMerged("r1")
	ids := map[string]bool{}
	for _, m := range members {
		ids[m.Identity] = true
	}
	if !ids["alice"] || !ids["bob"] {
		t.Fatalf("expected alice+bob, got %+v", members)
	}
}

func TestHub_SyncRoomToStore_DeletesEmpty(t *testing.T) {
	store := newMemStateStore()
	store.rooms["r1"] = bus.RoomMembersSnapshot{Room: "r1", Members: []bus.MemberRecord{{Identity: "x"}}}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "i")
	hub.syncRoomToStore("r1") // room not in local map
	store.mu.Lock()
	_, ok := store.rooms["r1"]
	store.mu.Unlock()
	if ok {
		t.Fatal("expected KV room deleted when local missing")
	}
}


type captureNotifier struct {
	mu     sync.Mutex
	events []string
	rooms  []string
}

func (c *captureNotifier) PublishInternal(ctx context.Context, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	if m, ok := payload.(map[string]interface{}); ok {
		if r, ok := m["room"].(string); ok {
			c.rooms = append(c.rooms, r)
		}
	}
	return nil
}

func TestHub_SyncRoomToStore_NotifiesPeers(t *testing.T) {
	store := newMemStateStore()
	note := &captureNotifier{}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	hub.SetStateNotifier(note)
	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name: "r1",
		Members: map[string]*MemberInfo{"s1": {ID: "s1", Identity: "alice"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()
	hub.syncRoomToStore("r1")
	note.mu.Lock()
	defer note.mu.Unlock()
	if len(note.events) != 1 || note.events[0] != EventStateRoomChanged {
		t.Fatalf("events=%v", note.events)
	}
	if len(note.rooms) != 1 || note.rooms[0] != "r1" {
		t.Fatalf("rooms=%v", note.rooms)
	}
}

func TestHub_GetMergedRooms_IncludesKVOnlyRoom(t *testing.T) {
	store := newMemStateStore()
	store.rooms["remote-only"] = bus.RoomMembersSnapshot{
		Room: "remote-only",
		Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}},
	}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	rooms := hub.getMergedRooms()
	found := false
	for _, r := range rooms {
		if r.Name == "remote-only" && r.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected remote-only room in list, got %+v", rooms)
	}
}

func TestHub_ApplyRemoteRoomState_BroadcastsLocal(t *testing.T) {
	store := newMemStateStore()
	store.rooms["r1"] = bus.RoomMembersSnapshot{
		Room: "r1",
		Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}},
	}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	server := newMockServer()
	hub.server = server
	hub.ApplyRemoteRoomState("r1")
	if server.broadcasts[EventRoomUpdated] == nil {
		t.Fatal("expected room:updated local broadcast")
	}
	if server.broadcasts[EventRoomListResult] == nil {
		t.Fatal("expected room:list:result local broadcast")
	}
}
