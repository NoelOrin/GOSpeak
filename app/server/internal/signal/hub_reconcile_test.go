package signal

import (
	"errors"
	"testing"
	"time"

	"GOSpeak/internal/sfu"
)

var graceMS = emptyRoomGracePeriod.Milliseconds()

func reconcileHub(provider *guardProvider, store membershipStore) *Hub {
	h := newTestHub()
	h.SetSFU(provider)
	h.SetMembershipStore(store, "local-instance")
	return h
}

func TestReconcileEmptyRooms_DeletesOrphanAfterGracePeriod(t *testing.T) {
	provider := &guardProvider{rooms: []sfu.RoomSummary{{Name: "orphan", MemberCount: 0}}}
	h := reconcileHub(provider, nil)

	h.reconcileEmptyRooms(func() int64 { return 0 })
	if len(provider.deleted) != 0 {
		t.Fatalf("first observation must not delete, got %v", provider.deleted)
	}

	h.reconcileEmptyRooms(func() int64 { return graceMS + 1 })
	if len(provider.deleted) != 1 || provider.deleted[0] != "orphan" {
		t.Fatalf("orphan room must be deleted after grace period, got %v", provider.deleted)
	}
}

func TestReconcileEmptyRooms_KeepsOccupiedRoom(t *testing.T) {
	provider := &guardProvider{rooms: []sfu.RoomSummary{{Name: "lobby", MemberCount: 2}}}
	h := reconcileHub(provider, nil)

	h.reconcileEmptyRooms(func() int64 { return 0 })
	h.reconcileEmptyRooms(func() int64 { return graceMS + 1 })

	if len(provider.deleted) != 0 {
		t.Fatalf("occupied room must never be deleted, got %v", provider.deleted)
	}
	if len(h.emptyRoomSeen) != 0 {
		t.Fatalf("occupied room must not be tracked, got %v", h.emptyRoomSeen)
	}
}

func TestReconcileEmptyRooms_ResetsTrackingWhenRoomRefills(t *testing.T) {
	provider := &guardProvider{rooms: []sfu.RoomSummary{{Name: "lobby", MemberCount: 0}}}
	h := reconcileHub(provider, nil)

	h.reconcileEmptyRooms(func() int64 { return 0 })
	provider.rooms = []sfu.RoomSummary{{Name: "lobby", MemberCount: 1}}
	h.reconcileEmptyRooms(func() int64 { return graceMS + 1 })
	provider.rooms = []sfu.RoomSummary{{Name: "lobby", MemberCount: 0}}
	h.reconcileEmptyRooms(func() int64 { return graceMS + 2 })
	if len(provider.deleted) != 0 {
		t.Fatalf("refill must reset the grace window, got premature delete %v", provider.deleted)
	}

	h.reconcileEmptyRooms(func() int64 { return 2*graceMS + 3 })
	if len(provider.deleted) != 1 {
		t.Fatalf("room must be deleted one full grace period after becoming empty again, got %v", provider.deleted)
	}
}

func TestReconcileEmptyRooms_SkipsRoomWithLiveSharedMembers(t *testing.T) {
	store := &guardStore{memStateStore: newMemStateStore()}
	now := time.Now().UnixMilli()
	putGuardSnapshot(t, store, "lobby", remoteMember("lobby", "remote-instance", now+60000))
	provider := &guardProvider{rooms: []sfu.RoomSummary{{Name: "lobby", MemberCount: 0}}}
	h := reconcileHub(provider, store)

	h.reconcileEmptyRooms(func() int64 { return 0 })
	h.reconcileEmptyRooms(func() int64 { return graceMS + 1 })

	if len(provider.deleted) != 0 {
		t.Fatalf("shared members must block reconcile delete, got %v", provider.deleted)
	}
}

func TestReconcileEmptyRooms_NoDeleteWhenListRoomsFails(t *testing.T) {
	provider := &guardProvider{listErr: errors.New("sfu down")}
	h := reconcileHub(provider, nil)

	h.reconcileEmptyRooms(func() int64 { return 0 })

	if len(provider.deleted) != 0 {
		t.Fatalf("ListRooms failure must not delete anything, got %v", provider.deleted)
	}
}

func TestReconcileEmptyRooms_SkipsWhenCapabilityMissing(t *testing.T) {
	caps := sfu.CapabilitiesFor("livekit")
	caps.ListRooms = false
	caps.ListLevel = sfu.EnforcementNone
	caps.DeleteRoom = false
	caps.DeleteLevel = sfu.EnforcementNone
	provider := &guardProvider{caps: &caps}
	h := reconcileHub(provider, nil)

	h.reconcileEmptyRooms(func() int64 { return 0 })

	if provider.listCalls != 0 {
		t.Fatalf("reconcile must skip when ListRooms capability is absent, got %d calls", provider.listCalls)
	}
}

func TestStartEmptyRoomReconciler_RunsAndStops(t *testing.T) {
	provider := &guardProvider{rooms: []sfu.RoomSummary{{Name: "orphan", MemberCount: 0}}}
	h := reconcileHub(provider, nil)

	stop := h.StartEmptyRoomReconciler(10 * time.Millisecond)
	time.Sleep(35 * time.Millisecond)
	stop()

	if provider.listCalls == 0 {
		t.Fatal("expected reconciler to tick")
	}
	after := provider.listCalls
	time.Sleep(35 * time.Millisecond)
	if provider.listCalls != after {
		t.Fatal("reconciler kept ticking after stop")
	}
}

func TestStartEmptyRoomReconciler_Idempotent(t *testing.T) {
	provider := &guardProvider{}
	h := reconcileHub(provider, nil)

	stop1 := h.StartEmptyRoomReconciler(10 * time.Millisecond)
	stop2 := h.StartEmptyRoomReconciler(10 * time.Millisecond)
	stop1()
	stop2()
}
