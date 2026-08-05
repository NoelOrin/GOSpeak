package signal

import (
	"context"
	"errors"
	"sync"
	"testing"

	"GOSpeak/internal/bus"

	"github.com/nats-io/nats.go"
)

var errCASConflict = nats.ErrKeyExists

var errNotFound = errors.New("not found")

type memStateStore struct {
	mu    sync.Mutex
	rooms map[string]bus.RoomMembersSnapshot
	metas map[string]bus.RoomMeta
	strm  map[string][2]string // stream -> [room, identity]
	revs  map[string]uint64
	// getRoomMembersCalls records GetRoomMembers invocations for batch regression assertions.
	getRoomMembersCalls int
}

func newMemStateStore() *memStateStore {
	return &memStateStore{
		rooms: make(map[string]bus.RoomMembersSnapshot),
		metas: make(map[string]bus.RoomMeta),
		strm:  make(map[string][2]string),
		revs:  make(map[string]uint64),
	}
}

func (m *memStateStore) PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := snap
	cp.Members = append([]bus.MemberRecord(nil), snap.Members...)
	m.rooms[snap.Room] = cp
	m.revs[snap.Room]++
	return nil
}

func (m *memStateStore) PutRoomMembersRev(ctx context.Context, snap bus.RoomMembersSnapshot, rev uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rev == 0 {
		if _, ok := m.rooms[snap.Room]; ok {
			return errCASConflict
		}
	} else if m.revs[snap.Room] != rev {
		return errCASConflict
	}
	cp := snap
	cp.Members = append([]bus.MemberRecord(nil), snap.Members...)
	m.rooms[snap.Room] = cp
	m.revs[snap.Room]++
	return nil
}

func (m *memStateStore) GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getRoomMembersCalls++
	snap, ok := m.rooms[room]
	if !ok {
		return bus.RoomMembersSnapshot{}, errNotFound
	}
	return snap, nil
}

// GetRoomMembersBatch 批量读取多个房间的成员快照，模拟生产 Redis MGet 路径。
func (m *memStateStore) GetRoomMembersBatch(ctx context.Context, rooms []string) (map[string]bus.RoomMembersSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]bus.RoomMembersSnapshot, len(rooms))
	for _, room := range rooms {
		if snap, ok := m.rooms[room]; ok {
			out[room] = snap
		}
	}
	return out, nil
}

func (m *memStateStore) GetRoomMembersRev(ctx context.Context, room string) (bus.RoomMembersSnapshot, uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap, ok := m.rooms[room]
	if !ok {
		return bus.RoomMembersSnapshot{}, 0, nats.ErrKeyNotFound
	}
	return snap, m.revs[room], nil
}

func (m *memStateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, room)
	delete(m.revs, room)
	return nil
}

func (m *memStateStore) DeleteRoomMembersRev(ctx context.Context, room string, rev uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rev == 0 {
		return nil
	}
	if m.revs[room] != rev {
		return errCASConflict
	}
	delete(m.rooms, room)
	delete(m.revs, room)
	return nil
}

func (m *memStateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strm[stream] = [2]string{room, identity}
	return nil
}

func (m *memStateStore) GetStream(ctx context.Context, stream string) (room, identity string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.strm[stream]
	if !ok {
		return "", "", errNotFound
	}
	return v[0], v[1], nil
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
	seen := make(map[string]struct{})
	for k := range m.rooms {
		out = append(out, k)
		seen[k] = struct{}{}
	}
	for k := range m.metas {
		if _, ok := seen[k]; ok {
			continue
		}
		out = append(out, k)
	}
	return out, nil
}

func (m *memStateStore) PutRoomMeta(ctx context.Context, room string, meta bus.RoomMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metas[room] = meta
	return nil
}

func (m *memStateStore) GetRoomMeta(ctx context.Context, room string) (bus.RoomMeta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, ok := m.metas[room]
	if !ok {
		return bus.RoomMeta{}, errNotFound
	}
	return meta, nil
}

func (m *memStateStore) DeleteRoomMeta(ctx context.Context, room string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.metas, room)
	return nil
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
		Name:     "r1",
		Members:  map[string]*MemberInfo{"s1": {ID: "s1", Identity: "alice"}},
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
	store.revs["r1"] = 1
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

func TestSyncRoomToStore_HeartbeatRenewalDoesNotNotify(t *testing.T) {
	store := newMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	notifier := &captureNotifier{}
	hub.SetStateNotifier(notifier)

	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name:     "r1",
		Members:  map[string]*MemberInfo{"sock-1": {ID: "sock-1", Identity: "alice"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()

	// 首次同步：写入并通知 peer 刷新。
	hub.syncRoomToStore("r1")
	notifier.mu.Lock()
	got := len(notifier.events)
	notifier.mu.Unlock()
	if got != 1 {
		t.Fatalf("expected 1 notify on first sync, got %d", got)
	}

	// 心跳续期：成员组成未变，只刷新 lease，不应再次通知。
	hub.syncRoomToStore("r1")
	notifier.mu.Lock()
	got = len(notifier.events)
	notifier.mu.Unlock()
	if got != 1 {
		t.Fatalf("expected no extra notify on heartbeat renewal, got %d", got)
	}

	// 成员状态变化：必须通知 peer 刷新。
	hub.mu.Lock()
	hub.rooms["r1"].Speaking["alice"] = true
	hub.mu.Unlock()
	hub.syncRoomToStore("r1")
	notifier.mu.Lock()
	got = len(notifier.events)
	notifier.mu.Unlock()
	if got != 2 {
		t.Fatalf("expected notify on membership change, got %d", got)
	}
}

// plainMemStateStore 仅实现无 revision 的 membershipStore，覆盖 Redis 等 plain 合并路径。
type plainMemStateStore struct {
	mu    sync.Mutex
	rooms map[string]bus.RoomMembersSnapshot
}

func newPlainMemStateStore() *plainMemStateStore {
	return &plainMemStateStore{rooms: make(map[string]bus.RoomMembersSnapshot)}
}

func (s *plainMemStateStore) PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rooms[snap.Room] = snap
	return nil
}

func (s *plainMemStateStore) GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.rooms[room]
	if !ok {
		return bus.RoomMembersSnapshot{}, nats.ErrKeyNotFound
	}
	return snap, nil
}

func (s *plainMemStateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, room)
	return nil
}

func (s *plainMemStateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	return nil
}
func (s *plainMemStateStore) GetStream(ctx context.Context, stream string) (string, string, error) {
	return "", "", errNotFound
}
func (s *plainMemStateStore) DeleteStream(ctx context.Context, stream string) error { return nil }
func (s *plainMemStateStore) ListRoomNames(ctx context.Context) ([]string, error)   { return nil, nil }

func TestSyncRoomToStorePlain_HeartbeatRenewalDoesNotNotify(t *testing.T) {
	store := newPlainMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	notifier := &captureNotifier{}
	hub.SetStateNotifier(notifier)

	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name:     "r1",
		Members:  map[string]*MemberInfo{"sock-1": {ID: "sock-1", Identity: "alice"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()

	hub.syncRoomToStore("r1")
	notifier.mu.Lock()
	got := len(notifier.events)
	notifier.mu.Unlock()
	if got != 1 {
		t.Fatalf("expected 1 notify on first sync, got %d", got)
	}

	hub.syncRoomToStore("r1")
	notifier.mu.Lock()
	got = len(notifier.events)
	notifier.mu.Unlock()
	if got != 1 {
		t.Fatalf("expected no extra notify on plain heartbeat renewal, got %d", got)
	}

	hub.mu.Lock()
	hub.rooms["r1"].Speaking["alice"] = true
	hub.mu.Unlock()
	hub.syncRoomToStore("r1")
	notifier.mu.Lock()
	got = len(notifier.events)
	notifier.mu.Unlock()
	if got != 2 {
		t.Fatalf("expected notify on plain membership change, got %d", got)
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
		Name:     "r1",
		Members:  map[string]*MemberInfo{"s1": {ID: "s1", Identity: "alice"}},
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
		Room:    "remote-only",
		Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}},
	}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	rooms := hub.getMergedRooms("")
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
		Room:    "r1",
		Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}},
	}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	server := newMockBroadcaster()
	hub.fanout = server
	hub.ApplyRemoteRoomState("r1")
	if len(server.roomCasts["__platform"][EventRoomUpdated]) == 0 {
		t.Fatal("expected room:updated local broadcast")
	}
	if len(server.roomCasts["__platform"][EventRoomListResult]) == 0 {
		t.Fatal("expected room:list:result local broadcast")
	}
}

func TestHub_SyncRoomToStore_PreservesRemoteMembers(t *testing.T) {
	store := newMemStateStore()
	// pre-seed remote member on another instance
	_ = store.PutRoomMembers(context.Background(), bus.RoomMembersSnapshot{
		Room: "r1",
		Members: []bus.MemberRecord{
			{Identity: "bob", InstanceID: "inst-b", Stream: "gs-b"},
		},
	})
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")

	// local join alice
	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name: "r1",
		Members: map[string]*MemberInfo{
			"s1": {Identity: "alice", Stream: "gs-a"},
		},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()
	hub.syncRoomToStore("r1")

	snap, err := store.GetRoomMembers(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range snap.Members {
		got[m.Identity] = m.InstanceID
	}
	if got["alice"] != "inst-a" || got["bob"] != "inst-b" {
		t.Fatalf("expected both members preserved, got %+v", snap.Members)
	}

	// local leave all: remote bob must remain
	hub.mu.Lock()
	delete(hub.rooms, "r1")
	hub.mu.Unlock()
	hub.syncRoomToStore("r1")
	snap, err = store.GetRoomMembers(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Members) != 1 || snap.Members[0].Identity != "bob" {
		t.Fatalf("expected only remote bob, got %+v", snap.Members)
	}
}

func TestHub_SyncRoomToStore_ConcurrentRevisionMerge(t *testing.T) {
	store := newMemStateStore()
	hubA := NewHub(nil, nil, nil, nil)
	hubB := NewHub(nil, nil, nil, nil)
	hubA.SetMembershipStore(store, "inst-a")
	hubB.SetMembershipStore(store, "inst-b")

	seedRoom := func(hub *Hub, sid, identity string) {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		hub.rooms["r1"] = &Room{
			Name:     "r1",
			Members:  map[string]*MemberInfo{sid: {ID: sid, Identity: identity}},
			MicMuted: map[string]bool{},
			Speaking: map[string]bool{},
		}
	}
	seedRoom(hubA, "s1", "alice")
	seedRoom(hubB, "s2", "bob")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); hubA.syncRoomToStore("r1") }()
	go func() { defer wg.Done(); hubB.syncRoomToStore("r1") }()
	wg.Wait()

	snap, rev, err := store.GetRoomMembersRev(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if rev == 0 {
		t.Fatal("expected nonzero revision")
	}
	got := map[string]bool{}
	for _, m := range snap.Members {
		got[m.Identity] = true
	}
	if !got["alice"] || !got["bob"] {
		t.Fatalf("concurrent sync lost members: %+v", snap.Members)
	}
}

type conflictOnceStore struct {
	*memStateStore
	conflicted bool
}

func (c *conflictOnceStore) PutRoomMembersRev(ctx context.Context, snap bus.RoomMembersSnapshot, rev uint64) error {
	if !c.conflicted {
		c.conflicted = true
		return errCASConflict
	}
	return c.memStateStore.PutRoomMembersRev(ctx, snap, rev)
}

func TestHub_SyncRoomToStore_RetriesRevisionConflict(t *testing.T) {
	store := &conflictOnceStore{memStateStore: newMemStateStore()}
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	hub.mu.Lock()
	hub.rooms["r1"] = &Room{
		Name:     "r1",
		Members:  map[string]*MemberInfo{"s1": {ID: "s1", Identity: "alice"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}
	hub.mu.Unlock()

	hub.syncRoomToStore("r1")
	snap, _, err := store.GetRoomMembersRev(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Members) != 1 || snap.Members[0].Identity != "alice" {
		t.Fatalf("expected alice after retry, got %+v", snap.Members)
	}
	if !store.conflicted {
		t.Fatal("conflict injection was not exercised")
	}
}
