package bus

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*goredis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	return rdb, func() {
		_ = rdb.Close()
		mr.Close()
	}
}

func TestRedisStateStore_PutGetDelete(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	t.Cleanup(cleanup)

	store, err := OpenRedisStateStore(RedisStateStoreConfig{Client: rdb, Prefix: "gs_test"})
	if err != nil {
		t.Fatal(err)
	}
	if store.Backend() != "redis" {
		t.Fatalf("backend=%s", store.Backend())
	}

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
	if err != nil || len(got.Members) != 1 || got.Members[0].Identity != "alice" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	names, err := store.ListRoomNames(context.Background())
	if err != nil || len(names) != 1 || names[0] != "r1" {
		t.Fatalf("names=%v err=%v", names, err)
	}
	if err := store.DeleteRoomMembers(context.Background(), "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetRoomMembers(context.Background(), "r1"); err == nil {
		t.Fatal("expected miss")
	}
}

func TestRedisStateStore_Stream(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	t.Cleanup(cleanup)
	store, err := OpenRedisStateStore(RedisStateStoreConfig{Client: rdb, Prefix: "gs_test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutStream(context.Background(), "gs-a", "room-a", "bob"); err != nil {
		t.Fatal(err)
	}
	room, id, err := store.GetStream(context.Background(), "gs-a")
	if err != nil || room != "room-a" || id != "bob" {
		t.Fatalf("room=%s id=%s err=%v", room, id, err)
	}
	if err := store.DeleteStream(context.Background(), "gs-a"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveMembershipStore_AutoPrefersRedis(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	t.Cleanup(cleanup)

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

	store, backend, err := ResolveMembershipStore(ResolveMembershipConfig{
		Mode:   "auto",
		Prefix: "gs_auto",
		Redis:  rdb,
		NATS:   nc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if backend != "redis" || store == nil || store.Backend() != "redis" {
		t.Fatalf("backend=%s store=%v", backend, store)
	}
	_ = store.Close()
}

func TestResolveMembershipStore_AutoFallsToNATS(t *testing.T) {
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

	store, backend, err := ResolveMembershipStore(ResolveMembershipConfig{
		Mode:   "auto",
		Prefix: "gs_auto_nats",
		Redis:  nil,
		NATS:   nc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if backend != "nats" || store == nil {
		t.Fatalf("backend=%s", backend)
	}
	_ = store.Close()
}

func TestResolveMembershipStore_AutoNone(t *testing.T) {
	store, backend, err := ResolveMembershipStore(ResolveMembershipConfig{
		Mode:  "auto",
		Redis: nil,
		NATS:  nil,
	})
	if err != nil || store != nil || backend != "none" {
		t.Fatalf("store=%v backend=%s err=%v", store, backend, err)
	}
}

func TestResolveMembershipStore_ForcedRedisUnavailable(t *testing.T) {
	_, _, err := ResolveMembershipStore(ResolveMembershipConfig{
		Mode:  "redis",
		Redis: nil,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveMembershipStore_None(t *testing.T) {
	store, backend, err := ResolveMembershipStore(ResolveMembershipConfig{Mode: "none"})
	if err != nil || store != nil || backend != "none" {
		t.Fatalf("store=%v backend=%s err=%v", store, backend, err)
	}
}

func TestRedisStateStore_RoomMetaAndBatchRead(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	t.Cleanup(cleanup)
	store, err := OpenRedisStateStore(RedisStateStoreConfig{Client: rdb, Prefix: "gs_test"})
	if err != nil {
		t.Fatal(err)
	}

	for _, room := range []string{"r1", "r2"} {
		if err := store.PutRoomMembers(context.Background(), RoomMembersSnapshot{
			Room:    room,
			Members: []MemberRecord{{Identity: "alice", InstanceID: "i1"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.PutRoomMeta(context.Background(), "remote-only", RoomMeta{Name: "remote-only", Password: "hash"}); err != nil {
		t.Fatal(err)
	}

	names, err := store.ListRoomNames(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 room names (2 members + 1 meta), got %v", names)
	}

	got, err := store.GetRoomMembersBatch(context.Background(), []string{"r1", "r2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["r1"].Members[0].Identity != "alice" || got["r2"].Members[0].Identity != "alice" {
		t.Fatalf("batch read mismatch: %+v", got)
	}

	if err := store.DeleteRoomMeta(context.Background(), "remote-only"); err != nil {
		t.Fatal(err)
	}
	names, err = store.ListRoomNames(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 room names after meta delete, got %v", names)
	}
}
