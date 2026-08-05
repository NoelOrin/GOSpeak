package signal

import (
	"sort"

	"GOSpeak/internal/bus"

	"golang.org/x/sync/errgroup"
)

// getMergedRooms 合并 DB 持久化房间 + 内存活跃房间（并行查询）。
func (h *Hub) getMergedRooms(domainUUID string) []RoomInfo {
	return h.getMergedRoomsScoped(domainUUID, false)
}

func (h *Hub) getMergedPlatformRooms() []RoomInfo {
	return h.getMergedRoomsScoped("", true)
}

func (h *Hub) getMergedRoomsScoped(domainUUID string, platformOnly bool) []RoomInfo {
	var dbRooms map[string]RoomInfo
	var memRooms map[string]RoomInfo

	var eg errgroup.Group

	// goroutine 1: 从 DB 获取所有房间（最多 200 条）
	eg.Go(func() error {
		dbRooms = make(map[string]RoomInfo)
		if h.roomStore != nil {
			if rooms, _, err := h.roomStore.List(1, 200, "", domainUUID); err == nil {
				for _, r := range rooms {
					if platformOnly && r.DomainUUID != "" {
						continue
					}
					key := roomKey(r.DomainUUID, r.Name)
					dbRooms[key] = RoomInfo{
						ID:            r.ID,
						UUID:          r.UUID,
						Name:          r.Name,
						DomainUUID:    r.DomainUUID,
						HasPassword:   r.Password != "",
						Description:   r.Description,
						Limit:         r.Limit,
						AudioOnly:     r.AudioOnly,
						AllowAudience: r.AllowAudience,
						Type:          r.Type,
						Members:       []MemberInfo{},
						Count:         0,
						CreatedAt:     r.CreatedAt.UnixMilli(),
					}
				}
			}
		}
		return nil
	})

	// goroutine 2: 读取内存活跃房间
	eg.Go(func() error {
		memRooms = make(map[string]RoomInfo)
		h.mu.RLock()
		for name := range h.rooms {
			if !roomKeyMatchesDomain(name, domainUUID) {
				continue
			}
			if platformOnly && !roomKeyIsPlatform(name) {
				continue
			}
			memRooms[name] = h.roomInfoLocked(name)
		}
		h.mu.RUnlock()
		// 无 KV 合并时在锁外补全成员资料/禁言；有 KV 时后续 GetRoomMembersMerged 已含该补全。
		if h.membershipStore == nil {
			for name, info := range memRooms {
				info.Members = h.enrichMembers(info.Members)
				info.Count = len(info.Members)
				memRooms[name] = info
			}
		}
		return nil
	})

	eg.Wait()

	// 合并：内存活跃数据覆盖 DB 数据，同时保留 DB 字段
	for name, info := range memRooms {
		roomDomainUUID, _ := splitRoomKey(name)
		info.DomainUUID = roomDomainUUID
		if existing, ok := dbRooms[name]; ok {
			info.ID = existing.ID
			info.UUID = existing.UUID
			info.Description = existing.Description
			info.Limit = existing.Limit
			info.AudioOnly = existing.AudioOnly
			info.AllowAudience = existing.AllowAudience
			info.CreatedAt = existing.CreatedAt
		}
		// 有 KV 时补齐其他实例成员，修正 Count；统一补全用户资料/禁言
		if h.membershipStore != nil {
			merged := h.GetRoomMembersMerged(name)
			info.Members = h.enrichMembers(merged)
			info.Count = len(merged)
		}
		dbRooms[name] = info
	}

	// KV 中仅远端存在的活跃房间也并入列表（跨实例可见）。
	// 批量读取成员快照，避免每个远端房间一次独立 KV Get（N+1）。
	if h.membershipStore != nil {
		ctx, cancel := kvTimeoutCtx()
		if names, err := h.membershipStore.ListRoomNames(ctx); err == nil {
			var remoteNames []string
			for _, name := range names {
				if name == "" {
					continue
				}
				if !roomKeyMatchesDomain(name, domainUUID) {
					continue
				}
				if platformOnly && !roomKeyIsPlatform(name) {
					continue
				}
				if _, ok := dbRooms[name]; ok {
					// already present; still refresh members/count from merge
					info := dbRooms[name]
					merged := h.GetRoomMembersMerged(name)
					info.Members = h.enrichMembers(merged)
					info.Count = len(merged)
					dbRooms[name] = info
					continue
				}
				remoteNames = append(remoteNames, name)
			}

			snaps := make(map[string]bus.RoomMembersSnapshot, len(remoteNames))
			if bulk, ok := h.membershipStore.(bulkRoomMembersReader); ok {
				snaps, _ = bulk.GetRoomMembersBatch(ctx, remoteNames)
			} else {
				for _, name := range remoteNames {
					if snap, getErr := h.membershipStore.GetRoomMembers(ctx, name); getErr == nil {
						snaps[name] = snap
					}
				}
			}
			for _, name := range remoteNames {
				info := h.roomInfoLocked(name)
				if meta, metaErr := h.getRoomMeta(name); metaErr == nil && meta.Name != "" {
					if info.Name == "" {
						info.Name = meta.Name
					}
					if info.DomainUUID == "" {
						info.DomainUUID = meta.DomainUUID
					}
					if info.Description == "" {
						info.Description = meta.Description
					}
					if info.Limit == 0 {
						info.Limit = meta.Limit
					}
					if info.Type == "" {
						info.Type = meta.Type
					}
					info.HasPassword = meta.Password != ""
					if info.CreatedAt == 0 {
						info.CreatedAt = meta.CreatedAtMS
					}
				}
				if info.Name == "" {
					_, logicalName := splitRoomKey(name)
					info.Name = logicalName
				}
				merged := h.mergeMemberSnapshot(name, snaps[name])
				info.Members = h.enrichMembers(merged)
				info.Count = len(merged)
				dbRooms[name] = info
			}
			cancel()
		} else {
			cancel()
		}
	}

	result := make([]RoomInfo, 0, len(dbRooms))
	for _, r := range dbRooms {
		if domainUUID != "" && r.DomainUUID != domainUUID {
			continue
		}
		if platformOnly && r.DomainUUID != "" {
			continue
		}
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// ─── 公共方法 ───

// HubStats 信令面实时统计，用于监控面板。

// broadcastRoomList 向全员推送最新房间列表。
// 事件名必须是 room:list:result（与 OnRoomList 一致）；room:list 是客户端请求事件，前端不监听。
func (h *Hub) broadcastRoomList(domainUUID string) {
	if h.fanout == nil {
		return
	}
	var rooms []RoomInfo
	if domainUUID == "" {
		rooms = h.getMergedPlatformRooms()
	} else {
		rooms = h.getMergedRooms(domainUUID)
	}
	h.fanout.BroadcastToRoom(domainRoomKey(domainUUID), EventRoomListResult, map[string]interface{}{
		"rooms": rooms,
		"count": len(rooms),
	})
}

func (h *Hub) broadcastRoomListKnownDomains() {
	h.mu.RLock()
	seen := make(map[string]struct{}, len(h.clientDomains))
	for _, domainUUID := range h.clientDomains {
		seen[domainUUID] = struct{}{}
	}
	h.mu.RUnlock()

	for domainUUID := range seen {
		h.broadcastRoomList(domainUUID)
	}
}

// CheckRoomLimit 检查房间是否已满，返回 (已满, 限制人数, 当前人数, 错误)
