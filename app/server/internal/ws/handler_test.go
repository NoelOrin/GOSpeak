package ws

import (
	"errors"
	"testing"

	"GOSpeak/internal/pkg"
)

type testClient struct {
	id      string
	claims  *pkg.Claims
	emitted []interface{}
}

func (c *testClient) ID() string              { return c.id }
func (c *testClient) Claims() *pkg.Claims     { return c.claims }
func (c *testClient) Send(v interface{}) bool { c.emitted = append(c.emitted, v); return true }
func (c *testClient) SendACK(id, event string, data interface{}) {
	c.emitted = append(c.emitted, ack{id: id, event: event, data: data})
}
func (c *testClient) SendErrorACK(id, event string, code int, msg string) {
	c.emitted = append(c.emitted, errAck{id: id, event: event, code: code, msg: msg})
}
func (c *testClient) Close() {}

type ack struct {
	id    string
	event string
	data  interface{}
}

type errAck struct {
	id    string
	event string
	code  int
	msg   string
}

func TestHandlerRegistry_Dispatch_NoAck(t *testing.T) {
	r := NewHandlerRegistry()
	called := false
	r.Handle("event:noack", func(c ClientMessenger, data string) {
		called = true
	})

	r.Dispatch(&testClient{id: "c1"}, Message{Event: "event:noack", Data: []byte(`{"key":"val"}`)})

	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestHandlerRegistry_Dispatch_Ack(t *testing.T) {
	r := NewHandlerRegistry()
	r.HandleAck("event:ack", func(c ClientMessenger, data string) (string, error) {
		return "ok", nil
	})

	tc := &testClient{id: "c1"}
	r.Dispatch(tc, Message{ID: "req-1", Event: "event:ack", Data: []byte(`{}`)})

	if len(tc.emitted) != 1 {
		t.Fatalf("expected 1 ACK, got %d", len(tc.emitted))
	}
	a, ok := tc.emitted[0].(ack)
	if !ok {
		t.Fatalf("expected ack struct, got %T", tc.emitted[0])
	}
	if a.id != "req-1" || a.event != "event:ack" {
		t.Fatalf("ack fields: id=%s event=%s", a.id, a.event)
	}
}

func TestHandlerRegistry_Dispatch_UnknownEvent(t *testing.T) {
	r := NewHandlerRegistry()
	tc := &testClient{id: "c1"}

	r.Dispatch(tc, Message{ID: "req-1", Event: "unknown:event", Data: []byte(`{}`)})

	if len(tc.emitted) != 1 {
		t.Fatalf("expected error ACK for unknown event, got %d", len(tc.emitted))
	}
	e, ok := tc.emitted[0].(errAck)
	if !ok {
		t.Fatalf("expected errAck, got %T", tc.emitted[0])
	}
	if e.id != "req-1" || e.event != "unknown:event" || e.code != int(pkg.INVALID_PARAMS) {
		t.Fatalf("error ack fields: %+v", e)
	}
}

func TestHandlerRegistry_Dispatch_PanicRecover(t *testing.T) {
	r := NewHandlerRegistry()
	r.HandleAck("event:panic", func(c ClientMessenger, data string) (string, error) {
		panic("test panic")
	})

	tc := &testClient{id: "c1"}
	r.Dispatch(tc, Message{ID: "req-1", Event: "event:panic", Data: []byte(`{}`)})

	if len(tc.emitted) == 0 {
		t.Fatal("expected error ACK after panic")
	}
	if _, ok := tc.emitted[0].(errAck); !ok {
		t.Fatal("expected errAck struct after panic")
	}
}

func TestHandlerRegistry_Dispatch_AckError(t *testing.T) {
	r := NewHandlerRegistry()
	r.HandleAck("event:fail", func(c ClientMessenger, data string) (string, error) {
		return "", errors.New("something went wrong")
	})

	tc := &testClient{id: "c1"}
	r.Dispatch(tc, Message{ID: "req-1", Event: "event:fail", Data: []byte(`{}`)})

	if len(tc.emitted) != 1 {
		t.Fatalf("expected 1 error ACK, got %d", len(tc.emitted))
	}
	if _, ok := tc.emitted[0].(errAck); !ok {
		t.Fatal("expected errAck struct")
	}
}

func TestHandlerRegistry_Dispatch_AckError_Sanitized(t *testing.T) {
	r := NewHandlerRegistry()
	r.HandleAck("event:fail", func(c ClientMessenger, data string) (string, error) {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "secret sql detail")
	})

	tc := &testClient{id: "c1"}
	r.Dispatch(tc, Message{ID: "req-1", Event: "event:fail", Data: []byte(`{}`)})

	if len(tc.emitted) != 1 {
		t.Fatalf("expected 1 error ACK, got %d", len(tc.emitted))
	}
	ea, ok := tc.emitted[0].(errAck)
	if !ok {
		t.Fatal("expected errAck struct")
	}
	if ea.code != int(pkg.INTERNAL_ERROR) || ea.msg != "internal server error" {
		t.Fatalf("expected sanitized error ACK, got code=%d msg=%q", ea.code, ea.msg)
	}
}

func TestHandlerRegistry_Dispatch_NullData(t *testing.T) {
	r := NewHandlerRegistry()
	var receivedData string
	r.Handle("event:null", func(c ClientMessenger, data string) {
		receivedData = data
	})

	r.Dispatch(&testClient{id: "c1"}, Message{Event: "event:null", Data: []byte(`null`)})

	// When dispatch sees JSON null, it should pass empty string to handler.
	// This is the current behavior — producers that need null vs "" should
	// encode an explicit string wrapper if required.
	if receivedData != "" {
		t.Fatalf("expected empty string for null data, got %q", receivedData)
	}
}

func TestHandlerRegistry_Dispatch_EmptyEvent(t *testing.T) {
	r := NewHandlerRegistry()
	tc := &testClient{id: "c1"}

	r.Dispatch(tc, Message{ID: "req-1", Event: "", Data: []byte(`{}`)})

	if len(tc.emitted) != 1 {
		t.Fatalf("expected error ACK for empty event, got %d", len(tc.emitted))
	}
	if _, ok := tc.emitted[0].(errAck); !ok {
		t.Fatalf("expected errAck, got %T", tc.emitted[0])
	}
}
