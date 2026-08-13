package bus

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartEmbeddedServer_ClientCanConnect(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatalf("StartEmbeddedServer: %v", err)
	}
	defer es.Shutdown()

	url := es.ClientURL()
	if url == "" {
		t.Fatal("empty ClientURL")
	}
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect embedded: %v", err)
	}
	defer nc.Close()
	if !nc.IsConnected() {
		t.Fatal("expected connected")
	}
}

func TestStartEmbeddedServer_JetStreamAvailable(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatalf("StartEmbeddedServer: %v", err)
	}
	defer es.Shutdown()

	nc, err := nats.Connect(es.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream context: %v", err)
	}
	info, err := js.AccountInfo()
	if err != nil {
		t.Fatalf("AccountInfo: %v", err)
	}
	if info == nil {
		t.Fatal("AccountInfo returned nil")
	}
}

func TestStartEmbeddedServerOnPort_Fixed(t *testing.T) {
	es, err := StartEmbeddedServerOnPort(0)
	if err != nil {
		t.Fatalf("bootstrap free port server: %v", err)
	}
	// Borrow a free port by starting random, then rebind via second start after shutdown
	// using OS-assigned free port discovery from first ClientURL is flaky; instead use port 0
	// and validate fixed path with a known free TCP port.
	es.Shutdown()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	es, err = StartEmbeddedServerOnPort(port)
	if err != nil {
		t.Fatalf("StartEmbeddedServerOnPort(%d): %v", port, err)
	}
	defer es.Shutdown()

	want := fmt.Sprintf("nats://127.0.0.1:%d", port)
	if es.ClientURL() != want {
		t.Fatalf("ClientURL = %q, want %q", es.ClientURL(), want)
	}
	nc, err := nats.Connect(es.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect fixed port: %v", err)
	}
	defer nc.Close()
}

func TestInit_EmbeddedPortFixed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	b, cleanup, err := Init(InitConfig{
		URL:          "",
		Prefix:       "gospeak",
		EmbeddedPort: port,
		Deliverer:    nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b.Mode() != "embedded" {
		t.Fatalf("mode = %q, want embedded", b.Mode())
	}
	nb, ok := b.(*NATSBus)
	if !ok {
		t.Fatal("expected *NATSBus")
	}
	want := fmt.Sprintf("nats://127.0.0.1:%d", port)
	if nb.Conn() == nil || nb.Conn().ConnectedUrl() != want {
		got := ""
		if nb.Conn() != nil {
			got = nb.Conn().ConnectedUrl()
		}
		t.Fatalf("ConnectedUrl = %q, want %q", got, want)
	}
}
