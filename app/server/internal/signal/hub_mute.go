package signal

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"errors"
	"log"
)

// BroadcastMute 广播禁言事件到所有客户端。
// 禁言策略始终以信令/业务层为准（始终 soft 可达）；若当前 SFU 支持 ServerMute，
// 再尽力对在线房间做媒体 hard/degraded mute，并在事件里标注 enforcement。
func (h *Hub) BroadcastMute(userID uint, info *MuteInfo) {
	ttlSeconds := 0
	if info != nil {
		if info.Permanent {
			ttlSeconds = sfu.PermanentMuteTTLSeconds
		} else if info.Duration > 0 {
			ttlSeconds = int(info.Duration)
		}
	}
	identity := h.identityForUserID(userID)
	var rooms []string
	if identity != "" {
		rooms = h.roomsForIdentity(identity)
	}
	enforcement := h.enforceUserMediaMuteInRooms(identity, rooms, true, ttlSeconds)
	data := map[string]interface{}{
		"user_id":     userID,
		"permanent":   info.Permanent,
		"reason":      info.Reason,
		"enforcement": enforcement,
	}
	if info != nil {
		if !info.Permanent && info.ExpiresAt != "" {
			data["expires_at"] = info.ExpiresAt
		}
		if info.Duration > 0 {
			data["duration"] = info.Duration
		}
	}
	h.publishNamespace(EventUserMuted, data)
	if identity != "" {
		h.publishMemberMuteEventInRooms(identity, true, rooms)
	}
}

// BroadcastUnmute 广播取消禁言事件到所有客户端。
func (h *Hub) BroadcastUnmute(userID uint) {
	identity := h.identityForUserID(userID)
	var rooms []string
	if identity != "" {
		rooms = h.roomsForIdentity(identity)
	}
	enforcement := h.enforceUserMediaMuteInRooms(identity, rooms, false, 0)
	h.publishNamespace(EventUserUnmuted, map[string]interface{}{
		"user_id":     userID,
		"enforcement": enforcement,
	})
	if identity != "" {
		h.publishMemberMuteEventInRooms(identity, false, rooms)
	}
}

// enforceUserMediaMute 在支持 ServerMute 的 provider 上，对 userID 当前所在房间做媒体层 mute/unmute。
// 返回 hard|degraded|soft。多房间必须全部成功才返回非 soft。
func (h *Hub) enforceUserMediaMute(userID uint, muted bool, ttlSeconds int) string {
	identity := h.identityForUserID(userID)
	return h.enforceUserMediaMuteByIdentity(identity, muted, ttlSeconds)
}

// enforceUserMediaMuteByIdentity 是 enforceUserMediaMute 的 identity 版本，
// 供 BroadcastMute/BroadcastUnmute 复用已解析的 identity，避免重复 DB 查询。
func (h *Hub) enforceUserMediaMuteByIdentity(identity string, muted bool, ttlSeconds int) string {
	return h.enforceUserMediaMuteInRooms(identity, h.roomsForIdentity(identity), muted, ttlSeconds)
}

// enforceUserMediaMuteInRooms 对已收集的房间列表做媒体层 mute/unmute。
// rooms 由调用方一次性计算，避免禁言事件与媒体强制各自触发一轮跨实例扫描。
func (h *Hub) enforceUserMediaMuteInRooms(identity string, rooms []string, muted bool, ttlSeconds int) string {
	if h.sfuProvider == nil {
		return sfu.EnforcementSoft
	}
	caps := h.sfuProvider.Capabilities()
	if !sfu.LevelEnabled(caps.MuteLevel) {
		return sfu.EnforcementSoft
	}

	if identity == "" {
		log.Printf("[Signal] mute media skip: cannot resolve identity")
		return sfu.EnforcementSoft
	}
	if len(rooms) == 0 {
		// No online media target: policy still soft-enforced; not a media hard success.
		return sfu.EnforcementSoft
	}

	success := 0
	for _, room := range rooms {
		var err error
		if tp, ok := h.sfuProvider.(sfu.TimedMuteProvider); ok {
			err = tp.MuteParticipantTimed(room, identity, "", muted, ttlSeconds)
		} else {
			err = h.sfuProvider.MuteParticipant(room, identity, "", muted)
		}
		if err != nil {
			if errors.Is(err, pkg.ErrSFUNotSupported) {
				if muted {
					log.Printf("[Signal] SFU mute unsupported, soft fallback room=%s identity=%s", room, identity)
				} else {
					log.Printf("[Signal] SFU unmute unsupported, soft unmute room=%s identity=%s", room, identity)
				}
			} else {
				log.Printf("[Signal] failed SFU mute (soft fallback) room=%s identity=%s err=%v", room, identity, err)
			}
			continue
		}
		success++
	}
	if success == 0 {
		return sfu.EnforcementSoft
	}
	if success < len(rooms) {
		// Partial multi-room success: do not overclaim full hard/degraded.
		log.Printf("[Signal] partial SFU mute success %d/%d identity=%s", success, len(rooms), identity)
		return sfu.EnforcementSoft
	}
	return sfu.EnforcementFromLevel(caps.MuteLevel)
}

// publishMemberMuteEventInRooms 将成员禁言状态事件定向发布到已收集的房间；
// 用户不在任何房间（离线）时退回全局广播，配合 join 自愈清理。
func (h *Hub) publishMemberMuteEventInRooms(identity string, muted bool, rooms []string) {
	event := EventMemberMuted
	if !muted {
		event = EventMemberUnmuted
	}
	payload := map[string]interface{}{
		"identity": identity,
		"muted":    muted,
	}
	if len(rooms) == 0 {
		h.publishNamespace(event, payload)
		return
	}
	for _, room := range rooms {
		h.publishRoom(room, event, payload)
	}
}

// roomsForIdentity 返回 identity 当前所在房间键（本地 rooms + 跨实例 membership KV）。
// 供禁言事件定向广播与媒体强制共用。
func (h *Hub) roomsForIdentity(identity string) []string {
	seen := make(map[string]struct{})
	rooms := make([]string, 0, 2)
	h.mu.RLock()
	for roomName, room := range h.rooms {
		if roomLookupIdentity(room, identity) != nil {
			seen[roomName] = struct{}{}
			rooms = append(rooms, roomName)
		}
	}
	h.mu.RUnlock()
	if h.membershipStore != nil {
		kvCtx, kvCancel := kvTimeoutCtx()
		names, err := h.membershipStore.ListRoomNames(kvCtx)
		kvCancel()
		if err == nil {
			for _, roomName := range names {
				if roomName == "" {
					continue
				}
				if _, ok := seen[roomName]; ok {
					continue
				}
				kvCtx2, kvCancel2 := kvTimeoutCtx()
				snap, err := h.membershipStore.GetRoomMembers(kvCtx2, roomName)
				kvCancel2()
				if err != nil {
					continue
				}
				for _, m := range snap.Members {
					if m.Identity == identity {
						seen[roomName] = struct{}{}
						rooms = append(rooms, roomName)
						break
					}
				}
			}
		}
	}
	return rooms
}

// identityForUserID resolves username from userStore by numeric user id.

// identityForUserID resolves username from userStore by numeric user id.
func (h *Hub) identityForUserID(userID uint) string {
	if h.userStore == nil {
		return ""
	}
	user, err := h.userStore.GetByID(userID)
	if err != nil || user == nil {
		log.Printf("[Signal] identityForUserID failed userID=%d err=%v", userID, err)
		return ""
	}
	return user.Name
}

// ─── 内部辅助 ───

// memberSnapshotLocked 仅拷贝房间成员快照与本地麦克风状态，不做任何 IO。
// 调用方须持有 h.mu（读或写）；用户资料与禁言状态由 enrichMembers 在锁外补齐。
