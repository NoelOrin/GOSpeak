package signal

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/ws"
	"encoding/json"
	"errors"
	"log"
	"time"
)

func (h *Hub) OnRoomCreate(c ws.ClientMessenger, data string) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{
			"error": "room name is required",
		}})
		return
	}

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{
			"error": "not a member of this domain",
		}})
		return
	}
	h.setClientDomain(c, req.DomainUUID)

	h.mu.Lock()
	if _, exists := h.rooms[roomKey(req.DomainUUID, req.Room)]; exists {
		h.mu.Unlock()
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{
			"error": "room already exists",
		}})
		return
	}

	hashedPassword, hashErr := pkg.HashPassword(req.Password)
	if hashErr != nil {
		h.mu.Unlock()
		c.Send(map[string]interface{}{"event": EventRoomCreated, "data": map[string]interface{}{"error": "invalid password"}})
		return
	}
	h.rooms[roomKey(req.DomainUUID, req.Room)] = &Room{
		Name:      req.Room,
		Password:  hashedPassword,
		Members:   make(map[string]*MemberInfo),
		MicMuted:  make(map[string]bool),
		Speaking:  make(map[string]bool),
		CreatedAt: time.Now(),
	}
	roomInfo := h.roomInfoLocked(roomKey(req.DomainUUID, req.Room))
	h.mu.Unlock()

	// 临时房间元数据同步到共享 KV，远端实例可校验密码/类型/人数上限。
	h.syncRoomMeta(roomKey(req.DomainUUID, req.Room), bus.RoomMeta{
		Name:        req.Room,
		DomainUUID:  req.DomainUUID,
		Password:    hashedPassword,
		Type:        model.RoomTypeVoice,
		CreatedAtMS: time.Now().UnixMilli(),
	})

	log.Printf("[Signal] room created: %s by %s", req.Room, c.ID())

	c.Send(map[string]interface{}{"event": EventRoomCreated, "data": roomInfo})
	h.broadcastRoomUpdatedLocal(roomKey(req.DomainUUID, req.Room))
}

// ─── 加入房间 ───
//
// 加入流程分两阶段：
//  1. OnRoomJoin     — 信令面。仅校验权限 + fanout room join，不写 h.rooms 成员。
//  2. OnRoomJoinSFU  — 媒体面。SFU 连接确认后写成员 + 广播 MemberJoined/RoomUpdated。

// ─── 加入房间 ───
//
// 加入流程分两阶段：
//  1. OnRoomJoin     — 信令面。仅校验权限 + fanout room join，不写 h.rooms 成员。
//  2. OnRoomJoinSFU  — 媒体面。SFU 连接确认后写成员 + 广播 MemberJoined/RoomUpdated。
func (h *Hub) OnRoomJoin(c ws.ClientMessenger, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{
			"error": "room name is required",
		})
	}

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		return marshalAck(map[string]interface{}{
			"error": "not a member of this domain",
		})
	}

	identity, err := resolveIdentity(c, req.Identity)
	if err != nil {
		return marshalAck(map[string]interface{}{
			"error": err.Error(),
		})
	}
	req.Identity = identity

	// 密码校验（DB 为准）
	if ok, pwdErr := h.CheckRoomPassword(req.DomainUUID, req.Room, req.Password); !ok {
		if pwdErr != nil {
			return marshalAck(map[string]interface{}{
				"error": "room requires password",
			})
		}
		return marshalAck(map[string]interface{}{
			"error": "wrong room password",
		})
	}

	// 禁言检查（fail-closed）
	if h.muteStore != nil {
		muted, mute, muteErr := h.muteStore.IsMutedByIdentity(identity)
		if muteErr != nil {
			log.Printf("[signal] IsMutedByIdentity error: identity=%q err=%v", identity, muteErr)
			return marshalAck(map[string]interface{}{
				"error": "mute check failed",
			})
		}
		if muted {
			remaining := int64(0)
			if !mute.Permanent && mute.ExpiresAt != nil {
				remaining = int64(time.Until(*mute.ExpiresAt).Seconds())
				if remaining < 0 {
					remaining = 0
				}
			}
			return marshalAck(map[string]interface{}{
				"error":     "user is muted",
				"muted":     true,
				"permanent": mute.Permanent,
				"remaining": remaining,
			})
		}
	}

	// 服务端校验房间人数上限
	if full, limit, count, _ := h.CheckRoomLimit(req.DomainUUID, req.Room); full {
		return marshalAck(map[string]interface{}{
			"error": "room is full",
			"limit": limit,
			"count": count,
		})
	}

	// 提前拦截重复身份：已通过 OnRoomJoinSFU 写入 h.rooms 的在线用户，新 socket 在 OnRoomJoin 阶段即阻断。
	// 并发场景由 OnRoomJoinSFU 的 duplicate check 兜底，此处为提前拦截以节省媒体连接开销。
	h.mu.RLock()
	dup := h.duplicateIdentityLocked(roomKey(req.DomainUUID, req.Room), identity, c.ID())
	h.mu.RUnlock()
	if dup {
		return marshalAck(map[string]interface{}{
			"error":    "duplicate connection not allowed",
			"room":     req.Room,
			"identity": identity,
		})
	}

	// Dual-slot tracking: resolve room type, manage text/voice slot per conn.
	key := roomKey(req.DomainUUID, req.Room)
	roomType := model.RoomTypeVoice
	if h.roomStore != nil {
		if dbRoom, dbErr := h.roomStore.GetByDomainAndName(req.DomainUUID, req.Room); dbErr == nil && dbRoom != nil {
			roomType = model.NormalizeRoomType(dbRoom.Type)
		}
	}
	if roomType == model.RoomTypeVoice {
		if meta, metaErr := h.getRoomMeta(roomKey(req.DomainUUID, req.Room)); metaErr == nil {
			roomType = model.NormalizeRoomType(meta.Type)
		}
	}
	h.mu.Lock()
	slots := h.connSlots[c.ID()]
	if slots == nil {
		slots = &connRoomSlots{}
		h.connSlots[c.ID()] = slots
	}
	if roomType == model.RoomTypeText {
		// Switching text rooms: leave old slot
		if slots.TextRoom != "" && slots.TextRoom != key {
			h.fanout.Leave(slots.TextRoom, c.ID())
		}
		slots.TextRoom = key
		h.fanout.Join(key, c.ID())
	} else {
		// Voice slot does not switch until room:join:sfu succeeds.
		// Fanout join is safe here: receiving room events in the new room
		// before SFU confirm is harmless and avoids stranding the socket if
		// the join is rejected.
		// 首次加入/同房重试提前 join fanout；切换场景保持旧房 fanout，
		// 避免 phase2 失败时 socket 被孤立在新房且旧槽位未回滚。
		if slots.VoiceRoom == "" || slots.VoiceRoom == key {
			h.fanout.Join(key, c.ID())
		}
	}
	h.mu.Unlock()

	if roomType == model.RoomTypeText {
		h.setClientDomain(c, req.DomainUUID)
	}
	log.Printf("[Signal] %s (%s) signaling ready for room: %s (type=%s)", c.ID(), identity, req.Room, roomType)

	return marshalAck(map[string]interface{}{
		"room":     req.Room,
		"identity": identity,
	})
}

// OnRoomJoinSFU 在 SFU 媒体连接确认后写成员并广播加入。

// OnRoomJoinSFU 在 SFU 媒体连接确认后写成员并广播加入。
func (h *Hub) OnRoomJoinSFU(c ws.ClientMessenger, data string) (string, error) {
	var req RoomRequest
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{"error": "room name is required"})
	}

	if !h.domainMemberAllowed(c, req.DomainUUID) {
		return marshalAck(map[string]interface{}{
			"error": "not a member of this domain",
		})
	}

	identity, err := resolveIdentity(c, req.Identity)
	if err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	req.Identity = identity

	// Text room guard: text rooms have no media, reject SFU join.
	roomType := ""
	if h.roomStore != nil {
		if dbRoom, dbErr := h.roomStore.GetByDomainAndName(req.DomainUUID, req.Room); dbErr == nil && dbRoom != nil {
			roomType = model.NormalizeRoomType(dbRoom.Type)
		}
	}
	if roomType == "" {
		if meta, metaErr := h.getRoomMeta(roomKey(req.DomainUUID, req.Room)); metaErr == nil {
			roomType = model.NormalizeRoomType(meta.Type)
		}
	}
	if roomType == model.RoomTypeText {
		return marshalAck(map[string]interface{}{"error": "text room has no media"})
	}

	// phase2 复检 mute / limit / password，避免 phase1 通过后状态变化
	if ok, pwdErr := h.CheckRoomPassword(req.DomainUUID, req.Room, req.Password); !ok {
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		if pwdErr != nil {
			return marshalAck(map[string]interface{}{"error": "room requires password"})
		}
		return marshalAck(map[string]interface{}{"error": "wrong room password"})
	}
	if h.muteStore != nil {
		muted, _, muteErr := h.muteStore.IsMutedByIdentity(identity)
		if muteErr != nil {
			h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
			return marshalAck(map[string]interface{}{"error": "mute check failed"})
		}
		if muted {
			h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
			return marshalAck(map[string]interface{}{"error": "user is muted", "muted": true})
		}
	}
	full, limit, count, _ := h.CheckRoomLimit(req.DomainUUID, req.Room)
	if full {
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		return marshalAck(map[string]interface{}{"error": "room is full", "limit": limit, "count": count})
	}

	// 服务端覆写 stream：客户端提报值不可信，使用服务端基于 room+identity 计算的预期值
	if h.streamResolver != nil {
		req.Stream = h.streamResolver.StreamName(roomKey(req.DomainUUID, req.Room), identity)
	}

	member := &MemberInfo{
		ID:       c.ID(),
		Identity: identity,
		JoinedAt: time.Now().UnixMilli(),
		Stream:   req.Stream,
	}

	newKey := roomKey(req.DomainUUID, req.Room)
	type switchCleanup struct {
		key         string
		identity    string
		stream      string
		wasSpeaking bool
		roomDeleted bool
	}
	var oldCleanups []*switchCleanup

	// 锁内构造注册快照并通过 KV CAS 原子注册。join 为低频路径，
	// 锁内 KV 写入可避免注册后、本地提交前其他成员的 sync 把新成员
	// 记录当作本实例旧记录丢弃；后端故障时最多阻塞数秒后按失败处理。
	now := time.Now().UnixMilli()
	regLocal := make([]bus.MemberRecord, 0, 2)
	h.mu.Lock()
	// 重复身份检查：同一房间不允许同一 identity 用不同 socket 重复加入
	if h.duplicateIdentityLocked(newKey, identity, c.ID()) {
		h.mu.Unlock()
		h.fanout.Leave(newKey, c.ID())
		return marshalAck(map[string]interface{}{
			"error":    "duplicate connection not allowed",
			"room":     req.Room,
			"identity": identity,
		})
	}
	if room, ok := h.rooms[newKey]; ok {
		for sid, m := range room.Members {
			if m == nil {
				continue
			}
			regLocal = append(regLocal, bus.MemberRecord{
				Room:        newKey,
				Identity:    m.Identity,
				SocketHint:  sid,
				InstanceID:  h.instanceID,
				Stream:      m.Stream,
				MicMuted:    room.MicMuted[m.Identity],
				Speaking:    room.Speaking[m.Identity],
				UpdatedAtMS: now,
				ExpiresAtMS: now + membershipLeaseDuration.Milliseconds(),
			})
		}
	}
	regLocal = append(regLocal, bus.MemberRecord{
		Room:        newKey,
		Identity:    identity,
		SocketHint:  c.ID(),
		InstanceID:  h.instanceID,
		Stream:      req.Stream,
		UpdatedAtMS: now,
		ExpiresAtMS: now + membershipLeaseDuration.Milliseconds(),
	})
	// 收集旧房间清理信息（暂不删除，注册失败时保持原状）
	for oldKey, oldRoom := range h.rooms {
		if oldKey == newKey {
			continue
		}
		oldMember, ok := oldRoom.Members[c.ID()]
		if !ok {
			continue
		}
		oldCleanups = append(oldCleanups, &switchCleanup{
			key:         oldKey,
			identity:    oldMember.Identity,
			stream:      oldMember.Stream,
			wasSpeaking: oldRoom.Speaking[oldMember.Identity],
		})
	}
	if regErr := h.registerRoomMembers(newKey, regLocal, int(limit), identity); regErr != nil {
		h.mu.Unlock()
		h.fanout.Leave(newKey, c.ID())
		switch {
		case errors.Is(regErr, errDuplicateRemoteIdentity):
			return marshalAck(map[string]interface{}{
				"error":    "duplicate connection not allowed",
				"room":     req.Room,
				"identity": identity,
			})
		case errors.Is(regErr, errRoomLimitExceeded):
			return marshalAck(map[string]interface{}{"error": "room is full", "limit": limit, "count": count})
		default:
			log.Printf("[Signal] sfu join registration failed room=%s identity=%s: %v", req.Room, identity, regErr)
			return marshalAck(map[string]interface{}{"error": "room join failed"})
		}
	}

	// 提交本地状态：先清理旧房间成员，再写入新房间
	for _, old := range oldCleanups {
		oldRoom, ok := h.rooms[old.key]
		if !ok {
			continue
		}
		if _, ok := oldRoom.Members[c.ID()]; ok {
			delete(oldRoom.Speaking, old.identity)
			delete(oldRoom.MicMuted, old.identity)
			h.unregisterStreamLocked(old.key, old.identity, old.stream)
		}
		roomDelMember(oldRoom, c.ID())
		old.roomDeleted = h.deleteRoomIfEmptyLocked(old.key)
		if h.fanout != nil {
			h.fanout.Leave(old.key, c.ID())
		}
	}
	slots := h.connSlots[c.ID()]
	if slots == nil {
		slots = &connRoomSlots{}
		h.connSlots[c.ID()] = slots
	}
	slots.VoiceRoom = newKey
	if h.fanout != nil {
		h.fanout.Join(newKey, c.ID())
	}

	room, exists := h.rooms[newKey]
	if !exists {
		room = &Room{
			Name:      req.Room,
			Members:   make(map[string]*MemberInfo),
			MicMuted:  make(map[string]bool),
			Speaking:  make(map[string]bool),
			CreatedAt: time.Now(),
		}
		h.rooms[newKey] = room
	}
	roomSetMember(room, c.ID(), identity, member)
	h.registerStreamLocked(newKey, identity, req.Stream)
	memberList := h.memberSnapshotLocked(newKey)
	h.mu.Unlock()

	for _, old := range oldCleanups {
		h.syncRoomToStore(old.key)
		if old.stream != "" {
			h.syncStreamDelete(old.stream)
		}
		oldDomain, oldRoomName := splitRoomKey(old.key)
		h.publishRoom(old.key, EventMemberLeft, map[string]interface{}{
			"room":        oldRoomName,
			"domain_uuid": oldDomain,
			"identity":    old.identity,
			"id":          c.ID(),
		})
		h.removeParticipantSafe(old.key, old.identity)
		if h.participantCleanup != nil {
			h.participantCleanup.OnParticipantLeft(old.key, old.identity)
		}
		if old.wasSpeaking && !old.roomDeleted {
			h.broadcastActiveSpeakers(oldDomain, oldRoomName)
		}
		if old.roomDeleted {
			h.deleteRoomSafe(old.key)
			h.broadcastRoomList(oldDomain)
		} else {
			h.broadcastRoomUpdatedLocal(old.key)
		}
	}

	memberList = h.enrichMembers(memberList)
	if req.Stream != "" {
		h.syncStreamPut(req.Stream, newKey, identity)
	}
	h.setClientDomain(c, req.DomainUUID)

	log.Printf("[Signal] sfu confirmed: %s in %s", c.ID(), req.Room)

	h.mu.RLock()
	_, memberExists := h.rooms[roomKey(req.DomainUUID, req.Room)]
	if memberExists {
		_, memberExists = h.rooms[roomKey(req.DomainUUID, req.Room)].Members[c.ID()]
	}
	h.mu.RUnlock()
	if !memberExists {
		log.Printf("[Signal] sfu join rejected: %s not in room %s", c.ID(), req.Room)
		// 回退 phase1 的 fanout.Join，避免 socket 残留在 WS room
		h.fanout.Leave(roomKey(req.DomainUUID, req.Room), c.ID())
		return marshalAck(map[string]interface{}{
			"error": "not in room",
		})
	}

	h.publishRoom(roomKey(req.DomainUUID, req.Room), EventMemberJoined, map[string]interface{}{
		"room":        req.Room,
		"domain_uuid": req.DomainUUID,
		"identity":    identity,
		"id":          c.ID(),
		"stream":      req.Stream,
	})

	h.broadcastRoomUpdatedLocal(roomKey(req.DomainUUID, req.Room))

	return marshalAck(map[string]interface{}{
		"ok":       true,
		"room":     req.Room,
		"identity": identity,
		"members":  memberList,
	})
}

func marshalAck(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"serialization failed"}`, nil
	}
	return string(data), nil
}

// ─── 离开房间 ───
