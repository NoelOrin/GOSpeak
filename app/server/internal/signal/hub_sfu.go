package signal

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/protocol"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/ws"
	"GOSpeak/internal/logger"
	"errors"
	"time"
)

// removeParticipantSafe 从 SFU 移除 participant。
// 信令层踢人始终先生效；这里只负责媒体层 enforcement。
// 返回 hard|degraded|soft，供事件 payload 透出。
func (h *Hub) removeParticipantSafe(room, identity string) string {
	if h.sfuProvider == nil || identity == "" {
		return sfu.EnforcementSoft
	}
	caps := h.sfuProvider.Capabilities()
	if !sfu.LevelEnabled(caps.KickLevel) {
		return sfu.EnforcementSoft
	}
	if err := h.sfuProvider.RemoveParticipant(room, identity); err != nil {
		if errors.Is(err, pkg.ErrSFUNotSupported) {
			logger.WithComponent("Signal").Infof("[Signal] SFU kick unsupported, soft fallback room=%s identity=%s", room, identity)
			return sfu.EnforcementSoft
		}
		logger.WithComponent("Signal").Infof("[Signal] failed to remove participant from SFU (soft fallback): %v", err)
		return sfu.EnforcementSoft
	}
	return sfu.EnforcementFromLevel(caps.KickLevel)
}

// roomHasLiveRemoteMember 判断共享成员快照里是否仍有其他实例的存活成员。
// KV 读失败返回 (true, err)，调用方须 fail-closed：宁可留孤儿房，不可删活人房。
func (h *Hub) roomHasLiveRemoteMember(room string) (bool, error) {
	ctx, cancel := kvTimeoutCtx()
	defer cancel()
	snap, err := h.membershipStore.GetRoomMembers(ctx, room)
	if err != nil {
		if bus.IsMembershipNotFound(err) {
			return false, nil
		}
		return true, err
	}
	now := time.Now().UnixMilli()
	for _, m := range snap.Members {
		if m.Identity == "" || m.InstanceID == h.instanceID {
			continue
		}
		if m.ExpiresAtMS != 0 && m.ExpiresAtMS < now {
			continue
		}
		return true, nil
	}
	return false, nil
}

// deleteRoomSafe 删除 SFU room，仅在共享成员里没有其他实例存活成员时执行。
// provider 不支持时静默降级。
func (h *Hub) deleteRoomSafe(room string) {
	if h.membershipStore != nil {
		hasRemote, err := h.roomHasLiveRemoteMember(room)
		if err != nil {
			logger.WithComponent("Signal").Infof("[Signal] skip SFU room delete %s: membership read failed: %v", room, err)
			return
		}
		if hasRemote {
			logger.WithComponent("Signal").Infof("[Signal] skip SFU room delete %s: live remote members", room)
			return
		}
	}
	h.forceDeleteRoom(room)
}

// forceDeleteRoom 无条件删除 SFU room，用于 Domain/业务房删除与 SFU 热切换。
func (h *Hub) forceDeleteRoom(room string) {
	if h.sfuProvider == nil {
		return
	}
	if err := h.sfuProvider.DeleteRoom(room); err != nil {
		if errors.Is(err, pkg.ErrSFUNotSupported) {
			return
		}
		logger.WithComponent("Signal").Infof("[Signal] failed to delete SFU room: %v", err)
	}
}

// CleanupParticipant is used by async job consumers for SFU-side cleanup.
// identity empty + deleteRoom cleans the SFU room only.

// CleanupParticipant is used by async job consumers for SFU-side cleanup.
// identity empty + deleteRoom cleans the SFU room only.
func (h *Hub) CleanupParticipant(room, identity string, deleteRoom bool) {
	if identity != "" {
		h.removeParticipantSafe(room, identity)
	}
	if deleteRoom {
		h.deleteRoomSafe(room)
	}
}

// ForceSFUProviderSwitch SFU 热切换：广播事件 → 清信令房间 → 尽力清理 SFU 侧。
// 客户端收到后强制断连并刷新页面。对端实例通过 HandleRemoteEvent 执行同等清理。

// ForceSFUProviderSwitch SFU 热切换：广播事件 → 清信令房间 → 尽力清理 SFU 侧。
// 客户端收到后强制断连并刷新页面。对端实例通过 HandleRemoteEvent 执行同等清理。
func (h *Hub) ForceSFUProviderSwitch(provider string) {
	h.publishNamespace(EventSFUProviderChanged, protocol.SFUProviderChangedPayload{
		Provider: provider,
	})
	h.clearLocalRoomsForSFUSwitch()
	logger.WithComponent("Signal").Infof("[Signal] SFU provider switched to %s, forced all sessions offline", provider)
}

// OnDomainDelete 清理指定 Domain 下所有信令房间（房间键前缀匹配）。
// - 断开所有在线成员连接
// - 删除本地 rooms map 中的条目
// - 调用 SFU Provider 清理实际房间

// clearLocalRoomsForSFUSwitch 清理本机信令房间与 stream 视图（不重复广播 provider-changed）。
func (h *Hub) clearLocalRoomsForSFUSwitch() {
	type cleanupItem struct {
		room     string
		identity string
	}
	var cleanups []cleanupItem
	var roomsToDelete []string
	var streamsToDelete []string

	h.mu.Lock()
	for roomName, room := range h.rooms {
		for sid, m := range room.Members {
			identity := m.Identity
			stream := m.Stream
			cleanups = append(cleanups, cleanupItem{room: roomName, identity: identity})
			if stream != "" {
				streamsToDelete = append(streamsToDelete, stream)
			}
			roomDelMember(room, sid)
			delete(room.Speaking, identity)
			delete(room.MicMuted, identity)
			if h.fanout != nil {
				h.fanout.ForEach(roomName, func(conn ws.ClientMessenger) bool {
					if conn != nil && conn.ID() == sid {
						h.fanout.Leave(roomName, conn.ID())
					}
					return true
				})
			}
		}
		roomsToDelete = append(roomsToDelete, roomName)
	}
	for _, roomName := range roomsToDelete {
		delete(h.rooms, roomName)
	}
	h.mu.Unlock()

	_ = streamsToDelete
	for _, roomName := range roomsToDelete {
		h.syncRoomToStore(roomName)
	}
	for _, c := range cleanups {
		h.removeParticipantSafe(c.room, c.identity)
	}
	for _, roomName := range roomsToDelete {
		h.forceDeleteRoom(roomName)
	}
	h.broadcastRoomListKnownDomains()
}

// BroadcastMute 广播禁言事件到所有客户端。
// 禁言策略始终以信令/业务层为准（始终 soft 可达）；若当前 SFU 支持 ServerMute，
// 再尽力对在线房间做媒体 hard/degraded mute，并在事件里标注 enforcement。
