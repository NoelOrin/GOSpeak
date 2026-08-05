package signal

import (
	"fmt"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

type mockUserStore struct {
	users map[string]*model.User
}

func (m *mockUserStore) GetByName(name string) (*model.User, error) {
	if u, ok := m.users[name]; ok {
		return u, nil
	}
	return nil, modelNotFound(name)
}

func (m *mockUserStore) GetByID(id uint) (*model.User, error) {
	for _, u := range m.users {
		if u != nil && u.ID == id {
			return u, nil
		}
	}
	return nil, modelNotFound(fmt.Sprintf("id=%d", id))
}

func (m *mockUserStore) GetByUUID(uuid string) (*model.User, error) {
	for _, u := range m.users {
		if u != nil && u.UUID == uuid {
			return u, nil
		}
	}
	return nil, modelNotFound(fmt.Sprintf("uuid=%s", uuid))
}

type mockPermChecker struct {
	// role -> set of permissions
	rolePerms map[string]map[string]bool
}

func (m *mockPermChecker) HasPermission(roleName, permCode string) bool {
	if m.rolePerms == nil {
		return false
	}
	return m.rolePerms[roleName][permCode]
}

func modelNotFound(name string) error {
	return errString("user not found: " + name)
}

type errString string

func (e errString) Error() string { return string(e) }

func seedKickRoom(hub *Hub, room string, members map[string]string) {
	// members: mapped to socketID
	hub.rooms[room] = &Room{
		Name:       room,
		Members:    make(map[string]*MemberInfo),
		ByIdentity: make(map[string]*MemberInfo),
		MicMuted:   make(map[string]bool),
		Speaking:   make(map[string]bool),
	}
	for sid, identity := range members {
		m := &MemberInfo{Identity: identity, Name: identity}
		hub.rooms[room].Members[sid] = m
		if identity != "" {
			hub.rooms[room].ByIdentity[identity] = m
		}
	}
}

func TestOnRoomKick_BotWithClaimPermissionCanKickUser(t *testing.T) {
	users := &mockUserStore{users: map[string]*model.User{
		"bot_helper": {Name: "bot_helper", Role: "user", IsBot: true},
		"alice":      {Name: "alice", Role: "user"},
	}}
	// role checker 故意不给 user 踢人权；Bot 仅靠 claims.Permissions 放行
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{
		"user": {},
	}}
	hub := NewHub(nil, nil, users, perms)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-1", map[string]string{
		"bot-socket": "bot_helper",
		"alice-sock": "alice",
	})

	bot := &mockClient{id: "bot-socket", claims: &pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"signal:kick"},
	}}

	hub.OnRoomKick(bot, `{"room":"room-1","targetIdentity":"alice"}`)

	if _, ok := hub.rooms["room-1"].Members["alice-sock"]; ok {
		t.Fatal("alice should be kicked by bot with signal:kick claim")
	}
}

func TestOnRoomKick_BotWithoutPermissionDenied(t *testing.T) {
	users := &mockUserStore{users: map[string]*model.User{
		"bot_helper": {Name: "bot_helper", Role: "user", IsBot: true},
		"alice":      {Name: "alice", Role: "user"},
	}}
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{
		"user": {},
	}}
	hub := NewHub(nil, nil, users, perms)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-1", map[string]string{
		"bot-socket": "bot_helper",
		"alice-sock": "alice",
	})

	bot := &mockClient{id: "bot-socket", claims: &pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"mute:manage"}, // 无 kick
	}}

	hub.OnRoomKick(bot, `{"room":"room-1","targetIdentity":"alice"}`)

	if _, ok := hub.rooms["room-1"].Members["alice-sock"]; !ok {
		t.Fatal("alice should remain when bot lacks signal:kick")
	}
}

func TestOnRoomKick_BotCannotKickAdmin(t *testing.T) {
	users := &mockUserStore{users: map[string]*model.User{
		"bot_helper": {Name: "bot_helper", Role: "user", IsBot: true},
		"root":       {Name: "root", Role: "admin"},
	}}
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{}}
	hub := NewHub(nil, nil, users, perms)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-1", map[string]string{
		"bot-socket": "bot_helper",
		"admin-sock": "root",
	})

	bot := &mockClient{id: "bot-socket", claims: &pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"signal:kick"},
	}}

	hub.OnRoomKick(bot, `{"room":"room-1","targetIdentity":"root"}`)

	if _, ok := hub.rooms["room-1"].Members["admin-sock"]; !ok {
		t.Fatal("admin should not be kicked by bot")
	}
}

func TestOnRoomKick_HumanAdminCanKickAdmin(t *testing.T) {
	users := &mockUserStore{users: map[string]*model.User{
		"root":   {Name: "root", Role: "admin"},
		"admin2": {Name: "admin2", Role: "admin"},
	}}
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{
		"admin": {"signal:kick": true},
	}}
	hub := NewHub(nil, nil, users, perms)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "room-1", map[string]string{
		"root-sock":  "root",
		"admin-sock": "admin2",
	})

	// 人类 admin token 不带 permissions 列表，走 role 映射
	caller := &mockClient{id: "root-sock", claims: &pkg.Claims{Username: "root", Role: "admin"}}

	hub.OnRoomKick(caller, `{"room":"room-1","targetIdentity":"admin2"}`)

	if _, ok := hub.rooms["room-1"].Members["admin-sock"]; ok {
		t.Fatal("human admin should be able to kick another admin")
	}
}

func TestOnRoomKick_ClearsTargetConnSlot(t *testing.T) {
	perms := &mockPermChecker{rolePerms: map[string]map[string]bool{
		"admin": {"signal:kick": true},
	}}
	hub := NewHub(nil, nil, nil, perms)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:room-1", map[string]string{
		"alice-sock": "alice",
		"admin-sock": "admin",
	})
	hub.connSlots["alice-sock"] = &connRoomSlots{VoiceRoom: "domain-a:room-1"}

	caller := &mockClient{id: "admin-sock", claims: &pkg.Claims{Username: "admin", UserUUID: "admin", Role: "admin"}}
	hub.OnRoomKick(caller, `{"room":"room-1","domain_uuid":"domain-a","targetIdentity":"alice"}`)

	hub.mu.RLock()
	_, exists := hub.connSlots["alice-sock"]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("expected target conn slot removed after kick")
	}
}

func TestKickUserFromDomain_KicksAllRooms(t *testing.T) {
	users := &mockUserStore{users: map[string]*model.User{
		"alice": {Name: "alice", UUID: "uuid-alice"},
	}}
	hub := NewHub(nil, nil, users, nil)
	hub.fanout = newMockBroadcaster()
	seedKickRoom(hub, "domain-a:lobby", map[string]string{"a-sock": "alice", "b-sock": "bob"})
	seedKickRoom(hub, "domain-a:stage", map[string]string{"a-sock2": "alice"})
	seedKickRoom(hub, "domain-b:lobby", map[string]string{"a-sock3": "alice"})

	hub.KickUserFromDomain("domain-a", "uuid-alice")

	if _, ok := hub.rooms["domain-a:lobby"].Members["a-sock"]; ok {
		t.Fatal("alice should be kicked from domain-a:lobby")
	}
	if _, ok := hub.rooms["domain-a:lobby"].Members["b-sock"]; !ok {
		t.Fatal("bob should remain in domain-a:lobby")
	}
	if _, ok := hub.rooms["domain-a:stage"]; ok {
		t.Fatal("empty domain-a:stage should be deleted")
	}
	if _, ok := hub.rooms["domain-b:lobby"].Members["a-sock3"]; !ok {
		t.Fatal("alice should remain in domain-b:lobby")
	}
}
