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
	// members: socketID -> identity
	hub.rooms[room] = &Room{
		Name:     room,
		Members:  make(map[string]*MemberInfo),
		MicMuted: make(map[string]bool),
		Speaking: make(map[string]bool),
	}
	for sid, identity := range members {
		hub.rooms[room].Members[sid] = &MemberInfo{Identity: identity, Name: identity}
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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-1", map[string]string{
		"bot-socket": "bot_helper",
		"alice-sock": "alice",
	})

	bot := newMockConn("bot-socket")
	bot.SetContext(&pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"signal:kick"},
	})

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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-1", map[string]string{
		"bot-socket": "bot_helper",
		"alice-sock": "alice",
	})

	bot := newMockConn("bot-socket")
	bot.SetContext(&pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"mute:manage"}, // 无 kick
	})

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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-1", map[string]string{
		"bot-socket": "bot_helper",
		"admin-sock": "root",
	})

	bot := newMockConn("bot-socket")
	bot.SetContext(&pkg.Claims{
		Username:    "bot_helper",
		Role:        "user",
		Permissions: []string{"signal:kick"},
	})

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
	hub.server = newMockServer()
	seedKickRoom(hub, "room-1", map[string]string{
		"root-sock":  "root",
		"admin-sock": "admin2",
	})

	// 人类 admin token 不带 permissions 列表，走 role 映射
	caller := newMockConn("root-sock")
	caller.SetContext(&pkg.Claims{Username: "root", Role: "admin"})

	hub.OnRoomKick(caller, `{"room":"room-1","targetIdentity":"admin2"}`)

	if _, ok := hub.rooms["room-1"].Members["admin-sock"]; ok {
		t.Fatal("human admin should be able to kick another admin")
	}
}
