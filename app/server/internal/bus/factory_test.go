package bus

import (
	"testing"
)

type nopDeliverer struct{}

func (nopDeliverer) BroadcastToNamespace(string, interface{})    {}
func (nopDeliverer) BroadcastToRoom(string, string, interface{}) {}

func TestInit_StartsEmbedded(t *testing.T) {
	b, cleanup, err := Init(InitConfig{
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
