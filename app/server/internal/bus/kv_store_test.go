package bus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestSanitizeKeyRemovesNATSInvalidColon(t *testing.T) {
	got := sanitizeKey("ws:user-uuid")
	if got != "ws_user-uuid" {
		t.Fatalf("expected ws_user-uuid, got %q", got)
	}
}

func TestMembershipKV_PutGet(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenStateStore(StateStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_mem",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	snap := RoomMembersSnapshot{
		Room: "r1",
		Members: []MemberRecord{{
			Room: "r1", Identity: "alice", InstanceID: "i1", UpdatedAtMS: 1,
		}},
		UpdatedAt: 1,
	}
	if err := store.PutRoomMembers(context.Background(), snap); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRoomMembers(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 1 || got.Members[0].Identity != "alice" {
		t.Fatalf("got %+v", got)
	}
}

func TestStreamKV_PutGetDelete(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenStateStore(StateStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_stream",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.PutStream(context.Background(), "gs-aaa", "room-a", "bob"); err != nil {
		t.Fatal(err)
	}
	room, identity, err := store.GetStream(context.Background(), "gs-aaa")
	if err != nil || room != "room-a" || identity != "bob" {
		t.Fatalf("room=%q identity=%q err=%v", room, identity, err)
	}
	if err := store.DeleteStream(context.Background(), "gs-aaa"); err != nil {
		t.Fatal(err)
	}
	_, _, err = store.GetStream(context.Background(), "gs-aaa")
	if err == nil {
		t.Fatal("expected miss after delete")
	}
	if !errors.Is(err, nats.ErrKeyNotFound) && !errors.Is(err, nats.ErrKeyDeleted) {
		// nats may wrap; still accept any error on miss
		t.Logf("delete miss err=%v (ok if key absent)", err)
	}
}

func TestOpenStateStore_ReusesConn(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	nc, err := nats.Connect(es.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	store, err := OpenStateStore(StateStoreConfig{
		Prefix: "gospeak_test_reuse",
		NC:     nc,
	})
	if err != nil {
		t.Fatal(err)
	}
	// own=false: Close must not close shared nc
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if !nc.IsConnected() {
		t.Fatal("shared conn should remain connected after StateStore.Close")
	}
}

func TestMembershipKV_RevisionCAS(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenStateStore(StateStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_rev",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	snap := RoomMembersSnapshot{
		Room:    "r-cas",
		Members: []MemberRecord{{Identity: "alice", InstanceID: "i1"}},
	}
	if err := store.PutRoomMembersRev(context.Background(), snap, 0); err != nil {
		t.Fatal(err)
	}
	got, rev, err := store.GetRoomMembersRev(context.Background(), "r-cas")
	if err != nil {
		t.Fatal(err)
	}
	if rev == 0 || len(got.Members) != 1 {
		t.Fatalf("got=%+v rev=%d", got, rev)
	}

	snap.Members = append(snap.Members, MemberRecord{Identity: "bob", InstanceID: "i2"})
	if err := store.PutRoomMembersRev(context.Background(), snap, rev+1); err == nil {
		t.Fatal("stale revision should conflict")
	}
	if err := store.PutRoomMembersRev(context.Background(), snap, rev); err != nil {
		t.Fatalf("matching revision should succeed: %v", err)
	}
	_, rev, err = store.GetRoomMembersRev(context.Background(), "r-cas")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteRoomMembersRev(context.Background(), "r-cas", rev+1); err == nil {
		t.Fatal("delete with stale revision should conflict")
	}
	if err := store.DeleteRoomMembersRev(context.Background(), "r-cas", rev); err != nil {
		t.Fatalf("delete with matching revision should succeed: %v", err)
	}
	if _, _, err := store.GetRoomMembersRev(context.Background(), "r-cas"); !errors.Is(err, nats.ErrKeyNotFound) {
		t.Fatalf("expected key not found after CAS delete, got %v", err)
	}
}
