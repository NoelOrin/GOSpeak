package signal

import (
	"context"
	"log"
	"time"

	"GOSpeak/internal/bus"
)

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
