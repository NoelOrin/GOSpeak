package signal

import (
	"encoding/json"
	"testing"
	"time"

	socketio "github.com/googollee/go-socket.io"
)

// ─── Mock Socket.IO Connection ───

type mockConn struct {
	id      string
	emitted map[string]interface{}
	joined  map[string]bool
	left    map[string]bool
}

func newMockConn(id string) *mockConn {
	return &mockConn{
		id:      id,
		emitted: make(map[string]interface{}),
		joined:  make(map[string]bool),
		left:    make(map[string]bool),
	}
}

func (m *mockConn) ID() string                            { return m.id }
func (m *mockConn) Close() error                          { return nil }
func (m *mockConn) URL() string                           { return "" }
func (m *mockConn) LocalAddr() interface{}                { return nil }
func (m *mockConn) RemoteAddr() interface{}               { return nil }
func (m *mockConn) RemoteHeader() interface{}             { return nil }
func (m *mockConn) Context() interface{}                  { return nil }
func (m *mockConn) SetContext(v interface{})              {}
func (m *mockConn) Namespace() string                     { return "/" }
func (m *mockConn) Leave(room string) error {
	m.left[room] = true
	return nil
}
func (m *mockConn) Join(room string) error {
	m.joined[room] = true
	return nil
}
func (m *mockConn) To(room string, ignore ...string) socketio.Broadcast {
	return nil
}
func (m *mockConn) Emit(event string, v ...interface{}) {
	m.emitted[event] = v
}
func (m *mockConn) Send(v ...interface{}) error {
	return nil
}

// ─── Mock Socket.IO Server ───

type mockServer struct {
	broadcasts map[string][]interface{}
	roomCasts  map[string]map[string][]interface{}
}

func newMockServer() *mockServer {
	return &mockServer{
		broadcasts: make(map[string][]interface{}),
		roomCasts:  make(map[string]map[string][]interface{}),
	}
}

func (m *mockServer) OnConnect(namespace string, f func(socketio.Conn) error) {}
func (m *mockServer) OnDisconnect(namespace string, f func(socketio.Conn, string)) {}
func (m *mockServer) OnError(namespace string, f func(socketio.Conn, error)) {}
func (m *mockServer) OnEvent(namespace string, event string, f func(socketio.Conn, ...interface{}) error) {}
func (m *mockServer) Emit(event string, v ...interface{}) error {
	m.broadcasts[event] = v
	return nil
}
func (m *mockServer) BroadcastToNamespace(namespace, event string, v ...interface{}) error {
	m.broadcasts[event] = v
	return nil
}
func (m *mockServer) BroadcastToRoom(namespace, room, event string, v ...interface{}) error {
	if m.roomCasts[room] == nil {
		m.roomCasts[room] = make(map[string][]interface{})
	}
	m.roomCasts[room][event] = v
	return nil
}
func (m *mockServer) ForEach(namespace, room string, f func(socketio.Conn)) {}
func (m *mockServer) Close() error {
	return nil
}

// ─── OnRoomCreate Tests ───

func TestHub_OnRoomCreate_Success(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	conn := newMockConn("socket-1")
	data := `{"room":"test-room"}`

	hub.OnRoomCreate(conn, data)

	if conn.emitted[EventRoomCreated] == nil {
		t.Fatal("expected room:created event to be emitted")
	}

	if _, exists := hub.rooms["test-room"]; !exists {
		t.Fatal("expected room to be created in hub")
	}

	if server.broadcasts[EventRoomUpdated] == nil {
		t.Fatal("expected room:updated broadcast")
	}
}

func TestHub_OnRoomCreate_Duplicate(t *testing.T) {
	hub := NewHub()
	conn1 := newMockConn("socket-1")
	conn2 := newMockConn("socket-2")

	hub.rooms["existing-room"] = &Room{Name: "existing-room", Members: make(map[string]*MemberInfo)}

	data := `{"room":"existing-room"}`
	hub.OnRoomCreate(conn1, data)

	emitted, ok := conn1.emitted[EventRoomCreated].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:created event")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	if emitData["error"] != "room already exists" {
		t.Errorf("expected error message, got %v", emitData["error"])
	}
}

func TestHub_OnRoomCreate_InvalidJSON(t *testing.T) {
	hub := NewHub()
	conn := newMockConn("socket-1")

	hub.OnRoomCreate(conn, "not json")

	emitted, ok := conn.emitted[EventRoomCreated].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:created event with error")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	if emitData["error"] != "room name is required" {
		t.Errorf("expected error message, got %v", emitData["error"])
	}
}

func TestHub_OnRoomCreate_MissingRoom(t *testing.T) {
	hub := NewHub()
	conn := newMockConn("socket-1")

	hub.OnRoomCreate(conn, `{}`)

	emitted, ok := conn.emitted[EventRoomCreated].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:created event with error")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	if emitData["error"] != "room name is required" {
		t.Errorf("expected error message, got %v", emitData["error"])
	}
}

// ─── OnRoomJoin Tests ───

func TestHub_OnRoomJoin_CreateAndJoin(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	conn := newMockConn("socket-1")
	data := `{"room":"test-room","identity":"user-1"}`

	hub.OnRoomJoin(conn, data)

	if !conn.joined["test-room"] {
		t.Fatal("expected socket to join room")
	}

	if conn.emitted[EventRoomJoined] == nil {
		t.Fatal("expected room:joined event")
	}

	emitted, ok := conn.emitted[EventRoomJoined].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected emit data")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	if members, ok := emitData["members"].([]interface{}); !ok || len(members) != 1 {
		t.Errorf("expected 1 member in room, got %v", members)
	}
}

func TestHub_OnRoomJoin_JoinExisting(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

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

	conn := newMockConn("socket-2")
	data := `{"room":"test-room","identity":"user-2"}`

	hub.OnRoomJoin(conn, data)

	emitted, ok := conn.emitted[EventRoomJoined].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:joined event")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	members, ok := emitData["members"].([]interface{})
	if !ok || len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestHub_OnRoomJoin_DefaultIdentity(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	conn := newMockConn("socket-1")
	data := `{"room":"test-room"}`

	hub.OnRoomJoin(conn, data)

	room := hub.rooms["test-room"]
	if room == nil {
		t.Fatal("expected room to be created")
	}

	member := room.Members["socket-1"]
	if member == nil {
		t.Fatal("expected member in room")
	}

	if member.Identity != "socket-1" {
		t.Errorf("expected identity to default to socket ID, got %s", member.Identity)
	}
}

// ─── OnRoomLeave Tests ───

func TestHub_OnRoomLeave_Success(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newMockConn("socket-1")
	data := `{"room":"test-room"}`

	hub.OnRoomLeave(conn, data)

	if !conn.left["test-room"] {
		t.Fatal("expected socket to leave room")
	}

	if _, exists := hub.rooms["test-room"].Members["socket-1"]; exists {
		t.Fatal("expected member to be removed from room")
	}

	emitted, ok := conn.emitted[EventRoomLeft].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:left event")
	}
}

func TestHub_OnRoomLeave_RemoveEmptyRoom(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newMockConn("socket-1")
	data := `{"room":"test-room"}`

	hub.OnRoomLeave(conn, data)

	if _, exists := hub.rooms["test-room"]; exists {
		t.Fatal("expected empty room to be removed")
	}
}

// ─── OnRoomList Tests ───

func TestHub_OnRoomList_Empty(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	conn := newMockConn("socket-1")

	hub.OnRoomList(conn)

	emitted, ok := conn.emitted[EventRoomListResult].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:list:result event")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	if count, ok := emitData["count"].(int); !ok || count != 0 {
		t.Errorf("expected count 0, got %v", emitData["count"])
	}
}

func TestHub_OnRoomList_Multiple(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

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

	conn := newMockConn("socket-1")
	hub.OnRoomList(conn)

	emitted, ok := conn.emitted[EventRoomListResult].([]interface{})
	if !ok || len(emitted) == 0 {
		t.Fatal("expected room:list:result event")
	}

	emitData, ok := emitted[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected emit data to be a map")
	}

	if count, ok := emitData["count"].(int); !ok || count != 2 {
		t.Errorf("expected count 2, got %v", emitData["count"])
	}
}

// ─── OnDisconnect Tests ───

func TestHub_OnDisconnect_RemoveFromRoom(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

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

	conn := newMockConn("socket-1")
	hub.OnDisconnect(conn, "client namespace disconnect")

	if _, exists := hub.rooms["test-room"].Members["socket-1"]; exists {
		t.Fatal("expected member to be removed on disconnect")
	}

	if _, exists := hub.rooms["test-room"].Members["socket-2"]; !exists {
		t.Fatal("expected other member to remain")
	}
}

func TestHub_OnDisconnect_RemoveEmptyRoom(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	hub.rooms["test-room"] = &Room{
		Name:    "test-room",
		Members: make(map[string]*MemberInfo),
	}
	hub.rooms["test-room"].Members["socket-1"] = &MemberInfo{
		ID:       "socket-1",
		Identity: "user-1",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newMockConn("socket-1")
	hub.OnDisconnect(conn, "client namespace disconnect")

	if _, exists := hub.rooms["test-room"]; exists {
		t.Fatal("expected empty room to be removed on disconnect")
	}
}

func TestHub_OnDisconnect_MultipleRooms(t *testing.T) {
	hub := NewHub()
	server := newMockServer()
	hub.server = server

	hub.rooms["room-1"] = &Room{
		Name:    "room-1",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
		},
	}
	hub.rooms["room-2"] = &Room{
		Name:    "room-2",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
			"socket-2": {ID: "socket-2", Identity: "user-2", JoinedAt: time.Now().UnixMilli()},
		},
	}

	conn := newMockConn("socket-1")
	hub.OnDisconnect(conn, "disconnect")

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

// ─── GetRooms Tests ───

func TestHub_GetRooms(t *testing.T) {
	hub := NewHub()

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

	rooms := hub.GetRooms()
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}

	if rooms[0].Count > 0 && rooms[0].Name == "room-1" && rooms[0].Count != 1 {
		t.Errorf("expected room-1 to have 1 member, got %d", rooms[0].Count)
	}
}

// ─── GetRoomMembers Tests ───

func TestHub_GetRoomMembers(t *testing.T) {
	hub := NewHub()

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
	hub := NewHub()

	members := hub.GetRoomMembers("nonexistent")
	if members != nil {
		t.Errorf("expected nil for nonexistent room, got %v", members)
	}
}
