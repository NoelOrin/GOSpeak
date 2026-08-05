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
	enforcement := h.enforceUserMediaMute(userID, true, ttlSeconds)
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
}

// BroadcastUnmute 广播取消禁言事件到所有客户端。

// BroadcastUnmute 广播取消禁言事件到所有客户端。
func (h *Hub) BroadcastUnmute(userID uint) {
	enforcement := h.enforceUserMediaMute(userID, false, 0)
	h.publishNamespace(EventUserUnmuted, map[string]interface{}{
		"user_id":     userID,
		"enforcement": enforcement,
	})
}

// enforceUserMediaMute 在支持 ServerMute 的 provider 上，对 userID 当前所在房间做媒体层 mute/unmute。
// 返回 hard|degraded|soft。多房间必须全部成功才返回非 soft。

// enforceUserMediaMute 在支持 ServerMute 的 provider 上，对 userID 当前所在房间做媒体层 mute/unmute。
// 返回 hard|degraded|soft。多房间必须全部成功才返回非 soft。
func (h *Hub) enforceUserMediaMute(userID uint, muted bool, ttlSeconds int) string {
	if h.sfuProvider == nil {
		return sfu.EnforcementSoft
	}
	caps := h.sfuProvider.Capabilities()
	if !sfu.LevelEnabled(caps.MuteLevel) {
		return sfu.EnforcementSoft
	}

	identity := h.identityForUserID(userID)
	if identity == "" {
		log.Printf("[Signal] mute media skip: cannot resolve identity for userID=%d", userID)
		return sfu.EnforcementSoft
	}

	type target struct {
		room     string // composite key (domainUUID:roomName)
		identity string
	}
	seen := make(map[string]struct{})
	targets := make([]target, 0)
	h.mu.RLock()
	for roomName, room := range h.rooms {
		for _, member := range room.Members {
			if member.Identity == identity {
				if _, ok := seen[roomName]; !ok {
					seen[roomName] = struct{}{}
					targets = append(targets, target{room: roomName, identity: identity})
				}
				break
			}
		}
	}
	h.mu.RUnlock()
	// Multi-instance: user may only be online on another process; still media-enforce via SFU APIs.
	if h.membershipStore != nil {
		kvCtx, kvCancel := kvTimeoutCtx()
		if names, err := h.membershipStore.ListRoomNames(kvCtx); err == nil {
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
						targets = append(targets, target{room: roomName, identity: identity})
						break
					}
				}
			}
		}
		kvCancel()
	}
	if len(targets) == 0 {
		// No online media target: policy still soft-enforced; not a media hard success.
		return sfu.EnforcementSoft
	}

	success := 0
	for _, t := range targets {
		var err error
		if tp, ok := h.sfuProvider.(sfu.TimedMuteProvider); ok {
			err = tp.MuteParticipantTimed(t.room, t.identity, "", muted, ttlSeconds)
		} else {
			err = h.sfuProvider.MuteParticipant(t.room, t.identity, "", muted)
		}
		if err != nil {
			if errors.Is(err, pkg.ErrSFUNotSupported) {
				log.Printf("[Signal] SFU mute unsupported, soft fallback room=%s identity=%s", t.room, t.identity)
			} else {
				log.Printf("[Signal] failed SFU mute (soft fallback) room=%s identity=%s err=%v", t.room, t.identity, err)
			}
			continue
		}
		success++
	}
	if success == 0 {
		return sfu.EnforcementSoft
	}
	if success < len(targets) {
		// Partial multi-room success: do not overclaim full hard/degraded.
		log.Printf("[Signal] partial SFU mute success %d/%d identity=%s", success, len(targets), identity)
		return sfu.EnforcementSoft
	}
	return sfu.EnforcementFromLevel(caps.MuteLevel)
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
