package signal

import (
	"context"
	"time"

	"GOSpeak/internal/logger"
)

// DefaultDisconnectGracePeriod 是 WS 断连到 SFU 清理的默认宽限期。
// 覆盖常见的信令抖动（切网/锁屏/代理空闲超时/后端滚动重启）时长。
const DefaultDisconnectGracePeriod = 15 * time.Second

// pendingCleanup 是一项被延迟的 SFU 清理任务。
type pendingCleanup struct {
	room     string
	identity string
	deleted  bool
	timer    *time.Timer
}

func pendingCleanupKey(room, identity string) string {
	return room + "\x00" + identity
}

// scheduleDisconnectCleanup 延迟执行一次断连 SFU 清理。
// grace 窗口内同一 identity 重新加入同一房间会取消清理（见 cancelPendingCleanup），
// 避免一次 WS 抖动就把媒体 participant 移除导致「被踢」。
func (h *Hub) scheduleDisconnectCleanup(c disconnectCleanup) {
	if c.identity == "" || h.disconnectGrace <= 0 {
		h.dispatchSFUCleanup(c)
		return
	}
	key := pendingCleanupKey(c.room, c.identity)
	pc := &pendingCleanup{room: c.room, identity: c.identity, deleted: c.deleted}
	h.cleanupMu.Lock()
	if old := h.pendingCleanups[key]; old != nil {
		old.timer.Stop()
	}
	pc.timer = time.AfterFunc(h.disconnectGrace, func() { h.runPendingCleanup(key, pc) })
	h.pendingCleanups[key] = pc
	h.cleanupMu.Unlock()
}

// runPendingCleanup 在宽限期结束后执行清理；若期间已重新加入则跳过。
func (h *Hub) runPendingCleanup(key string, pc *pendingCleanup) {
	h.cleanupMu.Lock()
	cur := h.pendingCleanups[key]
	if cur != pc {
		// 已被新断连任务替换或已取消。
		h.cleanupMu.Unlock()
		return
	}
	delete(h.pendingCleanups, key)
	h.cleanupMu.Unlock()

	if h.roomHasLiveIdentity(pc.room, pc.identity) {
		logger.WithComponent("Signal").Infof(
			"[Signal] disconnect grace: %s rejoined %s, skip sfu cleanup", pc.identity, pc.room,
		)
		return
	}
	h.dispatchSFUCleanup(disconnectCleanup{room: pc.room, identity: pc.identity, deleted: pc.deleted})
}

// cancelPendingCleanup 在成员重新加入房间时调用，取消该 (room, identity) 的延迟清理。
func (h *Hub) cancelPendingCleanup(room, identity string) {
	if identity == "" {
		return
	}
	key := pendingCleanupKey(room, identity)
	h.cleanupMu.Lock()
	if pc := h.pendingCleanups[key]; pc != nil {
		pc.timer.Stop()
		delete(h.pendingCleanups, key)
		logger.WithComponent("Signal").Infof(
			"[Signal] disconnect grace: cancel pending sfu cleanup for %s in %s", identity, room,
		)
	}
	h.cleanupMu.Unlock()
}

// dispatchSFUCleanup 执行实际的 SFU 清理（入队优先，失败降级内联）。
func (h *Hub) dispatchSFUCleanup(c disconnectCleanup) {
	// 空房删除前二次确认本实例房间仍为空/已不存在：宽限窗口内可能有他人已进房，
	// 此时只移除断连者，不删除房间。
	if c.deleted && !h.roomIsEmptyOrAbsent(c.room) {
		c.deleted = false
	}
	if h.cleanupPub != nil {
		if err := h.cleanupPub.PublishSFUCleanup(context.Background(), c.room, c.identity, c.deleted); err != nil {
			logger.WithComponent("Signal").Warnf(
				"[Signal] enqueue sfu cleanup failed, run inline: room=%s err=%v", c.room, err,
			)
			h.runCleanupsInline([]disconnectCleanup{c})
		}
		return
	}
	h.runCleanupsInline([]disconnectCleanup{c})
}

// roomHasLiveIdentity 判断房间当前是否仍有该 identity 的在线成员（本地）。
func (h *Hub) roomHasLiveIdentity(roomKey, identity string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room, ok := h.rooms[roomKey]
	if !ok {
		return false
	}
	return roomLookupIdentity(room, identity) != nil
}

// roomIsEmptyOrAbsent 判断本地房间是否已不存在或没有成员。
func (h *Hub) roomIsEmptyOrAbsent(roomKey string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room, ok := h.rooms[roomKey]
	return !ok || len(room.Members) == 0
}
