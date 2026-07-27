package signal

import (
	"context"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	socketio "github.com/googollee/go-socket.io"
)

// messageSendPayload is the client→server payload for message:send.
type messageSendPayload struct {
	Room    string `json:"room"`
	Content string `json:"content"`
	Text    string `json:"text"`
	ReplyTo string `json:"replyTo,omitempty"`
}

// messageSender abstracts MessageService for the bridge.
type messageSender interface {
	Send(ctx context.Context, in service.MessageSendInput) (*service.MessageDTO, error)
}

// OnMessageSend handles message:send events from clients.
// Validates the sender is a room member, then delegates to MessageService.
// Fanout (message:new) is handled by MessageService → EventBus, not here.
func (h *Hub) OnMessageSend(s socketio.Conn, data string) {
	if h.messageSvc == nil {
		return
	}

	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return
	}

	text := req.Content
	if text == "" {
		text = req.Text
	}
	if text == "" {
		return
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return
	}

	// Must be a member of the room
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
		return
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

	_, _ = h.messageSvc.Send(context.Background(), service.MessageSendInput{
		RoomKey:        req.Room,
		SenderIdentity: identity,
		SenderDisplay:  display,
		SenderRole:     role,
		Content:        text,
		ReplyToID:      req.ReplyTo,
	})
}
