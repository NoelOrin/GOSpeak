package signal

import (
	"testing"

	"GOSpeak/internal/cluster"
)

func TestHubHandleClusterCommandKick(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandKick, Room: "lobby", Identity: "alice"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}

func TestHubHandleClusterCommandRejectsInvalid(t *testing.T) {

	hub := NewHub(nil, nil, nil, nil)
	if err := hub.HandleClusterCommand(cluster.ControlCommand{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestHubKickFromRoomClosesTargetAndBroadcasts(t *testing.T) {

	hub := NewHub(nil, nil, nil, nil)
	mb := newMockBroadcaster()
	hub.fanout = mb
	seedKickRoom(hub, "domain-a:lobby", map[string]string{
		"alice-sock": "alice",
		"bob-sock":   "bob",
	})
	alice := newMockClient("alice-sock")
	bob := newMockClient("bob-sock")
	mb.clients["alice-sock"] = alice
	mb.clients["bob-sock"] = bob
	hub.connSlots["alice-sock"] = &connRoomSlots{VoiceRoom: "domain-a:lobby"}

	hub.KickFromRoom("domain-a", "lobby", "alice")

	hub.mu.RLock()
	room, exists := hub.rooms["domain-a:lobby"]
	_, slotExists := hub.connSlots["alice-sock"]
	hub.mu.RUnlock()
	if !exists {
		t.Fatal("room should remain when other members exist")
	}
	if len(room.Members) != 1 {
		t.Fatalf("expected one remaining member, got %d", len(room.Members))
	}
	if _, ok := room.Members["bob-sock"]; !ok {
		t.Fatal("non-target member should remain")
	}
	if slotExists {
		t.Fatal("target conn slot should be cleared")
	}
	if !alice.hasEvent(EventRoomKicked) {
		t.Fatal("target should receive room:kicked")
	}
	if bob.hasEvent(EventRoomKicked) {
		t.Fatal("non-target should not receive room:kicked")
	}
	if !alice.isClosed() {
		t.Fatal("target connection should be closed after kick")
	}
	if bob.isClosed() {
		t.Fatal("non-target connection should stay open")
	}
	if len(mb.roomCasts["domain-a:lobby"][EventRoomKicked]) != 1 {
		t.Fatal("room should broadcast room:kicked")
	}
	if len(mb.roomCasts["domain-a:lobby"][EventMemberLeft]) != 1 {
		t.Fatal("room should broadcast member:left")
	}
	leftData, ok := mb.roomCasts["domain-a:lobby"][EventMemberLeft][0].(map[string]interface{})
	if !ok || leftData["identity"] != "alice" || leftData["id"] != "alice-sock" {
		t.Fatalf("unexpected member:left payload: %#v", leftData)
	}
	assertLeft(t, hub, "alice-sock", "domain-a:lobby")
}

func TestHubKickFromRoomLastMemberDeletesRoom(t *testing.T) {

	hub := NewHub(nil, nil, nil, nil)
	mb := newMockBroadcaster()
	hub.fanout = mb
	seedKickRoom(hub, "domain-a:lobby", map[string]string{"alice-sock": "alice"})
	alice := newMockClient("alice-sock")
	mb.clients["alice-sock"] = alice

	hub.KickFromRoom("domain-a", "lobby", "alice")

	hub.mu.RLock()
	_, exists := hub.rooms["domain-a:lobby"]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("room should be deleted when last member is kicked")
	}
	if !alice.isClosed() {
		t.Fatal("target connection should be closed after kick")
	}
	if len(mb.roomCasts[domainRoomKey("domain-a")][EventRoomListResult]) == 0 {
		t.Fatal("room list should be broadcast after room deletion")
	}
}

func TestHubDeleteRoomByDomainNameClosesMembers(t *testing.T) {

	hub := NewHub(nil, nil, nil, nil)
	mb := newMockBroadcaster()
	hub.fanout = mb
	seedKickRoom(hub, "domain-a:lobby", map[string]string{
		"alice-sock": "alice",
		"bob-sock":   "bob",
	})
	alice := newMockClient("alice-sock")
	bob := newMockClient("bob-sock")
	mb.clients["alice-sock"] = alice
	mb.clients["bob-sock"] = bob

	hub.DeleteRoomByDomainName("domain-a", "lobby")

	if !alice.isClosed() || !bob.isClosed() {
		t.Fatal("all member connections should be closed when room is deleted")
	}
	hub.mu.RLock()
	_, exists := hub.rooms["domain-a:lobby"]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("room should be deleted")
	}
	assertLeft(t, hub, "alice-sock", "domain-a:lobby")
	assertLeft(t, hub, "bob-sock", "domain-a:lobby")
}

func TestHubHandleClusterCommandDeleteRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandDeleteRoom, DomainUUID: "domain-a", Room: "lobby"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}

func TestHubHandleClusterCommandDeleteServer(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandDeleteServer, DomainUUID: "domain-a"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}

func TestHubHandleClusterCommandUnsupported(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: "nope", NodeID: "node-a"}
	if err := hub.HandleClusterCommand(cmd); err == nil {
		t.Fatal("expected unsupported command error")
	}
}
