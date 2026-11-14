package signal

import (
	"log"
	"time"
)

const (
	// emptyRoomGracePeriod 是空房的对账宽限期：空房持续超过该时长才删除，
	// 避免误删正在 join、LiveKit 侧尚未计入参与者的房间。
	emptyRoomGracePeriod = 60 * time.Second
	// defaultEmptyRoomReconcileInterval 是对账扫描周期。
	defaultEmptyRoomReconcileInterval = 60 * time.Second
)

// reconcileEmptyRooms 对账 SFU 房间列表与共享成员状态，补删残留空房。
// now 注入时间源便于测试；返回已删除的房间数。
func (h *Hub) reconcileEmptyRooms(now func() int64) int {
	if h.sfuProvider == nil {
		return 0
	}
	caps := h.sfuProvider.Capabilities()
	if !caps.ListRooms || !caps.DeleteRoom {
		return 0
	}
	rooms, err := h.sfuProvider.ListRooms()
	if err != nil {
		log.Printf("[Signal] reconcile list SFU rooms: %v", err)
		return 0
	}

	nowMS := now()
	deleted := 0

	h.reconcileMu.Lock()
	emptyNow := make(map[string]int64, len(rooms))
	for _, room := range rooms {
		if room.Name == "" || room.MemberCount != 0 {
			continue
		}
		if first, ok := h.emptyRoomSeen[room.Name]; ok {
			emptyNow[room.Name] = first
			continue
		}
		emptyNow[room.Name] = nowMS
	}
	h.emptyRoomSeen = emptyNow
	due := make([]string, 0, len(emptyNow))
	for name, first := range emptyNow {
		if nowMS-first >= emptyRoomGracePeriod.Milliseconds() {
			due = append(due, name)
		}
	}
	h.reconcileMu.Unlock()

	for _, name := range due {
		if h.membershipStore != nil {
			hasRemote, err := h.roomHasLiveRemoteMember(name)
			if err != nil {
				log.Printf("[Signal] reconcile skip %s: membership read failed: %v", name, err)
				continue
			}
			if hasRemote {
				continue
			}
		}
		h.forceDeleteRoom(name)
		h.reconcileMu.Lock()
		delete(h.emptyRoomSeen, name)
		h.reconcileMu.Unlock()
		deleted++
		log.Printf("[Signal] reconcile deleted empty SFU room: %s", name)
	}
	return deleted
}

// StartEmptyRoomReconciler 启动周期对账任务，返回 stop 函数（可重复调用）。
func (h *Hub) StartEmptyRoomReconciler(interval time.Duration) func() {
	if interval <= 0 {
		interval = defaultEmptyRoomReconcileInterval
	}
	h.reconcileMu.Lock()
	if h.reconcileStop != nil {
		h.reconcileMu.Unlock()
		return func() {}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	h.reconcileStop = stop
	h.reconcileDone = done
	h.reconcileMu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				h.reconcileEmptyRooms(func() int64 { return time.Now().UnixMilli() })
			}
		}
	}()

	return func() {
		h.reconcileMu.Lock()
		stop := h.reconcileStop
		done := h.reconcileDone
		h.reconcileStop = nil
		h.reconcileDone = nil
		h.reconcileMu.Unlock()
		if stop != nil {
			close(stop)
			if done != nil {
				<-done
			}
		}
	}
}

// StopEmptyRoomReconciler 停止对账 goroutine，供优雅关闭调用。
func (h *Hub) StopEmptyRoomReconciler() {
	h.reconcileMu.Lock()
	stop := h.reconcileStop
	done := h.reconcileDone
	h.reconcileStop = nil
	h.reconcileDone = nil
	h.reconcileMu.Unlock()
	if stop != nil {
		close(stop)
		if done != nil {
			<-done
		}
	}
}
