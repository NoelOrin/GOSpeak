package ws

import (
	"encoding/json"
	"testing"

	"GOSpeak/internal/pkg"
)

type protoClient struct {
	id      string
	claims  *pkg.Claims
	acks    []interface{}
	errAcks []interface{}
}

func (c *protoClient) ID() string              { return c.id }
func (c *protoClient) Claims() *pkg.Claims     { return c.claims }
func (c *protoClient) Send(v interface{}) bool { c.acks = append(c.acks, v); return true }
func (c *protoClient) SendACK(id, event string, data interface{}) {
	c.acks = append(c.acks, map[string]interface{}{"id": id, "event": event, "data": data})
}
func (c *protoClient) SendErrorACK(id, event string, code int, msg string) {
	c.errAcks = append(c.errAcks, map[string]interface{}{"id": id, "event": event, "code": code, "message": msg})
}
func (c *protoClient) Close() {}

func TestProtocol_Dispatch_AckResponse(t *testing.T) {
	reg := NewHandlerRegistry()
	var called bool
	reg.HandleAck("room:join", func(c ClientMessenger, data string) (string, error) {
		called = true
		return `{"ok":true}`, nil
	})

	pc := &protoClient{id: "c1"}
	reg.Dispatch(pc, Message{
		ID:    "req-1",
		Event: "room:join",
		Data:  []byte(`{"room":"lobby"}`),
	})

	if !called {
		t.Fatal("handler should be called")
	}
	if len(pc.acks) == 0 {
		t.Fatal("expected ACK response")
	}
}

func TestProtocol_Dispatch_NoAck(t *testing.T) {
	reg := NewHandlerRegistry()
	reg.Handle("room:updated", func(c ClientMessenger, data string) {})

	pc := &protoClient{id: "c1"}
	reg.Dispatch(pc, Message{
		Event: "room:updated",
		Data:  []byte(`{"room":"lobby"}`),
	})

	if len(pc.acks) > 0 {
		t.Fatal("expected no ACK for push events without ID")
	}
}

func TestProtocol_Dispatch_ErrorAck(t *testing.T) {
	reg := NewHandlerRegistry()
	reg.HandleAck("room:create", func(c ClientMessenger, data string) (string, error) {
		return "", &pkg.AppError{Code: pkg.ALREADY_EXISTS, Message: "room already exists"}
	})

	pc := &protoClient{id: "c1"}
	reg.Dispatch(pc, Message{
		ID:    "req-1",
		Event: "room:create",
		Data:  []byte(`{"room":"lobby"}`),
	})

	if len(pc.errAcks) == 0 {
		t.Fatal("expected error ACK for failed operation")
	}
}

func TestProtocol_Dispatch_UnknownEvent(t *testing.T) {
	reg := NewHandlerRegistry()
	pc := &protoClient{id: "c1"}

	reg.Dispatch(pc, Message{
		ID: "req-1", Event: "unknown:event", Data: []byte(`{}`),
	})

	if len(pc.acks) > 0 {
		t.Fatal("expected no success ACK for unknown event")
	}
	if len(pc.errAcks) != 1 {
		t.Fatalf("expected one error ACK for unknown event, got %d", len(pc.errAcks))
	}
}

func TestProtocol_Dispatch_NullData(t *testing.T) {
	reg := NewHandlerRegistry()
	var gotData string
	reg.Handle("event:null", func(c ClientMessenger, data string) {
		gotData = data
	})

	pc := &protoClient{id: "c1"}
	reg.Dispatch(pc, Message{Event: "event:null", Data: []byte(`null`)})

	if gotData != "" {
		t.Fatalf("expected empty string, got %q", gotData)
	}
}

func TestProtocol_MessageFormat(t *testing.T) {
	msg := Message{ID: "req-1", Event: "room:join", Data: []byte(`{"room":"lobby"}`)}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)
	if decoded["id"] != "req-1" || decoded["event"] != "room:join" || decoded["data"] == nil {
		t.Fatal("message JSON format mismatch")
	}
}

func TestProtocol_ACKFormat(t *testing.T) {
	ack := ACK{ID: "req-1", Event: "room:joined", Data: map[string]interface{}{"members": []string{"alice"}}}
	data, err := json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)
	if decoded["id"] != "req-1" || decoded["event"] != "room:joined" {
		t.Fatal("ACK JSON format mismatch")
	}
}

func TestProtocol_ACKErrorFormat(t *testing.T) {
	errAck := ACK{ID: "req-1", Event: "room:create", Error: &ACKError{Code: 3002, Message: "room already exists"}}
	data, err := json.Marshal(errAck)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)
	errObj, ok := decoded["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error field in ACK")
	}
	if int(errObj["code"].(float64)) != 3002 {
		t.Fatalf("expected code 3002, got %v", errObj["code"])
	}
}
