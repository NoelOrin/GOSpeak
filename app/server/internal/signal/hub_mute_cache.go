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
	h.mu.Lock()
	if h.muteCache == nil {
		h.muteCache = make(map[string]muteCacheEntry)
	}
	h.muteCache[identity] = muteCacheEntry{muted: muted, expires: time.Now().Add(5 * time.Second)}
	h.mu.Unlock()
}

func (h *Hub) cleanupExpiredMuteCache() {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	for identity, entry := range h.muteCache {
		if now.After(entry.expires) {
			delete(h.muteCache, identity)
		}
	}
}
