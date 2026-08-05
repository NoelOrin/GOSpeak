package signal

import (
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/ws"
	"log"
)

// ─── 离开房间 ───

func (h *Hub) OnRoomLeave(c ws.ClientMessenger, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{
			"error": "room name is required",
		})
	}

	key := roomKey(req.DomainUUID, req.Room)
	h.fanout.Leave(key, c.ID())
	h.clearConnRoomSlot(c.ID(), key)

	var identity string
	var stream string
	wasSpeaking := false
	roomDeleted := false
	h.mu.Lock()
	if room, exists := h.rooms[key]; exists {
		if member, ok := room.Members[c.ID()]; ok {
			identity = member.Identity
			wasSpeaking = room.Speaking[identity]
			stream = member.Stream
			roomDelMember(room, c.ID())
			delete(room.Speaking, identity)
			delete(room.MicMuted, identity)
			h.unregisterStreamLocked(key, identity, stream)
			// 成员清空则删除房间，与 OnDisconnect 行为一致
			roomDeleted = h.deleteRoomIfEmptyLocked(key)
		}
	}
	h.mu.Unlock()
	h.syncRoomToStore(key)
	if stream != "" {
		h.syncStreamDelete(stream)
	}
	if wasSpeaking && !roomDeleted {
		h.broadcastActiveSpeakers(req.DomainUUID, req.Room)
	}

	// 同步清理 SFU 端：移除 participant，空房时删除 SFU room
	if identity != "" {
		h.removeParticipantSafe(key, identity)
		if h.participantCleanup != nil {
			h.participantCleanup.OnParticipantLeft(key, identity)
		}
	}
	if roomDeleted {
		h.deleteRoomSafe(key)
	}

	if identity == "" {
		identity = c.ID()
	}

	log.Printf("[Signal] %s (%s) left room: %s", c.ID(), identity, req.Room)

	c.Send(map[string]interface{}{"event": EventRoomLeft, "data": map[string]interface{}{
		"room":        req.Room,
		"domain_uuid": req.DomainUUID,
	}})

	h.publishRoom(key, EventMemberLeft, map[string]interface{}{
		"room":        req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":    identity,
		"id":          c.ID(),
	})

	// 房间已空被删除则广播房间列表，否则广播单房间更新（与 OnDisconnect 一致）
	if roomDeleted {
		h.broadcastRoomList(req.DomainUUID)
	} else {
		// NOTE: roomInfoLocked 在 !exists 时返回零值 RoomInfo（并发删房场景安全）
		h.broadcastRoomUpdatedLocal(key)
	}

	return marshalAck(map[string]interface{}{
		"ok":   true,
		"room": req.Room,
	})
}

// ─── 房间列表 ───

// ─── 房间列表 ───

func (h *Hub) OnRoomList(c ws.ClientMessenger, data string) {
	var req struct {
		DomainUUID string `json:"domain_uuid"`
	}
	_ = parseJSON(data, &req)

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		c.Send(map[string]interface{}{"event": EventRoomListResult, "data": map[string]interface{}{
			"rooms": []RoomInfo{},
			"count": 0,
		}})
		return
	}
	h.setClientDomain(c, req.DomainUUID)

	var rooms []RoomInfo
	if req.DomainUUID == "" {
		rooms = h.getMergedPlatformRooms()
	} else {
		rooms = h.getMergedRooms(req.DomainUUID)
	}
	c.Send(map[string]interface{}{"event": EventRoomListResult, "data": map[string]interface{}{
		"rooms": rooms,
		"count": len(rooms),
	}})
}

// ─── 踢出成员 ───

// ─── 踢出成员 ───

func (h *Hub) OnRoomKick(c ws.ClientMessenger, data string) {
	var req struct {
		Room           string `json:"room"`
		DomainUUID     string `json:"domain_uuid,omitempty"`
		TargetIdentity string `json:"targetIdentity"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.TargetIdentity == "" {
		return
	}

	// 取踢人者身份并校验 signal:kick 权限；DB/缓存查询在锁外执行，避免持锁等 IO
	h.mu.Lock()
	room, exists := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !exists {
		h.mu.Unlock()
		return
	}
	caller, ok := room.Members[c.ID()]
	if !ok {
		h.mu.Unlock()
		return
	}
	callerIdentity := caller.Identity
	h.mu.Unlock()

	// 权限：优先用连接 JWT claims.Permissions（Bot token），否则回退 role 映射。
	claims := c.Claims()
	role := ""
	if claims != nil {
		role = claims.Role
	}
	if role == "" {
		if h.userStore == nil {
			return
		}
		callerUser, err := h.userStore.GetByName(callerIdentity)
		if err != nil || callerUser == nil {
			return
		}
		role = callerUser.Role
	}
	if !middleware.PermissionGranted(claims, role, permcode.PermSignalKick, h.permChecker) {
		return
	}

	// 保护管理员：仅非 bot 的 admin 可踢 admin。Bot 即使持有 signal:kick 也不能踢管理账号。
	if h.userStore != nil {
		targetUser, err := h.userStore.GetByName(req.TargetIdentity)
		if err == nil && targetUser != nil && targetUser.Role == "admin" {
			callerIsAdminHuman := role == "admin" && (claims == nil || len(claims.Permissions) == 0)
			if !callerIsAdminHuman {
				return
			}
		}
	}

	// 权限通过，重新加锁执行踢出
	key := roomKey(req.DomainUUID, req.Room)
	h.mu.Lock()
	room, exists = h.rooms[key]
	if !exists {
		h.mu.Unlock()
		return
	}
	var targetSocketID string
	var targetConn ws.ClientMessenger
	var targetStream string
	targetWasSpeaking := false
	roomDeleted := false
	for sid, m := range room.Members {
		if m.Identity == req.TargetIdentity {
			targetSocketID = sid
			targetStream = m.Stream
			targetWasSpeaking = room.Speaking[m.Identity]
			roomDelMember(room, sid)
			delete(room.Speaking, m.Identity)
			delete(room.MicMuted, m.Identity)
			h.unregisterStreamLocked(key, m.Identity, targetStream)
			// 踢出最后一人也删房，与 OnRoomLeave / OnDisconnect 行为一致
			roomDeleted = h.deleteRoomIfEmptyLocked(key)
			break
		}
	}
	h.mu.Unlock()

	if targetSocketID == "" {
		return
	}
	// 记录短时踢出冷却，禁止被踢者立即重连同一房间。
	h.blockRejoin(key, req.TargetIdentity)
	h.clearConnRoomSlot(targetSocketID, key)
	if targetStream != "" {
		h.syncStreamDelete(targetStream)
	}

	// 从 fanout room 移除目标连接，避免继续收事件
	if h.fanout != nil {
		h.fanout.ForEach(key, func(conn ws.ClientMessenger) bool {
			if conn != nil && conn.ID() == targetSocketID {
				targetConn = conn
				return false
			}
			return true
		})
		if targetConn != nil {
			h.fanout.Leave(key, targetConn.ID())
		}
	}

	log.Printf("[Signal] %s kicked %s from room: %s", c.ID(), req.TargetIdentity, req.Room)

	// 信令层踢人始终先生效；SFU 层按 Capabilities.ServerKick 尽力 hard-enforce。
	enforcement := h.removeParticipantSafe(key, req.TargetIdentity)

	// 通知被踢者；payload 带 targetIdentity，前端按 identity 过滤避免误伤同房他人
	h.publishRoom(key, EventRoomKicked, map[string]interface{}{
		"room":           req.Room,
		"domain_uuid":    req.DomainUUID,
		"targetIdentity": req.TargetIdentity,
		"enforcement":    enforcement,
	})
	if targetConn != nil {
		targetConn.Send(map[string]interface{}{"event": EventRoomKicked, "data": map[string]interface{}{
			"room":           req.Room,
			"domain_uuid":    req.DomainUUID,
			"targetIdentity": req.TargetIdentity,
			"enforcement":    enforcement,
		}})
	}

	// 通知全员成员离开
	h.publishRoom(key, EventMemberLeft, map[string]interface{}{
		"room":        req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":    req.TargetIdentity,
		"id":          targetSocketID,
		"enforcement": enforcement,
	})

	// SFU remove 已在上方 removeParticipantSafe 完成；此处同步房间状态并做 provider 清理。
	h.syncRoomToStore(key)
	if h.participantCleanup != nil {
		h.participantCleanup.OnParticipantLeft(key, req.TargetIdentity)
	}

	// 空房删除 SFU room 并广播列表，否则广播单房间更新
	if roomDeleted {
		h.deleteRoomSafe(key)
		h.broadcastRoomList(req.DomainUUID)
	} else {
		h.broadcastRoomUpdatedLocal(key)
		if targetWasSpeaking {
			h.broadcastActiveSpeakers(req.DomainUUID, req.Room)
		}
	}
}

func (h *Hub) OnMemberMicState(c ws.ClientMessenger, data string) {
	var req struct {
		Room       string `json:"room"`
		DomainUUID string `json:"domain_uuid,omitempty"`
		Identity   string `json:"identity"`
		IsMicMuted bool   `json:"isMicMuted"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.Identity == "" {
		return
	}

	h.mu.Lock()
	room, ok := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !ok {
		h.mu.Unlock()
		return
	}
	caller := room.Members[c.ID()]
	if caller == nil || caller.Identity != req.Identity {
		h.mu.Unlock()
		return
	}
	if room.MicMuted == nil {
		room.MicMuted = make(map[string]bool)
	}
	room.MicMuted[req.Identity] = req.IsMicMuted
	h.mu.Unlock()

	// 同步共享成员快照，让其他实例看到最新的静音状态。
	h.syncRoomToStore(roomKey(req.DomainUUID, req.Room))

	h.BroadcastToRoom(roomKey(req.DomainUUID, req.Room), EventMemberUpdated, map[string]interface{}{
		"room":        req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":    req.Identity,
		"isMicMuted":  req.IsMicMuted,
	})
}

// OnMemberSpeaking 接收成员本地发言态上报（SRS / Cloudflare 等无 SFU 原生
// active speaker 的 provider）。服务端按 room 聚合后广播 room:active-speakers。
// 仅持有本地麦克风的成员可上报自身状态，避免伪造他人发言态。

// OnMemberSpeaking 接收成员本地发言态上报（SRS / Cloudflare 等无 SFU 原生
// active speaker 的 provider）。服务端按 room 聚合后广播 room:active-speakers。
// 仅持有本地麦克风的成员可上报自身状态，避免伪造他人发言态。
func (h *Hub) OnMemberSpeaking(c ws.ClientMessenger, data string) {
	var req struct {
		Room       string `json:"room"`
		DomainUUID string `json:"domain_uuid,omitempty"`
		Identity   string `json:"identity"`
		Speaking   bool   `json:"speaking"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.Identity == "" {
		return
	}

	// 先在校验锁内确认调用者是本人，再在锁外做禁言检查，避免 DB 查询阻塞信令。
	h.mu.RLock()
	room, ok := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !ok {
		h.mu.RUnlock()
		return
	}
	caller := room.Members[c.ID()]
	valid := caller != nil && caller.Identity == req.Identity
	h.mu.RUnlock()
	if !valid {
		return
	}

	// 与 join 一致做禁言检查（fail-closed）：被禁言或检查失败时忽略发言态上报。
	if h.muteStore != nil {
		muted, _, muteErr := h.muteStore.IsMutedByIdentity(req.Identity)
		if muteErr != nil {
			log.Printf("[signal] OnMemberSpeaking IsMutedByIdentity error: identity=%q err=%v", req.Identity, muteErr)
			return
		}
		if muted {
			return
		}
	}

	h.mu.Lock()
	room, ok = h.rooms[roomKey(req.DomainUUID, req.Room)]
	if !ok {
		h.mu.Unlock()
		return
	}
	caller = room.Members[c.ID()]
	if caller == nil || caller.Identity != req.Identity {
		h.mu.Unlock()
		return
	}
	if room.Speaking == nil {
		room.Speaking = make(map[string]bool)
	}
	room.Speaking[req.Identity] = req.Speaking
	h.mu.Unlock()

	h.broadcastActiveSpeakers(req.DomainUUID, req.Room)
}

// computeActiveSpeakersLocked 在持锁状态下返回房间内正在发言的成员 identity 列表。

// computeActiveSpeakersLocked 在持锁状态下返回房间内正在发言的成员 identity 列表。
func (h *Hub) computeActiveSpeakersLocked(roomName string) []string {
	room, ok := h.rooms[roomName]
	if !ok || room.Speaking == nil {
		return nil
	}
	out := make([]string, 0)
	for identity, speaking := range room.Speaking {
		if speaking {
			out = append(out, identity)
		}
	}
	return out
}

// broadcastActiveSpeakers 向房间广播当前 active speakers 列表（用于无 SFU 原生
// active speaker 的 provider）。发言人清空（离开/断连）时也同样广播空列表以重置高亮。

// broadcastActiveSpeakers 向房间广播当前 active speakers 列表（用于无 SFU 原生
// active speaker 的 provider）。发言人清空（离开/断连）时也同样广播空列表以重置高亮。
func (h *Hub) broadcastActiveSpeakers(domainUUID, roomName string) {
	if h.fanout == nil {
		return
	}
	rk := roomKey(domainUUID, roomName)
	h.mu.RLock()
	identities := h.computeActiveSpeakersLocked(rk)
	h.mu.RUnlock()
	h.publishRoom(rk, EventRoomActiveSpeakers, map[string]interface{}{
		"room":        roomName,
		"domain_uuid": domainUUID,
		"identities":  identities,
	})
}

// getMergedRooms 合并 DB 持久化房间 + 内存活跃房间（并行查询）。
