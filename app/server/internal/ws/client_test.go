package ws

import (
	"sync"
	"testing"

	"GOSpeak/internal/pkg"
)

func TestNewClient(t *testing.T) {
	claims := &pkg.Claims{Username: "user-1", UserUUID: "uuid-1"}
	c := NewTestClient("client-1", claims)

	if c.ID() != "client-1" {
		t.Fatalf("expected ID 'client-1', got %q", c.ID())
	}
	if c.Claims() != claims {
		t.Fatal("expected claims to match")
	}
}

func TestClient_Send(t *testing.T) {
	c := NewTestClient("c1", nil)
	ok := c.Send(map[string]string{"msg": "hello"})
	if !ok {
		t.Fatal("expected Send to return true")
	}

	select {
	case data := <-c.writeCh:
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	default:
		t.Fatal("expected data in writeCh")
	}
	// Drain writeCh so the next test starts clean
	for len(c.writeCh) > 0 {
		<-c.writeCh
	}
}

func TestClient_SendACK(t *testing.T) {
	c := NewTestClient("c1", nil)
	c.SendACK("req-1", "event:test", "result")

	select {
	case data := <-c.writeCh:
		if len(data) == 0 {
			t.Fatal("expected non-empty ACK data")
		}
	default:
		t.Fatal("expected ACK in writeCh")
	}
}

func TestClient_SendQueueFull_DropsIncrement(t *testing.T) {
	c := NewTestClient("c1", nil)
	for i := 0; i < cap(c.writeCh); i++ {
		c.writeCh <- []byte{1}
	}
	if ok := c.Send(map[string]string{"msg": "overflow"}); ok {
		t.Fatal("expected Send to fail when queue is full")
	}
	if got := c.DroppedCount(); got != 1 {
		t.Fatalf("expected dropped count 1, got %d", got)
	}
}

func TestClient_SendErrorACK(t *testing.T) {
	c := NewTestClient("c1", nil)
	c.SendErrorACK("req-1", "event:fail", 5001, "error msg")

	select {
	case data := <-c.writeCh:
		if len(data) == 0 {
			t.Fatal("expected non-empty error ACK data")
		}
	default:
		t.Fatal("expected error ACK in writeCh")
	}
}

func TestClient_SendAfterClose(t *testing.T) {
	c := NewTestClient("c1", nil)
	// Close without full WS conn - should not panic
	// Just test that Send doesn't panic
	_ = c.Send(map[string]string{"msg": "before close"})
}

func TestClient_Close(t *testing.T) {
	c := NewTestClient("c1", nil)
	// Close without full WS conn should signal the closed channel
	c.Close()

	select {
	case <-c.closed:
		// closed successfully
	default:
		t.Fatal("expected closed channel to be signaled")
	}
	// Close 幂等且 Send 立即失败
	c.Close()
	if ok := c.Send(map[string]string{"msg": "after close"}); ok {
		t.Fatal("expected Send to fail after Close")
	}
}

func TestClient_Claims(t *testing.T) {
	claims := &pkg.Claims{Username: "test-user", Role: "admin"}
	c := NewTestClient("c1", claims)

	if c.Claims().Username != "test-user" {
		t.Fatalf("expected username 'test-user', got %q", c.Claims().Username)
	}
	if c.Claims().Role != "admin" {
		t.Fatalf("expected role 'admin', got %q", c.Claims().Role)
	}
}

func TestClientConcurrentSend(t *testing.T) {
	c := NewTestClient("c1", nil)

	// Concurrent writes should not race (verified with -race)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			c.Send(map[string]int{"val": i})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			c.Send(map[string]int{"val": i + 100})
		}
	}()

	wg.Wait()
	// Drain writeCh
	for len(c.writeCh) > 0 {
		<-c.writeCh
	}
}

func TestClient_State_New(t *testing.T) {
	c := NewTestClient("c1", nil)
	if got := c.State(); got != ConnStateNew {
		t.Fatalf("State() = %q, want new", got)
	}
}

func TestClient_Close_StateTransitions(t *testing.T) {
	c := NewTestClient("c1", nil)
	var transitions []ConnState
	c.OnStateChange = func(oldState, newState ConnState) {
		transitions = append(transitions, oldState, newState)
	}

	c.Close()
	if got := c.State(); got != ConnStateClosed {
		t.Fatalf("State() after Close = %q, want closed", got)
	}
	if len(transitions) == 0 || transitions[len(transitions)-1] != ConnStateClosed {
		t.Fatalf("expected final transition to closed, got %v", transitions)
	}
}

func TestClient_StateMachine_RejectsBackwardTransition(t *testing.T) {
	c := NewTestClient("c1", nil)
	c.setState(ConnStateConnecting)
	c.setState(ConnStateOpen)
	if got := c.State(); got != ConnStateOpen {
		t.Fatalf("State() = %q, want open", got)
	}
	c.setState(ConnStateClosed)
	c.setState(ConnStateOpen)
	if got := c.State(); got != ConnStateClosed {
		t.Fatalf("closed state must not regress to %q", got)
	}
}

func TestValidConnStateTransition(t *testing.T) {
	cases := []struct {
		from ConnState
		to   ConnState
		want bool
	}{
		{ConnStateNew, ConnStateConnecting, true},
		{ConnStateNew, ConnStateOpen, false},
		{ConnStateConnecting, ConnStateOpen, true},
		{ConnStateOpen, ConnStateClosing, true},
		{ConnStateClosing, ConnStateOpen, false},
		{ConnStateClosed, ConnStateOpen, false},
	}
	for _, tc := range cases {
		if got := validConnStateTransition(tc.from, tc.to); got != tc.want {
			t.Errorf("validConnStateTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
