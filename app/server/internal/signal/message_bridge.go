package signal

import (
	"context"
	"sync"
	"time"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"GOSpeak/internal/ws"
)

const messageRateInterval = 250 * time.Millisecond

type messageSendPayload struct {
	Room    string `json:"room"`
	Content string `json:"content"`
	Text    string `json:"text"`
	ReplyTo string `json:"replyTo,omitempty"`
}

type messageSender interface {
	Send(ctx context.Context, in service.MessageSendInput) (*service.MessageDTO, error)
}

type msgRateEntry struct {
	mu   sync.Mutex
	last time.Time
}

func (h *Hub) allowMessageSend(identity string) bool {
	v, _ := h.msgRate.LoadOrStore(identity, &msgRateEntry{})
	e := v.(*msgRateEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if now.Sub(e.last) < messageRateInterval {
		return false
	}
	e.last = now
	return true
}

func (h *Hub) OnMessageSend(c ws.ClientMessenger, data string) (string, error) {
	if h.messageSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "service unavailable")
	}

	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "invalid request")
	}

	text := req.Content
	if text == "" {
		text = req.Text
	}
	if text == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "empty content")
	}

	identity := clientIdentity(c)
	if identity == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "not authenticated")
	}

	if !h.allowMessageSend(identity) {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "rate limited")
	}

	var member *MemberInfo

	// 1. 优先读 KV（无锁，跨实例可见）
	if h.membershipStore != nil {
		if snap, err := h.membershipStore.GetRoomMembers(context.Background(), req.Room); err == nil {
			for _, rec := range snap.Members {
				if rec.Identity == identity {
					member = &MemberInfo{Identity: rec.Identity, Name: rec.Identity}
					break
				}
			}
		}
	}

	// 2. KV 未命中 → fallback 本地 map（需 RLock）
	if member == nil {
		h.mu.RLock()
		if room, exists := h.rooms[req.Room]; exists {
			for _, m := range room.Members {
				if m.Identity == identity {
					member = m
					break
				}
			}
		}
		h.mu.RUnlock()
	}

	// 3. 确认不在房间里
	if member == nil {
		return "", pkg.NewAppError(pkg.NOT_FOUND, "not in room")
	}

	if h.muteStore != nil {
		muted, _, muteErr := h.muteStore.IsMutedByIdentity(identity)
		if muteErr == nil && muted {
			return "", pkg.NewAppError(pkg.FORBIDDEN, "muted")
		}
	}

	display := member.DisplayName
	if display == "" {
		display = member.Name
	}

	role := "member"
	if claims := c.Claims(); claims != nil && claims.Role != "" {
		role = claims.Role
	}

	dto, err := h.messageSvc.Send(context.Background(), service.MessageSendInput{
		RoomKey:        req.Room,
		SenderIdentity: identity,
		SenderDisplay:  display,
		SenderRole:     role,
		Content:        text,
		ReplyToID:      req.ReplyTo,
	})
	if err != nil {
		return "", err
	}
	return dto.ID, nil
}
