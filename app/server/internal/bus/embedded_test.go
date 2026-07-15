package bus

import (
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
