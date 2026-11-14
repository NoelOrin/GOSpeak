package signal

import (
	"GOSpeak/internal/protocol"
	"GOSpeak/internal/ws"
)

// memberRemoval 记录一次成员移除的锁内结果，供锁外收尾使用。
type memberRemoval struct {
	key         string
	socketID    string
	identity    string
	wasSpeaking bool
	roomDeleted bool
}

// removeMemberLocked 在 h.mu 写锁内执行成员移除的公共序列：
// 按 socket 定位成员 → 摘除成员与发言/静音态 → 空房即删。
// ok=false 表示房间或成员不存在，无需收尾。调用方须持有 h.mu（写）。
func (h *Hub) removeMemberLocked(key, socketID string) (memberRemoval, bool) {
	room, ok := h.rooms[key]
	if !ok {
		return memberRemoval{}, false
	}
	member, ok := room.Members[socketID]
	if !ok || member == nil {
		return memberRemoval{}, false
	}
	r := memberRemoval{
		key:         key,
		socketID:    socketID,
		identity:    member.Identity,
		wasSpeaking: room.Speaking[member.Identity],
	}
	roomDelMember(room, socketID)
	delete(room.Speaking, member.Identity)
	delete(room.MicMuted, member.Identity)
	r.roomDeleted = h.deleteRoomIfEmptyLocked(key)
	return r, true
}

// removeMemberByIdentityLocked 按 identity 定位成员后走同一移除序列（踢出路径）。
func (h *Hub) removeMemberByIdentityLocked(key, identity string) (memberRemoval, bool) {
	room, ok := h.rooms[key]
	if !ok {
		return memberRemoval{}, false
	}
	for sid, m := range room.Members {
		if m != nil && m.Identity == identity {
			return h.removeMemberLocked(key, sid)
		}
	}
	return memberRemoval{}, false
}

// notifyKicked 广播 kicked 事件并直发被踢者连接（payload 带 targetIdentity，前端按 identity 过滤）。
func (h *Hub) notifyKicked(r memberRemoval, domainUUID, roomName, enforcement string, targetConn ws.ClientMessenger) {
	payload := protocol.RoomKickedPayload{
		Room:           roomName,
		DomainUUID:     domainUUID,
		TargetIdentity: r.identity,
		Enforcement:    enforcement,
	}
	h.publishRoom(r.key, EventRoomKicked, payload)
	if targetConn != nil {
		targetConn.Send(map[string]interface{}{"event": EventRoomKicked, "data": payload})
	}
}

// notifyMemberLeft 广播成员离开事件。
func (h *Hub) notifyMemberLeft(r memberRemoval, domainUUID, roomName, enforcement string) {
	h.publishRoom(r.key, EventMemberLeft, protocol.MemberLeftPayload{
		Room:        roomName,
		DomainUUID:  domainUUID,
		Identity:    r.identity,
		ID:          r.socketID,
		Enforcement: enforcement,
	})
}

// broadcastRoomAfterRemoval 收尾广播：空房删除则删 SFU 房间并广播房间列表，
// 否则广播单房间更新；发言态变化时（未删房）重置 active speakers 高亮。
func (h *Hub) broadcastRoomAfterRemoval(r memberRemoval, domainUUID, roomName string) {
	if r.roomDeleted {
		h.deleteRoomSafe(r.key)
		h.broadcastRoomList(domainUUID)
		return
	}
	h.broadcastRoomUpdatedLocal(r.key)
	if r.wasSpeaking {
		h.broadcastActiveSpeakers(domainUUID, roomName)
	}
}
