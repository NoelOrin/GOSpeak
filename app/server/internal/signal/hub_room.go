package signal

import (
	"GOSpeak/internal/pkg"
	"context"
	"time"
)

type Room struct {
	Name     string
	Password string
	Members  map[string]*MemberInfo // socketID -> MemberInfo
	// ByIdentity 身份索引，O(1) 查成员（本地 fallback 时为 O(1)，不再遍历 Members）。
	ByIdentity map[string]*MemberInfo // identity -> MemberInfo
	MicMuted   map[string]bool
	// Speaking 维护房间内各成员本地发言状态（仅 SRS/Cloudflare 等不 SFU 原生 active speaker 的 provider 上报）。
	// 与 MicMuted 类似，仅用于日志れ聚广播，不持久化。
	Speaking  map[string]bool
	CreatedAt time.Time
}

// roomSetMember writes both Members (by socketID) and ByIdentity index.
// Caller must hold h.mu write-lock.

// roomSetMember writes both Members (by socketID) and ByIdentity index.
// Caller must hold h.mu write-lock.
func roomSetMember(room *Room, sid, identity string, member *MemberInfo) {
	room.Members[sid] = member
	if identity != "" && room.ByIdentity != nil {
		room.ByIdentity[identity] = member
	}
}

// roomDelMember removes from both Members and ByIdentity. Caller must hold write-lock.

// roomDelMember removes from both Members and ByIdentity. Caller must hold write-lock.
func roomDelMember(room *Room, sid string) {
	if m, ok := room.Members[sid]; ok {
		if m.Identity != "" && room.ByIdentity != nil {
			delete(room.ByIdentity, m.Identity)
		}
	}
	delete(room.Members, sid)
}

// roomLookupIdentity O(1) identity lookup with index fallback.

// roomLookupIdentity O(1) identity lookup with index fallback.
func roomLookupIdentity(room *Room, identity string) *MemberInfo {
	if room.ByIdentity != nil {
		return room.ByIdentity[identity]
	}
	for _, m := range room.Members {
		if m.Identity == identity {
			return m
		}
	}
	return nil
}

// socketServer is replaced by ws.Broadcaster.

// eventBus is the narrow publish surface used by Hub for client fanout.
// *bus.NATSBus satisfies this without importing the bus package here.

// socketServer is replaced by ws.Broadcaster.

// eventBus is the narrow publish surface used by Hub for client fanout.
// *bus.NATSBus satisfies this without importing the bus package here.
type eventBus interface {
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
}

// cleanupPublisher enqueues SFU cleanup jobs (phase-3). Nil = inline goroutine.

// cleanupPublisher enqueues SFU cleanup jobs (phase-3). Nil = inline goroutine.
type cleanupPublisher interface {
	PublishSFUCleanup(ctx context.Context, room, identity string, deleteRoom bool) error
}

// roomKey generates a domain-scoped composite key for room map isolation.
// Platform-level rooms (no DomainUUID) use a "platform:" prefix for backward compatibility.

// roomKey generates a domain-scoped composite key for room map isolation.
// Platform-level rooms (no DomainUUID) use a "platform:" prefix for backward compatibility.
func roomKey(domainUUID, roomName string) string {
	return pkg.RoomKey(domainUUID, roomName)
}

// splitRoomKey reverses roomKey to extract domainUUID and roomName.

// splitRoomKey reverses roomKey to extract domainUUID and roomName.
func splitRoomKey(key string) (domainUUID, roomName string) {
	return pkg.SplitRoomKey(key)
}

func roomKeyMatchesDomain(key, domainUUID string) bool {
	if domainUUID == "" {
		return true
	}
	g, _ := splitRoomKey(key)
	return g == domainUUID
}

func domainRoomKey(domainUUID string) string {
	if domainUUID == "" {
		return "__platform"
	}
	return "__domain:" + domainUUID
}

func roomKeyIsPlatform(key string) bool {
	g, _ := splitRoomKey(key)
	return g == ""
}

type connRoomSlots struct {
	TextRoom  string
	VoiceRoom string
}

// clearConnRoomSlot removes the matching room slot and drops the entry when empty.

// clearConnRoomSlot removes the matching room slot and drops the entry when empty.
func (h *Hub) clearConnRoomSlot(socketID, key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	slots := h.connSlots[socketID]
	if slots == nil {
		return
	}
	if slots.TextRoom == key {
		slots.TextRoom = ""
	}
	if slots.VoiceRoom == key {
		slots.VoiceRoom = ""
	}
	if slots.TextRoom == "" && slots.VoiceRoom == "" {
		delete(h.connSlots, socketID)
	}
}
