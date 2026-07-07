package srs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// stubRegistry 模拟 pkg.RoomRegistry 用于 SRS provider 测试。
type stubRegistry struct {
	rooms   []string
	streams map[string][]string
	cleared []string
}

func (s *stubRegistry) Rooms() []string      { return s.rooms }
func (s *stubRegistry) Streams(room string) []string {
	return s.streams[room]
}
func (s *stubRegistry) ClearRoom(room string) {
	s.cleared = append(s.cleared, room)
	delete(s.streams, room)
}

// srsTestServer 用 httptest 模拟 SRS HTTP API（/api/v1/clients/ + kick）。
type srsTestServer struct {
	srv       *httptest.Server
	mu        sync.Mutex
	clients   []clientsResponseClient
	kickedIDs []string
	kickFail  map[string]bool // id -> 模拟 kick 失败
}

func newSRSTestServer() *srsTestServer {
	ts := &srsTestServer{kickFail: map[string]bool{}}
	mux := http.NewServeMux()
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
	mux.HandleFunc("/api/v1/streams", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(streamsResponse{Code: 0})
	})
	ts.srv = httptest.NewServer(mux)
	return ts
}

func (ts *srsTestServer) close() { ts.srv.Close() }

func newServiceWithURL(baseURL string) *Service {
	return &Service{
		client:    NewClient(baseURL),
		whipURL:   "/rtc/v1/whip/",
		serverURL: baseURL,
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
	rooms, ok := got.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", got)
	}
	sort.Strings(rooms)
	if !reflect.DeepEqual(rooms, []string{"room-a", "room-b"}) {
		t.Fatalf("expected [room-a room-b], got %v", rooms)
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
	rooms, ok := got.([]string)
	if !ok || len(rooms) != 0 {
		t.Fatalf("expected empty []string, got %T %v", got, got)
	}
}

func TestListRooms_NoRegistry_Fallback(t *testing.T) {
	ts := newSRSTestServer()
	defer ts.close()
	s := newServiceWithURL(ts.srv.URL)
	// registry 为 nil，降级走 /api/v1/streams

	got, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms fallback: %v", err)
	}
	rooms, ok := got.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", got)
	}
	if len(rooms) != 0 {
		t.Fatalf("expected empty streams, got %v", rooms)
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
