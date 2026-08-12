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

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
)

// stubRegistry 模拟 pkg.RoomRegistry 用于 SRS provider 测试。
type stubRegistry struct {
	rooms           []string
	streams         map[string][]string
	cleared         []string
	identityStreams map[string]string // "room\x00identity" -> stream
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
	srv       *httptest.Server
	mu        sync.Mutex
	clients   []clientsResponseClient
	streams   []string
	kickedIDs []string
	kickFail  map[string]bool // id -> 模拟 kick 失败
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
	store := sfu.NewMemoryMuteRuleStore()
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
	store := sfu.NewMemoryMuteRuleStore()
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
