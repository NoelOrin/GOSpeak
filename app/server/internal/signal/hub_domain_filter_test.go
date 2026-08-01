package signal

import (
	"testing"
	"time"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

func TestHub_OnRoomList_FiltersDomainUUID(t *testing.T) {
	store := newMockRoomStore("lobby", "lobby", "general")
	store.rooms[0].DomainUUID = "domain-a"
	store.rooms[1].DomainUUID = "domain-b"

	hub := NewHub(store, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn, `{"domain_uuid":"domain-a"}`)

	emitData, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}
	if count, ok := emitData["count"].(int); !ok || count != 1 {
		t.Fatalf("expected 1 filtered room, got %v", emitData["count"])
	}
	rooms, ok := emitData["rooms"].([]RoomInfo)
	if !ok || len(rooms) != 1 || rooms[0].DomainUUID != "domain-a" {
		t.Fatalf("expected only domain-a room, got %#v", emitData["rooms"])
	}
}

func TestHub_GetMergedRooms_FiltersMemoryAndKV(t *testing.T) {
	store := newMemStateStore()
	store.rooms["domain-b:remote"] = bus.RoomMembersSnapshot{
		Room:    "domain-b:remote",
		Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}},
	}

	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	hub.rooms["domain-a:lobby"] = &Room{
		Name:       "lobby",
		Members:    map[string]*MemberInfo{},
		ByIdentity: map[string]*MemberInfo{},
		MicMuted:   map[string]bool{},
		Speaking:   map[string]bool{},
		CreatedAt:  time.Now(),
	}
	hub.rooms["domain-b:lobby"] = &Room{
		Name:       "lobby",
		Members:    map[string]*MemberInfo{},
		ByIdentity: map[string]*MemberInfo{},
		MicMuted:   map[string]bool{},
		Speaking:   map[string]bool{},
		CreatedAt:  time.Now(),
	}
	hub.rooms["general"] = &Room{
		Name:       "general",
		Members:    map[string]*MemberInfo{},
		ByIdentity: map[string]*MemberInfo{},
		MicMuted:   map[string]bool{},
		Speaking:   map[string]bool{},
		CreatedAt:  time.Now(),
	}

	domainBRooms := hub.getMergedRooms("domain-b")
	if len(domainBRooms) != 2 {
		t.Fatalf("expected 2 domain-b rooms, got %d: %+v", len(domainBRooms), domainBRooms)
	}
	for _, r := range domainBRooms {
		if r.DomainUUID != "domain-b" {
			t.Fatalf("expected domain-b only, got %+v", r)
		}
	}

	allRooms := hub.getMergedRooms("")
	if len(allRooms) != 4 {
		t.Fatalf("expected 4 total rooms, got %d: %+v", len(allRooms), allRooms)
	}
}

func TestHub_DomainEventPayloadsIncludeDomainUUID(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("conn-1", "alice")

	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)
	hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice"}`)
	fanout := hub.fanout.(*mockBroadcaster)

	assertDomainPayload(t, fanout.roomCasts["domain-a:lobby"][EventMemberJoined][0], "domain-a")

	hub.OnMemberMicState(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","isMicMuted":true}`)
	assertDomainPayload(t, fanout.roomCasts["domain-a:lobby"][EventMemberUpdated][0], "domain-a")

	hub.OnMemberSpeaking(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"alice","speaking":true}`)
	assertDomainPayload(t, fanout.roomCasts["domain-a:lobby"][EventRoomActiveSpeakers][0], "domain-a")

	hub.OnRoomLeave(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)
	left, ok := conn.lastEvent(EventRoomLeft).(map[string]interface{})
	if !ok || left["domain_uuid"] != "domain-a" {
		t.Fatalf("expected room:left to carry domain_uuid, got %#v", left)
	}
	assertDomainPayload(t, fanout.roomCasts["domain-a:lobby"][EventMemberLeft][0], "domain-a")

	users := &mockUserStore{users: map[string]*model.User{
		"bot_helper": {Name: "bot_helper", Role: "user"},
		"alice":      {Name: "alice", Role: "user"},
	}}
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{
		"user": {},
	}}
	kickHub := NewHub(nil, nil, users, perms)
	kickHub.fanout = newMockBroadcaster()
	seedKickRoom(kickHub, "domain-a:lobby", map[string]string{
		"bot-socket": "bot_helper",
		"alice-sock": "alice",
	})
	bot := &mockClient{id: "bot-socket", claims: &pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"signal:kick"},
	}}
	kickHub.OnRoomKick(bot, `{"room":"lobby","domain_uuid":"domain-a","targetIdentity":"alice"}`)
	kickFanout := kickHub.fanout.(*mockBroadcaster)
	assertDomainPayload(t, kickFanout.roomCasts["domain-a:lobby"][EventRoomKicked][0], "domain-a")
	assertDomainPayload(t, kickFanout.roomCasts["domain-a:lobby"][EventMemberLeft][0], "domain-a")
}

func assertDomainPayload(t *testing.T, payload interface{}, want string) {
	t.Helper()
	m, ok := payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T: %#v", payload, payload)
	}
	if got, _ := m["domain_uuid"].(string); got != want {
		t.Fatalf("expected domain_uuid %q, got %q in %#v", want, got, m)
	}
}

func TestHub_OnRoomList_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn, `{"domain_uuid":"domain-a"}`)

	data, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}
	if count, ok := data["count"].(int); !ok || count != 0 {
		t.Fatalf("expected empty count for non-member, got %#v", data["count"])
	}
	if rooms, ok := data["rooms"].([]RoomInfo); !ok || len(rooms) != 0 {
		t.Fatalf("expected empty rooms for non-member, got %#v", data["rooms"])
	}
}

func TestHub_OnRoomList_EmptyDomainUUIDReturnsPlatformOnly(t *testing.T) {
	store := newMockRoomStore("domain-lobby", "general")
	store.rooms[0].DomainUUID = "domain-a"

	hub := NewHub(store, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn, "")

	data, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}
	rooms, ok := data["rooms"].([]RoomInfo)
	if !ok || len(rooms) != 1 || rooms[0].DomainUUID != "" || rooms[0].Name != "general" {
		t.Fatalf("expected only platform room, got %#v", data["rooms"])
	}

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didJoin(conn.ID(), "__platform") {
		t.Fatal("expected platform client to join __platform scope")
	}
}

func TestHub_OnRoomCreate_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"lobby","domain_uuid":"domain-a"}`)

	data, ok := conn.lastEvent(EventRoomCreated).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:created error event")
	}
	if data["error"] == nil {
		t.Fatalf("expected non-member create error, got %#v", data)
	}
	hub.mu.RLock()
	_, exists := hub.rooms["domain-a:lobby"]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("non-member must not create domain room")
	}
}

func TestHub_OnRoomJoin_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	ack, err := hub.OnRoomJoin(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"user-1"}`)
	if err != nil {
		t.Fatalf("OnRoomJoin returned error: %v", err)
	}
	data := decodeAck(t, ack)
	if data["error"] == nil {
		t.Fatalf("expected non-member join error, got %#v", data)
	}
}

func TestHub_OnRoomJoinSFU_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetDomainChecker(func(domainUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	ack, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","domain_uuid":"domain-a","identity":"user-1"}`)
	if err != nil {
		t.Fatalf("OnRoomJoinSFU returned error: %v", err)
	}
	data := decodeAck(t, ack)
	if data["error"] == nil {
		t.Fatalf("expected non-member join SFU error, got %#v", data)
	}
}

func TestHub_BroadcastRoomUpdatedLocal_UsesDomainRoomScope(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	hub.broadcastRoomUpdatedLocal("domain-a:lobby")

	if len(fanout.roomCasts["__domain:domain-a"][EventRoomUpdated]) == 0 {
		t.Fatal("expected room:updated in domain scope")
	}
	if len(fanout.broadcasts[EventRoomUpdated]) != 0 {
		t.Fatal("room:updated must not use global namespace broadcast")
	}
}

func TestHub_BroadcastRoomList_UsesDomainRoomScope(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	hub.broadcastRoomList("domain-a")

	if len(fanout.roomCasts["__domain:domain-a"][EventRoomListResult]) == 0 {
		t.Fatal("expected room:list:result in domain scope")
	}
	if len(fanout.broadcasts[EventRoomListResult]) != 0 {
		t.Fatal("room:list:result must not use global namespace broadcast")
	}
}

func TestHub_SetClientDomain_SwitchesScope(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomList(conn, `{"domain_uuid":"domain-a"}`)
	hub.OnRoomList(conn, `{"domain_uuid":"domain-b"}`)

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didLeave(conn.ID(), "__domain:domain-a") {
		t.Fatal("expected client to leave previous domain scope")
	}
	if !fanout.didJoin(conn.ID(), "__domain:domain-b") {
		t.Fatal("expected client to join new domain scope")
	}
}

func TestHub_OnDisconnect_LeavesDomainScope(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomList(conn, `{"domain_uuid":"domain-a"}`)
	hub.OnDisconnect(conn)

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didLeave(conn.ID(), "__domain:domain-a") {
		t.Fatal("expected disconnect to leave domain scope")
	}
}
