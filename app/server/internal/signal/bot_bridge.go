package signal

import (
	"GOSpeak/internal/pkg"
	"encoding/json"
	"fmt"
	"time"

	socketio "github.com/googollee/go-socket.io"
)

// botMessagePayload is the client→server payload for bot:command and bot:message.
type botMessagePayload struct {
	Room    string `json:"room"`
	Text    string `json:"text"`
	Content string `json:"content"`
	ReplyTo string `json:"replyTo,omitempty"`
}

// broadcastBotMessage is the server→client payload for bot:command and bot:message broadcasts.
type broadcastBotMessage struct {
	Room      string `json:"room"`
	Content   string `json:"content"`
	ReplyTo   string `json:"replyTo,omitempty"`
	MessageID string `json:"messageId"`
	From      struct {
		Identity    string `json:"identity"`
		DisplayName string `json:"displayName"`
		Role        string `json:"role"`
	} `json:"from"`
	Timestamp int64 `json:"timestamp"`
}

// PublishBotCommand handles bot:command events from clients.
// Bot commands are text-only (≤500 chars) and only accepted from JWT-authenticated
// members already present in the room.
func (h *Hub) PublishBotCommand(s socketio.Conn, data string) {
	h.publishBotMessage(s, data, EventBotCommand)
}

// PublishBotMessage handles bot:message events from clients.
// Same validation as PublishBotCommand but supports optional replyTo.
func (h *Hub) PublishBotMessage(s socketio.Conn, data string) {
	h.publishBotMessage(s, data, EventBotMessage)
}

func (h *Hub) publishBotMessage(s socketio.Conn, data string, event string) {
	var req botMessagePayload
	if err := parseJSON(data, &req); err != nil {
		return
	}

	// Validate room
	if req.Room == "" {
		return
	}

	// Validate text/content length
	text := req.Text
	if text == "" {
		text = req.Content
	}
	if text == "" || len([]rune(text)) > 500 {
		return
	}

	// JWT identity
	callerIdentity := claimsIdentity(s)
	if callerIdentity == "" {
		return
	}

	// Must be in the room
	h.mu.RLock()
	room, exists := h.rooms[req.Room]
	if !exists {
		h.mu.RUnlock()
		return
	}
	var member *MemberInfo
	for _, m := range room.Members {
		if m.Identity == callerIdentity {
			member = m
			break
		}
	}
	h.mu.RUnlock()

	if member == nil {
		return
	}

	// Build broadcast payload
	var claims *pkg.Claims
	if ctx := s.Context(); ctx != nil {
		claims, _ = ctx.(*pkg.Claims)
	}
	displayName := member.DisplayName
	if displayName == "" {
		displayName = member.Name
	}
	role := "member"
	if claims != nil && claims.Role != "" {
		role = claims.Role
	}

	broadcast := broadcastBotMessage{
		Room:      req.Room,
		Content:   text,
		ReplyTo:   req.ReplyTo,
		MessageID: fmt.Sprintf("%s-%d", callerIdentity, time.Now().UnixMilli()),
		Timestamp: time.Now().UnixMilli(),
	}
	broadcast.From.Identity = callerIdentity
	broadcast.From.DisplayName = displayName
	broadcast.From.Role = role

	payload, _ := json.Marshal(broadcast)

	// Broadcast to entire room
	if h.server != nil {
		h.server.BroadcastToRoom("/", req.Room, event, string(payload))
	}
}
