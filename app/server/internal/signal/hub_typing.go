package signal

import (
	"GOSpeak/internal/policy"
	"GOSpeak/internal/ws"
	"GOSpeak/internal/logger"
	"time"
)

const (
	// typingBroadcastInterval：持续输入时最多每 2s 向房间广播一次「仍在输入」。
	// 前端收到后会刷新本地 5s TTL，因此 2s 的心跳足够维持提示不闪烁。
	typingBroadcastInterval = 2 * time.Second
	// typingStaleAfter：超过 30s 没有任何上报（例如浏览器崩溃）就回收状态。
	typingStaleAfter = 30 * time.Second
)

type typingEntry struct {
	active   bool
	lastSent time.Time
	lastSeen time.Time
}

// OnTyping 处理文字房「正在输入」上报并广播给同房间成员。
//
// 设计取舍：
//   - 上行/下行共用一个 "typing" 事件，payload 带 identity，前端过滤掉自己；
//     这样能继续走 publishRoom 的 eventBus 路径，支持多实例部署。
//   - 身份一律取连接上已鉴权的 identity，忽略客户端传来的 identity，防止伪造他人。
//   - 不落库、不发 ACK、不保证送达；纯 UI 提示，失败无需重试。
//   - 成员校验 + 禁言 fail-closed 与 OnMemberSpeaking 保持一致。
func (h *Hub) OnTyping(c ws.ClientMessenger, data string) {
	var req struct {
		Room       string `json:"room"`
		DomainUUID string `json:"domain_uuid,omitempty"`
		Typing     bool   `json:"typing"`
	}
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return
	}

	key := roomKey(req.DomainUUID, req.Room)

	h.mu.RLock()
	room, ok := h.rooms[key]
	var identity string
	if ok {
		if caller := room.Members[c.ID()]; caller != nil {
			identity = caller.Identity
		}
	}
	h.mu.RUnlock()
	if identity == "" {
		return
	}

	// 策略复查：key 已通过服务端成员表校验，其域前缀是权威域上下文。
	// 被封禁的身份不能在保持连接期间继续制造可见活动。
	domain, _ := splitRoomKey(key)
	if d := h.checkWSPolicy(c, policy.Action(EventTyping), domain); !d.Allowed {
		return
	}

	// 被禁言（或禁言状态查询失败）时不上报，避免绕过禁言制造可见活动。
	if h.muteStore != nil {
		if muted, cached := h.muteCacheGet(identity); cached {
			if muted {
				return
			}
		} else {
			muted, _, muteErr := h.muteStore.IsMutedByIdentity(identity)
			if muteErr != nil {
				logger.WithComponent("Signal").Infof("[signal] OnTyping IsMutedByIdentity error: identity=%q err=%v", identity, muteErr)
				return
			}
			h.muteCacheSet(identity, muted)
			if muted {
				return
			}
		}
	}

	now := time.Now()
	stateKey := key + ":" + identity

	h.typingMu.Lock()
	if h.typing == nil {
		h.typing = make(map[string]*typingEntry)
	}
	for k, entry := range h.typing {
		if now.Sub(entry.lastSeen) > typingStaleAfter {
			delete(h.typing, k)
		}
	}
	entry := h.typing[stateKey]
	if entry == nil {
		entry = &typingEntry{}
		h.typing[stateKey] = entry
	}
	entry.lastSeen = now

	broadcast := false
	if req.Typing {
		// 首次开始输入立即广播；持续输入按心跳节流向房间续期。
		if !entry.active || now.Sub(entry.lastSent) >= typingBroadcastInterval {
			entry.active = true
			entry.lastSent = now
			broadcast = true
		}
	} else if entry.active {
		// 只有「从输入中变为停止」才广播，重复 stop 不打扰房间。
		entry.active = false
		entry.lastSent = now
		broadcast = true
	}
	h.typingMu.Unlock()

	if broadcast {
		h.publishRoom(key, EventTyping, map[string]interface{}{
			"room":        req.Room,
			"domain_uuid": req.DomainUUID,
			"identity":    identity,
			"typing":      req.Typing,
		})
	}
}
