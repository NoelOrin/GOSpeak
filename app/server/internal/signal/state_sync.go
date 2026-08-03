package signal

import (
	"context"
	"errors"
	"log"
	"time"

	"GOSpeak/internal/bus"

	"github.com/nats-io/nats.go"
)

// EventStateRoomChanged is an internal (non-WebSocket) event: peers should
// recompute room:updated / room:list:result from shared membership KV.
const EventStateRoomChanged = "state:room-changed"

// membershipStore abstracts JetStream KV (or test doubles) for cross-instance
// membership and stream maps. Nil store means local-only mode (phase-1).
type membershipStore interface {
	PutRoomMembers(ctx context.Context, snap bus.RoomMembersSnapshot) error
	GetRoomMembers(ctx context.Context, room string) (bus.RoomMembersSnapshot, error)
	DeleteRoomMembers(ctx context.Context, room string) error
	PutStream(ctx context.Context, stream, room, identity string) error
	GetStream(ctx context.Context, stream string) (room, identity string, err error)
	DeleteStream(ctx context.Context, stream string) error
	ListRoomNames(ctx context.Context) ([]string, error)
}

// stateNotifier publishes internal state-change events (no WebSocket deliver).
type stateNotifier interface {
	PublishInternal(ctx context.Context, event string, payload interface{}) error
}

// SetMembershipStore injects optional shared state store + instance id for KV writes.
func (h *Hub) SetMembershipStore(store membershipStore, instanceID string) {
	h.membershipStore = store
	h.instanceID = instanceID
}

// SetStateNotifier injects bus.PublishInternal for membership change fanout.
func (h *Hub) SetStateNotifier(n stateNotifier) {
	h.stateNotifier = n
}

// syncRoomToStore merges this instance's local membership into shared KV.
// Other instances' members (different InstanceID) are preserved so multi-instance
// room views stay complete. If this instance has no local members and no remote
// members remain, the room key is deleted.
// Best-effort: errors are logged, never fail the socket path.

const maxMembershipSyncAttempts = 3

// revisionedMembershipStore 是 NATS KV 后端提供的乐观锁扩展接口。
type revisionedMembershipStore interface {
	GetRoomMembersRev(ctx context.Context, room string) (bus.RoomMembersSnapshot, uint64, error)
	PutRoomMembersRev(ctx context.Context, snap bus.RoomMembersSnapshot, rev uint64) error
	DeleteRoomMembersRev(ctx context.Context, room string, rev uint64) error
}

func (h *Hub) syncRoomToStore(key string) {
	if h.membershipStore == nil || key == "" {
		return
	}

	now := time.Now().UnixMilli()
	local := h.localRoomMembers(key, now)
	if rs, ok := h.membershipStore.(revisionedMembershipStore); ok {
		h.syncRoomToStoreWithRevision(rs, key, now, local)
		return
	}
	h.syncRoomToStorePlain(key, now, local)
}

func (h *Hub) localRoomMembers(key string, now int64) []bus.MemberRecord {
	local := make([]bus.MemberRecord, 0)
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, ok := h.rooms[key]; ok {
		local = make([]bus.MemberRecord, 0, len(room.Members))
		for sid, m := range room.Members {
			if m == nil {
				continue
			}
			local = append(local, bus.MemberRecord{
				Room:        key,
				Identity:    m.Identity,
				SocketHint:  sid,
				InstanceID:  h.instanceID,
				Stream:      m.Stream,
				MicMuted:    room.MicMuted[m.Identity],
				Speaking:    room.Speaking[m.Identity],
				UpdatedAtMS: now,
			})
		}
	}
	return local
}

// syncRoomToStorePlain 保留非 NATS 后端（如 Redis）的无 CAS 合并行为。
func (h *Hub) syncRoomToStorePlain(key string, now int64, local []bus.MemberRecord) {
	merged := local
	if prev, err := h.membershipStore.GetRoomMembers(context.Background(), key); err == nil {
		merged = mergeRemoteMembers(prev.Members, local, h.instanceID)
	}

	if len(merged) == 0 {
		if err := h.membershipStore.DeleteRoomMembers(context.Background(), key); err != nil {
			log.Printf("[Signal] state store delete room %s: %v", key, err)
		}
		h.notifyRoomStateChanged(key)
		return
	}

	snap := bus.RoomMembersSnapshot{
		Room:      key,
		UpdatedAt: now,
		Members:   merged,
	}
	if err := h.membershipStore.PutRoomMembers(context.Background(), snap); err != nil {
		log.Printf("[Signal] state store put room %s: %v", key, err)
		return
	}
	h.notifyRoomStateChanged(key)
}

// syncRoomToStoreWithRevision 用 NATS KV revision 做乐观锁：读取当前 rev，CAS 写入；
// 冲突时重试，超过次数则放弃并记录，避免并发实例互相覆盖。
func (h *Hub) syncRoomToStoreWithRevision(rs revisionedMembershipStore, key string, now int64, local []bus.MemberRecord) {
	for attempt := 0; attempt < maxMembershipSyncAttempts; attempt++ {
		prev, rev, err := rs.GetRoomMembersRev(context.Background(), key)
		if err != nil && !errors.Is(err, nats.ErrKeyNotFound) {
			log.Printf("[Signal] state store get room %s: %v", key, err)
			return
		}

		merged := mergeRemoteMembers(prev.Members, local, h.instanceID)
		if len(merged) == 0 {
			if rev != 0 {
				if err := rs.DeleteRoomMembersRev(context.Background(), key, rev); err != nil {
					if isMembershipCASConflict(err) {
						continue
					}
					log.Printf("[Signal] state store delete room %s: %v", key, err)
					return
				}
			}
			h.notifyRoomStateChanged(key)
			return
		}

		snap := bus.RoomMembersSnapshot{
			Room:      key,
			UpdatedAt: now,
			Members:   merged,
		}
		if err := rs.PutRoomMembersRev(context.Background(), snap, rev); err != nil {
			if isMembershipCASConflict(err) {
				continue
			}
			log.Printf("[Signal] state store put room %s: %v", key, err)
			return
		}
		h.notifyRoomStateChanged(key)
		return
	}
	log.Printf("[Signal] state store sync room %s: revision conflicts exceeded %d attempts", key, maxMembershipSyncAttempts)
}

func mergeRemoteMembers(prev []bus.MemberRecord, local []bus.MemberRecord, instanceID string) []bus.MemberRecord {
	merged := make([]bus.MemberRecord, 0, len(local))
	for _, rec := range prev {
		if rec.Identity == "" {
			continue
		}
		if rec.InstanceID == "" || (instanceID != "" && rec.InstanceID == instanceID) {
			continue
		}
		collide := false
		for _, l := range local {
			if l.Identity == rec.Identity {
				collide = true
				break
			}
		}
		if collide {
			continue
		}
		merged = append(merged, rec)
	}
	merged = append(merged, local...)
	return merged
}

// isMembershipCASConflict 识别 NATS KV 的 revision 不匹配 / 已存在冲突。
func isMembershipCASConflict(err error) bool {
	if errors.Is(err, nats.ErrKeyExists) {
		return true
	}
	var apiErr *nats.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode == nats.JSErrCodeStreamWrongLastSequence
}

func (h *Hub) syncStreamPut(stream, key, identity string) {
	if h.membershipStore == nil || stream == "" {
		return
	}
	if err := h.membershipStore.PutStream(context.Background(), stream, key, identity); err != nil {
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

// notifyRoomStateChanged tells peer instances to refresh room list/updated from KV.
// Does not carry membership snapshots (those live in KV).
func (h *Hub) notifyRoomStateChanged(room string) {
	if h.stateNotifier == nil {
		return
	}
	payload := map[string]interface{}{
		"room": room,
		"ts":   time.Now().UnixMilli(),
	}
	if err := h.stateNotifier.PublishInternal(context.Background(), EventStateRoomChanged, payload); err != nil {
		log.Printf("[Signal] publish state room-changed %s: %v", room, err)
	}
}

// GetRoomMembersMerged returns local socket members for room, then fills any
// identities present only in KV (other instances). Local socket wins on conflict.
func (h *Hub) GetRoomMembersMerged(key string) []MemberInfo {
	local := h.GetRoomMembers(key)
	if h.membershipStore == nil {
		return local
	}
	snap, err := h.membershipStore.GetRoomMembers(context.Background(), key)
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

// roomInfoMerged builds RoomInfo for a room using local+DB metadata and merged members.
func (h *Hub) roomInfoMerged(key string) RoomInfo {
	h.mu.RLock()
	info := h.roomInfoLocked(key)
	h.mu.RUnlock()

	domainUUID, logicalName := splitRoomKey(key)

	if h.roomStore != nil {
		if dbRoom, err := h.roomStore.GetByDomainAndName(domainUUID, logicalName); err == nil && dbRoom != nil {
			info.ID = dbRoom.ID
			info.UUID = dbRoom.UUID
			info.Name = dbRoom.Name
			info.DomainUUID = dbRoom.DomainUUID
			info.HasPassword = dbRoom.Password != ""
			info.Description = dbRoom.Description
			info.Limit = dbRoom.Limit
			info.AudioOnly = dbRoom.AudioOnly
			info.AllowAudience = dbRoom.AllowAudience
			if info.CreatedAt == 0 {
				info.CreatedAt = dbRoom.CreatedAt.UnixMilli()
			}
		}
	}
	if info.Name == "" {
		info.Name = logicalName
	}
	if h.membershipStore != nil {
		merged := h.GetRoomMembersMerged(key)
		info.Members = merged
		info.Count = len(merged)
	}
	return info
}

// broadcastRoomUpdatedLocal pushes room:updated to local sockets using merged view.
func (h *Hub) broadcastRoomUpdatedLocal(key string) {
	if h.fanout == nil || key == "" {
		return
	}
	info := h.roomInfoMerged(key)
	domainUUID, _ := splitRoomKey(key)
	h.fanout.BroadcastToRoom(domainRoomKey(domainUUID), EventRoomUpdated, info)
}

// ApplyRemoteRoomState refreshes local WebSocket clients after peer membership change.
func (h *Hub) ApplyRemoteRoomState(room string) {
	if room != "" {
		h.broadcastRoomUpdatedLocal(room)
	}
	// Always refresh list so room counts/new remote-only rooms appear.
	if room != "" {
		domainUUID, _ := splitRoomKey(room)
		h.broadcastRoomList(domainUUID)
		return
	}
	h.broadcastRoomListKnownDomains()
}
