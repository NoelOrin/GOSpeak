package signal

import (
	"encoding/json"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	message "GOSpeak/internal/pkg/message"
	"GOSpeak/internal/policy"
)

type countingConvSvc struct{ sent int }

func (c *countingConvSvc) SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*message.DTO, error) {
	c.sent++
	return &message.DTO{Content: content}, nil
}

func bannedPolicy(policy.Subject, policy.Action, string) policy.Decision {
	return policy.Decision{
		Allowed:   false,
		Code:      pkg.USER_BANNED,
		Message:   "user has been banned",
		Terminate: true,
	}
}

// signal 层用 DB 解析出的权威域复查策略：payload 不带 domain_uuid 时，
// 判决仍应拿到房间真实所属域，而不是空域或被伪造的域。
func TestOnMessageSend_BannedUserRejectedByPolicy(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{{
		UUID:       "uuid-text-chat",
		Name:       "text-chat",
		Type:       model.RoomTypeText,
		DomainUUID: "dom-1",
	}}}
	h := newTestHub()
	h.roomStore = store
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	var policyDomain string
	h.SetAccessPolicy(func(sub policy.Subject, action policy.Action, domainUUID string) policy.Decision {
		policyDomain = domainUUID
		return bannedPolicy(sub, action, domainUUID)
	})

	conn := newAuthedMockClient("conn-1", "alice")
	if _, err := h.OnRoomJoin(conn, `{"room":"text-chat","domain_uuid":"dom-1","identity":"alice"}`); err != nil {
		t.Fatalf("join: %v", err)
	}

	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","content":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var ackMap map[string]interface{}
	if err := json.Unmarshal([]byte(ack), &ackMap); err != nil {
		t.Fatalf("bad ack json: %v", err)
	}
	if ackMap["error"] != "user has been banned" {
		t.Fatalf("expected policy rejection, got %v", ackMap)
	}
	if msgSvc.sendCalled != 0 {
		t.Fatalf("message service must not be called for banned user, got %d", msgSvc.sendCalled)
	}
	if policyDomain != "dom-1" {
		t.Fatalf("policy must use the DB-resolved domain, got %q", policyDomain)
	}
}

func TestOnPrivateMessageSend_BannedUserRejectedByPolicy(t *testing.T) {
	h := newTestHub()
	convSvc := &countingConvSvc{}
	h.SetConversationService(convSvc)
	h.SetAccessPolicy(bannedPolicy)

	conn := newAuthedMockClient("conn-1", "alice")
	ack, err := h.OnPrivateMessageSend(conn, `{"target_identity":"bob","content":"hi"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var ackMap map[string]interface{}
	if err := json.Unmarshal([]byte(ack), &ackMap); err != nil {
		t.Fatalf("bad ack json: %v", err)
	}
	if ackMap["error"] != "user has been banned" {
		t.Fatalf("expected policy rejection, got %v", ackMap)
	}
	if convSvc.sent != 0 {
		t.Fatalf("private message must not be delivered, got %d", convSvc.sent)
	}
}

// 未注入策略时保持原有行为，避免影响嵌入/测试路径。
func TestOnPrivateMessageSend_NoPolicyAllows(t *testing.T) {
	h := newTestHub()
	convSvc := &countingConvSvc{}
	h.SetConversationService(convSvc)

	conn := newAuthedMockClient("conn-1", "alice")
	ack, err := h.OnPrivateMessageSend(conn, `{"target_identity":"bob","content":"hi"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ack == "" || convSvc.sent != 1 {
		t.Fatalf("without policy the existing path must still work: ack=%q sent=%d", ack, convSvc.sent)
	}
}
