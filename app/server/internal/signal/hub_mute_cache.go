package signal

import "time"

type muteCacheEntry struct {
	muted   bool
	expires time.Time
}

func (h *Hub) muteCacheGet(identity string) (bool, bool) {
	h.mu.RLock()
	entry, ok := h.muteCache[identity]
	h.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		return false, false
	}
	return entry.muted, true
}

func (h *Hub) muteCacheSet(identity string, muted bool) {
	now := time.Now()
	h.mu.Lock()
	if h.muteCache == nil {
		h.muteCache = make(map[string]muteCacheEntry)
	}
	// 惰性清理：写入时顺手淘汰过期条目，无需后台 goroutine。
	for id, entry := range h.muteCache {
		if now.After(entry.expires) {
			delete(h.muteCache, id)
		}
	}
	h.muteCache[identity] = muteCacheEntry{muted: muted, expires: now.Add(5 * time.Second)}
	h.mu.Unlock()
}
