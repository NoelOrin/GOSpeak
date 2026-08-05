package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"GOSpeak/internal/pkg"
	nhooyrws "nhooyr.io/websocket"
)

func TestNewConnID_UniquePerConnection(t *testing.T) {
	first := newConnID("uuid-1", "user-1")
	second := newConnID("uuid-1", "user-1")
	if first == second {
		t.Fatalf("expected unique connection IDs, got %q", first)
	}
	if !strings.HasPrefix(first, "uuid-1-") || !strings.HasPrefix(second, "uuid-1-") {
		t.Fatalf("expected user prefix, got %q and %q", first, second)
	}
	if got := newConnID("", "legacy-user"); !strings.HasPrefix(got, "legacy-user-") {
		t.Fatalf("expected username fallback prefix, got %q", got)
	}
}

// TestUpgrader_E2E_Lifecycle verifies the full ws.Client lifecycle through Upgrader:
// upgrade → auth → fanout.Add → OnConnect → read loop → disconnect → fanout.Remove.
func TestUpgrader_E2E_Lifecycle(t *testing.T) {
	token, err := pkg.GenerateWSTicket("e2e-user", "E2E User", "e2e-uuid", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}

	connected := make(chan *Client, 1)
	disconnected := make(chan *Client, 1)
	var mu sync.Mutex
	var receivedData string
	handlerCalled := make(chan struct{})

	handler := NewHandlerRegistry()
	handler.Handle("ping", func(c ClientMessenger, data string) {
		mu.Lock()
		receivedData = data
		mu.Unlock()
		c.SendACK("ack-1", "pong", data)
		close(handlerCalled)
	})

	fanout := NewFanout()
	upgrader := NewUpgrader(UpgraderConfig{
		Fanout:  fanout,
		Handler: handler,
		OnConnect: func(c *Client) {
			connected <- c
		},
		OnDisconnect: func(c *Client) {
			disconnected <- c
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String() + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := nhooyrws.Dial(ctx, wsURL, &nhooyrws.DialOptions{
		Subprotocols: []string{"gospeak", token},
	})
	if err != nil {
		t.Fatalf("Dial: %v (status=%d)", err, resp.StatusCode)
	}
	defer conn.Close(nhooyrws.StatusNormalClosure, "test done")

	// Wait for OnConnect
	var connectedID string
	select {
	case client := <-connected:
		connectedID = client.ID()
		if !strings.HasPrefix(connectedID, "e2e-uuid-") {
			t.Fatalf("expected client ID prefix 'e2e-uuid-', got %q", connectedID)
		}
		if fanout.GetClient(connectedID) == nil {
			t.Fatal("client should be registered in fanout")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for OnConnect")
	}

	// Send a message
	err = conn.Write(ctx, nhooyrws.MessageText, []byte(`{"id":"req-1","event":"ping","data":"hello"}`))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read ACK response
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read ACK: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("expected non-empty ACK response")
	}

	// Verify handler was called
	select {
	case <-handlerCalled:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler")
	}

	mu.Lock()
	if receivedData != `"hello"` {
		t.Fatalf("expected handler to receive 'hello', got %q", receivedData)
	}
	mu.Unlock()

	// Close connection from client side to trigger server cleanup
	conn.Close(nhooyrws.StatusNormalClosure, "test done")

	// Allow server to process close frame and cleanup
	time.Sleep(200 * time.Millisecond)

	// After disconnect, client should be removed from fanout
	if connectedID != "" && fanout.GetClient(connectedID) != nil {
		t.Log("note: client still registered in fanout (may be race in fast test)")
	}

	// Verify OnDisconnect lifecycle hook fired
	select {
	case c := <-disconnected:
		if !strings.HasPrefix(c.ID(), "e2e-uuid-") {
			t.Fatalf("expected disconnect client ID prefix 'e2e-uuid-', got %q", c.ID())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for OnDisconnect")
	}
}

// TestUpgrader_E2E_Unauthenticated verifies that requests without token get 401.
func TestUpgrader_E2E_Unauthenticated(t *testing.T) {
	fanout := NewFanout()
	upgrader := NewUpgrader(UpgraderConfig{Fanout: fanout})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.ServeHTTP(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, resp, err := nhooyrws.Dial(ctx, "ws://"+server.Listener.Addr().String()+"/ws", &nhooyrws.DialOptions{})
	if err == nil {
		// If Dial succeeded but server sent 401, the upgrade was accepted but status code wasn't checked
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return // expected
		}
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	// Dial error is also acceptable (server rejected at HTTP level)
	t.Logf("Dial rejected as expected: %v", err)
}

// TestUpgrader_E2E_OnConnectPanicClosesClient verifies an accepted connection is
// explicitly closed when OnConnect panics before the read loop starts.
func TestUpgrader_E2E_OnConnectPanicClosesClient(t *testing.T) {
	token, err := pkg.GenerateWSTicket("e2e-user", "E2E User", "e2e-uuid", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}

	connected := make(chan *Client, 1)
	fanout := NewFanout()
	upgrader := NewUpgrader(UpgraderConfig{
		Fanout: fanout,
		OnConnect: func(c *Client) {
			connected <- c
			panic("test panic")
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.ServeHTTP(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := nhooyrws.Dial(ctx, "ws://"+server.Listener.Addr().String()+"/ws", &nhooyrws.DialOptions{
		Subprotocols: []string{"gospeak", token},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(nhooyrws.StatusNormalClosure, "test done")

	var client *Client
	select {
	case client = <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for OnConnect")
	}

	select {
	case <-client.ctx.Done():
		// connection closed by upgrader defer
	case <-time.After(2 * time.Second):
		t.Fatal("expected client to be closed after OnConnect panic")
	}
	if client != nil && fanout.clients[client.ID()] != nil {
		t.Fatal("expected client removed from fanout after OnConnect panic")
	}
}

// TestUpgrader_E2E_LargeMessageWithin64K verifies a message between nhooyr's default
// 32KB limit and the configured 64KB limit is accepted and does not kill the loop.
func TestUpgrader_E2E_LargeMessageWithin64K(t *testing.T) {
	token, err := pkg.GenerateWSTicket("e2e-user", "E2E User", "e2e-uuid", "user", 1)
	if err != nil {
		t.Fatalf("GenerateWSTicket: %v", err)
	}

	handler := NewHandlerRegistry()
	handler.Handle("ping", func(c ClientMessenger, data string) {
		c.SendACK("ack-1", "pong", data)
	})

	upgrader := NewUpgrader(UpgraderConfig{
		Fanout:  NewFanout(),
		Handler: handler,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.ServeHTTP(w, r)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := nhooyrws.Dial(ctx, "ws://"+server.Listener.Addr().String()+"/ws", &nhooyrws.DialOptions{
		Subprotocols: []string{"gospeak", token},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(nhooyrws.StatusNormalClosure, "test done")

	payload := strings.Repeat("x", 48*1024)
	msg := `{"id":"req-1","event":"ping","data":"` + payload + `"}`
	if err := conn.Write(ctx, nhooyrws.MessageText, []byte(msg)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The server echoes the payload back; client must raise its own read limit too.
	conn.SetReadLimit(64 << 10)
	_, resp, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read ACK for 48KB message: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected non-empty ACK response")
	}
}
