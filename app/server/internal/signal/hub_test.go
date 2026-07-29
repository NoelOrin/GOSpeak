package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/ws"
)

// ─── Mock roomStore ───

type mockRoomStore struct {
	rooms []model.Room
}

func (m *mockRoomStore) List(page, pageSize int, roomType string) ([]model.Room, int64, error) {
	return m.rooms, int64(len(m.rooms)), nil
}

func (m *mockRoomStore) GetByName(name string) (*model.Room, error) {
	for _, r := range m.rooms {
		if r.Name == name {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("room not found: %s", name)
}

func newMockRoomStore(names ...string) *mockRoomStore {
	rooms := make([]model.Room, 0, len(names))
	for _, name := range names {
		rooms = append(rooms, model.Room{
			Name:      name,
			CreatedAt: time.Now(),
		})
	}
	return &mockRoomStore{rooms: rooms}
}

// ─── Mock ws.ClientMessenger ───

type mockClient struct {
	id      string
	claims  *pkg.Claims
	emitted []interface{}
	mu      sync.Mutex
}

func newMockClient(id string) *mockClient {
	return &mockClient{id: id}
}

func newAuthedMockClient(id, username string) *mockClient {
	return &mockClient{
		id:     id,
		claims: &pkg.Claims{Username: username, UserUUID: username, Role: "user"},
	}
}

func (m *mockClient) ID() string { return m.id }

func (m *mockClient) Claims() *pkg.Claims { return m.claims }

func (m *mockClient) Send(v interface{}) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitted = append(m.emitted, v)
	return true
}

func (m *mockClient) SendACK(id, event string, data interface{}) {
	m.Send(map[string]interface{}{"id": id, "event": event, "data": data})
}

func (m *mockClient) SendErrorACK(id, event string, code int, message string) {
	m.Send(map[string]interface{}{"id": id, "event": event, "error": map[string]interface{}{"code": code, "message": message}})
}

func (m *mockClient) Close() {}

// lastEvent returns the data of the most recent message with the given event name.
func (m *mockClient) lastEvent(event string) interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.emitted) - 1; i >= 0; i-- {
		if v, ok := m.emitted[i].(map[string]interface{}); ok && v["event"] == event {
			return v["data"]
		}
	}
	return nil
}

// hasEvent returns true if any emitted message has the given event name.
func (m *mockClient) hasEvent(event string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.emitted {
		if vm, ok := v.(map[string]interface{}); ok && vm["event"] == event {
			return true
		}
	}
	return false
}

// ─── Mock ws.Broadcaster ───

type mockBroadcaster struct {
	broadcasts map[string][]interface{}
	roomCasts  map[string]map[string][]interface{}
	clients    map[string]ws.ClientMessenger
	rooms      map[string]map[string]bool
	joined     map[string]map[string]bool // clientID -> rooms joined
	left       map[string]map[string]bool // clientID -> rooms left
}

func newMockBroadcaster() *mockBroadcaster {
	return &mockBroadcaster{
		broadcasts: make(map[string][]interface{}),
		roomCasts:  make(map[string]map[string][]interface{}),
		clients:    make(map[string]ws.ClientMessenger),
		rooms:      make(map[string]map[string]bool),
		joined:     make(map[string]map[string]bool),
		left:       make(map[string]map[string]bool),
	}
}

// helper methods for test assertions
func (m *mockBroadcaster) didJoin(clientID, room string) bool {
	return m.joined[clientID] != nil && m.joined[clientID][room]
}
func (m *mockBroadcaster) didLeave(clientID, room string) bool {
	return m.left[clientID] != nil && m.left[clientID][room]
}

func (m *mockBroadcaster) Add(c *ws.Client) {
	m.clients[c.ID()] = c
}

func (m *mockBroadcaster) Remove(clientID string) []string {
	delete(m.clients, clientID)
	return nil
}

func (m *mockBroadcaster) Join(room, clientID string) {
	if m.rooms[room] == nil {
		m.rooms[room] = make(map[string]bool)
	}
	m.rooms[room][clientID] = true
	if m.joined[clientID] == nil {
		m.joined[clientID] = make(map[string]bool)
	}
	m.joined[clientID][room] = true
}

func (m *mockBroadcaster) Leave(room, clientID string) {
	if m.rooms[room] != nil {
		delete(m.rooms[room], clientID)
	}
	if m.left[clientID] == nil {
		m.left[clientID] = make(map[string]bool)
	}
	m.left[clientID][room] = true
}

func (m *mockBroadcaster) BroadcastToNamespace(event string, data interface{}) {
	m.broadcasts[event] = append(m.broadcasts[event], data)
}

func (m *mockBroadcaster) BroadcastToRoom(room, event string, data interface{}) {
	if m.roomCasts[room] == nil {
		m.roomCasts[room] = make(map[string][]interface{})
	}
	m.roomCasts[room][event] = append(m.roomCasts[room][event], data)
}

func (m *mockBroadcaster) ForEach(room string, fn func(ws.ClientMessenger) bool) {
	// No-op in tests
}

func (m *mockBroadcaster) RoomExists(room string) bool {
	return len(m.rooms[room]) > 0
}

func (m *mockBroadcaster) GetClient(clientID string) ws.ClientMessenger {
	return m.clients[clientID]
}


// assertJoined checks if client joined room via the mock broadcaster.
func assertJoined(t *testing.T, hub *Hub, clientID, room string) {
	t.Helper()
	mb, ok := hub.fanout.(*mockBroadcaster)
	if !ok || !mb.didJoin(clientID, room) {
		t.Fatalf("expected client %s to have joined room %s", clientID, room)
	}
}

// assertNotJoined checks that client did NOT join room.
func assertNotJoined(t *testing.T, hub *Hub, clientID, room string) {
	t.Helper()
	mb, ok := hub.fanout.(*mockBroadcaster)
	if ok && mb.didJoin(clientID, room) {
		t.Fatalf("expected client %s NOT to have joined room %s", clientID, room)
	}
}

// assertLeft checks if client left room via the mock broadcaster.
func assertLeft(t *testing.T, hub *Hub, clientID, room string) {
	t.Helper()
	mb, ok := hub.fanout.(*mockBroadcaster)
	if !ok || !mb.didLeave(clientID, room) {
		t.Fatalf("expected client %s to have left room %s", clientID, room)
	}
}


// newTestHub returns a Hub pre-configured with mock broadcaster and stream resolver.
// Use in tests that don't need custom setup.
func newTestHub() *Hub {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetStreamResolver(fakeStreamResolver{})
	return hub
}
type fakeStreamResolver struct{}

func (fakeStreamResolver) StreamName(room, identity string) string {
	return "server-computed-" + room + "-" + identity
}

func decodeAck(t *testing.T, ack string) map[string]interface{} {
	t.Helper()
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(ack), &data); err != nil {
		t.Fatalf("failed to decode ack: %v", err)
	}
	return data
}

// ─── OnRoomCreate Tests ───

func TestHub_OnRoomCreate_Success(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	conn := newAuthedMockClient("socket-1", "user-1")
	data := `{"room":"test-room"}`

	hub.OnRoomCreate(conn, data)

	if !conn.hasEvent(EventRoomCreated) {
		t.Fatal("expected room:created event to be emitted")
	}

	if _, exists := hub.rooms["test-room"]; !exists {
		t.Fatal("expected room to be created in hub")
	}

	if len(server.broadcasts[EventRoomUpdated]) == 0 {
		t.Fatal("expected room:updated broadcast")
	}
}

func TestHub_OnRoomCreate_Duplicate(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	conn1 := newMockClient("socket-1")

	hub.rooms["existing-room"] = &Room{Name: "existing-room", Members: make(map[string]*MemberInfo)}

	data := `{"room":"existing-room"}`
	hub.OnRoomCreate(conn1, data)

	emitData, ok := conn1.lastEvent(EventRoomCreated).(map[string]interface{})
	if !ok {
		t.Fatal("expected event")
	}

	if emitData["error"] != "room already exists" {
		t.Errorf("expected error message, got %v", emitData["error"])
	}
}

func TestHub_OnRoomCreate_InvalidJSON(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomCreate(conn, "not json")

	emitData, ok := conn.lastEvent(EventRoomCreated).(map[string]interface{})
	if !ok {
		t.Fatal("expected event")
	}

	if emitData["error"] != "room name is required" {
		t.Errorf("expected error message, got %v", emitData["error"])
	}
}

func TestHub_OnRoomCreate_MissingRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomCreate(conn, `{}`)

	emitData, ok := conn.lastEvent(EventRoomCreated).(map[string]interface{})
	if !ok {
		t.Fatal("expected event")
	}

	if emitData["error"] != "room name is required" {
		t.Errorf("expected error message, got %v", emitData["error"])
	}
}

// ─── OnRoomJoin Tests ───

func TestHub_OnRoomJoin_CreateAndJoin(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	conn := newAuthedMockClient("socket-1", "user-1")
	data := `{"room":"test-room","identity":"user-1"}`

	ack, err := hub.OnRoomJoin(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertJoined(t, hub, conn.ID(), "test-room")
	// OnRoomJoin ack does not include members -- members come from OnRoomJoinSFU
	joinData := decodeAck(t, ack)
	if _, hasMembers := joinData["members"]; hasMembers {
		t.Error("OnRoomJoin ack should not include members")
	}

	sfuAck, err := hub.OnRoomJoinSFU(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sfuData := decodeAck(t, sfuAck)

	switch members := sfuData["members"].(type) {
	case []interface{}:
		if len(members) != 1 {
			t.Errorf("expected 1 member in room, got %d", len(members))
		}
	case []MemberInfo:
		if len(members) != 1 {
			t.Errorf("expected 1 member in room, got %d", len(members))
		}
	default:
		t.Errorf("expected members slice, got %T: %v", sfuData["members"], sfuData["members"])
	}
}

func TestHub_OnRoomJoin_JoinExisting(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["test-room"] = &Room{
		Name:      "test-room",
		Members:   make(map[string]*MemberInfo),
		CreatedAt: time.Now(),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newAuthedMockClient("socket-2", "user-2")
	data := `{"room":"test-room","identity":"user-2"}`

	ack, err := hub.OnRoomJoin(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertJoined(t, hub, conn.ID(), "test-room")

	// Members come from OnRoomJoinSFU ack
	joinData := decodeAck(t, ack)
	if _, hasMembers := joinData["members"]; hasMembers {
		t.Error("OnRoomJoin ack should not include members")
	}

	sfuAck, err := hub.OnRoomJoinSFU(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sfuData := decodeAck(t, sfuAck)

	switch members := sfuData["members"].(type) {
	case []interface{}:
		if len(members) != 2 {
			t.Errorf("expected 2 members, got %d", len(members))
		}
	case []MemberInfo:
		if len(members) != 2 {
			t.Errorf("expected 2 members, got %d", len(members))
		}
	default:
		t.Errorf("expected members slice, got %T: %v", sfuData["members"], sfuData["members"])
	}
}

func TestHub_OnRoomJoin_IdentityFromClaims(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	conn := newAuthedMockClient("socket-1", "user-1")
	data := `{"room":"test-room"}`

	if _, err := hub.OnRoomJoin(conn, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OnRoomJoin creates fanout room but not h.rooms — need OnRoomJoinSFU
	if _, err := hub.OnRoomJoinSFU(conn, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	room := hub.rooms["test-room"]
	if room == nil {
		t.Fatal("expected room to be created after OnRoomJoinSFU")
	}

	member := room.Members["socket-1"]
	if member == nil {
		t.Fatal("expected member in room")
	}

	if member.Identity != "user-1" {
		t.Errorf("expected identity from JWT claims, got %s", member.Identity)
	}
}

func TestHub_OnRoomJoin_RoomFull(t *testing.T) {
	store := newMockRoomStore("limited-room")
	store.rooms[0].Limit = 1
	hub := NewHub(store, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	// 先让一个用户加入
	hub.rooms["limited-room"] = &Room{
		Name: "limited-room",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
		},
		CreatedAt: time.Now(),
	}

	// 第二个用户尝试加入，应该被拒绝
	conn := newAuthedMockClient("socket-2", "user-2")
	ack, err := hub.OnRoomJoin(conn, `{"room":"limited-room","identity":"user-2"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emitData := decodeAck(t, ack)

	if emitData["error"] != "room is full" {
		t.Errorf("expected 'room is full' error, got %v", emitData["error"])
	}

	// socket-2 不应该被加入房间
	assertNotJoined(t, hub, conn.ID(), "limited-room")
}

// ─── OnRoomLeave Tests ───

func TestHub_OnRoomLeave_Success(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}
	// Add a second member so the room isn't deleted after socket-1 leaves
	hub.rooms["test-room"].Members["socket-2"] = &MemberInfo{
		ID:       "socket-2",
		Identity: "user-2",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	data := `{"room":"test-room"}`

	resp, err := hub.OnRoomLeave(conn, data)
	if err != nil {
		t.Fatalf("OnRoomLeave failed: %v", err)
	}
	_ = resp

	assertLeft(t, hub, conn.ID(), "test-room")

	if _, exists := hub.rooms["test-room"].Members["socket-1"]; exists {
		t.Fatal("expected member to be removed from room")
	}

	emitted, ok := conn.lastEvent(EventRoomLeft).(map[string]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:left event")
	}
}

func TestHub_OnRoomLeave_RemoveEmptyRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	data := `{"room":"test-room"}`

	resp, err := hub.OnRoomLeave(conn, data)
	if err != nil {
		t.Fatalf("OnRoomLeave failed: %v", err)
	}
	_ = resp

	if _, exists := hub.rooms["test-room"]; exists {
		t.Fatal("expected empty room to be removed")
	}
}

// ─── OnRoomList Tests ───

func TestHub_OnRoomList_Empty(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomList(conn)

	emitData, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected event")
	}

	if count, ok := emitData["count"].(int); !ok || count != 0 {
		t.Errorf("expected count 0, got %v", emitData["count"])
	}
}

func TestHub_OnRoomList_Multiple(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["room-1"] = &Room{
		Name:      "room-1",
		Members:   make(map[string]*MemberInfo),
		CreatedAt: time.Now(),
	}
	hub.rooms["room-1"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	hub.rooms["room-2"] = &Room{
		Name:      "room-2",
		Members:   make(map[string]*MemberInfo),
		CreatedAt: time.Now(),
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn)

	emitData, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}

	if count, ok := emitData["count"].(int); !ok || count != 2 {
		t.Errorf("expected count 2, got %v", emitData["count"])
	}
}

// ─── OnDisconnect Tests ───

func TestHub_OnDisconnect_RemoveFromRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}
	hub.rooms["test-room"].Members["socket-2"] = &MemberInfo{
		ID:       "socket-2",
		Identity: "user-2",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnDisconnect(conn)

	if _, exists := hub.rooms["test-room"].Members["socket-1"]; exists {
		t.Fatal("expected member to be removed on disconnect")
	}

	if _, exists := hub.rooms["test-room"].Members["socket-2"]; !exists {
		t.Fatal("expected other member to remain")
	}
}

func TestHub_OnDisconnect_RemoveEmptyRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnDisconnect(conn)

	if _, exists := hub.rooms["test-room"]; exists {
		t.Fatal("expected empty room to be removed on disconnect")
	}
}

func TestHub_OnDisconnect_MultipleRooms(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	hub.rooms["room-1"] = &Room{
		Name: "room-1",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
		},
	}
	hub.rooms["room-2"] = &Room{
		Name: "room-2",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
			"socket-2": {ID: "socket-2", Identity: "user-2", JoinedAt: time.Now().UnixMilli()},
		},
	}

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnDisconnect(conn)

	if _, exists := hub.rooms["room-1"]; exists {
		t.Fatal("expected empty room-1 to be removed")
	}

	if _, exists := hub.rooms["room-2"].Members["socket-1"]; exists {
		t.Fatal("expected socket-1 to be removed from room-2")
	}

	if _, exists := hub.rooms["room-2"].Members["socket-2"]; !exists {
		t.Fatal("expected socket-2 to remain in room-2")
	}
}

// ─── GetSFURooms Tests ───

func TestHub_GetSFURooms(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)

	hub.rooms["room-1"] = &Room{
		Name:      "room-1",
		Members:   map[string]*MemberInfo{"socket-1": {ID: "socket-1"}},
		CreatedAt: time.Now(),
	}
	hub.rooms["room-2"] = &Room{
		Name:      "room-2",
		Members:   make(map[string]*MemberInfo),
		CreatedAt: time.Now(),
	}

	rooms := hub.GetSFURooms()
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}

	if rooms[0].Count > 0 && rooms[0].Name == "room-1" && rooms[0].Count != 1 {
		t.Errorf("expected room-1 to have 1 member, got %d", rooms[0].Count)
	}
}

// ─── GetRoomMembers Tests ───

func TestHub_GetRoomMembers(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)

	hub.rooms["test-room"] = &Room{
		Name: "test-room",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: 1000},
			"socket-2": {ID: "socket-2", Identity: "user-2", JoinedAt: 2000},
		},
	}

	members := hub.GetRoomMembers("test-room")
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}

	ids := make(map[string]bool)
	for _, m := range members {
		ids[m.ID] = true
	}
	if !ids["socket-1"] || !ids["socket-2"] {
		t.Errorf("expected both socket IDs in members")
	}
}

func TestHub_GetRoomMembers_NotFound(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)

	members := hub.GetRoomMembers("nonexistent")
	if members != nil {
		t.Errorf("expected nil for nonexistent room, got %v", members)
	}
}

// ─── GetRooms Tests ───

func TestHub_GetMergedRooms_DBOnly(t *testing.T) {
	hub := NewHub(newMockRoomStore("综合大厅", "游戏频道", "音乐房间"), nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	rooms := hub.GetRooms()
	if len(rooms) != 3 {
		t.Errorf("expected 3 rooms from DB, got %d", len(rooms))
	}

	names := map[string]bool{}
	for _, r := range rooms {
		names[r.Name] = true
	}
	for _, name := range []string{"综合大厅", "游戏频道", "音乐房间"} {
		if !names[name] {
			t.Errorf("expected room %s in result", name)
		}
	}
}

func TestHub_GetMergedRooms_MemoryOverDB(t *testing.T) {
	hub := NewHub(newMockRoomStore("房间A", "房间B"), nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	// 内存中有一个活跃房间（有成员）
	hub.rooms["房间A"] = &Room{
		Name: "房间A",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
		},
		CreatedAt: time.Now(),
	}

	rooms := hub.GetRooms()

	// 应该有 2 个房间：DB 的 房间B + 内存覆盖的 房间A
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}

	for _, r := range rooms {
		if r.Name == "房间A" && r.Count != 1 {
			t.Errorf("expected 房间A to have 1 member (memory version), got %d", r.Count)
		}
		if r.Name == "房间B" && r.Count != 0 {
			t.Errorf("expected 房间B to have 0 members (DB version), got %d", r.Count)
		}
	}
}

func TestHub_OnRoomList_WithDB(t *testing.T) {
	store := newMockRoomStore("测试房间1", "测试房间2")
	hub := NewHub(store, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn)

	emitData, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}

	if count, ok := emitData["count"].(int); !ok || count != 2 {
		t.Errorf("expected count 2, got %v", emitData["count"])
	}
}

func TestOnRoomJoinSFU_StoresAndBroadcastsStream(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	creator := newAuthedMockClient("creator-socket", "creator")
	hub.OnRoomCreate(creator, `{"room":"r1"}`)

	member := newAuthedMockClient("mem-1", "alice")
	ackRaw, err := hub.OnRoomJoinSFU(member, `{"room":"r1","identity":"alice","stream":"gs-aaa"}`)
	if err != nil {
		t.Fatalf("OnRoomJoinSFU error: %v", err)
	}
	var ack struct {
		OK      bool         `json:"ok"`
		Members []MemberInfo `json:"members"`
	}
	if err := json.Unmarshal([]byte(ackRaw), &ack); err != nil {
		t.Fatalf("parse ack: %v", err)
	}
	if !ack.OK {
		t.Fatal("ack should be ok")
	}
	if len(ack.Members) != 1 || ack.Members[0].Stream != "gs-aaa" {
		t.Fatalf("ack members should carry stream, got %+v", ack.Members)
	}

	hub.mu.RLock()
	stored := hub.rooms["r1"].Members["mem-1"].Stream
	hub.mu.RUnlock()
	if stored != "gs-aaa" {
		t.Fatalf("MemberInfo.Stream should be stored, got %q", stored)
	}

	broadcasted := false
	if roomEvents, ok := server.roomCasts["r1"]; ok {
		if vals, ok := roomEvents[EventMemberJoined]; ok && len(vals) > 0 {
			payload, _ := json.Marshal(vals[0])
			if strings.Contains(string(payload), "gs-aaa") {
				broadcasted = true
			}
		}
	}
	if !broadcasted {
		t.Fatal("member:joined should broadcast stream")
	}
}

func TestOnRoomJoinSFU_ServerRecomputesStream(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockBroadcaster()
	hub.fanout = server

	// 注入 fake resolver 模拟 SRS 服务端流名计算
	hub.SetStreamResolver(fakeStreamResolver{})

	creator := newAuthedMockClient("creator-socket", "creator")
	hub.OnRoomCreate(creator, `{"room":"r1"}`)

	// 客户端提报错误 stream，服务端应覆写
	member := newAuthedMockClient("mem-1", "alice")
	ackRaw, err := hub.OnRoomJoinSFU(member, `{"room":"r1","identity":"alice","stream":"client-supplied-bad"}`)
	if err != nil {
		t.Fatalf("OnRoomJoinSFU error: %v", err)
	}
	var ack struct {
		OK      bool         `json:"ok"`
		Members []MemberInfo `json:"members"`
	}
	if err := json.Unmarshal([]byte(ackRaw), &ack); err != nil {
		t.Fatalf("parse ack: %v", err)
	}
	if !ack.OK {
		t.Fatal("ack should be ok")
	}

	// 成员列表中的 stream 应为服务端计算值
	expected := "server-computed-r1-alice"
	if len(ack.Members) != 1 || ack.Members[0].Stream != expected {
		t.Fatalf("expected stream %q in ack members, got %+v", expected, ack.Members)
	}

	// Hub 中存储的 stream 也应为服务端计算值
	hub.mu.RLock()
	stored := hub.rooms["r1"].Members["mem-1"].Stream
	hub.mu.RUnlock()
	if stored != expected {
		t.Fatalf("MemberInfo.Stream should be %q, got %q", expected, stored)
	}

	// 广播中的 stream 也应为服务端计算值
	broadcasted := false
	if roomEvents, ok := server.roomCasts["r1"]; ok {
		if vals, ok := roomEvents[EventMemberJoined]; ok && len(vals) > 0 {
			payload, _ := json.Marshal(vals[0])
			if expectedInJSON := `"stream":"` + expected + `"`; strings.Contains(string(payload), expectedInJSON) {
				broadcasted = true
			}
		}
	}
	if !broadcasted {
		t.Fatal("member:joined should broadcast server-computed stream")
	}
}

func TestStreamRegistry(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	if hub.IsStreamActive("gs-x") {
		t.Fatal("unknown stream should not be active")
	}
	hub.RegisterStream("gs-x")
	if !hub.IsStreamActive("gs-x") {
		t.Fatal("registered stream should be active")
	}
	hub.UnregisterStream("gs-x")
	if hub.IsStreamActive("gs-x") {
		t.Fatal("unregistered stream should not be active")
	}
}

// TestHub_RoomRegistry_JoinLeave 验证 WS join/leave 同步 room→streams 聚合视图，
// 供 SRS 等无原生 room 维度的 provider 查询。
func TestHub_RoomRegistry_JoinLeave(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"room-a"}`)
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"room-a","identity":"user-1"}`); err != nil {
		t.Fatalf("join: %v", err)
	}

	// join 后 registry 应聚合出 room
	rooms := hub.Rooms()
	if len(rooms) != 1 || rooms[0] != "room-a" {
		t.Fatalf("expected [room-a], got %v", rooms)
	}
	streams := hub.Streams("room-a")
	if len(streams) != 1 || streams[0] != "server-computed-room-a-user-1" {
		t.Fatalf("expected computed stream, got %v", streams)
	}

	// leave 后聚合清空
	if _, err := hub.OnRoomLeave(conn, `{"room":"room-a"}`); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if got := hub.Rooms(); len(got) != 0 {
		t.Fatalf("expected no rooms after leave, got %v", got)
	}
	if got := hub.Streams("room-a"); got != nil {
		t.Fatalf("expected nil streams after leave, got %v", got)
	}
}

// TestHub_RoomRegistry_CallbackReverseLookup 验证 SRS on_publish 回调
// 仅给 stream 名时，经 streamRoomCache 反查 room 同步 roomStreams。
func TestHub_RoomRegistry_CallbackReverseLookup(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"room-b"}`)
	hub.OnRoomJoinSFU(conn, `{"room":"room-b","identity":"user-1"}`)

	stream := "server-computed-room-b-user-1"
	// on_publish 回调登记（模拟 SRS 推流确认）
	hub.RegisterStream(stream)

	// activeStreams 与 roomStreams 双登记
	if !hub.IsStreamActive(stream) {
		t.Fatal("stream should be active")
	}
	if got := hub.Streams("room-b"); len(got) != 1 || got[0] != stream {
		t.Fatalf("roomStreams should contain stream after callback, got %v", got)
	}

	// on_unpublish 清两表
	hub.UnregisterStream(stream)
	if hub.IsStreamActive(stream) {
		t.Fatal("stream should be inactive after unpublish")
	}
	// room 仍有 WS 成员，roomStreams 应保留（成员仍在）
	if got := hub.Streams("room-b"); len(got) != 0 {
		t.Fatalf("roomStreams should be empty after unpublish (stream removed), got %v", got)
	}
}

// TestHub_RoomRegistry_Disconnect 验证 OnDisconnect 清成员 stream 映射。
func TestHub_RoomRegistry_Disconnect(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"room-c"}`)
	hub.OnRoomJoinSFU(conn, `{"room":"room-c","identity":"user-1"}`)

	hub.OnDisconnect(conn)

	if got := hub.Rooms(); len(got) != 0 {
		t.Fatalf("expected no rooms after disconnect, got %v", got)
	}
}

// TestHub_RoomRegistry_ClearRoom 验证 ClearRoom 重置 room 聚合状态。
func TestHub_RoomRegistry_ClearRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetStreamResolver(fakeStreamResolver{})

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"room-d"}`)
	hub.OnRoomJoinSFU(conn, `{"room":"room-d","identity":"user-1"}`)

	hub.ClearRoom("room-d")
	if got := hub.Streams("room-d"); got != nil {
		t.Fatalf("expected nil streams after ClearRoom, got %v", got)
	}
	if got := hub.Rooms(); len(got) != 0 {
		t.Fatalf("expected no rooms after ClearRoom, got %v", got)
	}
}

// ─── IsRoomMember KV-priority tests ───

func TestIsRoomMember_KVPriority_FindsInKV(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()

	kv := newMemStateStore()
	kv.PutRoomMembers(context.Background(), bus.RoomMembersSnapshot{
		Room: "room-x",
		Members: []bus.MemberRecord{
			{Room: "room-x", Identity: "charlie", InstanceID: "inst-other"},
		},
	})
	hub.SetMembershipStore(kv, "inst-local")

	// local rooms is empty
	if !hub.IsRoomMember("room-x", "charlie") {
		t.Fatal("expected IsRoomMember true from KV, got false")
	}
	if hub.IsRoomMember("room-x", "unknown") {
		t.Fatal("expected IsRoomMember false for unknown identity")
	}
}

func TestIsRoomMember_KVPriority_MissThenLocal(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-y", map[string]string{"sock-1": "dave"})

	kv := newMemStateStore()
	hub.SetMembershipStore(kv, "inst-local")

	// KV miss — room-y not in KV
	if !hub.IsRoomMember("room-y", "dave") {
		t.Fatal("expected IsRoomMember true from local fallback, got false")
	}
}

func TestIsRoomMember_KVPriority_NotFound(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-z", map[string]string{"sock-1": "eve"})

	kv := newMemStateStore()
	hub.SetMembershipStore(kv, "inst-local")

	if hub.IsRoomMember("room-z", "stranger") {
		t.Fatal("expected IsRoomMember false for stranger, got true")
	}
}

func TestIsRoomMember_KVPriority_NilMembershipStore(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-a", map[string]string{"sock-1": "alice"})

	// membershipStore is nil — should use local map directly
	if !hub.IsRoomMember("room-a", "alice") {
		t.Fatal("expected IsRoomMember true from local with nil KV")
	}
}

func TestIsRoomMember_KVPriority_KVMissReturnsFalse(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	hub.fanout = newMockBroadcaster()

	kv := newMemStateStore()
	hub.SetMembershipStore(kv, "inst-local")

	// Neither KV nor local has anyone
	if hub.IsRoomMember("empty-room", "nobody") {
		t.Fatal("expected false for empty room")
	}
}
