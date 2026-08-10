package bus

import (
	"path/filepath"
	"testing"
)

func TestPendingWAL_AppendReadTruncate(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pending.wal")
	w, err := newPendingWAL(path)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	env := Envelope{V: 1, InstanceID: "i1", Scope: "room", Room: "r1", Event: "kick", TS: 1}
	if err := w.Append("gospeak.signal.room.r1", env); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := newPendingWAL(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	items, err := reopened.ReadAll()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(items) != 1 || items[0].subject != "gospeak.signal.room.r1" || items[0].env.Event != "kick" {
		t.Fatalf("recovered %+v, want 1 kick envelope", items)
	}
	if err := reopened.Truncate(); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	items, err = reopened.ReadAll()
	if err != nil || len(items) != 0 {
		t.Fatalf("after truncate: items=%v err=%v, want empty", items, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened: %v", err)
	}
}
