package signal

import (
	"context"
	"log"
)

func (h *Hub) SetEventBus(b eventBus) {
	h.eventBus = b
}

func (h *Hub) SetCleanupPublisher(p cleanupPublisher) {
	h.cleanupPub = p
}

func (h *Hub) SetMessageService(svc messageSender) {
	h.msgSvc = svc
}

func (h *Hub) SetConversationService(svc conversationSender) {
	h.convSvc = svc
}

// IsRoomMember checks if identity is currently in room's member list.
// room 支持复合键（domainUUID:roomName）或逻辑名；逻辑名回退扫描同后缀 domain 房。

func (h *Hub) publishRoom(room, event string, data interface{}) {
	// eventBus 由启动路径初始化（embedded NATS / external NATS），生产环境始终在线。
	// 下面的 BroadcastToRoom 直调仅在 eventBus 为 nil 时使用（仅测试机的裸 WS 路径）。
	if h.eventBus != nil {
		if err := h.eventBus.PublishRoom(context.Background(), room, event, data); err != nil {
			log.Printf("[Signal] eventbus publish room %s %s: %v", room, event, err)
		}
		return
	}
	if h.fanout != nil {
		h.fanout.BroadcastToRoom(room, event, data)
	}
}

func (h *Hub) publishNamespace(event string, data interface{}) {
	if h.eventBus != nil {
		if err := h.eventBus.PublishNamespace(context.Background(), event, data); err != nil {
			log.Printf("[Signal] eventbus publish ns %s: %v", event, err)
		}
		return
	}
	if h.fanout != nil {
		h.fanout.BroadcastToNamespace(event, data)
	}
}

func (h *Hub) BroadcastToRoom(room string, event string, data interface{}) {
	h.publishRoom(room, event, data)
}

// removeParticipantSafe 从 SFU 移除 participant。
// 信令层踢人始终先生效；这里只负责媒体层 enforcement。
// 返回 hard|degraded|soft，供事件 payload 透出。
