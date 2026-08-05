package signal

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/pkg"
)

// failingGetStore 模拟共享状态后端读故障，且不实现 revision 接口，
// 用于验证 plain 合并路径在读失败时不会覆盖/删除远程快照。
type failingGetStore struct {
	mu      sync.Mutex
	rooms   map[string]bus.RoomMembersSnapshot
	getErr  error
	writes  int
	deletes int
}

func newFailingGetStore(getErr error) *failingGetStore {
	return &failingGetStore{
		rooms:  make(map[string]bus.RoomMembersSnapshot),
		getErr: getErr,
	}
}

func (f *failingGetStore) PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	f.rooms[snap.Room] = snap
	return nil
}

func (f *failingGetStore) GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error) {
	return bus.RoomMembersSnapshot{}, f.getErr
}

func (f *failingGetStore) DeleteRoomMembers(ctx context.Context, room string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes++
	return nil
}

func (f *failingGetStore) PutStream(ctx context.Context, stream, room, identity string) error {
	return nil
}

func (f *failingGetStore) GetStream(ctx context.Context, stream string) (string, string, error) {
	return "", "", nil
}

func (f *failingGetStore) DeleteStream(ctx context.Context, stream string) error { return nil }

func (f *failingGetStore) ListRoomNames(ctx context.Context) ([]string, error) { return nil, nil }

func TestSyncRoomToStorePlain_ReadErrorDoesNotClobber(t *testing.T) {
	store := newFailingGetStore(errors.New("redis unavailable"))
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")

	// 最后一名本地成员离开时，若远端快照读失败，绝不能执行删除/覆盖。
	hub.syncRoomToStore("domain:lobby")

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.writes != 0 || store.deletes != 0 {
		t.Fatalf("read failure must abort sync: writes=%d deletes=%d", store.writes, store.deletes)
	}
}

func TestMergeRemoteMembers_FiltersExpiredGhosts(t *testing.T) {
	now := time.Now().UnixMilli()
	prev := []bus.MemberRecord{
		{Identity: "ghost", InstanceID: "dead-instance", ExpiresAtMS: now - 1},
		{Identity: "alive", InstanceID: "other", ExpiresAtMS: now + 60_000},
		{Identity: "legacy", InstanceID: "old-instance", ExpiresAtMS: 0},
		{Identity: "local-collide", InstanceID: "other", ExpiresAtMS: now + 60_000},
	}
	local := []bus.MemberRecord{
		{Identity: "local", InstanceID: "inst-a", ExpiresAtMS: now + 60_000},
		{Identity: "local-collide", InstanceID: "inst-a", ExpiresAtMS: now + 60_000},
	}

	merged := mergeRemoteMembers(prev, local, "inst-a", now)
	identities := make(map[string]struct{}, len(merged))
	for _, rec := range merged {
		identities[rec.Identity] = struct{}{}
	}
	if _, ok := identities["ghost"]; ok {
		t.Fatal("expired remote member should be filtered")
	}
	if _, ok := identities["alive"]; !ok {
		t.Fatal("unexpired remote member should be preserved")
	}
	if _, ok := identities["legacy"]; !ok {
		t.Fatal("legacy record without lease should be preserved")
	}
	if len(identities) != 4 {
		t.Fatalf("expected local, local-collide, alive, legacy; got %v", identities)
	}
}

func TestRegisterRoomMembers_RejectsRemoteDuplicateIdentity(t *testing.T) {
	store := newMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")

	now := time.Now().UnixMilli()
	store.rooms["domain:lobby"] = bus.RoomMembersSnapshot{
		Room: "domain:lobby",
		Members: []bus.MemberRecord{{
			Identity:    "alice",
			InstanceID:  "inst-b",
			ExpiresAtMS: now + 60_000,
		}},
	}

	local := []bus.MemberRecord{{
		Room:        "domain:lobby",
		Identity:    "alice",
		SocketHint:  "conn-2",
		InstanceID:  "inst-a",
		ExpiresAtMS: now + 60_000,
	}}
	err := hub.registerRoomMembers("domain:lobby", local, 0, "alice")
	if !errors.Is(err, errDuplicateRemoteIdentity) {
		t.Fatalf("expected errDuplicateRemoteIdentity, got %v", err)
	}
}

func TestRegisterRoomMembers_EnforcesLimitAtomically(t *testing.T) {
	store := newMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")

	now := time.Now().UnixMilli()
	store.rooms["domain:lobby"] = bus.RoomMembersSnapshot{
		Room: "domain:lobby",
		Members: []bus.MemberRecord{{
			Identity:    "bob",
			InstanceID:  "inst-b",
			ExpiresAtMS: now + 60_000,
		}},
	}

	local := []bus.MemberRecord{{
		Room:        "domain:lobby",
		Identity:    "alice",
		SocketHint:  "conn-1",
		InstanceID:  "inst-a",
		ExpiresAtMS: now + 60_000,
	}}
	err := hub.registerRoomMembers("domain:lobby", local, 1, "alice")
	if !errors.Is(err, errRoomLimitExceeded) {
		t.Fatalf("expected errRoomLimitExceeded, got %v", err)
	}
}

func TestOnMemberMicState_SyncsSharedState(t *testing.T) {
	store := newMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	hub.fanout = newMockBroadcaster()

	client := newAuthedMockClient("conn-1", "alice")
	key := roomKey("", "lobby")
	hub.mu.Lock()
	hub.rooms[key] = &Room{
		Name:     "lobby",
		Members:  map[string]*MemberInfo{"conn-1": {ID: "conn-1", Identity: "alice"}},
		MicMuted: make(map[string]bool),
		Speaking: make(map[string]bool),
	}
	hub.connSlots["conn-1"] = &connRoomSlots{}
	hub.mu.Unlock()

	hub.OnMemberMicState(client, `{"room":"lobby","identity":"alice","isMicMuted":true}`)

	snap, err := store.GetRoomMembers(context.Background(), key)
	if err != nil {
		t.Fatalf("GetRoomMembers: %v", err)
	}
	if len(snap.Members) != 1 || !snap.Members[0].MicMuted {
		t.Fatalf("expected mic muted synced to KV, got %+v", snap.Members)
	}
}

func TestCheckRoomPassword_UsesSharedRoomMeta(t *testing.T) {
	store := newMemStateStore()
	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")

	hash, err := pkg.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	key := roomKey("domain-a", "remote-lobby")
	store.metas[key] = bus.RoomMeta{Name: "remote-lobby", Password: hash, DomainUUID: "domain-a"}

	if ok, pwdErr := hub.CheckRoomPassword("domain-a", "remote-lobby", "secret"); !ok || pwdErr != nil {
		t.Fatalf("expected password accepted, ok=%v err=%v", ok, pwdErr)
	}
	if ok, pwdErr := hub.CheckRoomPassword("domain-a", "remote-lobby", "wrong"); ok || pwdErr != nil {
		t.Fatalf("expected wrong password rejected, ok=%v err=%v", ok, pwdErr)
	}
	if _, pwdErr := hub.CheckRoomPassword("domain-a", "remote-lobby", ""); pwdErr == nil {
		t.Fatal("expected missing password error")
	}
}
