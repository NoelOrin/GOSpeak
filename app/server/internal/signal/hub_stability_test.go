package signal

import (
	"errors"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/model"
)

// ─── Mocks for stability tests ───

type muteStoreCalls struct {
	IsMutedByIdentity int
	IsMutedBatch      int
}

type mockMuteStore struct {
	mu    sync.Mutex
	muted map[string]bool
	err   error
	calls muteStoreCalls
}

func newMockMuteStore() *mockMuteStore {
	return &mockMuteStore{muted: make(map[string]bool)}
}

func (m *mockMuteStore) IsMutedByIdentity(identity string) (bool, *model.Mute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls.IsMutedByIdentity++
	if m.err != nil {
		return false, nil, m.err
	}
	return m.muted[identity], nil, nil
}

func (m *mockMuteStore) IsMutedBatch(identities []string) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls.IsMutedBatch++
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

func newTestHubWithMuteStore(t *testing.T) *Hub {
	t.Helper()
	hub := NewHub(nil, newMockMuteStore(), nil, allowAllPermChecker{})
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "r1", map[string]string{"alice": "alice"})
	return hub
}

func fakeConn(identity string) *mockClient {
	return newAuthedMockClient(identity, identity)
}

func (h *Hub) primeMuteCache(identity string, muted bool) error {
	h.mu.Lock()
	if h.muteCache == nil {
		h.muteCache = make(map[string]muteCacheEntry)
	}
	h.muteCache[identity] = muteCacheEntry{muted: muted, expires: time.Now().Add(5 * time.Second)}
	h.mu.Unlock()
	return nil
}

func testMuteStoreCallCount(t *testing.T, h *Hub) muteStoreCalls {
	t.Helper()
	store, ok := h.muteStore.(*mockMuteStore)
	if !ok {
		t.Fatalf("expected *mockMuteStore, got %T", h.muteStore)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.calls
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

func TestOnMemberSpeaking_UsesMuteCache(t *testing.T) {
	h := newTestHubWithMuteStore(t)
	if err := h.primeMuteCache("alice", false); err != nil {
		t.Fatalf("primeMuteCache: %v", err)
	}
	// 桩 muteStore 每次调用都计数；缓存命中后不再走 DB
	h.OnMemberSpeaking(fakeConn("alice"), `{"room":"r1","identity":"alice","speaking":true}`)
	if calls := testMuteStoreCallCount(t, h); calls.IsMutedByIdentity != 0 {
		t.Fatalf("expected cached mute check, got %d DB calls", calls.IsMutedByIdentity)
	}
}

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

// TestHub_OnMemberSpeaking_DedupesUnchangedState 验证同值上报不再重复广播
// room:active-speakers，只有状态翻转（true/false）才广播。
func TestHub_OnMemberSpeaking_DedupesUnchangedState(t *testing.T) {
	hub := NewHub(nil, newMockMuteStore(), nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:lobby", map[string]string{"sock-1": "alice"})

	conn := newAuthedMockClient("sock-1", "alice")
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":true}`)
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":true}`)

	fanout := hub.fanout.(*mockBroadcaster)
	if got := len(fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers]); got != 1 {
		t.Fatalf("duplicate same-value speaking must not re-broadcast, got %d broadcasts", got)
	}

	// 状态翻转必须广播
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":false}`)
	if got := len(fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers]); got != 2 {
		t.Fatalf("state flip must broadcast, got %d broadcasts", got)
	}

	// 同值 false 再次上报不再广播
	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":false}`)
	if got := len(fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers]); got != 2 {
		t.Fatalf("duplicate same-value speaking must not re-broadcast, got %d broadcasts", got)
	}
}

// TestOnRoomJoinSFU_ReplaysActiveSpeakers 验证新成员 SFU join 成功后，
// 服务端向房间回放当前 active speakers，让无 SFU 原生检测的 provider 下
// 加入者立即看到正在发言的成员。
func TestOnRoomJoinSFU_ReplaysActiveSpeakers(t *testing.T) {
	hub := newTestHub()

	alice := fakeConn("alice")
	if _, err := hub.OnRoomJoin(alice, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("alice room join: %v", err)
	}
	if _, err := hub.OnRoomJoinSFU(alice, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice"}`); err != nil {
		t.Fatalf("alice sfu join: %v", err)
	}
	hub.OnMemberSpeaking(alice, `{"room":"lobby","domain_uuid":"dom-a","identity":"alice","speaking":true}`)

	fanout := hub.fanout.(*mockBroadcaster)
	castsBefore := len(fanout.roomCasts["dom-a:lobby"][EventRoomActiveSpeakers])
	if castsBefore == 0 {
		t.Fatal("alice speaking should have broadcast active speakers")
	}

	bob := fakeConn("bob")
	if _, err := hub.OnRoomJoin(bob, `{"room":"lobby","domain_uuid":"dom-a","identity":"bob"}`); err != nil {
		t.Fatalf("bob room join: %v", err)
	}
	if _, err := hub.OnRoomJoinSFU(bob, `{"room":"lobby","domain_uuid":"dom-a","identity":"bob"}`); err != nil {
		t.Fatalf("bob sfu join: %v", err)
	}

	casts := fanout.roomCasts["dom-a:lobby"][EventRoomActiveSpeakers]
	if len(casts) != castsBefore+1 {
		t.Fatalf("join must replay active speakers: got %d broadcasts, want %d", len(casts), castsBefore+1)
	}
	last := casts[len(casts)-1].(map[string]interface{})
	ids, ok := last["identities"].([]string)
	if !ok {
		t.Fatalf("replay payload identities type = %T, want []string", last["identities"])
	}
	foundAlice := false
	for _, id := range ids {
		if id == "alice" {
			foundAlice = true
		}
	}
	if !foundAlice {
		t.Fatalf("join replay identities = %v, want alice present", ids)
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
