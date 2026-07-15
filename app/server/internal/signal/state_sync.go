package signal

import (
	"context"
	"log"
	"time"

	"GOSpeak/internal/bus"
)

// membershipStore abstracts JetStream KV (or test doubles) for cross-instance
// membership and stream maps. Nil store means local-only mode (phase-1).
type membershipStore interface {
	PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error
	GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error)
	DeleteRoomMembers(ctx context.Context, room string) error
	PutStream(ctx context.Context, stream, room, identity string) error
	DeleteStream(ctx context.Context, stream string) error
}

// SetMembershipStore injects optional shared state store + instance id for KV writes.
func (h *Hub) SetMembershipStore(store membershipStore, instanceID string) {
	h.membershipStore = store
	h.instanceID = instanceID
}

// syncRoomToStore writes current local membership for roomName to KV.
// If the room no longer exists locally, deletes the KV key.
// Best-effort: errors are logged, never fail the socket path.
func (h *Hub) syncRoomToStore(roomName string) {
	if h.membershipStore == nil || roomName == "" {
		return
	}
	h.mu.RLock()
	room, ok := h.rooms[roomName]
	if !ok {
		h.mu.RUnlock()
		if err := h.membershipStore.DeleteRoomMembers(context.Background(), roomName); err != nil {
			log.Printf("[Signal] state store delete room %s: %v", roomName, err)
		}
		return
	}
	snap := bus.RoomMembersSnapshot{
		Room:      roomName,
		UpdatedAt: time.Now().UnixMilli(),
		Members:   make([]bus.MemberRecord, 0, len(room.Members)),
	}
	for sid, m := range room.Members {
		if m == nil {
			continue
		}
		snap.Members = append(snap.Members, bus.MemberRecord{
			Room:        roomName,
			Identity:    m.Identity,
			SocketHint:  sid,
			InstanceID:  h.instanceID,
			Stream:      m.Stream,
			MicMuted:    room.MicMuted[m.Identity],
			Speaking:    room.Speaking[m.Identity],
			UpdatedAtMS: snap.UpdatedAt,
		})
	}
	h.mu.RUnlock()

	if err := h.membershipStore.PutRoomMembers(context.Background(), snap); err != nil {
		log.Printf("[Signal] state store put room %s: %v", roomName, err)
	}
}

func (h *Hub) syncStreamPut(stream, room, identity string) {
	if h.membershipStore == nil || stream == "" {
		return
	}
	if err := h.membershipStore.PutStream(context.Background(), stream, room, identity); err != nil {
		log.Printf("[Signal] state store put stream %s: %v", stream, err)
	}
}

func (h *Hub) syncStreamDelete(stream string) {
	if h.membershipStore == nil || stream == "" {
		return
	}
	if err := h.membershipStore.DeleteStream(context.Background(), stream); err != nil {
		log.Printf("[Signal] state store delete stream %s: %v", stream, err)
	}
}

// GetRoomMembersMerged returns local socket members for room, then fills any
// identities present only in KV (other instances). Local socket wins on conflict.
func (h *Hub) GetRoomMembersMerged(roomName string) []MemberInfo {
	local := h.GetRoomMembers(roomName)
	if h.membershipStore == nil {
		return local
	}
	snap, err := h.membershipStore.GetRoomMembers(context.Background(), roomName)
	if err != nil {
		return local
	}
	seen := make(map[string]struct{}, len(local))
	for _, m := range local {
		seen[m.Identity] = struct{}{}
	}
	out := append([]MemberInfo(nil), local...)
	for _, rec := range snap.Members {
		if rec.Identity == "" {
			continue
		}
		if _, ok := seen[rec.Identity]; ok {
			continue
		}
		out = append(out, MemberInfo{
			Identity: rec.Identity,
			Name:     rec.Identity,
			Stream:   rec.Stream,
		})
		seen[rec.Identity] = struct{}{}
	}
	return out
}
