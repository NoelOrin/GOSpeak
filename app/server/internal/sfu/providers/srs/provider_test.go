package srs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// stubRegistry 模拟 pkg.RoomRegistry 用于 SRS provider 测试。
type stubRegistry struct {
	rooms            []string
	streams          map[string][]string
	cleared          []string
	identityStreams  map[string]string // "room\x00identity" -> stream
	streamIdentities map[string]string // stream -> identity（支持同一 identity 多 stream）
}

func (s *stubRegistry) Rooms() []string { return s.rooms }
func (s *stubRegistry) Streams(room string) []string {
	return s.streams[room]
}
func (s *stubRegistry) ClearRoom(room string) {
	s.cleared = append(s.cleared, room)
	delete(s.streams, room)
}
func (s *stubRegistry) StreamForIdentity(room, identity string) (string, bool) {
	if s.identityStreams == nil {
		return "", false
	}
	st, ok := s.identityStreams[room+"\x00"+identity]
	return st, ok
}
func (s *stubRegistry) IdentityForStream(room, stream string) (string, bool) {
	if s.streamIdentities != nil {
		if id, ok := s.streamIdentities[stream]; ok {
			return id, true
		}
	}
	if s.identityStreams == nil {
		return "", false
	}
	prefix := room + "\x00"
	for k, st := range s.identityStreams {
		if st == stream && len(k) > len(prefix) && k[:len(prefix)] == prefix {
			return k[len(prefix):], true
		}
	}
	return "", false
}

// stubResolver 模拟 pkg.StreamRoomResolver：stream -> room（复合键）。
type stubResolver struct {
	roomForStream map[string]string
}

func (r *stubResolver) RoomForStream(stream string) (string, bool) {
	if r == nil || r.roomForStream == nil {
		return "", false
	}
	room, ok := r.roomForStream[stream]
	return room, ok
}

// srsTestServer 用 httptest 模拟 SRS HTTP API（/api/v1/clients/ + kick）。
type srsTestServer struct {
	srv                  *httptest.Server
	mu                   sync.Mutex
	clients              []clientsResponseClient
	streams              []string
	kickedIDs            []string
	kickFail             map[string]bool // id -> 模拟 kick 失败
	afterKickAddStream   string          // 模拟 kick 成功瞬间新流加入
	failStreamsAfterKick bool            // kick 后复查 streams 接口返回错误
}

func newSRSTestServer() *srsTestServer {
	ts := &srsTestServer{kickFail: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/streams/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		ts.mu.Lock()
		defer ts.mu.Unlock()
		if ts.failStreamsAfterKick && len(ts.kickedIDs) > 0 {
			http.Error(w, "srs streams unavailable after kick", http.StatusServiceUnavailable)
			return
		}
		streams := make([]map[string]string, 0, len(ts.streams))
		for _, name := range ts.streams {
			streams = append(streams, map[string]string{"app": "live", "name": name})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "streams": streams})
	})
	mux.HandleFunc("/api/v1/clients/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			ts.mu.Lock()
			defer ts.mu.Unlock()
			_ = json.NewEncoder(w).Encode(clientsResponse{Code: 0, Clients: ts.clients})
			return
		}
		if r.Method == http.MethodDelete {
			id := r.URL.Path[len("/api/v1/clients/"):]
			ts.mu.Lock()
			defer ts.mu.Unlock()
			if ts.kickFail[id] {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 2049})
				return
			}
			ts.kickedIDs = append(ts.kickedIDs, id)
			for _, cl := range ts.clients {
				if cl.ID == id {
					name := cl.streamName()
					for i, st := range ts.streams {
						if st == name {
							ts.streams = append(ts.streams[:i], ts.streams[i+1:]...)
							break
						}
					}
					break
				}
			}
			if ts.afterKickAddStream != "" {
				ts.streams = append(ts.streams, ts.afterKickAddStream)
				ts.afterKickAddStream = ""
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0})
		}
	})
	ts.srv = httptest.NewServer(mux)
	return ts
}

func (ts *srsTestServer) close() { ts.srv.Close() }

func newServiceWithURL(baseURL string) *Service {
	return &Service{
		client:  NewClient(baseURL),
		whipURL: "/rtc/v1/whip/",
	}
}

func TestClientInfo_ReturnsSameOriginWHIPURL(t *testing.T) {
	s := NewService(&config.Config{
		SRSHost:    "srs.example.com",
		SRSApiPort: "1985",
	})

	got, _ := s.ClientInfo()["whipUrl"].(string)
	want := "/rtc/v1/whip/"
	if got != want {
		t.Fatalf("whipUrl = %q, want %q", got, want)
	}
}

func TestClientInfo_AbsoluteWHIPURLOverride(t *testing.T) {
	s := NewService(&config.Config{
		SRSHost:    "srs.example.com",
		SRSApiPort: "1985",
		SRSWHIPURL: "https://srs.example.com:1985/rtc/v1/whip/",
	})
	got, _ := s.ClientInfo()["whipUrl"].(string)
	want := "https://srs.example.com:1985/rtc/v1/whip/"
	if got != want {
		t.Fatalf("whipUrl = %q, want %q", got, want)
	}
}

func TestGetHost_EmptyForSRS(t *testing.T) {
	s := NewService(&config.Config{
		SRSHost:    "srs.example.com",
		SRSApiPort: "1985",
	})
	want := "http://srs.example.com:1985"
	if got := s.GetHost(); got != want {
		t.Fatalf("GetHost = %q, want %q", got, want)
	}
}

func TestListRooms_WithRegistry(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{
		rooms:   []string{"room-a", "room-b"},
		streams: map[string][]string{},
	}

	got, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	names := make([]string, len(got))
	for i, r := range got {
		names[i] = r.Name
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"room-a", "room-b"}) {
		t.Fatalf("expected [room-a room-b], got %v", names)
	}
}

func TestListRooms_WithRegistry_NilReturnsEmpty(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{rooms: nil, streams: map[string][]string{}}

	got, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestListRooms_NoRegistry_ReturnsError(t *testing.T) {
	s := newServiceWithURL("http://srs.example")

	_, err := s.ListRooms()
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.SFU_NOT_CONFIGURED {
		t.Fatalf("expected SFU_NOT_CONFIGURED, got %v", err)
	}
}

func TestDeleteRoom_KickByStreams(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Stream: "gs-aaa"},
		{ID: "cid-2", Stream: "gs-bbb"},
		{ID: "cid-3", Stream: "gs-aaa"},
	}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{
		streams: map[string][]string{"room-x": {"gs-aaa", "gs-bbb"}},
	}

	if err := s.DeleteRoom("room-x"); err != nil {
		t.Fatalf("DeleteRoom: %v", err)
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()
	sort.Strings(ts.kickedIDs)
	if !reflect.DeepEqual(ts.kickedIDs, []string{"cid-1", "cid-2", "cid-3"}) {
		t.Fatalf("expected 3 kicks, got %v", ts.kickedIDs)
	}
}

func TestDeleteRoom_PartialFailure(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Stream: "gs-aaa"},
		{ID: "cid-2", Stream: "gs-aaa"},
	}
	ts.kickFail["cid-2"] = true
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{
		streams: map[string][]string{"room-x": {"gs-aaa"}},
	}

	err := s.DeleteRoom("room-x")
	if err == nil {
		t.Fatal("expected partial failure error")
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.kickedIDs) != 1 || ts.kickedIDs[0] != "cid-1" {
		t.Fatalf("expected only cid-1 kicked, got %v", ts.kickedIDs)
	}
}

func TestDeleteRoom_Empty(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{streams: map[string][]string{"room-x": {}}}

	err := s.DeleteRoom("room-x")
	if err == nil {
		t.Fatal("expected not found error for empty room")
	}
}

func TestRemoveParticipant_StreamBridge(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	// 算出 room+identity 对应 stream，mock 一个匹配 client
	stream := GenerateStreamName("room-r", "alice")
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{{ID: "cid-1", Stream: stream}}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	if err := s.RemoveParticipant("room-r", "alice"); err != nil {
		t.Fatalf("RemoveParticipant: %v", err)
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.kickedIDs) != 1 || ts.kickedIDs[0] != "cid-1" {
		t.Fatalf("expected cid-1 kicked, got %v", ts.kickedIDs)
	}
}

func TestRemoveParticipant_NotFound(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	// 无匹配 client
	s := newServiceWithURL(ts.srv.URL)
	err := s.RemoveParticipant("room-r", "nobody")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestRemoveParticipant_RegistryIdentityStream(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	// registry 登记的 stream 与反算值不同：验证优先用 registry 实际登记值，
	// 模拟命名约定变更后旧连接仍可按 identity 查到真实 stream。
	registryStream := "gs-legacy-stream"
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{{ID: "cid-9", Stream: registryStream}}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{
		identityStreams: map[string]string{"room-r\x00alice": registryStream},
	}
	if err := s.RemoveParticipant("room-r", "alice"); err != nil {
		t.Fatalf("RemoveParticipant: %v", err)
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.kickedIDs) != 1 || ts.kickedIDs[0] != "cid-9" {
		t.Fatalf("expected cid-9 (registry stream) kicked, got %v", ts.kickedIDs)
	}
}

func TestStreamInfo_EmptySecretReturnsError(t *testing.T) {
	s := newServiceWithURL("http://srs.example")
	// secret 未配置 → GenerateStreamToken 拒签，StreamInfo 必须返回 error 而非空 token
	stream, token, err := s.StreamInfo("room-a", "alice")
	if err == nil {
		t.Fatal("expected error when secret empty, got nil")
	}
	if token != "" {
		t.Fatalf("expected empty token on error, got %q", token)
	}
	if stream == "" {
		t.Fatal("stream name should still be returned for diagnostics")
	}
}

func TestStreamInfo_WithSecretSucceeds(t *testing.T) {
	s := newServiceWithURL("http://srs.example")
	s.secret = "test-secret"
	stream, token, err := s.StreamInfo("room-a", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == "" || token == "" {
		t.Fatalf("expected non-empty stream+token, got stream=%q token=%q", stream, token)
	}
}

func TestListParticipants_WithRegistry(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	streamA := GenerateStreamName("room-x", "alice")
	streamB := GenerateStreamName("room-x", "bob")
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Stream: streamA},
		{ID: "cid-2", Stream: streamB},
		{ID: "cid-3", Stream: "gs-other"},
	}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{
		streams: map[string][]string{"room-x": {streamA, streamB}},
		identityStreams: map[string]string{
			"room-x\x00alice": streamA,
			"room-x\x00bob":   streamB,
		},
	}

	got, err := s.ListParticipants("room-x")
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, p.Identity)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"alice", "bob"}) {
		t.Fatalf("participants = %v, want [alice bob]", names)
	}
}

func TestListParticipants_NoRegistryStreamsEmpty(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{{ID: "cid-1", Stream: "gs-aaa"}}
	ts.mu.Unlock()
	s := newServiceWithURL(ts.srv.URL)
	// registry nil or empty streams → empty, not match by room name
	got, err := s.ListParticipants("room-x")
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty without registry streams, got %v", got)
	}
}

func TestGetHost_ReturnsAPIBase(t *testing.T) {
	s := NewService(&config.Config{
		SRSHost:    "srs.example.com",
		SRSApiPort: "1985",
	})
	want := "http://srs.example.com:1985"
	if got := s.GetHost(); got != want {
		t.Fatalf("GetHost = %q, want %q", got, want)
	}
}

func TestListRooms_MemberCount(t *testing.T) {
	s := newServiceWithURL("http://srs.example")
	s.registry = &stubRegistry{
		rooms:   []string{"room-a"},
		streams: map[string][]string{"room-a": {"gs-1", "gs-2"}},
	}
	got, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	if len(got) != 1 || got[0].Name != "room-a" || got[0].MemberCount != 2 {
		t.Fatalf("unexpected rooms: %+v", got)
	}
}

func TestKickByStreams_UsesNameNotInternalStreamID(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	// 模拟 SRS6 真实响应：id + name=业务流名，stream=内部 vid
	ts.clients = []clientsResponseClient{
		{ID: "b3078950", Stream: "vid-15o35g6", Name: "gs-6401mpp6htg6", URL: "/live/gs-6401mpp6htg6", Publish: true},
		{ID: "other", Stream: "vid-x", Name: "gs-other", URL: "/live/gs-other", Publish: true},
	}
	c := NewClient(ts.srv.URL)
	kicked, remaining, err := c.KickByStreams([]string{"gs-6401mpp6htg6"})
	if err != nil {
		t.Fatalf("KickByStreams err: %v", err)
	}
	if kicked != 1 || remaining != 0 {
		t.Fatalf("kicked=%d remaining=%d, want 1/0", kicked, remaining)
	}
	if len(ts.kickedIDs) != 1 || ts.kickedIDs[0] != "b3078950" {
		t.Fatalf("kickedIDs=%v, want [b3078950]", ts.kickedIDs)
	}
}

func newSRSTestService(t *testing.T) *Service {
	t.Helper()
	return &Service{client: NewClient("http://127.0.0.1:9")}
}

func TestMuteParticipantTimed_MuteWritesPublishBlock(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	stream := GenerateStreamName("room-r", "alice")

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{identityStreams: map[string]string{"room-r\x00alice": stream}}
	store := newMemMuteRuleStore()
	s.SetMuteRuleStore(store)

	if err := s.MuteParticipantTimed("room-r", "alice", "", true, 60); err != nil {
		t.Fatalf("mute err: %v", err)
	}
	if id, _ := store.Get(context.Background(), PublishBlockKey(stream)); id != publishBlockRuleID {
		t.Fatalf("publish block not saved, id=%d", id)
	}
	if len(ts.kickedIDs) != 0 {
		t.Fatalf("Discord-style mute must NOT kick, kickedIDs=%v", ts.kickedIDs)
	}
}

func TestMuteParticipantTimed_UnmuteDeletesPublishBlock(t *testing.T) {
	s := newSRSTestService(t)
	store := newMemMuteRuleStore()
	s.SetMuteRuleStore(store)
	stream := GenerateStreamName("room-r", "alice")
	_ = store.Save(context.Background(), PublishBlockKey(stream), publishBlockRuleID, 0)

	if err := s.MuteParticipantTimed("room-r", "alice", "", false, 0); err != nil {
		t.Fatalf("unmute err: %v", err)
	}
	if id, _ := store.Get(context.Background(), PublishBlockKey(stream)); id != 0 {
		t.Fatalf("publish block should be deleted, id=%d", id)
	}
}

func TestRuleStore_StableAcrossCalls_WithoutInjection(t *testing.T) {
	svc := NewService(&config.Config{})
	// 未注入 store 时，mute/unmute 必须安全 no-op（不再惰性创建内存 store）。
	// 多实例 rule 跟踪完全依赖注入的共享 store，未注入即不持久化。
	if err := svc.MuteParticipantTimed("dom-a:r1", "alice", "", true, 0); err != nil {
		t.Fatal(err)
	}
	// 无共享 store：ruleStore 返回 nil，黑名单不持久化，也不 panic。
	if st := svc.ruleStore(); st != nil {
		t.Fatalf("ruleStore() = %v, want nil (no memory fallback)", st)
	}
	if err := svc.MuteParticipantTimed("dom-a:r1", "alice", "", false, 0); err != nil {
		t.Fatal(err)
	}
	if st := svc.ruleStore(); st != nil {
		t.Fatalf("ruleStore() = %v, want nil", st)
	}
}

func TestRuleStore_NoFallback_ForDirectConstruction(t *testing.T) {
	svc := &Service{} // 直构（newSRSTestService 同款）：muteRules 为 nil
	// 未注入时 ruleStore 返回 nil（不再惰性兜底内存 store），调用必须安全不 panic。
	if st := svc.ruleStore(); st != nil {
		t.Fatalf("ruleStore() = %v, want nil (no memory fallback)", st)
	}
	if err := svc.MuteParticipantTimed("dom-a:r1", "alice", "", true, 0); err != nil {
		t.Fatal(err)
	}
	if st := svc.ruleStore(); st != nil {
		t.Fatalf("ruleStore() = %v, want nil", st)
	}
	if err := svc.MuteParticipantTimed("dom-a:r1", "alice", "", false, 0); err != nil {
		t.Fatal(err)
	}
}

// blockingMuteRuleStore 让 Save 挂起，用于验证 store 替换不能插入到写入临界区。
type blockingMuteRuleStore struct {
	saveStarted chan struct{}
	releaseSave chan struct{}
	startOnce   sync.Once
}

func (b *blockingMuteRuleStore) Save(context.Context, string, int, time.Duration) error {
	b.startOnce.Do(func() { close(b.saveStarted) })
	<-b.releaseSave
	return nil
}

func (b *blockingMuteRuleStore) Get(context.Context, string) (int, error) { return 1, nil }
func (b *blockingMuteRuleStore) Delete(context.Context, string) error     { return nil }
func (b *blockingMuteRuleStore) Backend() string                          { return "memory" }

func TestRuleStore_StoreSwapCannotInterleaveWithSave(t *testing.T) {
	svc := &Service{}
	blocking := &blockingMuteRuleStore{
		saveStarted: make(chan struct{}),
		releaseSave: make(chan struct{}),
	}
	svc.SetMuteRuleStore(blocking)

	muteDone := make(chan error, 1)
	go func() {
		muteDone <- svc.MuteParticipantTimed("dom-a:r1", "alice", "", true, 0)
	}()
	<-blocking.saveStarted

	swapDone := make(chan struct{})
	go func() {
		svc.SetMuteRuleStore(newMemMuteRuleStore())
		close(swapDone)
	}()

	// 替换与 Save 必须互斥：Save 未结束时 SetMuteRuleStore 不能换 store，
	// 否则写入落在旧 store、后续读取落在新 store（旧写新读，黑名单静默丢失）。
	select {
	case <-swapDone:
		close(blocking.releaseSave)
		<-muteDone
		t.Fatal("SetMuteRuleStore returned while Save was still in flight")
	case <-time.After(500 * time.Millisecond):
		close(blocking.releaseSave)
	}
	if err := <-muteDone; err != nil {
		t.Fatal(err)
	}
	<-swapDone
}

func TestRuleStore_ConcurrentAccess_NoRace(t *testing.T) {
	ctx := context.Background()
	stream := GenerateStreamName("dom-a:r1", "alice")

	// 注入共享 store 后并发 MuteParticipantTimed：并发 mute 全部写入后仍能读回 1。
	// 未注入时 mute 是安全 no-op（不再惰性兜底内存 store），持久化必须依赖注入的 store。
	svc := &Service{}
	svc.SetMuteRuleStore(newMemMuteRuleStore())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.MuteParticipantTimed("dom-a:r1", "alice", "", true, 0)
		}()
	}
	wg.Wait()
	if id, err := svc.ruleStore().Get(ctx, PublishBlockKey(stream)); err != nil {
		t.Fatal(err)
	} else if id != publishBlockRuleID {
		t.Fatalf("after concurrent mute rule id = %d, want %d", id, publishBlockRuleID)
	}

	// 注入替换与读写并发执行不能有数据竞争（由 -race 覆盖）。
	var swapWG sync.WaitGroup
	for i := 0; i < 8; i++ {
		swapWG.Add(1)
		go func() {
			defer swapWG.Done()
			svc.SetMuteRuleStore(newMemMuteRuleStore())
			_, _ = svc.ruleStore().Get(ctx, PublishBlockKey("gs-race"))
		}()
	}
	swapWG.Wait()
}

func TestSRS_ImplementsTimedMuteProvider(t *testing.T) {
	var _ sfu.TimedMuteProvider = (*Service)(nil)
}

func TestCapabilities_ServerMuteEnabled(t *testing.T) {
	caps := (&Service{}).Capabilities()
	if !caps.ServerMute {
		t.Fatal("srs ServerMute should be true via force-unpublish fallback")
	}
	if caps.MuteLevel != "degraded" {
		t.Fatalf("srs MuteLevel=%q, want degraded", caps.MuteLevel)
	}
	if caps.ListLevel != "hard" {
		t.Fatalf("srs ListLevel=%q, want hard", caps.ListLevel)
	}
	if !reflect.DeepEqual(caps, sfu.CapabilitiesFor("srs")) {
		t.Fatalf("Capabilities() = %+v, want %+v", caps, sfu.CapabilitiesFor("srs"))
	}
}

func assertSRSAppErrorCode(t *testing.T, err error, want pkg.ErrCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %d, got nil", want)
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("error code = %d, want %d: %v", appErr.Code, want, err)
	}
}

func TestRemoveParticipant_KickError(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	stream := GenerateStreamName("room-r", "alice")
	ts.mu.Lock()
	ts.clients = []clientsResponseClient{{ID: "cid-1", Stream: stream}}
	ts.kickFail["cid-1"] = true
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.registry = &stubRegistry{streams: map[string][]string{"room-r": {stream}}}
	assertSRSAppErrorCode(t, s.RemoveParticipant("room-r", "alice"), pkg.SFU_ERROR)
}

func TestDeleteRoom_StreamAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/api/v1/streams/room-x" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 2049})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	s := newServiceWithURL(srv.URL)
	assertSRSAppErrorCode(t, s.DeleteRoom("room-x"), pkg.SFU_ERROR)
}

func TestListStreams_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-a", "gs-b"}
	ts.mu.Unlock()

	got, err := NewClient(ts.srv.URL).ListStreams()
	if err != nil {
		t.Fatalf("ListStreams err: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"gs-a", "gs-b"}) {
		t.Fatalf("ListStreams = %v, want [gs-a gs-b]", got)
	}
}

func TestListStreams_APICodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 2048})
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL).ListStreams(); err == nil {
		t.Fatal("expected error for non-zero SRS api code")
	}
}

func TestListStreams_EmptyStreams(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()

	got, err := NewClient(ts.srv.URL).ListStreams()
	if err != nil {
		t.Fatalf("ListStreams err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListStreams = %v, want empty", got)
	}
}

func TestListRooms_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-abc", "gs-def", "gs-abc2", "other-stream"}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{
		"gs-abc":  "dom:room-a",
		"gs-def":  "dom:room-b",
		"gs-abc2": "dom:room-a",
	}}

	rooms, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms err: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("rooms = %+v, want 2 rooms", rooms)
	}
	byName := map[string]int{}
	for _, r := range rooms {
		byName[r.Name] = r.MemberCount
	}
	if byName["dom:room-a"] != 2 || byName["dom:room-b"] != 1 {
		t.Fatalf("rooms = %+v, want room-a:2 room-b:1", rooms)
	}
}

func TestListParticipants_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-a1", "gs-a2", "gs-b1"}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom:room-a",
		"gs-a2": "dom:room-a",
		"gs-b1": "dom:room-b",
	}}
	s.registry = &stubRegistry{identityStreams: map[string]string{
		"dom:room-a\x00alice": "gs-a1",
		"dom:room-a\x00bob":   "gs-a2",
	}}

	parts, err := s.ListParticipants("dom:room-a")
	if err != nil {
		t.Fatalf("ListParticipants err: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("participants = %+v, want 2", parts)
	}
	ids := map[string]bool{}
	for _, p := range parts {
		ids[p.Identity] = true
	}
	if !ids["alice"] || !ids["bob"] {
		t.Fatalf("participants = %+v, want alice+bob (identity from registry)", parts)
	}
}

func TestListParticipants_FromSRSAPI_SkipsUnresolvableStream(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-orphan", "gs-a1"}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{"gs-a1": "dom:room-a"}}
	s.registry = &stubRegistry{identityStreams: map[string]string{
		"dom:room-a\x00alice": "gs-a1",
	}}

	parts, err := s.ListParticipants("dom:room-a")
	if err != nil {
		t.Fatalf("ListParticipants err: %v", err)
	}
	if len(parts) != 1 || parts[0].Identity != "alice" {
		t.Fatalf("participants = %+v, want [alice] only", parts)
	}
}

func TestDeleteRoom_FromSRSAPI(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-a1", "gs-b1"}
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Name: "gs-a1"},
		{ID: "cid-2", Name: "gs-b1"},
	}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom:room-a",
		"gs-b1": "dom:room-b",
	}}
	reg := &stubRegistry{}
	s.registry = reg

	if err := s.DeleteRoom("dom:room-a"); err != nil {
		t.Fatalf("DeleteRoom err: %v", err)
	}
	if len(ts.kickedIDs) != 1 || ts.kickedIDs[0] != "cid-1" {
		t.Fatalf("kickedIDs=%v, want [cid-1] (only room-a stream)", ts.kickedIDs)
	}
	if len(reg.cleared) != 1 || reg.cleared[0] != "dom:room-a" {
		t.Fatalf("cleared=%v, want [dom:room-a]", reg.cleared)
	}
}

func TestDeleteRoom_FromSRSAPI_NotFound(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	ts.mu.Lock()
	ts.streams = []string{"gs-b1"}
	ts.mu.Unlock()

	s := newServiceWithURL(ts.srv.URL)
	s.resolver = &stubResolver{roomForStream: map[string]string{"gs-b1": "dom:room-b"}}
	s.registry = &stubRegistry{}

	assertSRSAppErrorCode(t, s.DeleteRoom("dom:room-a"), pkg.NOT_FOUND)
}

func TestRoomMatches_ExactKeyOnly(t *testing.T) {
	cases := []struct {
		name      string
		candidate string
		room      string
		want      bool
	}{
		{"same composite key", "dom-a:lobby", "dom-a:lobby", true},
		{"same platform key", "lobby", "lobby", true},
		{"cross-domain same name must NOT match", "dom-b:lobby", "lobby", false},
		{"bare name must NOT match composite", "dom-a:lobby", "lobby", false},
		{"different domain same room", "dom-a:lobby", "dom-b:lobby", false},
		{"empty keys", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := roomMatches(tc.candidate, tc.room); got != tc.want {
				t.Fatalf("roomMatches(%q, %q) = %v, want %v", tc.candidate, tc.room, got, tc.want)
			}
		})
	}
}

func TestListRooms_FallsBackToRegistry_WhenSRSUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "srs down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	svc := NewService(&config.Config{SRSHost: "127.0.0.1", SRSApiPort: "1"})
	_ = srv
	svc.SetRoomRegistry(&stubRegistry{
		rooms:   []string{"dom-a:r1"},
		streams: map[string][]string{"dom-a:r1": {"gs-1"}},
	})
	svc.SetStreamRoomResolver(&stubResolver{})

	rooms, err := svc.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms should fall back to registry, got err=%v", err)
	}
	if len(rooms) != 1 || rooms[0].Name != "dom-a:r1" {
		t.Fatalf("expected registry room dom-a:r1, got %+v", rooms)
	}
}

func TestListRooms_MemberCount_DeduplicatedByIdentity(t *testing.T) {
	ts := newSRSTestServer()
	ts.streams = []string{"gs-a1", "gs-a2", "gs-b1"}
	defer ts.srv.Close()

	reg := &stubRegistry{
		streamIdentities: map[string]string{
			"gs-a1": "alice",
			"gs-a2": "alice", // 同 identity 双 stream
			"gs-b1": "bob",
		},
	}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetRoomRegistry(reg)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1", "gs-b1": "dom-a:r1",
	}})

	rooms, err := svc.listRoomsFromSRS()
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 1 || rooms[0].MemberCount != 2 {
		t.Fatalf("expected 2 unique members in dom-a:r1, got %+v", rooms)
	}
}

func TestCollectRoomStreams_FiltersByPrefixAndRoom(t *testing.T) {
	svc := NewService(&config.Config{})
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1", "gs-b1": "dom-b:r1",
	}})
	got := svc.collectRoomStreams([]string{"gs-a1", "gs-a2", "gs-b1", "x-other"}, "dom-a:r1")
	if len(got) != 2 {
		t.Fatalf("expected 2 streams for dom-a:r1, got %+v", got)
	}
	if got[0].stream != "gs-a1" || got[1].stream != "gs-a2" {
		t.Fatalf("unexpected streams: %+v", got)
	}
}

type countingResolver struct {
	stubResolver
	calls int
}

func (r *countingResolver) RoomForStream(stream string) (string, bool) {
	r.calls++
	return r.stubResolver.RoomForStream(stream)
}

func TestStreamRoomCache_ReusesLookupsAcrossCalls(t *testing.T) {
	ts := newSRSTestServer()
	ts.streams = []string{"gs-a1"}
	defer ts.srv.Close()

	resolver := &countingResolver{stubResolver: stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1",
	}}}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetStreamRoomResolver(resolver)
	svc.SetRoomRegistry(&stubRegistry{})

	if _, err := svc.listRoomsFromSRS(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.listParticipantsFromSRS("dom-a:r1"); err != nil {
		t.Fatal(err)
	}
	if resolver.calls > 1 {
		t.Fatalf("expected <=1 RoomForStream calls across two lists, got %d", resolver.calls)
	}
}

func TestStreamRoomCache_ExpiredEntryIsEvicted(t *testing.T) {
	ts := newSRSTestServer()
	ts.streams = []string{"gs-a1"}
	defer ts.srv.Close()

	resolver := &countingResolver{stubResolver: stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1",
	}}}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetStreamRoomResolver(resolver)

	room, ok := svc.resolveStreamRoom("gs-a1")
	if !ok || room != "dom-a:r1" {
		t.Fatalf("first resolve = %q/%v, want dom-a:r1/true", room, ok)
	}
	if resolver.calls != 1 {
		t.Fatalf("first resolve calls = %d, want 1", resolver.calls)
	}

	// 回拨缓存时间戳代替固定 sleep，确定性地让缓存项过期。
	svc.cacheMu.Lock()
	entry := svc.streamCache["gs-a1"]
	entry.at = time.Now().Add(-streamRoomCacheTTL - time.Second)
	svc.streamCache["gs-a1"] = entry
	svc.cacheMu.Unlock()

	// resolver 侧变化后，过期命中必须重新反查而不是复用旧值。
	resolver.roomForStream["gs-a1"] = "dom-a:r2"
	room, ok = svc.resolveStreamRoom("gs-a1")
	if !ok || room != "dom-a:r2" {
		t.Fatalf("expired resolve = %q/%v, want dom-a:r2/true", room, ok)
	}
	if resolver.calls != 2 {
		t.Fatalf("expired resolve calls = %d, want 2 (cache must not serve stale entry)", resolver.calls)
	}

	// resolver 缺失时过期项必须被淘汰，不能长期残留在 map 中。
	svc.cacheMu.Lock()
	entry = svc.streamCache["gs-a1"]
	entry.at = time.Now().Add(-streamRoomCacheTTL - time.Second)
	svc.streamCache["gs-a1"] = entry
	svc.cacheMu.Unlock()
	svc.SetStreamRoomResolver(nil)

	if _, ok := svc.resolveStreamRoom("gs-a1"); ok {
		t.Fatal("resolve without resolver must not report found")
	}
	svc.cacheMu.Lock()
	_, retained := svc.streamCache["gs-a1"]
	svc.cacheMu.Unlock()
	if retained {
		t.Fatal("expired cache entry must be evicted, not retained indefinitely")
	}
}

func TestDeleteRoom_PartialKick_DoesNotClearRegistry(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.srv.Close()
	ts.streams = []string{"gs-a1", "gs-a2"}
	ts.clients = []clientsResponseClient{
		{ID: "cid-1", Name: "gs-a1"},
		{ID: "cid-2", Name: "gs-a2"},
	}
	ts.kickFail["cid-2"] = true

	reg := &stubRegistry{
		streams: map[string][]string{"dom-a:r1": {"gs-a1", "gs-a2"}},
	}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetRoomRegistry(reg)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1",
	}})

	err := svc.DeleteRoom("dom-a:r1")
	if err == nil {
		t.Fatal("partial kick must surface as error")
	}
	if len(reg.cleared) != 0 {
		t.Fatalf("registry must NOT be cleared on partial failure, cleared=%v", reg.cleared)
	}
}

func TestDeleteRoom_RecheckListFailure_DoesNotClearRegistry(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.srv.Close()
	ts.streams = []string{"gs-a1"}
	ts.clients = []clientsResponseClient{{ID: "cid-1", Name: "gs-a1"}}
	ts.failStreamsAfterKick = true

	reg := &stubRegistry{
		streams: map[string][]string{"dom-a:r1": {"gs-a1"}},
	}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetRoomRegistry(reg)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1",
	}})

	err := svc.DeleteRoom("dom-a:r1")
	if err == nil {
		t.Fatal("re-check list failure must surface as partial failure")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.SFU_ERROR {
		t.Fatalf("expected SFU_ERROR AppError, got %v", err)
	}
	if !errors.Is(err, errDeleteRoomPartial) {
		t.Fatalf("expected recognizable partial-failure error, got %v", err)
	}
	if len(reg.cleared) != 0 {
		t.Fatalf("registry must NOT be cleared when re-check fails, cleared=%v", reg.cleared)
	}
}

func TestDeleteRoom_NewStreamJoinedDuringKick_DoesNotClearRegistry(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.srv.Close()
	ts.streams = []string{"gs-a1"}
	ts.clients = []clientsResponseClient{{ID: "cid-1", Name: "gs-a1"}}
	ts.afterKickAddStream = "gs-a2"

	reg := &stubRegistry{
		streams: map[string][]string{"dom-a:r1": {"gs-a1"}},
	}
	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetRoomRegistry(reg)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-a1": "dom-a:r1", "gs-a2": "dom-a:r1",
	}})

	err := svc.DeleteRoom("dom-a:r1")
	if err == nil {
		t.Fatal("new stream joined during kick must surface as error")
	}
	if len(reg.cleared) != 0 {
		t.Fatalf("registry must NOT be cleared when streams remain, cleared=%v", reg.cleared)
	}
}

func TestService_ConcurrentInjectAndList_NoRace(t *testing.T) {
	svc := NewService(&config.Config{})
	svc.SetRoomRegistry(&stubRegistry{})
	svc.SetStreamRoomResolver(&stubResolver{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.SetStreamRoomResolver(&stubResolver{})
			svc.SetMuteRuleStore(newMemMuteRuleStore())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.ListRooms()
			_ = svc.MuteParticipantTimed("r", "alice", "", true, 0)
		}()
	}
	wg.Wait()
}

func TestListParticipants_UnresolvableStream_IncludedWithPlaceholder(t *testing.T) {
	ts := newSRSTestServer()
	ts.streams = []string{"gs-orphan"}
	defer ts.srv.Close()

	svc := NewService(&config.Config{})
	svc.client = NewClient(ts.srv.URL)
	svc.SetStreamRoomResolver(&stubResolver{roomForStream: map[string]string{
		"gs-orphan": "dom-a:r1",
	}})
	svc.SetRoomRegistry(&stubRegistry{})

	parts, err := svc.listParticipantsFromSRS("dom-a:r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("unresolvable active stream must still be listed, got %+v", parts)
	}
}

func TestListParticipants_FallsBackToRegistry_WhenSRSStreamsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/streams/" && r.Method == http.MethodGet:
			http.Error(w, "srs streams down", http.StatusServiceUnavailable)
		case r.URL.Path == "/api/v1/clients/" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(clientsResponse{
				Code:    0,
				Clients: []clientsResponseClient{{ID: "cid-1", Name: "gs-a"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s := newServiceWithURL(srv.URL)
	s.registry = &stubRegistry{
		streams: map[string][]string{"room-x": {"gs-a"}},
		identityStreams: map[string]string{
			"room-x\x00alice": "gs-a",
		},
	}
	s.resolver = &stubResolver{roomForStream: map[string]string{"gs-a": "room-x"}}

	parts, err := s.ListParticipants("room-x")
	if err != nil {
		t.Fatalf("ListParticipants should fall back to registry, got err=%v", err)
	}
	if len(parts) != 1 || parts[0].Identity != "alice" {
		t.Fatalf("expected registry participant alice, got %+v", parts)
	}
}

func TestDeleteRoom_FallsBackToRegistry_WhenSRSStreamsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/streams/" && r.Method == http.MethodGet:
			http.Error(w, "srs streams down", http.StatusServiceUnavailable)
		case r.URL.Path == "/api/v1/clients/" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(clientsResponse{
				Code:    0,
				Clients: []clientsResponseClient{{ID: "cid-1", Name: "gs-a"}},
			})
		case r.URL.Path == "/api/v1/clients/cid-1" && r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reg := &stubRegistry{
		streams: map[string][]string{"room-x": {"gs-a"}},
	}
	s := newServiceWithURL(srv.URL)
	s.registry = reg
	s.resolver = &stubResolver{roomForStream: map[string]string{"gs-a": "room-x"}}

	if err := s.DeleteRoom("room-x"); err != nil {
		t.Fatalf("DeleteRoom should fall back to registry, got err=%v", err)
	}
	if len(reg.cleared) != 1 || reg.cleared[0] != "room-x" {
		t.Fatalf("expected registry room room-x cleared, got %+v", reg.cleared)
	}
}


// memMuteRuleStore 是测试用 MuteRuleStore 实现（不进入生产代码，杜绝被误用作降级后端）。
type memMuteRuleStore struct {
	mu   sync.Mutex
	data map[string]memMuteEntry
}

type memMuteEntry struct {
	ruleID    int
	expiresAt time.Time
}

func newMemMuteRuleStore() *memMuteRuleStore {
	return &memMuteRuleStore{data: map[string]memMuteEntry{}}
}

func (s *memMuteRuleStore) Backend() string { return "mem" }

func (s *memMuteRuleStore) Save(_ context.Context, key string, ruleID int, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ruleID <= 0 {
		delete(s.data, key)
		return nil
	}
	e := memMuteEntry{ruleID: ruleID}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = e
	return nil
}

func (s *memMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok {
		return 0, nil
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(s.data, key)
		return 0, nil
	}
	return e.ruleID, nil
}

func (s *memMuteRuleStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}
