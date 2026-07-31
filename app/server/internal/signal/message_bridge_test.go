package signal

import (
	"encoding/json"
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
)

// ─── mock message service ───

type mockMessageSvc struct {
	sendCalled    int
	editCalled    int
	deleteCalled  int
	reactCalled   int
	unreactCalled int
}

func (m *mockMessageSvc) Send(roomUUID, authorID, content, replyTo, clientNonce string, mentions []string) (*service.MessageDTO, error) {
	m.sendCalled++
	return &service.MessageDTO{UUID: "mock-uuid", RoomUUID: roomUUID, AuthorID: authorID, Content: content}, nil
}

func (m *mockMessageSvc) Edit(roomUUID, messageUUID, authorID, content string) (*service.MessageDTO, error) {
	m.editCalled++
	return &service.MessageDTO{UUID: messageUUID, Content: content}, nil
}

func (m *mockMessageSvc) Delete(roomUUID, messageUUID, actorID string, canDeleteOthers bool) error {
	m.deleteCalled++
	return nil
}

func (m *mockMessageSvc) React(roomUUID, messageUUID, userID, emoji string) error {
	m.reactCalled++
	return nil
}

func (m *mockMessageSvc) Unreact(roomUUID, messageUUID, userID, emoji string) error {
	m.unreactCalled++
	return nil
}

// ─── helpers ───

func newMockRoomStoreWithType(namesAndTypes ...string) *mockRoomStore {
	rooms := make([]model.Room, 0, len(namesAndTypes)/2)
	for i := 0; i < len(namesAndTypes); i += 2 {
		name := namesAndTypes[i]
		typ := namesAndTypes[i+1]
		rooms = append(rooms, model.Room{
			UUID:      "uuid-" + name,
			Name:      name,
			Type:      typ,
			CreatedAt: time.Now(),
		})
	}
	return &mockRoomStore{rooms: rooms}
}

// ─── Tests ───

func TestOnRoomJoinSFU_TextRoomRejected(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := NewHub(store, nil, nil, nil)
	conn := newAuthedMockClient("conn-1", "alice")

	data := `{"room":"text-chat","identity":"alice"}`
	ack, err := h.OnRoomJoinSFU(conn, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ackMap map[string]interface{}
	if err := json.Unmarshal([]byte(ack), &ackMap); err != nil {
		t.Fatalf("bad ack json: %v", err)
	}
	if ackMap["error"] != "text room has no media" {
		t.Errorf("expected 'text room has no media', got %v", ackMap["error"])
	}
}

func TestOnRoomJoin_DualSlot(t *testing.T) {
	store := newMockRoomStoreWithType(
		"text-a", model.RoomTypeText,
		"text-c", model.RoomTypeText,
		"voice-b", model.RoomTypeVoice,
	)
	h := newTestHub()
	h.roomStore = store
	// nil muteStore bypasses check
	// nil permChecker bypasses permission check (OnRoomJoin calls CheckRoomPermission)

	conn := newAuthedMockClient("conn-1", "alice")

	// Join text room A
	_, err := h.OnRoomJoin(conn, `{"room":"text-a","identity":"alice"}`)
	if err != nil {
		t.Fatalf("join text A: %v", err)
	}

	// Join voice room B
	_, err = h.OnRoomJoin(conn, `{"room":"voice-b","identity":"alice"}`)
	if err != nil {
		t.Fatalf("join voice B: %v", err)
	}

	// Both slots set
	h.mu.RLock()
	slots := h.connSlots["conn-1"]
	h.mu.RUnlock()
	if slots == nil {
		t.Fatal("expected connSlots entry")
	}
	if slots.TextRoom != "text-a" {
		t.Errorf("expected text slot 'text-a', got %s", slots.TextRoom)
	}
	if slots.VoiceRoom != "voice-b" {
		t.Errorf("expected voice slot 'voice-b', got %s", slots.VoiceRoom)
	}

	// Join text room C replaces text A only
	_, err = h.OnRoomJoin(conn, `{"room":"text-c","identity":"alice"}`)
	if err != nil {
		t.Fatalf("join text C: %v", err)
	}

	h.mu.RLock()
	slots = h.connSlots["conn-1"]
	h.mu.RUnlock()
	if slots.TextRoom != "text-c" {
		t.Errorf("expected text slot replaced to 'text-c', got %s", slots.TextRoom)
	}
	if slots.VoiceRoom != "voice-b" {
		t.Errorf("expected voice slot still 'voice-b', got %s", slots.VoiceRoom)
	}

	// Old text room should be left
	fanout := h.fanout.(*mockBroadcaster)
	if !fanout.didLeave("conn-1", "text-a") {
		t.Error("expected text-a to be left")
	}
}

func TestOnMessageSend_NotInRoom(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := newTestHub()
	h.roomStore = store
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	// Conn authed but never joined the room
	conn := newAuthedMockClient("conn-1", "alice")

	// Try sending without joining
	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","content":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ackMap map[string]interface{}
	json.Unmarshal([]byte(ack), &ackMap)
	if ackMap["error"] != "not in room: text-chat" {
		t.Errorf("expected 'not in room', got %v", ackMap["error"])
	}
	if msgSvc.sendCalled > 0 {
		t.Error("expected no Send call when not in room")
	}
}

func TestOnMessageSend_Success(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := newTestHub()
	h.roomStore = store
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	conn := newAuthedMockClient("conn-1", "alice")

	// First join the room
	_, err := h.OnRoomJoin(conn, `{"room":"text-chat","identity":"alice"}`)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	// Send message
	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","content":"hello world"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ackMap map[string]interface{}
	json.Unmarshal([]byte(ack), &ackMap)
	if ackMap["success"] != true {
		t.Errorf("expected success, got %v", ackMap)
	}
	if msgSvc.sendCalled != 1 {
		t.Errorf("expected 1 send call, got %d", msgSvc.sendCalled)
	}
}

func TestOnMessageSend_Unauthed(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := NewHub(store, nil, nil, nil)
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	// Unauthed conn (no claims in context)
	conn := newMockClient("conn-1")

	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","content":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ackMap map[string]interface{}
	json.Unmarshal([]byte(ack), &ackMap)
	if ackMap["error"] != "unauthorized" {
		t.Errorf("expected 'unauthorized', got %v", ackMap["error"])
	}
}

func TestOnMessageDelete_WithModPerm(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := newTestHub()
	h.roomStore = store
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	// Admin role conn
	conn := newMockClient("conn-1")
	conn.claims = &pkg.Claims{Username: "admin", UserUUID: "admin", Role: "admin"}

	// Join first
	_, err := h.OnRoomJoin(conn, `{"room":"text-chat","identity":"admin"}`)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	ack, err := h.OnMessageDelete(conn, `{"room":"text-chat","message_uuid":"msg-1"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ackMap map[string]interface{}
	json.Unmarshal([]byte(ack), &ackMap)
	if ackMap["success"] != true {
		t.Errorf("expected success, got %v", ackMap)
	}
	if msgSvc.deleteCalled != 1 {
		t.Errorf("expected 1 delete call, got %d", msgSvc.deleteCalled)
	}
}

func TestOnDisconnect_CleansConnSlots(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := newTestHub()
	h.roomStore = store

	conn := newAuthedMockClient("conn-1", "alice")

	// Join a room
	_, err := h.OnRoomJoin(conn, `{"room":"text-chat","identity":"alice"}`)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	// Verify slot exists
	h.mu.RLock()
	_, exists := h.connSlots["conn-1"]
	h.mu.RUnlock()
	if !exists {
		t.Fatal("expected connSlots entry before disconnect")
	}

	// Handle disconnect (nil server cast via safeOnDisconnect wrapper)
	// Call the handler directly
	h.OnDisconnect(conn)

	// Verify slot cleaned
	h.mu.RLock()
	_, exists = h.connSlots["conn-1"]
	h.mu.RUnlock()
	if exists {
		t.Error("expected connSlots entry cleaned after disconnect")
	}
}

func TestResolveMessageRoom_IdentityRequired(t *testing.T) {
	store := newMockRoomStoreWithType("text-chat", model.RoomTypeText)
	h := newTestHub()
	h.roomStore = store

	conn := newMockClient("conn-1") // no claims

	_, _, ackErr := h.resolveMessageRoom(conn, "text-chat")
	if ackErr == "" {
		t.Fatal("expected error")
	}
	if ackErr != `{"error":"unauthorized"}` {
		t.Errorf("expected unauthorized, got %s", ackErr)
	}
}

func TestMarshalAck(t *testing.T) {
	result, err := marshalAck(map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected 'value', got %v", m["key"])
	}
}

func TestRoomJoinSFU_WithServer(t *testing.T) {
	store := newMockRoomStoreWithType(
		"text-chat", model.RoomTypeText,
		"voice-1", model.RoomTypeVoice,
	)
	h := newTestHub()
	h.roomStore = store

	conn := newAuthedMockClient("conn-1", "alice")

	// Voice room SFU should succeed (no error)
	_, err := h.OnRoomJoin(conn, `{"room":"voice-1","identity":"alice"}`)
	if err != nil {
		t.Fatalf("OnRoomJoin voice: %v", err)
	}

	// Text room SFU should fail
	ack, err := h.OnRoomJoinSFU(conn, `{"room":"text-chat","identity":"alice"}`)
	if err != nil {
		t.Fatalf("OnRoomJoinSFU: %v", err)
	}
	var ackMap map[string]interface{}
	json.Unmarshal([]byte(ack), &ackMap)
	if ackMap["error"] != "text room has no media" {
		t.Errorf("expected error, got %v", ackMap)
	}
}
