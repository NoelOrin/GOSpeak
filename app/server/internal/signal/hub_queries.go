package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"errors"
	"fmt"
	"log"
	"strings"
)

// ErrRoomNotFound indicates the room does not exist in DB or shared metadata.
var ErrRoomNotFound = errors.New("room not found")

// ─── 公共方法 ───

// HubStats 信令面实时统计，用于监控面板。
type HubStats struct {
	RoomCount        int    `json:"room_count"`
	ParticipantCount int    `json:"participant_count"`
	OnlineUserCount  int    `json:"online_user_count"`
	WSClientDropped  uint64 `json:"ws_client_dropped"`
}

// GetStats 返回信令面房间数、参与者总数、去重在线用户数。

// GetStats 返回信令面房间数、参与者总数、去重在线用户数。
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()

	roomCount := len(h.rooms)
	identities := make(map[string]struct{}, len(h.rooms))
	participants := 0
	for _, room := range h.rooms {
		participants += len(room.Members)
		for _, m := range room.Members {
			if m.Identity != "" {
				identities[m.Identity] = struct{}{}
			}
		}
	}
	h.mu.RUnlock()

	st := HubStats{
		RoomCount:        roomCount,
		ParticipantCount: participants,
		OnlineUserCount:  len(identities),
	}
	if dc, ok := h.fanout.(interface{ DroppedCount() uint64 }); ok {
		st.WSClientDropped = dc.DroppedCount()
	}
	return st
}

func (h *Hub) GetSFURooms() []RoomInfo {
	h.mu.RLock()
	result := make([]RoomInfo, 0, len(h.rooms))
	for key := range h.rooms {
		result = append(result, h.roomInfoLocked(key))
	}
	h.mu.RUnlock()
	for i := range result {
		result[i].Members = h.enrichMembers(result[i].Members)
		result[i].Count = len(result[i].Members)
	}
	return result
}

// GetRooms returns DB-persisted rooms + in-memory active rooms merged.

// GetRooms returns DB-persisted rooms + in-memory active rooms merged.
func (h *Hub) GetRooms() []RoomInfo {
	return h.getMergedRooms("")
}

// broadcastRoomList 向全员推送最新房间列表。
// 事件名必须是 room:list:result（与 OnRoomList 一致）；room:list 是客户端请求事件，前端不监听。

// CheckRoomLimit 检查房间是否已满，返回 (已满, 限制人数, 当前人数, 错误)
func (h *Hub) CheckRoomLimit(domainUUID, roomName string) (bool, uint, int, error) {
	if h.roomStore == nil {
		return false, 0, 0, nil
	}
	dbRoom, err := h.roomStore.GetByDomainAndName(domainUUID, roomName)
	limit := uint(0)
	switch {
	case err == nil && dbRoom != nil:
		limit = dbRoom.Limit
	case errors.Is(err, pkg.ErrNotFound):
		// 临时房间：继续用共享元数据兜底，not-found 不是错误。
	default:
		// DB 故障时 fail-closed：调用方不得按“无限制”放行。
		return false, 0, 0, err
	}
	if limit == 0 {
		// 远端临时房间：使用共享元数据中的人数上限
		if meta, metaErr := h.getRoomMeta(roomKey(domainUUID, roomName)); metaErr == nil {
			limit = meta.Limit
		}
	}
	if limit == 0 {
		return false, 0, 0, nil
	}
	currentCount := len(h.GetRoomMembersMerged(roomKey(domainUUID, roomName)))
	return currentCount >= int(limit), limit, currentCount, nil
}

// CheckRoomPassword 检查房间密码，返回 (通过, 错误)
// 密码以 DB 为准；内存房间仅作兜底。ok=true 表示通过；ok=false+err!=nil 表示需密码未提供；ok=false+err==nil 表示密码错误。
// CheckRoomPassword 检查房间密码。ErrRoomNotFound 表示房间不存在；其他 error 表示需要密码；nil error 且 ok=false 表示密码错误。
func (h *Hub) CheckRoomPassword(domainUUID, roomName, password string) (ok bool, err error) {
	expected := ""
	found := false

	if h.roomStore != nil {
		if dbRoom, dbErr := h.roomStore.GetByDomainAndName(domainUUID, roomName); dbErr == nil && dbRoom != nil {
			expected = dbRoom.Password
			found = true
		}
	}

	if !found {
		h.mu.RLock()
		room, exists := h.rooms[roomKey(domainUUID, roomName)]
		h.mu.RUnlock()
		if exists {
			expected = room.Password
			found = true
		}
	}

	// 远端临时房间：DB 无记录时使用共享元数据中的密码 hash 校验
	if !found {
		if meta, metaErr := h.getRoomMeta(roomKey(domainUUID, roomName)); metaErr == nil {
			expected = meta.Password
			found = true
		}
	}

	// 房间尚未创建：DB/内存/KV 均无记录，视为临时房首次创建，允许通过（不在此阶段以未找到阻断）。
	if !found {
		return true, nil
	}
	if expected == "" {
		return true, nil
	}
	if password == "" {
		return false, fmt.Errorf("room requires password")
	}
	if pkg.VerifyPassword(expected, password) {
		return true, nil
	}
	return false, nil
}

func (h *Hub) GetRoomMembers(roomName string) []MemberInfo {
	return h.getMembers(roomName)
}

// IsIdentityMuted 检查指定 identity（用户名）是否被禁言

// IsIdentityMuted 检查指定 identity（用户名）是否被禁言
func (h *Hub) IsIdentityMuted(identity string) (bool, *model.Mute, error) {
	if h.muteStore == nil {
		return false, nil, nil
	}
	return h.muteStore.IsMutedByIdentity(identity)
}

// IsMuted 是 JoinPolicy 接口适配：仅返 bool/error，剥离 *model.Mute（pkg.JoinPolicy 不依赖 model）。

// IsMuted 是 JoinPolicy 接口适配：仅返 bool/error，剥离 *model.Mute（pkg.JoinPolicy 不依赖 model）。
func (h *Hub) IsMuted(identity string) (bool, error) {
	muted, _, err := h.IsIdentityMuted(identity)
	return muted, err
}

// 编译期断言：Hub 实现 pkg.JoinPolicy，供 SFUService 经接口注入。

// 编译期断言：Hub 实现 pkg.JoinPolicy，供 SFUService 经接口注入。
var _ pkg.JoinPolicy = (*Hub)(nil)

// IsRoomMember checks if identity is currently in room's member list.
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描同后缀 domain 房。
func (h *Hub) IsRoomMember(room, identity string) bool {
	// 1. 优先读 KV（无锁，跨实例可见）
	if h.membershipStore != nil {
		kvCtx, kvCancel := kvTimeoutCtx()
		if snap, err := h.membershipStore.GetRoomMembers(kvCtx, room); err == nil {
			for _, m := range snap.Members {
				if m.Identity == identity {
					return true
				}
			}
		}
		kvCancel()
	}

	// 2. KV 未命中 → fallback 本地 map
	h.mu.RLock()
	defer h.mu.RUnlock()
	if r, ok := h.rooms[room]; ok {
		return roomLookupIdentity(r, identity) != nil
	}
	// 纯逻辑名回退扫描
	if !strings.Contains(room, ":") {
		for rk, r := range h.rooms {
			if _, rn := splitRoomKey(rk); rn == room {
				return roomLookupIdentity(r, identity) != nil
			}
		}
	}
	return false
}

// ─── 内部辅助 ───

// memberSnapshotLocked 仅拷贝房间成员快照与本地麦克风状态，不做任何 IO。
// 调用方须持有 h.mu（读或写）；用户资料与禁言状态由 enrichMembers 在锁外补齐。
func (h *Hub) memberSnapshotLocked(roomName string) []MemberInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		return nil
	}
	members := make([]MemberInfo, 0, len(room.Members))
	for _, m := range room.Members {
		info := *m
		info.IsMicMuted = room.MicMuted[m.Identity]
		members = append(members, info)
	}
	return members
}

// enrichMembers 在 h.mu 锁外为成员快照补全用户资料与禁言状态（DB/缓存 IO）。
// 失败按原逻辑降级：查不到用户时保留成员自身字段，禁言查询失败按未禁言处理。
func (h *Hub) enrichMembers(members []MemberInfo) []MemberInfo {
	if len(members) == 0 {
		return members
	}
	out := make([]MemberInfo, len(members))
	copy(out, members)

	identities := make([]string, 0, len(members))
	for i := range out {
		if out[i].Identity != "" {
			identities = append(identities, out[i].Identity)
		}
	}

	users := map[string]*model.User{}
	if h.userStore != nil {
		if got, err := h.userStore.GetByNames(identities); err == nil {
			users = got
		}
	}
	muted := map[string]bool{}
	if h.muteStore != nil {
		if got, err := h.muteStore.IsMutedBatch(identities); err == nil {
			muted = got
		} else {
			// fail-closed：查询失败按禁言展示
			for _, id := range identities {
				muted[id] = true
			}
			log.Printf("[Signal] mute batch check failed: %v", err)
		}
	}

	for i := range out {
		m := &out[i]
		if u := users[m.Identity]; u != nil {
			m.Name = u.Name
			m.DisplayName = u.DisplayName
			m.Avatar = u.Avatar
		}
		if muted[m.Identity] {
			m.IsMuted = true
		}
	}
	return out
}

// getMembers 返回房间成员完整信息；锁内只做快照，锁外补全 IO，避免大房间阻塞全部信令。
func (h *Hub) getMembers(roomName string) []MemberInfo {
	h.mu.RLock()
	members := h.memberSnapshotLocked(roomName)
	h.mu.RUnlock()
	return h.enrichMembers(members)
}

func (h *Hub) roomInfoLocked(roomName string) RoomInfo {
	room, exists := h.rooms[roomName]
	if !exists {
		domainUUID, _ := splitRoomKey(roomName)
		return RoomInfo{Name: roomName, DomainUUID: domainUUID, Members: []MemberInfo{}, Count: 0}
	}
	return RoomInfo{
		Name:        room.Name,
		DomainUUID:  func() string { g, _ := splitRoomKey(roomName); return g }(),
		HasPassword: room.Password != "",
		Members:     h.memberSnapshotLocked(roomName),
		Count:       len(room.Members),
		CreatedAt:   room.CreatedAt.UnixMilli(),
	}
}

// duplicateIdentityLocked 判断同一房间是否已有其他 socket 占用 identity。调用方须持有 h.mu（读或写）。

// duplicateIdentityLocked 判断同一房间是否已有其他 socket 占用 identity。调用方须持有 h.mu（读或写）。
func (h *Hub) duplicateIdentityLocked(roomName, identity, socketID string) bool {
	room, exists := h.rooms[roomName]
	if !exists {
		return false
	}
	for sid, m := range room.Members {
		if sid != socketID && m.Identity == identity {
			return true
		}
	}
	return false
}

// deleteRoomIfEmptyLocked 在房间无成员时删除并返回 true。调用方须持有 h.mu（写）。

// deleteRoomIfEmptyLocked 在房间无成员时删除并返回 true。调用方须持有 h.mu（写）。
func (h *Hub) deleteRoomIfEmptyLocked(roomName string) bool {
	room, exists := h.rooms[roomName]
	if !exists || len(room.Members) != 0 {
		return false
	}
	delete(h.rooms, roomName)
	return true
}

// registerStreamLocked 在 WS join 时登记 stream→room 反查 + room→streams 聚合 + identity→stream 映射。
// 调用方须持有 h.mu（写）。stream 为空则跳过（provider 无 stream 概念）。
