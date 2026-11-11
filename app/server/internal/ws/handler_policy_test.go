package ws

import (
	"testing"

	"GOSpeak/internal/pkg"
)

type closeTrackingClient struct {
	*testClient
	closed bool
}

func (c *closeTrackingClient) Close() { c.closed = true }

func TestHandlerRegistry_Dispatch_PolicyGuardRejectsAndTerminates(t *testing.T) {
	r := NewHandlerRegistry()
	called := false
	r.HandleAck("message:send", func(c ClientMessenger, data string) (string, error) {
		called = true
		return "ok", nil
	})
	r.SetPolicyGuard(func(c ClientMessenger, msg Message) PolicyDecision {
		return PolicyDecision{Code: pkg.FORBIDDEN, Message: "user has been banned", Terminate: true}
	})

	tc := &closeTrackingClient{testClient: &testClient{id: "c1"}}
	r.Dispatch(tc, Message{ID: "req-1", Event: "message:send", Data: []byte(`{}`)})

	if called {
		t.Fatal("policy rejection must stop handler execution")
	}
	if len(tc.emitted) != 1 {
		t.Fatalf("expected 1 error ACK, got %d", len(tc.emitted))
	}
	ea, ok := tc.emitted[0].(errAck)
	if !ok || ea.code != int(pkg.FORBIDDEN) || ea.msg != "user has been banned" {
		t.Fatalf("unexpected ACK: %+v", tc.emitted[0])
	}
	if !tc.closed {
		t.Fatal("banned connection must be closed to stop downstream broadcasts")
	}
}

func TestHandlerRegistry_Dispatch_PolicyGuardRejectsWithoutClosing(t *testing.T) {
	r := NewHandlerRegistry()
	r.HandleAck("private:send", func(c ClientMessenger, data string) (string, error) {
		return "ok", nil
	})
	r.SetPolicyGuard(func(c ClientMessenger, msg Message) PolicyDecision {
		return PolicyDecision{Code: pkg.FORBIDDEN, Message: "forbidden"}
	})

	tc := &closeTrackingClient{testClient: &testClient{id: "c1"}}
	r.Dispatch(tc, Message{ID: "req-1", Event: "private:send", Data: []byte(`{}`)})

	if tc.closed {
		t.Fatal("whitelist rejection must not disconnect the client")
	}
	if len(tc.emitted) != 1 {
		t.Fatalf("expected 1 error ACK, got %d", len(tc.emitted))
	}
}

// NoAck 类事件（typing）被策略拒绝时静默丢弃，不产生 ACK 也不执行 handler。
func TestHandlerRegistry_Dispatch_PolicyGuardDropsNoAckEvent(t *testing.T) {
	r := NewHandlerRegistry()
	called := false
	r.Handle("typing", func(c ClientMessenger, data string) { called = true })
	r.SetPolicyGuard(func(c ClientMessenger, msg Message) PolicyDecision {
		return PolicyDecision{Code: pkg.FORBIDDEN, Message: "user has been banned", Terminate: true}
	})

	tc := &closeTrackingClient{testClient: &testClient{id: "c1"}}
	r.Dispatch(tc, Message{Event: "typing", Data: []byte(`{}`)})

	if called || len(tc.emitted) != 0 || !tc.closed {
		t.Fatalf("rejected no-ack event must be dropped and terminated: called=%v emitted=%d closed=%v",
			called, len(tc.emitted), tc.closed)
	}
}

func TestHandlerRegistry_Dispatch_PolicyGuardAllows(t *testing.T) {
	r := NewHandlerRegistry()
	called := false
	r.HandleAck("message:send", func(c ClientMessenger, data string) (string, error) {
		called = true
		return "ok", nil
	})
	r.SetPolicyGuard(func(c ClientMessenger, msg Message) PolicyDecision {
		return PolicyDecision{Code: pkg.SUCCESS}
	})

	tc := &closeTrackingClient{testClient: &testClient{id: "c1"}}
	r.Dispatch(tc, Message{ID: "req-1", Event: "message:send", Data: []byte(`{}`)})

	if !called || tc.closed {
		t.Fatalf("allowed event must reach handler: called=%v closed=%v", called, tc.closed)
	}
	if _, ok := tc.emitted[0].(ack); !ok {
		t.Fatalf("expected success ACK, got %T", tc.emitted[0])
	}
}
