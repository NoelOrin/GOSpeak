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

// 房间名允许中文/emoji/空格，NATS KV key 只接受 [-/_=.a-zA-Z0-9]；
// 编码后必须能被 NATS 接受，且能无损还原。
func TestSanitizeKeyRoundTripNonASCII(t *testing.T) {
	cases := []string{
		"测试",
		"kn38algfltlveio7p9cgqr1j:测试",
		"emoji-🎧",
		"综合大厅",
		"plain-room-1",
	}
	for _, in := range cases {
		enc := sanitizeKey(in)
		if !isValidNATSKey(enc) {
			t.Fatalf("sanitizeKey(%q) = %q is not a valid NATS key", in, enc)
		}
		if got := desanitizeKey(enc); got != in {
			t.Fatalf("roundtrip %q -> %q -> %q", in, enc, got)
		}
	}
	// ASCII 房间名沿用历史有损映射（空格/冒号 → 下划线），
	// 保证升级前写入的 KV 记录仍能命中；这类 key 不做可逆编码。
	for in, want := range map[string]string{
		"domain:lobby": "domain_lobby",
		"a b":          "a_b",
		"ws:user-uuid": "ws_user-uuid",
	} {
		if got := sanitizeKey(in); got != want {
			t.Fatalf("legacy mapping %q = %q, want %q", in, got, want)
		}
	}
}

func TestMembershipKV_ChineseRoomName(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	store, err := OpenStateStore(StateStoreConfig{
		URL:    es.ClientURL(),
		Prefix: "gospeak_test_cjk",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	room := "kn38algfltlveio7p9cgqr1j:测试"
	snap := RoomMembersSnapshot{
		Room: room,
		Members: []MemberRecord{{
			Room: room, Identity: "admin", InstanceID: "i1", UpdatedAtMS: 1,
		}},
		UpdatedAt: 1,
	}
	if err := store.PutRoomMembers(ctx, snap); err != nil {
		t.Fatalf("PutRoomMembers: %v", err)
	}
	got, err := store.GetRoomMembers(ctx, room)
	if err != nil {
		t.Fatalf("GetRoomMembers: %v", err)
	}
	if len(got.Members) != 1 || got.Members[0].Identity != "admin" {
		t.Fatalf("got %+v", got)
	}

	if err := store.PutRoomMeta(ctx, room, RoomMeta{Name: "测试", DomainUUID: "kn38algfltlveio7p9cgqr1j"}); err != nil {
		t.Fatalf("PutRoomMeta: %v", err)
	}
	meta, err := store.GetRoomMeta(ctx, room)
	if err != nil || meta.Name != "测试" {
		t.Fatalf("GetRoomMeta = %+v, %v", meta, err)
	}

	names, err := store.ListRoomNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range names {
		if n == room {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListRoomNames missing %q, got %v", room, names)
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
