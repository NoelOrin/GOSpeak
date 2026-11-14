package signal

import (
	"GOSpeak/internal/protocol"
	"GOSpeak/internal/ws"
	"GOSpeak/internal/logger"
)

// disconnectCleanup 记录断连后需要执行的 SFU 清理项。
type disconnectCleanup struct {
	room     string
	identity string
	deleted  bool
}

func (h *Hub) OnDisconnect(c ws.ClientMessenger) {
	type leaveEvent struct {
		room     string
		identity string
		id       string
	}
	var cleanups []disconnectCleanup
	var updatedRooms []string
	var speakingChanged []string
	var leaveEvents []leaveEvent
	// Leave personal room on disconnect
	if identity := clientIdentity(c); identity != "" {
		if h.fanout != nil {
			h.fanout.Leave("__user:"+identity, c.ID())
		}
	}
	h.mu.Lock()
	domainScope := h.clientDomains[c.ID()]
	delete(h.clientDomains, c.ID())
	// connSlots 是连接房间槽位的唯一事实来源：按槽位收集房间键，
	// 只清理该连接登记过的 Text/Voice 房间，不再全量遍历 h.rooms。
	roomKeys := make([]string, 0, 2)
	if slots := h.connSlots[c.ID()]; slots != nil {
		if slots.TextRoom != "" {
			roomKeys = append(roomKeys, slots.TextRoom)
		}
		if slots.VoiceRoom != "" && slots.VoiceRoom != slots.TextRoom {
			roomKeys = append(roomKeys, slots.VoiceRoom)
		}
	}
	delete(h.connSlots, c.ID())
	for _, roomName := range roomKeys {
		removal, ok := h.removeMemberLocked(roomName, c.ID())
		if !ok {
			continue
		}
		if removal.wasSpeaking {
			speakingChanged = append(speakingChanged, roomName)
		}
		leaveEvents = append(leaveEvents, leaveEvent{
			room:     roomName,
			identity: removal.identity,
			id:       c.ID(),
		})
		updatedRooms = append(updatedRooms, roomName)
		cleanups = append(cleanups, disconnectCleanup{roomName, removal.identity, removal.roomDeleted})
	}
	h.mu.Unlock()

	if h.fanout != nil {
		h.fanout.Leave(domainRoomKey(domainScope), c.ID())
	}

	// 先同步 KV，再发布事件：远端收到 member:left 时读到的一定是已更新的成员快照。
	for _, name := range updatedRooms {
		h.syncRoomToStore(name)
	}

	// publish after unlock: avoid holding hub mu across NATS/WebSocket I/O
	for _, e := range leaveEvents {
		domainUUID, logicalName := splitRoomKey(e.room)
		h.publishRoom(e.room, EventMemberLeft, protocol.MemberLeftPayload{
			Room:       logicalName,
			DomainUUID: domainUUID,
			Identity:   e.identity,
			ID:         e.id,
		})
	}

	for _, name := range speakingChanged {
		domainUUID, logicalName := splitRoomKey(name)
		h.broadcastActiveSpeakers(domainUUID, logicalName)
	}

	// SFU 清理走「断连宽限」：RemoveParticipant/DeleteRoom 是 HTTP/gRPC 调用，可能慢，
	// 且一次 WS 抖动不应立即把媒体 participant 移除。延迟到宽限期结束执行，
	// 期间同一 identity 重新加入同房间则取消（重新加入时 cancelPendingCleanup）。
	// 信令侧成员移除/member:left 广播仍在上面同步完成，保持房间列表一致。
	for _, c := range cleanups {
		h.scheduleDisconnectCleanup(c)
	}

	for _, name := range updatedRooms {
		h.mu.RLock()
		_, exists := h.rooms[name]
		h.mu.RUnlock()
		if !exists {
			// 房间已空被删除，广播房间列表更新（含 DB 持久化房间）
			domainUUID, _ := splitRoomKey(name)
			h.broadcastRoomList(domainUUID)
			continue
		}
		h.broadcastRoomUpdatedLocal(name)
	}

	logger.WithComponent("Signal").Infof("[Signal] client disconnected: %s", c.ID())
}

// runCleanupsInline 在后台 goroutine 中执行 SFU 清理，
// 用于 cleanup queue 不可用或入队失败时的兜底路径。
func (h *Hub) runCleanupsInline(cleanups []disconnectCleanup) {
	if len(cleanups) == 0 {
		return
	}
	go func(cleanups []disconnectCleanup) {
		for _, c := range cleanups {
			h.removeParticipantSafe(c.room, c.identity)
			if c.deleted {
				h.deleteRoomSafe(c.room)
			}
		}
	}(cleanups)
}
