package signal

import (
	"errors"
	"strings"
	"testing"

	"GOSpeak/internal/sfu"
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

// TestOnRoomJoinSFU_GuestSpeakOffDegraded 验证：降级 provider（MuteLevel=degraded）下，
// 禁说访客在 SFU 进房确认后被强制服务端禁言；禁言失败则拒绝进房。
func TestOnRoomJoinSFU_GuestSpeakOffDegraded(t *testing.T) {
	provider := &plainMuteProvider{caps: sfu.Capabilities{MuteLevel: sfu.EnforcementDegraded, ServerMute: true}}
	hub := NewHubWithOptions(nil, nil, nil, nil, HubOptions{
		SFUProvider:    provider,
		DomainChecker:  func(string, string) bool { return true },
		DomainPermissionChecker: func(string, string, string) bool { return true },
	})
	// guest speak disabled
	hub.SetGuestSpeakPolicy(func(domainUUID, userUUID string) (bool, error) {
		return false, nil
	})
	client := newAuthedMockClient("conn-g", "guest-1")
	ack, err := hub.OnRoomJoinSFU(client, `{"domain_uuid":"dom-1","room":"r1","identity":"guest-1"}`)
	if err != nil {
		t.Fatalf("ack error: %v", err)
	}
	// 降级 provider 无法直接限制发布，但应在进房确认后强制服务端禁言；
	// 禁言成功则进房照常成功（静音已生效），禁言失败才 fail-closed 拒绝。
	if len(provider.muted) == 0 {
		t.Fatalf("expect MuteParticipant called on degraded provider for speak-off guest, ack=%s", ack)
	}
}

// TestOnRoomJoinSFU_GuestSpeakOnNoMute 验证：可发言访客不触发强制禁言。
func TestOnRoomJoinSFU_GuestSpeakOnNoMute(t *testing.T) {
	provider := &plainMuteProvider{caps: sfu.Capabilities{MuteLevel: sfu.EnforcementDegraded, ServerMute: true}}
	hub := NewHubWithOptions(nil, nil, nil, nil, HubOptions{
		SFUProvider:    provider,
		DomainChecker:  func(string, string) bool { return true },
		DomainPermissionChecker: func(string, string, string) bool { return true },
	})
	hub.SetGuestSpeakPolicy(func(domainUUID, userUUID string) (bool, error) {
		return true, nil
	})
	client := newAuthedMockClient("conn-g", "guest-2")
	ack, err := hub.OnRoomJoinSFU(client, `{"domain_uuid":"dom-1","room":"r1","identity":"guest-2"}`)
	if err != nil {
		t.Fatalf("ack error: %v", err)
	}
	if !strings.Contains(ack, `"ok":true`) {
		t.Fatalf("expect successful join, got %s", ack)
	}
	if len(provider.muted) != 0 {
		t.Fatalf("expect no forced mute for speaking guest, got %v", provider.muted)
	}
}
