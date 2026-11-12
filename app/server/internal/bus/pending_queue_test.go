package bus

import "testing"

func TestNATSBus_PendingQueueCap(t *testing.T) {
	b := &NATSBus{}
	for i := 0; i < maxPendingPublish+5; i++ {
		b.enqueuePending("gospeak.signal.room.r1", Envelope{
			InstanceID: "inst-a",
			Scope:      "room",
			Room:       "r1",
			Event:      "room:updated",
		})
	}
	if len(b.pending) != maxPendingPublish {
		t.Fatalf("expected pending capped at %d, got %d", maxPendingPublish, len(b.pending))
	}
}
