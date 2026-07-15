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
	if GetStats(b).FallbackFromExternal {
		t.Fatal("FallbackFromExternal should be false")
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
	if GetStats(b).FallbackFromExternal {
		t.Fatal("FallbackFromExternal should be false")
	}
}

func TestInit_ExternalURL_Unavailable_FallsBackEmbedded(t *testing.T) {
	b, cleanup, err := Init(InitConfig{
		URL:            "nats://127.0.0.1:1",
		Prefix:         "gospeak",
		ConnectTimeout: 300 * time.Millisecond,
		Deliverer:      nopDeliverer{},
	})
	if err != nil {
		t.Fatalf("should fallback embedded, not fail: %v", err)
	}
	defer cleanup()
	if b.Mode() != "embedded" {
		t.Fatalf("mode = %q, want embedded fallback", b.Mode())
	}
	if !b.IsConnected() {
		t.Fatal("expected connected to embedded")
	}
	if !GetStats(b).FallbackFromExternal {
		t.Fatal("expected FallbackFromExternal=true")
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

func TestInit_ExternalBadURL_MessageContainsProbe(t *testing.T) {
	b, cleanup, err := Init(InitConfig{
		URL:            "nats://127.0.0.1:1",
		Prefix:         "gospeak",
		ConnectTimeout: 200 * time.Millisecond,
		Deliverer:      nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.Contains(b.Mode(), "embedded") {
		t.Fatalf("mode=%s", b.Mode())
	}
}
