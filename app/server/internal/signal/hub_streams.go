package signal

import (
	"strings"
)

func (h *Hub) RegisterStream(stream string) {
	var room, identity string
	h.mu.Lock()
	h.activeStreams[stream] = struct{}{}
	// SRS callback 仅给 stream，反查 streamRoomCache 同步 roomStreams 聚合视图。
	if r, ok := h.streamRoomCache[stream]; ok {
		room = r
		if h.roomStreams[room] == nil {
			h.roomStreams[room] = make(map[string]struct{})
		}
		h.roomStreams[room][stream] = struct{}{}
		if byID := h.streamByIdentity[room]; byID != nil {
			for id, st := range byID {
				if st == stream {
					identity = id
					break
				}
			}
		}
	}
	h.mu.Unlock()
	if room != "" {
		h.syncStreamPut(stream, room, identity)
	}
}

func (h *Hub) UnregisterStream(stream string) {
	h.mu.Lock()
	delete(h.activeStreams, stream)
	if room, ok := h.streamRoomCache[stream]; ok {
		if streams, ok := h.roomStreams[room]; ok {
			delete(streams, stream)
			if len(streams) == 0 {
				delete(h.roomStreams, room)
			}
		}
	}
	h.mu.Unlock()
	h.syncStreamDelete(stream)
}

func (h *Hub) IsStreamActive(stream string) bool {
	h.mu.RLock()
	_, ok := h.activeStreams[stream]
	h.mu.RUnlock()
	if ok {
		return true
	}
	if stream == "" || h.membershipStore == nil {
		return false
	}
	kvCtx, kvCancel := kvTimeoutCtx()
	room, _, err := h.membershipStore.GetStream(kvCtx, stream)
	kvCancel()
	return err == nil && room != ""
}

func (h *Hub) RoomForStream(stream string) (string, bool) {
	h.mu.RLock()
	room, ok := h.streamRoomCache[stream]
	h.mu.RUnlock()
	if ok {
		return room, true
	}
	if stream == "" || h.membershipStore == nil {
		return "", false
	}
	kvCtx, kvCancel := kvTimeoutCtx()
	room, _, err := h.membershipStore.GetStream(kvCtx, stream)
	kvCancel()
	if err != nil || room == "" {
		return "", false
	}
	return room, true
}

// registerStreamLocked 在 WS join 时登记 stream→room 反查 + room→streams 聚合 + identity→stream 映射。
// 调用方须持有 h.mu（写）。stream 为空则跳过（provider 无 stream 概念）。
func (h *Hub) registerStreamLocked(room, identity, stream string) {
	if stream == "" {
		return
	}
	h.streamRoomCache[stream] = room
	if h.roomStreams[room] == nil {
		h.roomStreams[room] = make(map[string]struct{})
	}
	h.roomStreams[room][stream] = struct{}{}
	if identity != "" {
		if h.streamByIdentity[room] == nil {
			h.streamByIdentity[room] = make(map[string]string)
		}
		h.streamByIdentity[room][identity] = stream
	}
}

// unregisterStreamLocked 在 WS leave/disconnect 时清除 stream 映射 + identity→stream 映射。
// 调用方须持有 h.mu（写）。stream 为空则跳过。

// unregisterStreamLocked 在 WS leave/disconnect 时清除 stream 映射 + identity→stream 映射。
// 调用方须持有 h.mu（写）。stream 为空则跳过。
func (h *Hub) unregisterStreamLocked(room, identity, stream string) {
	if stream == "" {
		return
	}
	delete(h.streamRoomCache, stream)
	if streams, ok := h.roomStreams[room]; ok {
		delete(streams, stream)
		if len(streams) == 0 {
			delete(h.roomStreams, room)
		}
	}
	if identity != "" {
		if identities, ok := h.streamByIdentity[room]; ok {
			delete(identities, identity)
			if len(identities) == 0 {
				delete(h.streamByIdentity, room)
			}
		}
	}
}

// Rooms 返回当前有活跃 stream 的 room 列表（RoomRegistry 实现）。
// 本地 roomStreams 优先；membership KV 中带 Stream 的远端房间一并并入。

// Rooms 返回当前有活跃 stream 的 room 列表（RoomRegistry 实现）。
// 本地 roomStreams 优先；membership KV 中带 Stream 的远端房间一并并入。
func (h *Hub) Rooms() []string {
	h.mu.RLock()
	seen := make(map[string]struct{}, len(h.roomStreams))
	out := make([]string, 0, len(h.roomStreams))
	for room := range h.roomStreams {
		if room == "" {
			continue
		}
		seen[room] = struct{}{}
		out = append(out, room)
	}
	h.mu.RUnlock()
	if h.membershipStore != nil {
		kvCtx, kvCancel := kvTimeoutCtx()
		if names, err := h.membershipStore.ListRoomNames(kvCtx); err == nil {
			for _, room := range names {
				if room == "" {
					continue
				}
				if _, ok := seen[room]; ok {
					continue
				}
				kvCtx2, kvCancel2 := kvTimeoutCtx()
				snap, err := h.membershipStore.GetRoomMembers(kvCtx2, room)
				kvCancel2()
				if err != nil {
					continue
				}
				hasStream := false
				for _, m := range snap.Members {
					if m.Stream != "" {
						hasStream = true
						break
					}
				}
				if !hasStream {
					continue
				}
				seen[room] = struct{}{}
				out = append(out, room)
			}
		}
		kvCancel()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Streams 返回指定 room 下登记的 stream 名集合（RoomRegistry 实现）。
// 本地 cache 优先，再并入 membership KV 中远端成员的 stream。
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描同后缀 domain 房。

// Streams 返回指定 room 下登记的 stream 名集合（RoomRegistry 实现）。
// 本地 cache 优先，再并入 membership KV 中远端成员的 stream。
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描同后缀 domain 房。
func (h *Hub) Streams(room string) []string {
	h.mu.RLock()
	seen := make(map[string]struct{})
	out := make([]string, 0)
	if !strings.Contains(room, ":") {
		// 纯逻辑名：扫描所有同后缀的 domain 房 + 平台房
		for rk, streams := range h.roomStreams {
			if _, r := splitRoomKey(rk); r == room || rk == room {
				for s := range streams {
					if s == "" {
						continue
					}
					if _, exists := seen[s]; !exists {
						seen[s] = struct{}{}
						out = append(out, s)
					}
				}
			}
		}
	} else {
		// 复合键：精确匹配
		if rstreams, ok := h.roomStreams[room]; ok {
			for s := range rstreams {
				if s == "" {
					continue
				}
				if _, exists := seen[s]; !exists {
					seen[s] = struct{}{}
					out = append(out, s)
				}
			}
		}
	}
	h.mu.RUnlock()
	if h.membershipStore != nil {
		kvCtx, kvCancel := kvTimeoutCtx()
		if snap, err := h.membershipStore.GetRoomMembers(kvCtx, room); err == nil {
			for _, m := range snap.Members {
				if m.Stream == "" {
					continue
				}
				if _, exists := seen[m.Stream]; !exists {
					seen[m.Stream] = struct{}{}
					out = append(out, m.Stream)
				}
			}
		}
		kvCancel()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ClearRoom 清除指定 room 的 stream 聚合登记（RoomRegistry 实现）。
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描匹配所有同后缀的 domain 房。

// ClearRoom 清除指定 room 的 stream 聚合登记（RoomRegistry 实现）。
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描匹配所有同后缀的 domain 房。
func (h *Hub) ClearRoom(room string) {
	streams := make([]string, 0)
	h.mu.Lock()
	if !strings.Contains(room, ":") {
		// 纯逻辑名：清除所有同后缀的 domain 房 stream 登记
		for rk, local := range h.roomStreams {
			if _, rn := splitRoomKey(rk); rn == room || rk == room {
				for s := range local {
					streams = append(streams, s)
					delete(h.streamRoomCache, s)
				}
				delete(h.roomStreams, rk)
			}
		}
		for rk := range h.streamByIdentity {
			if _, r := splitRoomKey(rk); r == room || rk == room {
				delete(h.streamByIdentity, rk)
			}
		}
	} else {
		// 复合键：精确匹配
		if local, ok := h.roomStreams[room]; ok {
			for s := range local {
				streams = append(streams, s)
				delete(h.streamRoomCache, s)
			}
			delete(h.roomStreams, room)
		}
		delete(h.streamByIdentity, room)
	}
	h.mu.Unlock()
	for _, s := range streams {
		if s != "" {
			h.syncStreamDelete(s)
		}
	}
	h.syncRoomToStore(room)
}

// StreamForIdentity 返回 join 时按 identity 实际登记的 stream 名（RoomRegistry 实现）。
// 未登记返 ok=false，调用方（SRS RemoveParticipant）降级反算命名约定保持兼容。

// StreamForIdentity 返回 join 时按 identity 实际登记的 stream 名（RoomRegistry 实现）。
// 未登记返 ok=false，调用方（SRS RemoveParticipant）降级反算命名约定保持兼容。
func (h *Hub) StreamForIdentity(room, identity string) (string, bool) {
	h.mu.RLock()
	if identities, ok := h.streamByIdentity[room]; ok {
		if s, ok := identities[identity]; ok && s != "" {
			h.mu.RUnlock()
			return s, true
		}
	}
	h.mu.RUnlock()
	if room == "" || identity == "" || h.membershipStore == nil {
		return "", false
	}
	kvCtx, kvCancel := kvTimeoutCtx()
	snap, err := h.membershipStore.GetRoomMembers(kvCtx, room)
	kvCancel()
	if err != nil {
		return "", false
	}
	for _, m := range snap.Members {
		if m.Identity == identity && m.Stream != "" {
			return m.Stream, true
		}
	}
	return "", false
}

// IdentityForStream 返回登记该 stream 的 identity（RoomRegistry 实现）。
// 未登记返 ok=false，调用方（SRS ListParticipants）可降级返回 client id。

// IdentityForStream 返回登记该 stream 的 identity（RoomRegistry 实现）。
// 未登记返 ok=false，调用方（SRS ListParticipants）可降级返回 client id。
func (h *Hub) IdentityForStream(room, stream string) (string, bool) {
	h.mu.RLock()
	if identities, ok := h.streamByIdentity[room]; ok {
		for identity, st := range identities {
			if st == stream && identity != "" {
				h.mu.RUnlock()
				return identity, true
			}
		}
	}
	h.mu.RUnlock()
	if stream == "" || h.membershipStore == nil {
		return "", false
	}
	// Prefer stream KV (authoritative stream→identity).
	kvCtx, kvCancel := kvTimeoutCtx()
	r, identity, err := h.membershipStore.GetStream(kvCtx, stream)
	kvCancel()
	if err == nil && identity != "" && (room == "" || r == room || r == "") {
		return identity, true
	}
	if room == "" {
		return "", false
	}
	kvCtx2, kvCancel2 := kvTimeoutCtx()
	snap, err := h.membershipStore.GetRoomMembers(kvCtx2, room)
	kvCancel2()
	if err != nil {
		return "", false
	}
	for _, m := range snap.Members {
		if m.Stream == stream && m.Identity != "" {
			return m.Identity, true
		}
	}
	return "", false
}
