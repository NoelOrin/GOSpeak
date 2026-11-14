package signal

import (
	"GOSpeak/internal/ws"
	"GOSpeak/internal/logger"
	"strings"
)

// OnDomainDelete 清理指定 Domain 下所有信令房间（房间键前缀匹配）。
// - 断开所有在线成员连接
// - 删除本地 rooms map 中的条目
// - 调用 SFU Provider 清理实际房间
func (h *Hub) OnDomainDelete(domainUUID string) {
	if domainUUID == "" {
		return
	}
	prefix := domainUUID + ":"

	var sfuRooms []string
	var roomKeys []string
	var memberSIDs []string
	h.mu.Lock()
	for key, room := range h.rooms {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		// 收集 SFU 房间名（复合 key 与 token/媒体层一致）
		sfuRooms = append(sfuRooms, key)
		// 断开所有成员：锁内只收集 socket ID，fanout/Close 全部移到锁外，
		// 避免 h.mu -> fanout.mu 锁序与正常路径相反。
		for sid := range room.Members {
			delete(h.connSlots, sid)
			memberSIDs = append(memberSIDs, sid)
		}
		delete(h.rooms, key)
		roomKeys = append(roomKeys, key)
	}
	h.mu.Unlock()

	// 在锁外关闭成员连接，避免 Close 阻塞拖住所有信令。
	closed := make(map[string]struct{}, len(memberSIDs))
	for _, sid := range memberSIDs {
		client := h.fanout.GetClient(sid)
		h.fanout.Remove(sid)
		if client == nil {
			continue
		}
		if _, ok := closed[client.ID()]; ok {
			continue
		}
		closed[client.ID()] = struct{}{}
		client.Close()
	}

	// 清理 SFU 侧
	for _, room := range sfuRooms {
		h.forceDeleteRoom(room)
	}
	// 清理共享状态和 stream 映射
	for _, key := range roomKeys {
		if h.membershipStore != nil {
			kvCtx, kvCancel := kvTimeoutCtx()
			_ = h.membershipStore.DeleteRoomMembers(kvCtx, key)
			kvCancel()
			h.notifyRoomStateChanged(key)
		}
	}
}

// KickFromRoom 从指定房间移除 identity 对应的本地信令成员，并尽力触发 SFU 媒体移除。
func (h *Hub) KickFromRoom(domainUUID, room, targetIdentity string) {
	if targetIdentity == "" {
		return
	}
	key := roomKey(domainUUID, room)
	h.mu.Lock()
	removal, ok := h.removeMemberByIdentityLocked(key, targetIdentity)
	h.mu.Unlock()
	if !ok {
		return
	}

	h.blockRejoin(key, targetIdentity)
	h.clearConnRoomSlot(removal.socketID, key)

	var targetConn ws.ClientMessenger
	if h.fanout != nil {
		h.fanout.Leave(key, removal.socketID)
		targetConn = h.fanout.GetClient(removal.socketID)
	}

	enforcement := h.removeParticipantSafe(key, targetIdentity)

	h.notifyKicked(removal, domainUUID, room, enforcement, targetConn)
	h.notifyMemberLeft(removal, domainUUID, room, enforcement)

	if targetConn != nil {
		targetConn.Close()
	}

	h.syncRoomToStore(key)
	h.broadcastRoomAfterRemoval(removal, domainUUID, room)
}

// KickUserFromDomain 按用户 UUID 解析身份后，将该用户从 Domain 下全部在线房间踢出。
// 同时清理 Fanout、SFU 媒体流与共享成员状态。
func (h *Hub) KickUserFromDomain(domainUUID, userUUID string) {
	if domainUUID == "" || userUUID == "" {
		return
	}
	if h.userStore == nil {
		return
	}
	user, err := h.userStore.GetByUUID(userUUID)
	if err != nil || user == nil {
		logger.WithComponent("Signal").Infof("[Signal] domain kick resolve failed domain=%s user=%s err=%v", domainUUID, userUUID, err)
		return
	}
	h.kickIdentityFromDomain(domainUUID, user.Name)
}

func (h *Hub) kickIdentityFromDomain(domainUUID, identity string) {
	if domainUUID == "" || identity == "" {
		return
	}
	prefix := domainUUID + ":"
	var removals []memberRemoval
	h.mu.Lock()
	for key := range h.rooms {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if removal, ok := h.removeMemberByIdentityLocked(key, identity); ok {
			removals = append(removals, removal)
		}
	}
	h.mu.Unlock()
	if len(removals) == 0 {
		return
	}

	closed := make(map[string]struct{}, len(removals))
	for _, r := range removals {
		h.clearConnRoomSlot(r.socketID, r.key)
		if h.fanout != nil {
			h.fanout.Leave(r.key, r.socketID)
		}
		if client := h.fanout.GetClient(r.socketID); client != nil {
			closed[client.ID()] = struct{}{}
		}
	}

	for _, r := range removals {
		domain, roomName := splitRoomKey(r.key)
		enforcement := h.removeParticipantSafe(r.key, r.identity)
		targetConn := h.fanout.GetClient(r.socketID)
		h.notifyKicked(r, domain, roomName, enforcement, targetConn)
		h.notifyMemberLeft(r, domain, roomName, enforcement)
		h.syncRoomToStore(r.key)
		h.broadcastRoomAfterRemoval(r, domain, roomName)
	}

	for id := range closed {
		if client := h.fanout.GetClient(id); client != nil {
			client.Close()
		}
	}
}

// DeleteRoomByDomainName 删除指定 Domain 下的单个信令房间，并清理 SFU 与共享状态。
func (h *Hub) DeleteRoomByDomainName(domainUUID, room string) {
	key := roomKey(domainUUID, room)
	var closedClients []ws.ClientMessenger
	h.mu.Lock()
	roomState, ok := h.rooms[key]
	if !ok {
		h.mu.Unlock()
		return
	}
	for sid, member := range roomState.Members {
		if member == nil {
			continue
		}
		roomDelMember(roomState, sid)
		delete(h.connSlots, sid)
		if h.fanout != nil {
			h.fanout.Leave(key, sid)
			if client := h.fanout.GetClient(sid); client != nil {
				closedClients = append(closedClients, client)
			}
		}
	}
	delete(h.rooms, key)
	h.mu.Unlock()

	// 在锁外关闭成员连接，避免 Close 阻塞拖住所有信令。
	closed := make(map[string]struct{}, len(closedClients))
	for _, client := range closedClients {
		if _, ok := closed[client.ID()]; ok {
			continue
		}
		closed[client.ID()] = struct{}{}
		client.Close()
	}
	h.forceDeleteRoom(key)
	h.syncRoomToStore(key)
	h.broadcastRoomList(domainUUID)
}

// 当前仅对 sfu:provider-changed 做本机房间清理；其余事件已由 EventBus 投递到本地 WebSocket。
func (h *Hub) HandleRemoteEvent(event string, payload interface{}) {
	switch event {
	case EventSFUProviderChanged:
		provider := ""
		switch p := payload.(type) {
		case map[string]interface{}:
			if v, ok := p["provider"].(string); ok {
				provider = v
			}
		}
		logger.WithComponent("Signal").Infof("[Signal] remote SFU provider switch received: %s", provider)
		h.clearLocalRoomsForSFUSwitch()
	case EventStateRoomChanged:
		room := ""
		switch p := payload.(type) {
		case map[string]interface{}:
			if v, ok := p["room"].(string); ok {
				room = v
			}
		}
		h.ApplyRemoteRoomState(room)
	default:
		// other events already delivered to local WebSocket by EventBus
	}
}

// clearLocalRoomsForSFUSwitch 清理本机信令房间与 stream 视图（不重复广播 provider-changed）。
