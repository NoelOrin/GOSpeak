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

const (
	// membershipLeaseDuration 是成员记录 lease 时长；实例心跳每 30s 续期，
	// 崩溃/分区后其他实例在 lease 到期即过滤该成员。
	membershipLeaseDuration  = 2 * time.Minute
	membershipHeartbeatEvery = 30 * time.Second
	membershipKVTimeout      = 2 * time.Second
)

// kvTimeoutCtx 为共享状态读写提供统一超时，避免后端卡住阻塞信令 goroutine。
func kvTimeoutCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), membershipKVTimeout)
}

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

// StartMembershipHeartbeat 周期续期本实例房间成员的 lease，避免在线但静默的成员
// 被其他实例当作崩溃成员过滤；崩溃/分区时 lease 自然过期，幽灵成员不再无限残留。
func (h *Hub) StartMembershipHeartbeat() {
	if h.membershipStore == nil {
		return
	}
	h.mu.Lock()
	if h.membershipHeartbeatStop != nil {
		h.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	h.membershipHeartbeatStop = stop
	h.mu.Unlock()

	go func() {
		ticker := time.NewTicker(membershipHeartbeatEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				h.syncAllLocalRoomsToStore()
			}
		}
	}()
}

// StopMembershipHeartbeat 停止成员 lease 心跳 goroutine。
func (h *Hub) StopMembershipHeartbeat() {
	h.mu.Lock()
	stop := h.membershipHeartbeatStop
	h.membershipHeartbeatStop = nil
	h.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (h *Hub) syncAllLocalRoomsToStore() {
	if h.membershipStore == nil {
		return
	}
	h.mu.RLock()
	keys := make([]string, 0, len(h.rooms))
	for key, room := range h.rooms {
		if len(room.Members) > 0 {
			keys = append(keys, key)
		}
	}
	h.mu.RUnlock()
	for _, key := range keys {
		h.syncRoomToStore(key)
	}
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
				ExpiresAtMS: now + membershipLeaseDuration.Milliseconds(),
			})
		}
	}
	return local
}

// syncRoomToStorePlain 保留非 NATS 后端（如 Redis）的无 CAS 合并行为。
func (h *Hub) syncRoomToStorePlain(key string, now int64, local []bus.MemberRecord) {
	merged := local
	ctx, cancel := kvTimeoutCtx()
	prev, err := h.membershipStore.GetRoomMembers(ctx, key)
	cancel()
	switch {
	case err == nil:
		merged = mergeRemoteMembers(prev.Members, local, h.instanceID, now)
	case bus.IsMembershipNotFound(err):
		// 首次写入：允许继续
	default:
		// 读失败必须中止，避免把远程成员快照覆盖/删除
		log.Printf("[Signal] state store get room %s: %v; skip merge to avoid clobber", key, err)
		return
	}

	if len(merged) == 0 {
		ctx, cancel = kvTimeoutCtx()
		err = h.membershipStore.DeleteRoomMembers(ctx, key)
		cancel()
		if err != nil {
			log.Printf("[Signal] state store delete room %s: %v", key, err)
		}
		h.deleteRoomMeta(key)
		h.notifyRoomStateChanged(key)
		return
	}

	snap := bus.RoomMembersSnapshot{
		Room:      key,
		UpdatedAt: now,
		Members:   merged,
	}
	ctx, cancel = kvTimeoutCtx()
	err = h.membershipStore.PutRoomMembers(ctx, snap)
	cancel()
	if err != nil {
		log.Printf("[Signal] state store put room %s: %v", key, err)
		return
	}
	h.notifyRoomStateChanged(key)
}

// syncRoomToStoreWithRevision 用 NATS KV revision 做乐观锁：读取当前 rev，CAS 写入；
// 冲突时重试，超过次数则放弃并记录，避免并发实例互相覆盖。
func (h *Hub) syncRoomToStoreWithRevision(rs revisionedMembershipStore, key string, now int64, local []bus.MemberRecord) {
	for attempt := 0; attempt < maxMembershipSyncAttempts; attempt++ {
		ctx, cancel := kvTimeoutCtx()
		prev, rev, err := rs.GetRoomMembersRev(ctx, key)
		cancel()
		if err != nil && !errors.Is(err, nats.ErrKeyNotFound) && !bus.IsMembershipNotFound(err) {
			log.Printf("[Signal] state store get room %s: %v", key, err)
			return
		}

		merged := mergeRemoteMembers(prev.Members, local, h.instanceID, now)
		if len(merged) == 0 {
			if rev != 0 {
				ctx, cancel = kvTimeoutCtx()
				err = rs.DeleteRoomMembersRev(ctx, key, rev)
				cancel()
				if err != nil {
					if isMembershipCASConflict(err) {
						continue
					}
					log.Printf("[Signal] state store delete room %s: %v", key, err)
					return
				}
			}
			h.deleteRoomMeta(key)
			h.notifyRoomStateChanged(key)
			return
		}

		snap := bus.RoomMembersSnapshot{
			Room:      key,
			UpdatedAt: now,
			Members:   merged,
		}
		ctx, cancel = kvTimeoutCtx()
		err = rs.PutRoomMembersRev(ctx, snap, rev)
		cancel()
		if err != nil {
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

// mergeRemoteMembers 保留其他实例未过期的成员，丢弃本实例旧记录与冲突 identity。
// 过期记录（实例崩溃/分区且未续期）直接过滤，避免幽灵成员长期残留。
func mergeRemoteMembers(prev []bus.MemberRecord, local []bus.MemberRecord, instanceID string, now int64) []bus.MemberRecord {
	merged := make([]bus.MemberRecord, 0, len(local))
	for _, rec := range prev {
		if rec.Identity == "" {
			continue
		}
		if rec.InstanceID == "" || (instanceID != "" && rec.InstanceID == instanceID) {
			continue
		}
		if rec.ExpiresAtMS != 0 && rec.ExpiresAtMS < now {
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

var (
	errRoomLimitExceeded       = errors.New("room limit exceeded")
	errDuplicateRemoteIdentity = errors.New("duplicate identity on another instance")
	errMembershipConflict      = errors.New("membership registration conflict")
)

// registerRoomMembers 在共享 KV 上原子注册本实例成员快照（含新加入成员）。
// NATS 后端使用 revision CAS 重试；合并时若发现其他实例持有同一 identity
// （未过期），或合并后人数超过 limit，则拒绝注册，避免跨实例重复身份与超限。
// Redis 等无 CAS 后端尽力检查后写入。
func (h *Hub) registerRoomMembers(key string, local []bus.MemberRecord, limit int, checkIdentity string) error {
	if h.membershipStore == nil {
		return nil
	}
	if rs, ok := h.membershipStore.(revisionedMembershipStore); ok {
		now := time.Now().UnixMilli()
		for attempt := 0; attempt < maxMembershipSyncAttempts; attempt++ {
			ctx, cancel := kvTimeoutCtx()
			prev, rev, err := rs.GetRoomMembersRev(ctx, key)
			cancel()
			if err != nil && !errors.Is(err, nats.ErrKeyNotFound) && !bus.IsMembershipNotFound(err) {
				return err
			}
			if checkIdentity != "" {
				for _, rec := range prev.Members {
					if rec.Identity == checkIdentity && rec.InstanceID != "" && rec.InstanceID != h.instanceID {
						if rec.ExpiresAtMS == 0 || rec.ExpiresAtMS >= now {
							return errDuplicateRemoteIdentity
						}
					}
				}
			}
			merged := mergeRemoteMembers(prev.Members, local, h.instanceID, now)
			if limit > 0 && len(merged) > limit {
				return errRoomLimitExceeded
			}
			snap := bus.RoomMembersSnapshot{
				Room:      key,
				UpdatedAt: now,
				Members:   merged,
			}
			ctx, cancel = kvTimeoutCtx()
			err = rs.PutRoomMembersRev(ctx, snap, rev)
			cancel()
			if err == nil {
				h.notifyRoomStateChanged(key)
				return nil
			}
			if !isMembershipCASConflict(err) {
				return err
			}
		}
		return errMembershipConflict
	}

	now := time.Now().UnixMilli()
	ctx, cancel := kvTimeoutCtx()
	prev, err := h.membershipStore.GetRoomMembers(ctx, key)
	cancel()
	if err != nil && !bus.IsMembershipNotFound(err) {
		return err
	}
	if checkIdentity != "" {
		for _, rec := range prev.Members {
			if rec.Identity == checkIdentity && rec.InstanceID != "" && rec.InstanceID != h.instanceID {
				if rec.ExpiresAtMS == 0 || rec.ExpiresAtMS >= now {
					return errDuplicateRemoteIdentity
				}
			}
		}
	}
	merged := mergeRemoteMembers(prev.Members, local, h.instanceID, now)
	if limit > 0 && len(merged) > limit {
		return errRoomLimitExceeded
	}
	snap := bus.RoomMembersSnapshot{Room: key, UpdatedAt: now, Members: merged}
	ctx, cancel = kvTimeoutCtx()
	err = h.membershipStore.PutRoomMembers(ctx, snap)
	cancel()
	if err != nil {
		return err
	}
	h.notifyRoomStateChanged(key)
	return nil
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
	ctx, cancel := kvTimeoutCtx()
	err := h.membershipStore.PutStream(ctx, stream, key, identity)
	cancel()
	if err != nil {
		log.Printf("[Signal] state store put stream %s: %v", stream, err)
	}
}

func (h *Hub) syncStreamDelete(stream string) {
	if h.membershipStore == nil || stream == "" {
		return
	}
	ctx, cancel := kvTimeoutCtx()
	err := h.membershipStore.DeleteStream(ctx, stream)
	cancel()
	if err != nil {
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
	ctx, cancel := kvTimeoutCtx()
	err := h.stateNotifier.PublishInternal(ctx, EventStateRoomChanged, payload)
	cancel()
	if err != nil {
		log.Printf("[Signal] publish state room-changed %s: %v", room, err)
	}
}

// GetRoomMembersMerged returns local socket members for room, then fills any
// identities present only in KV (other instances). Local socket wins on conflict.
func (h *Hub) GetRoomMembersMerged(key string) []MemberInfo {
	if h.membershipStore == nil {
		return h.GetRoomMembers(key)
	}
	ctx, cancel := kvTimeoutCtx()
	snap, err := h.membershipStore.GetRoomMembers(ctx, key)
	cancel()
	if err != nil {
		return h.GetRoomMembers(key)
	}
	return h.mergeMemberSnapshot(key, snap)
}

// mergeMemberSnapshot 合并本地成员与一份 KV 成员快照（用于批量读取路径）。
func (h *Hub) mergeMemberSnapshot(key string, snap bus.RoomMembersSnapshot) []MemberInfo {
	local := h.GetRoomMembers(key)
	now := time.Now().UnixMilli()
	seen := make(map[string]struct{}, len(local))
	for _, m := range local {
		seen[m.Identity] = struct{}{}
	}
	out := append([]MemberInfo(nil), local...)
	for _, rec := range snap.Members {
		if rec.Identity == "" {
			continue
		}
		if rec.ExpiresAtMS != 0 && rec.ExpiresAtMS < now {
			continue
		}
		if _, ok := seen[rec.Identity]; ok {
			continue
		}
		out = append(out, MemberInfo{
			Identity:   rec.Identity,
			Name:       rec.Identity,
			Stream:     rec.Stream,
			IsMicMuted: rec.MicMuted,
		})
		seen[rec.Identity] = struct{}{}
	}
	return out
}

// bulkRoomMembersReader 是 membershipStore 可选实现的批量读取扩展，
// Redis 后端用 MGet 消除房间列表的 N+1 查询。
type bulkRoomMembersReader interface {
	GetRoomMembersBatch(ctx context.Context, rooms []string) (map[string]bus.RoomMembersSnapshot, error)
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
	// 远端临时房间元数据兜底（DB 无记录时）
	if meta, err := h.getRoomMeta(key); err == nil && meta.Name != "" {
		if info.Name == "" {
			info.Name = meta.Name
		}
		if info.DomainUUID == "" {
			info.DomainUUID = meta.DomainUUID
		}
		if info.Description == "" {
			info.Description = meta.Description
		}
		if info.Limit == 0 {
			info.Limit = meta.Limit
		}
		if info.Type == "" {
			info.Type = meta.Type
		}
		info.HasPassword = meta.Password != ""
		if info.CreatedAt == 0 {
			info.CreatedAt = meta.CreatedAtMS
		}
	}
	if info.Name == "" {
		info.Name = logicalName
	}
	if h.membershipStore != nil {
		merged := h.GetRoomMembersMerged(key)
		info.Members = h.enrichMembers(merged)
		info.Count = len(merged)
	} else {
		// 无 KV 时在锁外补全成员资料/禁言，避免 roomInfoLocked 持锁查 DB。
		info.Members = h.enrichMembers(info.Members)
		info.Count = len(info.Members)
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
