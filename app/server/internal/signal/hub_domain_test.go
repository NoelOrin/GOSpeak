package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"encoding/json"
	"testing"
)

func TestHub_RoomKey_Format(t *testing.T) {
	tests := []struct {
		domainUUID string
		roomName   string
		want       string
	}{
		{"domain-uuid-1", "lobby", "domain-uuid-1:lobby"},
		{"", "lobby", "lobby"},
		{"domain-uuid-1", "", "domain-uuid-1:"},
		{"", "", ""},
	}
	for _, tt := range tests {
		got := roomKey(tt.domainUUID, tt.roomName)
		if got != tt.want {
			t.Errorf("roomKey(%q, %q) = %q, want %q", tt.domainUUID, tt.roomName, got, tt.want)
		}
	}
}

func TestHub_OnRoomJoin_DomainSlotUsesCompositeKey(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{
		{Name: "text-a", Type: model.RoomTypeText, DomainUUID: "domain-a"},
		{Name: "text-c", Type: model.RoomTypeText, DomainUUID: "domain-a"},
	}}
	h := newTestHub()
	h.roomStore = store
	conn := newAuthedMockClient("conn-1", "alice")

	if _, err := h.OnRoomJoin(conn, `{"room":"text-a","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join text A: %v", err)
	}
	if _, err := h.OnRoomJoin(conn, `{"room":"text-c","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join text C: %v", err)
	}

	h.mu.RLock()
	slots := h.connSlots["conn-1"]
	h.mu.RUnlock()
	if slots == nil || slots.TextRoom != "domain-a:text-c" {
		t.Fatalf("expected text slot 'domain-a:text-c', got %#v", slots)
	}
	fanout := h.fanout.(*mockBroadcaster)
	if !fanout.didLeave("conn-1", "domain-a:text-a") {
		t.Error("expected domain-a:text-a to be left with composite key")
	}
}

func TestHub_OnRoomLeave_ClearsConnSlot(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{
		{Name: "text-chat", Type: model.RoomTypeText, DomainUUID: "domain-a"},
	}}
	h := newTestHub()
	h.roomStore = store
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)
	conn := newAuthedMockClient("conn-1", "alice")

	if _, err := h.OnRoomJoin(conn, `{"room":"text-chat","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := h.OnRoomLeave(conn, `{"room":"text-chat","domain_uuid":"domain-a"}`); err != nil {
		t.Fatalf("leave: %v", err)
	}

	h.mu.RLock()
	_, exists := h.connSlots["conn-1"]
	h.mu.RUnlock()
	if exists {
		t.Fatal("expected conn slot removed after leave")
	}
	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","domain_uuid":"domain-a","content":"hello"}`)
	if err != nil {
		t.Fatalf("send after leave: %v", err)
	}
	var ackMap map[string]interface{}
	if err := json.Unmarshal([]byte(ack), &ackMap); err != nil {
		t.Fatalf("bad ack json: %v", err)
	}
	if ackMap["error"] != "not in room: text-chat" {
		t.Errorf("expected not in room error, got %v", ackMap["error"])
	}
}

func TestHub_DomainRoomIsolation(t *testing.T) {
	hub := newTestHub()

	domainA := newAuthedMockClient("sock-a", "user-a")
	domainB := newAuthedMockClient("sock-b", "user-b")

	hub.OnRoomCreate(domainA, `{"room":"lobby","domain_uuid":"domain-a-uuid"}`)
	hub.OnRoomCreate(domainB, `{"room":"lobby","domain_uuid":"domain-b-uuid"}`)

	hub.mu.RLock()
	_, existsA := hub.rooms["domain-a-uuid:lobby"]
	_, existsB := hub.rooms["domain-b-uuid:lobby"]
	hub.mu.RUnlock()

	if !existsA {
		t.Fatal("expected room 'domain-a-uuid:lobby' to exist")
	}
	if !existsB {
		t.Fatal("expected room 'domain-b-uuid:lobby' to exist")
	}
	if len(hub.rooms) != 2 {
		t.Fatalf("expected 2 rooms in hub, got %d", len(hub.rooms))
	}
}

func TestHub_DomainRoomCreate(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"game-room","domain_uuid":"domain-x","password":"secret"}`)

	expectedKey := "domain-x:game-room"
	hub.mu.RLock()
	room, exists := hub.rooms[expectedKey]
	hub.mu.RUnlock()

	if !exists {
		t.Fatalf("expected room with key %q to exist", expectedKey)
	}
	if room.Name != "game-room" {
		t.Fatalf("expected room name 'game-room', got %q", room.Name)
	}
	if room.Password == "secret" || !pkg.VerifyPassword(room.Password, "secret") {
		t.Fatalf("expected hashed password that verifies 'secret', got %q", room.Password)
	}
}

func TestHub_PlatformRoomCompat(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"general"}`)

	hub.mu.RLock()
	_, platformExists := hub.rooms["general"]
	hub.mu.RUnlock()
	if !platformExists {
		t.Fatal("expected platform room 'general' to exist")
	}

	hub.OnRoomCreate(conn, `{"room":"general","domain_uuid":"domain-y"}`)
	hub.mu.RLock()
	_, domainRoomExists := hub.rooms["domain-y:general"]
	hub.mu.RUnlock()

	if !domainRoomExists {
		t.Fatal("expected domain-scoped room 'domain-y:general' to exist separately")
	}
}

func TestHub_DomainDisconnect_CrossDomainIsolation(t *testing.T) {
	hub := newTestHub()

	connA := newAuthedMockClient("sock-a", "user-a")
	connB := newAuthedMockClient("sock-b", "user-b")

	hub.OnRoomCreate(connA, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomJoinSFU(connA, `{"room":"lobby","domain_uuid":"domain-a","identity":"user-a"}`)

	hub.OnRoomCreate(connB, `{"room":"lobby","domain_uuid":"domain-b"}`)
	hub.OnRoomJoinSFU(connB, `{"room":"lobby","domain_uuid":"domain-b","identity":"user-b"}`)

	hub.OnDisconnect(connA)

	hub.mu.RLock()
	roomA := hub.rooms["domain-a:lobby"]
	roomB := hub.rooms["domain-b:lobby"]
	hub.mu.RUnlock()

	if roomA != nil && len(roomA.Members) > 0 {
		t.Fatal("expected domain-a:lobby to be empty after user-a disconnect")
	}

	if roomB == nil {
		t.Fatal("expected domain-b:lobby to still exist")
	}
	if len(roomB.Members) == 0 {
		t.Fatal("expected domain-b:lobby to have members")
	}
}

func TestHub_DomainRoomBroadcast(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	conn := newAuthedMockClient("sock-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)

	hub.BroadcastToRoom("domain-a:lobby", "event:test", map[string]string{"msg": "hello"})

	calls := fanout.roomCasts["domain-a:lobby"]["event:test"]
	if len(calls) == 0 {
		t.Fatal("expected BroadcastToRoom for domain-a:lobby event:test")
	}
}

func TestHub_DomainMemberVisibility(t *testing.T) {
	hub := newTestHub()

	conn := newAuthedMockClient("sock-1", "user-1")

	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-b"}`)
	hub.OnRoomCreate(conn, `{"room":"general","domain_uuid":"domain-a"}`)

	rooms := hub.getMergedRooms("")
	if len(rooms) != 3 {
		t.Fatalf("expected 3 merged rooms, got %d", len(rooms))
	}
}

func TestHub_GetMergedRooms_KeepsDomainScopedDBRooms(t *testing.T) {
	store := newMockRoomStore("lobby", "lobby")
	store.rooms[0].UUID = "room-a"
	store.rooms[0].DomainUUID = "domain-a"
	store.rooms[1].UUID = "room-b"
	store.rooms[1].DomainUUID = "domain-b"

	hub := NewHub(store, nil, nil, nil)
	rooms := hub.getMergedRooms("")
	if len(rooms) != 2 {
		t.Fatalf("expected 2 merged rooms, got %d", len(rooms))
	}
	found := map[string]bool{}
	for _, r := range rooms {
		if r.DomainUUID != "domain-a" && r.DomainUUID != "domain-b" {
			t.Fatalf("unexpected domain_uuid %q on room %s", r.DomainUUID, r.Name)
		}
		if r.Name != "lobby" {
			t.Fatalf("expected name lobby, got %q", r.Name)
		}
		found[r.DomainUUID] = true
	}
	if !found["domain-a"] || !found["domain-b"] {
		t.Fatalf("expected both domains in merged rooms, got %#v", found)
	}
}

func TestHub_CheckRoomLimit_UsesDomainScopedRoom(t *testing.T) {
	store := newMockRoomStore("lobby", "lobby")
	store.rooms[0].DomainUUID = "domain-a"
	store.rooms[0].Limit = 1
	store.rooms[1].DomainUUID = "domain-b"
	store.rooms[1].Limit = 1

	hub := NewHub(store, nil, nil, nil)
	hub.rooms["domain-b:lobby"] = &Room{
		Name:     "lobby",
		Members:  map[string]*MemberInfo{"s1": {ID: "s1", Identity: "user-b"}},
		MicMuted: map[string]bool{},
		Speaking: map[string]bool{},
	}

	if full, _, _, err := hub.CheckRoomLimit("domain-a", "lobby"); err != nil || full {
		t.Fatalf("domain-a room should not be full, full=%v err=%v", full, err)
	}
	if full, _, _, err := hub.CheckRoomLimit("domain-b", "lobby"); err != nil || !full {
		t.Fatalf("domain-b room should be full, full=%v err=%v", full, err)
	}
}

func TestHub_OnRoomJoinSFU_SwitchesVoiceRoomAcrossDomains(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("sock-1", "alice")

	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-b"}`)

	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join domain-a: %v", err)
	}
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join sfu domain-a: %v", err)
	}

	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"domain-b","identity":"alice"}`); err != nil {
		t.Fatalf("join domain-b: %v", err)
	}
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"domain-b","identity":"alice"}`); err != nil {
		t.Fatalf("join sfu domain-b: %v", err)
	}

	hub.mu.RLock()
	_, aExists := hub.rooms["domain-a:lobby"]
	bRoom := hub.rooms["domain-b:lobby"]
	slots := hub.connSlots["sock-1"]
	hub.mu.RUnlock()

	if aExists {
		t.Fatal("domain-a room should be removed after switching to domain-b")
	}
	if bRoom == nil || len(bRoom.Members) != 1 {
		t.Fatalf("expected alice only in domain-b room, got %#v", bRoom)
	}
	if slots == nil || slots.VoiceRoom != "domain-b:lobby" {
		t.Fatalf("expected voice slot domain-b:lobby, got %#v", slots)
	}

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didLeave("sock-1", "domain-a:lobby") {
		t.Fatal("expected fanout leave for domain-a:lobby")
	}
	if !fanout.didJoin("sock-1", "domain-b:lobby") {
		t.Fatal("expected fanout join for domain-b:lobby")
	}
	if len(fanout.roomCasts["domain-a:lobby"][EventMemberLeft]) == 0 {
		t.Fatal("expected member:left for domain-a:lobby after switch")
	}
}

func TestHub_OnRoomJoin_DoesNotSwitchVoiceSlotBeforeSFUConfirm(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("sock-1", "alice")

	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-b"}`)
	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join domain-a: %v", err)
	}
	if _, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice"}`); err != nil {
		t.Fatalf("join sfu domain-a: %v", err)
	}

	if _, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"domain-b","identity":"alice"}`); err != nil {
		t.Fatalf("signaling join domain-b: %v", err)
	}

	hub.mu.RLock()
	slots := hub.connSlots["sock-1"]
	hub.mu.RUnlock()
	if slots == nil || slots.VoiceRoom != "domain-a:lobby" {
		t.Fatalf("voice slot must stay on domain-a until sfu confirm, got %#v", slots)
	}

	fanout := hub.fanout.(*mockBroadcaster)
	if fanout.didJoin("sock-1", "domain-b:lobby") {
		t.Fatal("domain-b fanout must not be joined before sfu confirm")
	}
	if fanout.didLeave("sock-1", "domain-a:lobby") {
		t.Fatal("domain-a fanout must not be left before sfu confirm")
	}
}
