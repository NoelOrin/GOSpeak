package signal

import (
	"context"
	"log"

	"GOSpeak/internal/ws"
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
	var deletedStreams []string
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
		room := h.rooms[roomName]
		if room == nil {
			continue
		}
		if member, ok := room.Members[c.ID()]; ok {
			identity := member.Identity
			stream := member.Stream
			wasSpeaking := room.Speaking[identity]
			roomDelMember(room, c.ID())
			delete(room.Speaking, identity)
			delete(room.MicMuted, identity)
			h.unregisterStreamLocked(roomName, identity, stream)
			if stream != "" {
				deletedStreams = append(deletedStreams, stream)
			}
			if wasSpeaking {
				speakingChanged = append(speakingChanged, roomName)
			}
			leaveEvents = append(leaveEvents, leaveEvent{
				room:     roomName,
				identity: identity,
				id:       c.ID(),
			})
			updatedRooms = append(updatedRooms, roomName)
			deleted := h.deleteRoomIfEmptyLocked(roomName)
			cleanups = append(cleanups, disconnectCleanup{roomName, identity, deleted})
		}
	}
	h.mu.Unlock()

	if h.fanout != nil {
		h.fanout.Leave(domainRoomKey(domainScope), c.ID())
	}

	// 先同步 KV，再发布事件：远端收到 member:left 时读到的一定是已更新的成员快照。
	for _, name := range updatedRooms {
		h.syncRoomToStore(name)
	}
	for _, stream := range deletedStreams {
		h.syncStreamDelete(stream)
	}

	// publish after unlock: avoid holding hub mu across NATS/WebSocket I/O
	for _, e := range leaveEvents {
		domainUUID, logicalName := splitRoomKey(e.room)
		h.publishRoom(e.room, EventMemberLeft, map[string]interface{}{
			"room":        logicalName,
			"domain_uuid": domainUUID,
			"identity":    e.identity,
			"id":          e.id,
		})
	}

	for _, name := range speakingChanged {
		domainUUID, logicalName := splitRoomKey(name)
		h.broadcastActiveSpeakers(domainUUID, logicalName)
	}

	// SFU 清理异步：RemoveParticipant/DeleteRoom 是 HTTP/gRPC 调用，可能慢。
	// 同步阻塞会拉长 OnDisconnect 持续时间，加剧连接 goroutine
	// 竞态（连接已 failed 时库 serveRead 状态错乱触发 gorilla panic）。
	// 丢后台 goroutine，handler 立即返回。
	if len(cleanups) > 0 {
		if h.cleanupPub != nil {
			for _, c := range cleanups {
				if err := h.cleanupPub.PublishSFUCleanup(context.Background(), c.room, c.identity, c.deleted); err != nil {
					log.Printf("[Signal] enqueue sfu cleanup: %v; fallback to inline goroutine", err)
					h.runCleanupsInline([]disconnectCleanup{c})
				}
			}
		} else {
			h.runCleanupsInline(cleanups)
		}
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

	log.Printf("[Signal] client disconnected: %s", c.ID())
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
			if h.participantCleanup != nil {
				h.participantCleanup.OnParticipantLeft(c.room, c.identity)
			}
		}
	}(cleanups)
}
