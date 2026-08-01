package signal

import (
	"testing"
	"time"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

func TestHub_OnRoomList_FiltersGuildUUID(t *testing.T) {
	store := newMockRoomStore("lobby", "lobby", "general")
	store.rooms[0].GuildUUID = "guild-a"
	store.rooms[1].GuildUUID = "guild-b"

	hub := NewHub(store, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	hub.SetGuildChecker(func(guildUUID, userUUID string) bool { return true })

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn, `{"guild_uuid":"guild-a"}`)

	emitData, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}
	if count, ok := emitData["count"].(int); !ok || count != 1 {
		t.Fatalf("expected 1 filtered room, got %v", emitData["count"])
	}
	rooms, ok := emitData["rooms"].([]RoomInfo)
	if !ok || len(rooms) != 1 || rooms[0].GuildUUID != "guild-a" {
		t.Fatalf("expected only guild-a room, got %#v", emitData["rooms"])
	}
}

func TestHub_GetMergedRooms_FiltersMemoryAndKV(t *testing.T) {
	store := newMemStateStore()
	store.rooms["guild-b:remote"] = bus.RoomMembersSnapshot{
		Room:    "guild-b:remote",
		Members: []bus.MemberRecord{{Identity: "bob", InstanceID: "other"}},
	}

	hub := NewHub(nil, nil, nil, nil)
	hub.SetMembershipStore(store, "inst-a")
	hub.rooms["guild-a:lobby"] = &Room{
		Name:       "lobby",
		Members:    map[string]*MemberInfo{},
		ByIdentity: map[string]*MemberInfo{},
		MicMuted:   map[string]bool{},
		Speaking:   map[string]bool{},
		CreatedAt:  time.Now(),
	}
	hub.rooms["guild-b:lobby"] = &Room{
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

	guildBRooms := hub.getMergedRooms("guild-b")
	if len(guildBRooms) != 2 {
		t.Fatalf("expected 2 guild-b rooms, got %d: %+v", len(guildBRooms), guildBRooms)
	}
	for _, r := range guildBRooms {
		if r.GuildUUID != "guild-b" {
			t.Fatalf("expected guild-b only, got %+v", r)
		}
	}

	allRooms := hub.getMergedRooms("")
	if len(allRooms) != 4 {
		t.Fatalf("expected 4 total rooms, got %d: %+v", len(allRooms), allRooms)
	}
}

func TestHub_GuildEventPayloadsIncludeGuildUUID(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("conn-1", "alice")

	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-a"}`)
	hub.OnRoomJoinSFU(conn, `{"room":"lobby","guild_uuid":"guild-a","identity":"alice"}`)
	fanout := hub.fanout.(*mockBroadcaster)

	assertGuildPayload(t, fanout.roomCasts["guild-a:lobby"][EventMemberJoined][0], "guild-a")

	hub.OnMemberMicState(conn, `{"room":"lobby","guild_uuid":"guild-a","identity":"alice","isMicMuted":true}`)
	assertGuildPayload(t, fanout.roomCasts["guild-a:lobby"][EventMemberUpdated][0], "guild-a")

	hub.OnMemberSpeaking(conn, `{"room":"lobby","guild_uuid":"guild-a","identity":"alice","speaking":true}`)
	assertGuildPayload(t, fanout.roomCasts["guild-a:lobby"][EventRoomActiveSpeakers][0], "guild-a")

	hub.OnRoomLeave(conn, `{"room":"lobby","guild_uuid":"guild-a"}`)
	left, ok := conn.lastEvent(EventRoomLeft).(map[string]interface{})
	if !ok || left["guild_uuid"] != "guild-a" {
		t.Fatalf("expected room:left to carry guild_uuid, got %#v", left)
	}
	assertGuildPayload(t, fanout.roomCasts["guild-a:lobby"][EventMemberLeft][0], "guild-a")

	users := &mockUserStore{users: map[string]*model.User{
		"bot_helper": {Name: "bot_helper", Role: "user"},
		"alice":      {Name: "alice", Role: "user"},
	}}
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{
		"user": {},
	}}
	kickHub := NewHub(nil, nil, users, perms)
	kickHub.fanout = newMockBroadcaster()
	seedKickRoom(kickHub, "guild-a:lobby", map[string]string{
		"bot-socket": "bot_helper",
		"alice-sock": "alice",
	})
	bot := &mockClient{id: "bot-socket", claims: &pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"signal:kick"},
	}}
	kickHub.OnRoomKick(bot, `{"room":"lobby","guild_uuid":"guild-a","targetIdentity":"alice"}`)
	kickFanout := kickHub.fanout.(*mockBroadcaster)
	assertGuildPayload(t, kickFanout.roomCasts["guild-a:lobby"][EventRoomKicked][0], "guild-a")
	assertGuildPayload(t, kickFanout.roomCasts["guild-a:lobby"][EventMemberLeft][0], "guild-a")
}

func assertGuildPayload(t *testing.T, payload interface{}, want string) {
	t.Helper()
	m, ok := payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T: %#v", payload, payload)
	}
	if got, _ := m["guild_uuid"].(string); got != want {
		t.Fatalf("expected guild_uuid %q, got %q in %#v", want, got, m)
	}
}

func TestHub_OnRoomList_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetGuildChecker(func(guildUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn, `{"guild_uuid":"guild-a"}`)

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

func TestHub_OnRoomList_EmptyGuildUUIDReturnsPlatformOnly(t *testing.T) {
	store := newMockRoomStore("guild-lobby", "general")
	store.rooms[0].GuildUUID = "guild-a"

	hub := NewHub(store, nil, nil, nil)
	hub.fanout = newMockBroadcaster()
	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomList(conn, "")

	data, ok := conn.lastEvent(EventRoomListResult).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:list:result event")
	}
	rooms, ok := data["rooms"].([]RoomInfo)
	if !ok || len(rooms) != 1 || rooms[0].GuildUUID != "" || rooms[0].Name != "general" {
		t.Fatalf("expected only platform room, got %#v", data["rooms"])
	}

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didJoin(conn.ID(), "__platform") {
		t.Fatal("expected platform client to join __platform scope")
	}
}

func TestHub_OnRoomCreate_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetGuildChecker(func(guildUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	hub.OnRoomCreate(conn, `{"room":"lobby","guild_uuid":"guild-a"}`)

	data, ok := conn.lastEvent(EventRoomCreated).(map[string]interface{})
	if !ok {
		t.Fatal("expected room:created error event")
	}
	if data["error"] == nil {
		t.Fatalf("expected non-member create error, got %#v", data)
	}
	hub.mu.RLock()
	_, exists := hub.rooms["guild-a:lobby"]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("non-member must not create guild room")
	}
}

func TestHub_OnRoomJoin_RejectsNonMember(t *testing.T) {
	hub := newTestHub()
	hub.SetGuildChecker(func(guildUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	ack, err := hub.OnRoomJoin(conn, `{"room":"lobby","guild_uuid":"guild-a","identity":"user-1"}`)
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
	hub.SetGuildChecker(func(guildUUID, userUUID string) bool { return false })

	conn := newAuthedMockClient("socket-1", "user-1")
	ack, err := hub.OnRoomJoinSFU(conn, `{"room":"lobby","guild_uuid":"guild-a","identity":"user-1"}`)
	if err != nil {
		t.Fatalf("OnRoomJoinSFU returned error: %v", err)
	}
	data := decodeAck(t, ack)
	if data["error"] == nil {
		t.Fatalf("expected non-member join SFU error, got %#v", data)
	}
}

func TestHub_BroadcastRoomUpdatedLocal_UsesGuildRoomScope(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	hub.broadcastRoomUpdatedLocal("guild-a:lobby")

	if len(fanout.roomCasts["__guild:guild-a"][EventRoomUpdated]) == 0 {
		t.Fatal("expected room:updated in guild scope")
	}
	if len(fanout.broadcasts[EventRoomUpdated]) != 0 {
		t.Fatal("room:updated must not use global namespace broadcast")
	}
}

func TestHub_BroadcastRoomList_UsesGuildRoomScope(t *testing.T) {
	hub := newTestHub()
	fanout := hub.fanout.(*mockBroadcaster)

	hub.broadcastRoomList("guild-a")

	if len(fanout.roomCasts["__guild:guild-a"][EventRoomListResult]) == 0 {
		t.Fatal("expected room:list:result in guild scope")
	}
	if len(fanout.broadcasts[EventRoomListResult]) != 0 {
		t.Fatal("room:list:result must not use global namespace broadcast")
	}
}

func TestHub_SetClientGuild_SwitchesScope(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomList(conn, `{"guild_uuid":"guild-a"}`)
	hub.OnRoomList(conn, `{"guild_uuid":"guild-b"}`)

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didLeave(conn.ID(), "__guild:guild-a") {
		t.Fatal("expected client to leave previous guild scope")
	}
	if !fanout.didJoin(conn.ID(), "__guild:guild-b") {
		t.Fatal("expected client to join new guild scope")
	}
}

func TestHub_OnDisconnect_LeavesGuildScope(t *testing.T) {
	hub := newTestHub()
	conn := newAuthedMockClient("socket-1", "user-1")

	hub.OnRoomList(conn, `{"guild_uuid":"guild-a"}`)
	hub.OnDisconnect(conn)

	fanout := hub.fanout.(*mockBroadcaster)
	if !fanout.didLeave(conn.ID(), "__guild:guild-a") {
		t.Fatal("expected disconnect to leave guild scope")
	}
}
