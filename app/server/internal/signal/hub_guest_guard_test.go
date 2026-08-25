package signal

import (
	"errors"
	"strings"
	"testing"
)

func TestOnRoomJoin_GuestJoinGuardBlocks(t *testing.T) {
	hub := NewHubWithOptions(nil, nil, nil, nil, HubOptions{
		DomainChecker: func(string, string) bool { return true },
	})
	blocked := errors.New("guest has been banned")
	hub.SetGuestJoinGuard(func(domainUUID, userUUID string) error {
		if userUUID == "banned-guest" {
			return blocked
		}
		return nil
	})
	client := newAuthedMockClient("conn-1", "banned-guest")
	ack, err := hub.OnRoomJoin(client, `{"domain_uuid":"dom-1","room":"r1","identity":"banned-guest"}`)
	if err != nil {
		t.Fatalf("ack error: %v", err)
	}
	if !strings.Contains(ack, "guest has been banned") {
		t.Fatalf("expect ban ack, got %s", ack)
	}
}

func TestDomainOnlineIdentities(t *testing.T) {
	hub := NewHubWithOptions(nil, nil, nil, nil, HubOptions{})
	hub.mu.Lock()
	hub.rooms["dom-1:a"] = &Room{Members: map[string]*MemberInfo{
		"m1": {Identity: "guest_aaa"},
	}}
	hub.rooms["dom-1:b"] = &Room{Members: map[string]*MemberInfo{
		"m2": {Identity: "guest_aaa"}, // 跨房间去重
		"m3": {Identity: "alice"},
	}}
	hub.rooms["dom-2:c"] = &Room{Members: map[string]*MemberInfo{
		"m4": {Identity: "bob"},
	}}
	hub.mu.Unlock()

	got := hub.DomainOnlineIdentities("dom-1")
	if len(got) != 2 {
		t.Fatalf("expect 2 distinct identities in dom-1, got %v", got)
	}
	if ids := hub.DomainOnlineIdentities("dom-2"); len(ids) != 1 || ids[0] != "bob" {
		t.Fatalf("dom-2 expect [bob], got %v", ids)
	}
}
