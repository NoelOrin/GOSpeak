package signal

import (
	"encoding/json"
	"fmt"
	"time"

	"GOSpeak/internal/ws"
)

// botMessagePayload is the client→server payload for bot:command and bot:message.
type botMessagePayload struct {
	Room       string `json:"room"`
	DomainUUID string `json:"domain_uuid,omitempty"`
	Text       string `json:"text"`
	Content    string `json:"content"`
	ReplyTo    string `json:"replyTo,omitempty"`
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
func (h *Hub) PublishBotCommand(c ws.ClientMessenger, data string) {
	h.publishBotMessage(c, data, EventBotCommand)
}

// PublishBotMessage handles bot:message events from clients.
// Same validation as PublishBotCommand but supports optional replyTo.
func (h *Hub) PublishBotMessage(c ws.ClientMessenger, data string) {
	h.publishBotMessage(c, data, EventBotMessage)
}

func (h *Hub) publishBotMessage(c ws.ClientMessenger, data string, event string) {
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
	callerIdentity := clientIdentity(c)
	if callerIdentity == "" {
		return
	}

	// Must be in the room
	h.mu.RLock()
	rk := roomKey(req.DomainUUID, req.Room)
	room, exists := h.rooms[rk]
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
	claims := c.Claims()
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
	if h.fanout != nil {
		h.fanout.BroadcastToRoom(rk, event, string(payload))
	}
}
