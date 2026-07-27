package signal

import (
	"context"
	"sync"
	"time"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	socketio "github.com/googollee/go-socket.io"
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

func (h *Hub) OnMessageSend(s socketio.Conn, data string) (string, error) {
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

	identity := claimsIdentity(s)
	if identity == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "not authenticated")
	}

	if !h.allowMessageSend(identity) {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "rate limited")
	}

	h.mu.RLock()
	room, exists := h.rooms[req.Room]
	var member *MemberInfo
	if exists {
		for _, m := range room.Members {
			if m.Identity == identity {
				member = m
				break
			}
		}
	}
	h.mu.RUnlock()

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
	if ctx := s.Context(); ctx != nil {
		if claims, ok := ctx.(*pkg.Claims); ok && claims.Role != "" {
			role = claims.Role
		}
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
