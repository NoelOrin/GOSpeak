package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	nhooyrws "nhooyr.io/websocket"

	"GOSpeak/internal/pkg"
)

type closeTrackingClient struct {
	*testClient
	closed bool
}

func (c *closeTrackingClient) Close() { c.closed = true }

// allowAllPolicy 是测试用的放行策略：fail-closed 之后，任何走 Dispatch 的
// 测试都必须显式注入 guard，否则会被当成「组合根漏接线」拒绝。
func allowAllPolicy(ClientMessenger, Message) PolicyDecision {
	return PolicyDecision{Code: pkg.SUCCESS}
}

// newPermissiveRegistry 返回已注入放行策略的注册表，供不关心策略的协议测试使用。
func newPermissiveRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.SetPolicyGuard(allowAllPolicy)
	return r
}

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

// 封禁命中时客户端必须真的收到错误 ACK：ACK 只入队就 Close 会因 writeLoop 在
// writeCh 与 ctx.Done() 之间 select 而丢失（实测送达率 0%）。
func TestHandlerRegistry_Dispatch_BanACKDeliveredBeforeClose(t *testing.T) {
	token, err := pkg.GenerateToken("banned-user", "Banned", "banned-uuid", "ban", 1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	reg := NewHandlerRegistry()
	reg.HandleAck("message:send", func(c ClientMessenger, data string) (string, error) {
		t.Error("banned client must not reach the handler")
		return "", nil
	})
	reg.SetPolicyGuard(func(c ClientMessenger, msg Message) PolicyDecision {
		return PolicyDecision{Code: pkg.USER_BANNED, Message: "user has been banned", Terminate: true}
	})

	upgrader := NewUpgrader(UpgraderConfig{Fanout: NewFanout(), Handler: reg})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String() + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const rounds = 25
	for i := 0; i < rounds; i++ {
		conn, _, err := nhooyrws.Dial(ctx, wsURL, wsDialOptionsWithCookie(token))
		if err != nil {
			t.Fatalf("round %d: Dial: %v", i, err)
		}

		if err := conn.Write(ctx, nhooyrws.MessageText, []byte(`{"id":"req-ban","event":"message:send","data":{}}`)); err != nil {
			conn.CloseNow()
			t.Fatalf("round %d: Write: %v", i, err)
		}

		_, payload, err := conn.Read(ctx)
		if err != nil {
			conn.CloseNow()
			t.Fatalf("round %d: ban ACK lost before close: %v", i, err)
		}
		var ack struct {
			ID    string `json:"id"`
			Event string `json:"event"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(payload, &ack); err != nil {
			conn.CloseNow()
			t.Fatalf("round %d: bad ACK json: %v", i, err)
		}
		if ack.ID != "req-ban" || ack.Error == nil || ack.Error.Code != int(pkg.USER_BANNED) {
			conn.CloseNow()
			t.Fatalf("round %d: unexpected ACK: %s", i, payload)
		}

		_, _, err = conn.Read(ctx)
		if err == nil {
			conn.CloseNow()
			t.Fatalf("round %d: connection must be closed after ban ACK", i)
		}
		if status := nhooyrws.CloseStatus(err); status != nhooyrws.StatusPolicyViolation {
			conn.CloseNow()
			t.Fatalf("round %d: close status = %d, want %d (%v)", i, status, nhooyrws.StatusPolicyViolation, err)
		}
		conn.CloseNow()
	}
}

// guard 未注入时必须 fail-closed：拒绝分发并回 INTERNAL_ERROR，而不是静默放行。
func TestHandlerRegistry_Dispatch_FailClosedWithoutPolicyGuard(t *testing.T) {
	r := NewHandlerRegistry()
	called := false
	r.HandleAck("message:send", func(c ClientMessenger, data string) (string, error) {
		called = true
		return "ok", nil
	})

	tc := &testClient{id: "c1"}
	r.Dispatch(tc, Message{ID: "req-1", Event: "message:send", Data: []byte(`{}`)})

	if called {
		t.Fatal("dispatch without policy guard must not reach the handler")
	}
	if len(tc.emitted) != 1 {
		t.Fatalf("expected 1 error ACK, got %d", len(tc.emitted))
	}
	ea, ok := tc.emitted[0].(errAck)
	if !ok || ea.code != int(pkg.INTERNAL_ERROR) {
		t.Fatalf("expected INTERNAL_ERROR ACK, got %+v", tc.emitted[0])
	}
}
