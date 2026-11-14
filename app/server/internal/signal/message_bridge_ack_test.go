package signal

import (
	"errors"
	"strings"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/ws"
)

type stubConn struct {
	ws.ClientMessenger
}

func (stubConn) ID() string            { return "stub-1" }
func (stubConn) Claims() *pkg.Claims   { return nil }

// ACK 收口约定：失败必须走 error 返回路径（*AppError，data 为空），
// 禁止把错误塞进成功 ACK 的 data（旧 {"error","code"} 旁路）。

func TestOnMessageSendValidationFailsViaErrorPath(t *testing.T) {
	h := &Hub{}
	h.SetMessageService(&mockMessageSvc{})
	data, err := h.OnMessageSend(stubConn{}, `{}`)
	if err == nil {
		t.Fatal("expected error path, got nil")
	}
	if data != "" {
		t.Fatalf("expected empty data on error, got %q", data)
	}
	if !strings.Contains(err.Error(), "room is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOnMessageSendNilService(t *testing.T) {
	h := &Hub{}
	if _, err := h.OnMessageSend(stubConn{}, `{}`); err == nil {
		t.Fatal("expected error when msg service unavailable")
	}
}

func TestAckErrorsAreAppError(t *testing.T) {
	h := &Hub{}
	_, err := h.OnMessageEdit(stubConn{}, `{}`)
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
}

func TestOnPrivateMessageSendValidationFailsViaErrorPath(t *testing.T) {
	h := &Hub{}
	data, err := h.OnPrivateMessageSend(stubConn{}, `{}`)
	if err == nil {
		t.Fatal("expected error path, got nil")
	}
	if data != "" {
		t.Fatalf("expected empty data on error, got %q", data)
	}
}

func TestOnRoomLeaveAckDataIsRoomName(t *testing.T) {
	h := &Hub{}
	h.fanout = newMockBroadcaster()
	data, err := h.OnRoomLeave(stubConn{}, `{"room":"general"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != `"general"` {
		t.Fatalf("expected room name ack, got %q", data)
	}
}
