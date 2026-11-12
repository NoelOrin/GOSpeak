package signal

import "time"

// kickBanTTL 是被踢者禁止立即重连同一房间的冷却时长。
const kickBanTTL = 60 * time.Second

// kickBanKey 生成踢出冷却 key（room 复合键 + identity）。
func kickBanKey(room, identity string) string {
	return room + "|" + identity
}

// blockRejoin 记录被踢者短时禁止重连同一房间。
func (h *Hub) blockRejoin(room, identity string) {
	if room == "" || identity == "" {
		return
	}
	h.mu.Lock()
	if h.kickBans == nil {
		h.kickBans = make(map[string]time.Time)
	}
	h.kickBans[kickBanKey(room, identity)] = time.Now().Add(kickBanTTL)
	h.mu.Unlock()
}

// isRejoinBlocked 检查被踢者是否仍在冷却窗口内。
func (h *Hub) isRejoinBlocked(room, identity string) bool {
	if room == "" || identity == "" {
		return false
	}
	h.mu.RLock()
	expires, ok := h.kickBans[kickBanKey(room, identity)]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		h.mu.Lock()
		delete(h.kickBans, kickBanKey(room, identity))
		h.mu.Unlock()
		return false
	}
	return true
}
