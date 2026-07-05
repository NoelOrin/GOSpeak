package signal

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"GOSpeak/internal/model"

	socketio "github.com/googollee/go-socket.io"
)

// ─── Mock roomStore ───

type mockRoomStore struct {
	rooms []model.Room
}

func (m *mockRoomStore) List(page, pageSize int) ([]model.Room, int64, error) {
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

// ─── Mock socketio.Conn ───
// Implements the socketio.Conn interface (io.Closer + Namespace).

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

// io.Closer
func (m *mockConn) Close() error { return nil }

// socketio.Conn
func (m *mockConn) ID() string                { return m.id }
func (m *mockConn) URL() url.URL              { return url.URL{} }
func (m *mockConn) LocalAddr() net.Addr       { return nil }
func (m *mockConn) RemoteAddr() net.Addr      { return nil }
func (m *mockConn) RemoteHeader() http.Header { return nil }

// Namespace interface
func (m *mockConn) Context() interface{}     { return nil }
func (m *mockConn) SetContext(v interface{}) {}
func (m *mockConn) Namespace() string        { return "/" }
func (m *mockConn) Emit(event string, v ...interface{}) {
	m.emitted[event] = v
}
func (m *mockConn) Join(room string)  { m.joined[room] = true }
func (m *mockConn) Leave(room string) { m.left[room] = true }
func (m *mockConn) LeaveAll()         {}
func (m *mockConn) Rooms() []string   { return nil }

// ─── Mock socketServer ───

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

func (m *mockServer) BroadcastToNamespace(namespace, event string, v ...interface{}) bool {
	m.broadcasts[event] = v
	return true
}
func (m *mockServer) BroadcastToRoom(namespace, room, event string, v ...interface{}) bool {
	if m.roomCasts[room] == nil {
		m.roomCasts[room] = make(map[string][]interface{})
	}
	m.roomCasts[room][event] = v
	return true
}
func (m *mockServer) ForEach(namespace, room string, f socketio.EachFunc) bool { return true }

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
	hub := NewHub(nil, nil, nil, nil)
	conn1 := newMockConn("socket-1")

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
	hub := NewHub(nil, nil, nil, nil)
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
	hub := NewHub(nil, nil, nil, nil)
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
	hub := NewHub(nil, nil, nil, nil)
	server := newMockServer()
	hub.server = server

	conn := newMockConn("socket-1")
	data := `{"room":"test-room","identity":"user-1"}`

	ack, err := hub.OnRoomJoin(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !conn.joined["test-room"] {
		t.Fatal("expected socket to join room")
	}
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

	ack, err := hub.OnRoomJoin(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !conn.joined["test-room"] {
		t.Fatal("expected socket to join room")
	}

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

func TestHub_OnRoomJoin_DefaultIdentity(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockServer()
	hub.server = server

	conn := newMockConn("socket-1")
	data := `{"room":"test-room"}`

	if _, err := hub.OnRoomJoin(conn, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OnRoomJoin creates socketio room but not h.rooms — need OnRoomJoinSFU
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

	if member.Identity != "socket-1" {
		t.Errorf("expected identity to default to socket ID, got %s", member.Identity)
	}
}

func TestHub_OnRoomJoin_RoomFull(t *testing.T) {
	store := newMockRoomStore("limited-room")
	store.rooms[0].Limit = 1
	hub := NewHub(store, nil, nil, nil)
	server := newMockServer()
	hub.server = server

	// 先让一个用户加入
	hub.rooms["limited-room"] = &Room{
		Name: "limited-room",
		Members: map[string]*MemberInfo{
			"socket-1": {ID: "socket-1", Identity: "user-1", JoinedAt: time.Now().UnixMilli()},
		},
		CreatedAt: time.Now(),
	}

	// 第二个用户尝试加入，应该被拒绝
	conn := newMockConn("socket-2")
	ack, err := hub.OnRoomJoin(conn, `{"room":"limited-room","identity":"user-2"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emitData := decodeAck(t, ack)

	if emitData["error"] != "room is full" {
		t.Errorf("expected 'room is full' error, got %v", emitData["error"])
	}

	// socket-2 不应该被加入房间
	if conn.joined["limited-room"] {
		t.Fatal("expected socket NOT to join room when full")
	}
}

// ─── OnRoomLeave Tests ───

func TestHub_OnRoomLeave_Success(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
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
	// Add a second member so the room isn't deleted after socket-1 leaves
	hub.rooms["test-room"].Members["socket-2"] = &MemberInfo{
		ID:       "socket-2",
		Identity: "user-2",
		JoinedAt: time.Now().UnixMilli(),
	}

	conn := newMockConn("socket-1")
	data := `{"room":"test-room"}`

	resp, err := hub.OnRoomLeave(conn, data)
	if err != nil {
		t.Fatalf("OnRoomLeave failed: %v", err)
	}
	_ = resp

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
	hub := NewHub(nil, nil, nil, nil)
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
	hub := NewHub(nil, nil, nil, nil)
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
	hub := NewHub(nil, nil, nil, nil)
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
	hub := NewHub(nil, nil, nil, nil)
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
	hub := NewHub(nil, nil, nil, nil)
	server := newMockServer()
	hub.server = server

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
	server := newMockServer()
	hub.server = server

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
	server := newMockServer()
	hub.server = server

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

	if count, ok := emitData["count"].(int); !ok || count != 2 {
		t.Errorf("expected count 2, got %v", emitData["count"])
	}
}

func TestOnRoomJoinSFU_StoresAndBroadcastsStream(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	server := newMockServer()
	hub.server = server

	creator := newMockConn("creator-socket")
	hub.OnRoomCreate(creator, `{"room":"r1"}`)

	member := newMockConn("mem-1")
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
	if vals, ok := server.broadcasts[EventMemberJoined]; ok && len(vals) > 0 {
		payload, _ := json.Marshal(vals[0])
		if strings.Contains(string(payload), "gs-aaa") {
			broadcasted = true
		}
	}
	if !broadcasted {
		t.Fatal("member:joined should broadcast stream")
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
