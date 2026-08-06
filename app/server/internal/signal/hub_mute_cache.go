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
