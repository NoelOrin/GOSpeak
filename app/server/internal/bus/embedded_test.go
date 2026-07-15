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
