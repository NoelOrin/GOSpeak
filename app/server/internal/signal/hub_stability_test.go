package signal

import (
	"errors"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/model"
)

// ─── Mocks for stability tests ───

type mockMuteStore struct {
	mu    sync.Mutex
	muted map[string]bool
	err   error
}

func newMockMuteStore() *mockMuteStore {
	return &mockMuteStore{muted: make(map[string]bool)}
}

func (m *mockMuteStore) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return false, nil, m.err
	}
	return m.muted[identity], nil, nil
}

func (m *mockMuteStore) IsMutedBatch(identities []string) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	out := make(map[string]bool, len(identities))
	for _, identity := range identities {
		if m.muted[identity] {
			out[identity] = true
		}
	}
	return out, nil
}

func (m *mockMuteStore) setMuted(identity string, muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted[identity] = muted
}

func (m *mockMuteStore) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

type blockingUserStore struct {
	users   map[string]*model.User
	called  chan struct{}
	release chan struct{}
}

func (m *blockingUserStore) GetByName(name string) (*model.User, error) {
	select {
	case m.called <- struct{}{}:
	default:
	}
	<-m.release
	u := m.users[name]
	if u == nil {
		return nil, modelNotFound(name)
	}
	return u, nil
}

func (m *blockingUserStore) GetByNames(names []string) (map[string]*model.User, error) {
	select {
	case m.called <- struct{}{}:
	default:
	}
	<-m.release
	out := make(map[string]*model.User, len(names))
	for _, name := range names {
		if u, ok := m.users[name]; ok {
			out[name] = u
		}
	}
	return out, nil
}

func (m *blockingUserStore) GetByID(id uint) (*model.User, error) {
	return nil, modelNotFound("id")
}

func (m *blockingUserStore) GetByUUID(uuid string) (*model.User, error) {
	return nil, modelNotFound("uuid")
}

// ─── OnMemberSpeaking mute checks ───

func TestHub_OnMemberSpeaking_MutedIgnored(t *testing.T) {
	mute := newMockMuteStore()
	mute.setMuted("alice", true)
	hub := NewHub(nil, mute, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:lobby", map[string]string{"sock-1": "alice"})

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":true}`)

	fanout := hub.fanout.(*mockBroadcaster)
	if len(fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers]) != 0 {
		t.Fatal("muted member speaking must not broadcast active speakers")
	}
	hub.mu.RLock()
	speaking := hub.rooms["domain-a:lobby"].Speaking["alice"]
	hub.mu.RUnlock()
	if speaking {
		t.Fatal("muted member speaking state must not be updated")
	}
}

func TestHub_OnMemberSpeaking_UnmutedBroadcasts(t *testing.T) {
	hub := NewHub(nil, newMockMuteStore(), nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:lobby", map[string]string{"sock-1": "alice"})

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":true}`)

	fanout := hub.fanout.(*mockBroadcaster)
	if len(fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers]) != 1 {
		t.Fatal("unmuted member speaking should broadcast active speakers")
	}
	hub.mu.RLock()
	speaking := hub.rooms["domain-a:lobby"].Speaking["alice"]
	hub.mu.RUnlock()
	if !speaking {
		t.Fatal("unmuted member speaking state should be updated")
	}
}

func TestHub_OnMemberSpeaking_MuteCheckErrorIgnored(t *testing.T) {
	mute := newMockMuteStore()
	mute.setErr(errors.New("db down"))
	hub := NewHub(nil, mute, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:lobby", map[string]string{"sock-1": "alice"})

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":true}`)

	fanout := hub.fanout.(*mockBroadcaster)
	if len(fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers]) != 0 {
		t.Fatal("speaking must be ignored when mute check fails")
	}
}

// ─── Member enrichment outside h.mu ───

func TestHub_GetRoomMembers_EnrichesOutsideLock(t *testing.T) {
	users := &blockingUserStore{
		users: map[string]*model.User{
			"alice": {Name: "alice", DisplayName: "Alice", Avatar: "a.png"},
		},
		called:  make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(users.release) }) }
	defer release()

	hub := NewHub(nil, nil, users, nil)
	seedKickRoom(hub, "r1", map[string]string{"sock-1": "alice"})

	result := make(chan []MemberInfo, 1)
	go func() {
		result <- hub.GetRoomMembers("r1")
	}()

	select {
	case <-users.called:
	case <-time.After(time.Second):
		t.Fatal("user store query never started")
	}

	// 查询阻塞期间 h.mu 必须立即可获取，证明 DB 查询不在锁内。
	acquired := make(chan struct{})
	go func() {
		hub.mu.RLock()
		close(acquired)
		hub.mu.RUnlock()
	}()
	select {
	case <-acquired:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("hub mutex held during member enrichment")
	}

	release()
	select {
	case members := <-result:
		if len(members) != 1 || members[0].DisplayName != "Alice" || members[0].Avatar != "a.png" {
			t.Fatalf("unexpected enriched members: %+v", members)
		}
	case <-time.After(time.Second):
		t.Fatal("GetRoomMembers did not return")
	}
}

func TestHub_GetRoomMembers_EnrichesProfileAndMute(t *testing.T) {
	users := &mockUserStore{users: map[string]*model.User{
		"alice": {Name: "alice", DisplayName: "Alice", Avatar: "a.png"},
	}}
	mute := newMockMuteStore()
	mute.setMuted("alice", true)

	hub := NewHub(nil, mute, users, nil)
	seedKickRoom(hub, "r1", map[string]string{"sock-1": "alice"})
	hub.mu.Lock()
	hub.rooms["r1"].MicMuted["alice"] = true
	hub.mu.Unlock()

	members := hub.GetRoomMembers("r1")
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	m := members[0]
	if m.DisplayName != "Alice" || m.Avatar != "a.png" {
		t.Fatalf("user profile not enriched: %+v", m)
	}
	if !m.IsMuted || !m.IsMicMuted {
		t.Fatalf("mute flags not enriched: %+v", m)
	}
}

// ─── Domain deletion closes connections ───

func TestHub_OnDomainDelete_ClosesDomainConnections(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")
	fanout.clients["sock-a"] = connA
	fanout.clients["sock-b"] = connB

	seedKickRoom(hub, "domain-a:lobby", map[string]string{"sock-a": "user-a"})
	seedKickRoom(hub, "domain-b:lobby", map[string]string{"sock-b": "user-b"})

	hub.OnDomainDelete("domain-a")

	if !connA.isClosed() {
		t.Fatal("domain-a member connection should be closed")
	}
	if connB.isClosed() {
		t.Fatal("domain-b member connection must not be closed")
	}
}
