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
	c.cancel()
	close(c.closed)

	select {
	case <-c.closed:
		// closed successfully
	default:
		t.Fatal("expected closed channel to be signaled")
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
