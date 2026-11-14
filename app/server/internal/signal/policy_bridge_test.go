package signal

import (
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

// message:send 不在 signal 层重复评估策略：ws.HandlerRegistry 的 PolicyGuard
// 已对每条上行消息判决过一次（通用入口），这里只保留房间/权限检查。
// 该断言固定住「单点强制」契约，避免策略评估被重新加回产生重复与不一致。
func TestOnMessageSend_DoesNotReevaluateAccessPolicy(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{{
		UUID:       "uuid-text-chat",
		Name:       "text-chat",
		Type:       model.RoomTypeText,
		DomainUUID: "dom-1",
	}}}
	h := newTestHub()
	h.roomStore = store
	h.SetDomainPermissionChecker(func(domainUUID, userUUID, permCode string) bool { return true })
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	evaluated := 0
	h.SetAccessPolicy(func(sub policy.Subject, action policy.Action, domainUUID string) policy.Decision {
		evaluated++
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
	if evaluated != 0 {
		t.Fatalf("message:send must not re-run access policy, got %d evaluations", evaluated)
	}
	if ack != "" || msgSvc.sendCalled != 1 {
		t.Fatalf("message must be delivered when handler-level guard allowed it: ack=%q sent=%d", ack, msgSvc.sendCalled)
	}
}

func TestOnPrivateMessageSend_BannedUserRejectedByPolicy(t *testing.T) {
	h := newTestHub()
	convSvc := &countingConvSvc{}
	h.SetConversationService(convSvc)
	h.SetAccessPolicy(bannedPolicy)

	conn := newAuthedMockClient("conn-1", "alice")
	ack, err := h.OnPrivateMessageSend(conn, `{"target_identity":"bob","content":"hi"}`)
	if err == nil {
		t.Fatal("expected error path")
	}
	if err.Error() != "user has been banned" {
		t.Fatalf("expected policy rejection, got %v", err)
	}
	if ack != "" {
		t.Fatalf("expected empty ack data on error, got %q", ack)
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
