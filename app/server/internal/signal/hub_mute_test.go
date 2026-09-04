package signal

import (
	"context"
	"sync"
	"testing"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/sfu"
)

// captureMuteBus 记录 namespace 事件与 payload，用于验证 mute 事件广播。
type captureMuteBus struct {
	mu   sync.Mutex
	ns   map[string][]map[string]interface{}
	room map[string]map[string][]map[string]interface{}
}

func (c *captureMuteBus) PublishNamespace(ctx context.Context, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ns == nil {
		c.ns = map[string][]map[string]interface{}{}
	}
	if m, ok := payload.(map[string]interface{}); ok {
		c.ns[event] = append(c.ns[event], m)
	} else {
		c.ns[event] = append(c.ns[event], nil)
	}
	return nil
}
func (c *captureMuteBus) PublishRoom(ctx context.Context, room, event string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room == nil {
		c.room = map[string]map[string][]map[string]interface{}{}
	}
	if c.room[room] == nil {
		c.room[room] = map[string][]map[string]interface{}{}
	}
	if m, ok := payload.(map[string]interface{}); ok {
		c.room[room][event] = append(c.room[room][event], m)
	} else {
		c.room[room][event] = append(c.room[room][event], nil)
	}
	return nil
}
func (c *captureMuteBus) Mode() string       { return "test" }
func (c *captureMuteBus) IsConnected() bool  { return true }
func (c *captureMuteBus) InstanceID() string { return "test" }
func (c *captureMuteBus) Close() error       { return nil }

// countingMembershipStore 统计跨实例成员扫描调用次数，验证 roomsForIdentity 只计算一次。
type countingMembershipStore struct {
	membershipStore
	mu        sync.Mutex
	listCalls int
	getCalls  int
}

func (c *countingMembershipStore) ListRoomNames(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	c.listCalls++
	c.mu.Unlock()
	return c.membershipStore.ListRoomNames(ctx)
}

func (c *countingMembershipStore) GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error) {
	c.mu.Lock()
	c.getCalls++
	c.mu.Unlock()
	return c.membershipStore.GetRoomMembers(ctx, room)
}

func (c *countingMembershipStore) calls() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.listCalls, c.getCalls
}

func TestBroadcastMute_PublishesMemberMuted(t *testing.T) {
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}, nil)
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	hub.BroadcastMute(7, &MuteInfo{Permanent: true})

	bus.mu.Lock()
	defer bus.mu.Unlock()
	events := bus.ns[EventMemberMuted]
	if len(events) != 1 {
		t.Fatalf("member:muted events = %d, want 1", len(events))
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != true {
		t.Fatalf("member:muted payload = %+v, want identity=alice muted=true", events[0])
	}
}

func TestBroadcastMute_PublishesMemberMuted_ToMemberRooms(t *testing.T) {
	hub := newTestHub()
	hub.userStore = &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	conn := newAuthedMockClient("conn-1", "alice")
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join sfu room: %v", err)
	}
	// 另一个 namespace 的房间不应收到 alice 的禁言事件（跨租户泄漏回归防护）。
	otherConn := newAuthedMockClient("conn-2", "bob")
	if _, err := hub.OnRoomJoinSFU(otherConn, `{"room":"lobby2","domain_uuid":"dom-b","identity":"bob"}`); err != nil {
		t.Fatalf("join other sfu room: %v", err)
	}

	hub.BroadcastMute(7, &MuteInfo{Permanent: true})

	bus.mu.Lock()
	defer bus.mu.Unlock()
	if len(bus.room["dom-a:lobby"][EventMemberMuted]) != 1 {
		t.Fatalf("expected member:muted published to room dom-a:lobby, got %+v", bus.room)
	}
	if len(bus.ns[EventMemberMuted]) != 0 {
		t.Fatalf("member:muted leaked to namespace, got %+v", bus.ns[EventMemberMuted])
	}
	if len(bus.room["dom-b:lobby2"][EventMemberMuted]) != 0 {
		t.Fatalf("member:muted leaked to non-member room dom-b:lobby2, got %+v", bus.room["dom-b:lobby2"][EventMemberMuted])
	}
}

func TestBroadcastUnmute_PublishesMemberUnmuted(t *testing.T) {
	hub := NewHub(nil, nil, &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}, nil)
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	hub.BroadcastUnmute(7)

	bus.mu.Lock()
	defer bus.mu.Unlock()
	events := bus.ns[EventMemberUnmuted]
	if len(events) != 1 {
		t.Fatalf("member:unmuted events = %d, want 1", len(events))
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != false {
		t.Fatalf("member:unmuted payload = %+v, want identity=alice muted=false", events[0])
	}
}

func TestBroadcastUnmute_PublishesMemberUnmuted_ToMemberRooms(t *testing.T) {
	hub := newTestHub()
	hub.userStore = &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}
	bus := &captureMuteBus{}
	hub.SetEventBus(bus)

	conn := newAuthedMockClient("conn-1", "alice")
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("join sfu room: %v", err)
	}
	otherConn := newAuthedMockClient("conn-2", "bob")
	if _, err := hub.OnRoomJoinSFU(otherConn, `{"room":"lobby2","domain_uuid":"dom-b","identity":"bob"}`); err != nil {
		t.Fatalf("join other sfu room: %v", err)
	}

	hub.BroadcastUnmute(7)

	bus.mu.Lock()
	defer bus.mu.Unlock()
	events := bus.room["dom-a:lobby"][EventMemberUnmuted]
	if len(events) != 1 {
		t.Fatalf("expected member:unmuted published to room dom-a:lobby, got %+v", bus.room)
	}
	if events[0]["identity"] != "alice" || events[0]["muted"] != false {
		t.Fatalf("member:unmuted payload = %+v, want identity=alice muted=false", events[0])
	}
	if len(bus.ns[EventMemberUnmuted]) != 0 {
		t.Fatalf("member:unmuted leaked to namespace, got %+v", bus.ns[EventMemberUnmuted])
	}
	if len(bus.room["dom-b:lobby2"][EventMemberUnmuted]) != 0 {
		t.Fatalf("member:unmuted leaked to non-member room dom-b:lobby2, got %+v", bus.room["dom-b:lobby2"][EventMemberUnmuted])
	}
}

func TestBroadcastMuteUnmute_ComputesRoomsOnce(t *testing.T) {
	hub := newTestHub()
	hub.userStore = &idUserStore{users: map[uint]*model.User{
		7: {ID: 7, Name: "alice"},
	}}
	hub.SetSFU(&capsProvider{caps: sfu.Capabilities{ServerMute: true, MuteLevel: sfu.EnforcementHard}})

	store := &countingMembershipStore{membershipStore: newMemStateStore()}
	if err := store.PutRoomMembers(context.Background(), bus.RoomMembersSnapshot{
		Room:    "dom-a:lobby",
		Members: []bus.MemberRecord{{Identity: "alice"}},
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	hub.SetMembershipStore(store, "test-instance")
	hub.SetEventBus(&captureMuteBus{})

	hub.BroadcastMute(7, &MuteInfo{Permanent: true})
	lists, gets := store.calls()
	if lists != 1 || gets != 1 {
		t.Fatalf("BroadcastMute KV scans: list=%d get=%d, want 1/1 (rooms computed once)", lists, gets)
	}

	hub.BroadcastUnmute(7)
	lists, gets = store.calls()
	if lists != 2 || gets != 2 {
		t.Fatalf("BroadcastUnmute KV scans: list=%d get=%d, want 2/2 cumulative (rooms computed once per broadcast)", lists, gets)
	}
}
