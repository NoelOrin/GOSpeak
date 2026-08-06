package bus

import (
	"strings"
	"testing"
	"time"
)

type nopDeliverer struct{}

func (nopDeliverer) BroadcastToNamespace(string, interface{})    {}
func (nopDeliverer) BroadcastToRoom(string, string, interface{}) {}

func TestInit_EmptyURL_StartsEmbedded(t *testing.T) {
	b, cleanup, err := Init(InitConfig{
		URL:       "",
		Prefix:    "gospeak",
		Deliverer: nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b.Mode() != "embedded" {
		t.Fatalf("mode = %q, want embedded", b.Mode())
	}
	if !b.IsConnected() {
		t.Fatal("expected connected")
	}
	if b.InstanceID() == "" {
		t.Fatal("empty instance id")
	}
}

func TestInit_ExternalURL_Available_UsesExternalNoEmbed(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	b, cleanup, err := Init(InitConfig{
		URL:            es.ClientURL(),
		Prefix:         "gospeak",
		Name:           "ext-test",
		ConnectTimeout: 2 * time.Second,
		Deliverer:      nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b.Mode() != "external" {
		t.Fatalf("mode = %q, want external", b.Mode())
	}
	if !b.IsConnected() {
		t.Fatal("expected connected")
	}
}

func TestInit_ExternalURL_ProbeFailed_NoFallback(t *testing.T) {
	_, cleanup, err := Init(InitConfig{
		URL:            "nats://127.0.0.1:1",
		Prefix:         "gospeak",
		ConnectTimeout: 300 * time.Millisecond,
		Deliverer:      nopDeliverer{},
	})
	if err == nil {
		cleanup()
		t.Fatal("expected probe failure error, no embedded fallback")
	}
	if !strings.Contains(err.Error(), "probe failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProbeExternal_OKAndFail(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	if err := ProbeExternal(es.ClientURL(), time.Second); err != nil {
		t.Fatalf("probe ok url: %v", err)
	}
	if err := ProbeExternal("nats://127.0.0.1:1", 200*time.Millisecond); err == nil {
		t.Fatal("probe bad url should fail")
	}
}
