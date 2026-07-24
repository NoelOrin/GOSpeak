package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"fmt"

	socketio "github.com/googollee/go-socket.io"
)

// messageSender abstracts message operations for the signal layer.
// Satisfied by *service.MessageService.
type messageSender interface {
	Send(roomUUID, authorID, content, replyTo, clientNonce string, mentions []string) (*service.MessageDTO, error)
	Edit(roomUUID, messageUUID, authorID, content string) (*service.MessageDTO, error)
	Delete(roomUUID, messageUUID, actorID string, canDeleteOthers bool) error
	React(roomUUID, messageUUID, userID, emoji string) error
	Unreact(roomUUID, messageUUID, userID, emoji string) error
}

// SetMessageService injects the message service into the Hub.
func (h *Hub) SetMessageService(svc messageSender) {
	h.msgSvc = svc
}

// messageSendPayload is the client->server payload for message:send.
type messageSendPayload struct {
	Room        string   `json:"room"`
	Content     string   `json:"content"`
	ReplyTo     string   `json:"reply_to,omitempty"`
	Mentions    []string `json:"mentions,omitempty"`
	ClientNonce string   `json:"client_nonce,omitempty"`
}

// messageMutatePayload is the client->server payload for message operations
// targeting an existing message (edit/delete/react/unreact).
type messageMutatePayload struct {
	Room        string `json:"room"`
	MessageUUID string `json:"message_uuid"`
	Content     string `json:"content,omitempty"`
	Emoji       string `json:"emoji,omitempty"`
}

// resolveMessageRoom looks up a DB room by name, validates the conn slot,
// and returns (roomUUID, roomType) or an ack error string.
func (h *Hub) resolveMessageRoom(s socketio.Conn, roomName string) (roomUUID, roomType string, ackErr string) {
	identity := claimsIdentity(s)
	if identity == "" {
		return "", "", `{"error":"unauthorized"}`
	}

	if h.roomStore == nil {
		return "", "", `{"error":"room store unavailable"}`
	}
	dbRoom, err := h.roomStore.GetByName(roomName)
	if err != nil || dbRoom == nil {
		return "", "", fmt.Sprintf(`{"error":"room not found: %s"}`, roomName)
	}
	roomUUID = dbRoom.UUID
	roomType = model.NormalizeRoomType(dbRoom.Type)

	// Validate conn slot: must have joined this room
	h.mu.RLock()
	slots := h.connSlots[s.ID()]
	allowed := false
	if slots != nil {
		if roomType == model.RoomTypeText {
			allowed = slots.TextRoom == roomName
		} else {
			allowed = slots.VoiceRoom == roomName
		}
	}
	h.mu.RUnlock()
	if !allowed {
		return "", "", fmt.Sprintf(`{"error":"not in room: %s"}`, roomName)
	}

	return roomUUID, roomType, ""
}

// checkMessagePerm returns an ack error string if the connection lacks message:send.
func (h *Hub) checkMessagePerm(s socketio.Conn) string {
	if h.permChecker == nil {
		return ""
	}
	ctx := s.Context()
	if claims, ok := ctx.(*pkg.Claims); ok {
		if !h.permChecker.HasPermission(claims.Role, permcode.PermMessageSend) {
			return `{"error":"permission denied"}`
		}
	}
	return ""
}

// OnMessageSend handles message:send events from clients.
func (h *Hub) OnMessageSend(s socketio.Conn, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(s); ack != "" {
		return ack, nil
	}

	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{"error": "room is required"})
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(s, req.Room)
	if ackErr != "" {
		return ackErr, nil
	}

	if _, err := h.msgSvc.Send(roomUUID, identity, req.Content, req.ReplyTo, req.ClientNonce, req.Mentions); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageEdit handles message:edit events from clients.
func (h *Hub) OnMessageEdit(s socketio.Conn, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(s); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" {
		return marshalAck(map[string]interface{}{"error": "room and message_uuid are required"})
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(s, req.Room)
	if ackErr != "" {
		return ackErr, nil
	}

	if _, err := h.msgSvc.Edit(roomUUID, req.MessageUUID, identity, req.Content); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageDelete handles message:delete events from clients.
func (h *Hub) OnMessageDelete(s socketio.Conn, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(s); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" {
		return marshalAck(map[string]interface{}{"error": "room and message_uuid are required"})
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(s, req.Room)
	if ackErr != "" {
		return ackErr, nil
	}

	// canDeleteOthers: check permission via permChecker
	canDeleteOthers := false
	if ctx := s.Context(); ctx != nil {
		if claims, ok := ctx.(*pkg.Claims); ok {
			if h.permChecker != nil {
				canDeleteOthers = h.permChecker.HasPermission(claims.Role, permcode.PermMessageDeleteOthers)
			}
		}
	}

	if err := h.msgSvc.Delete(roomUUID, req.MessageUUID, identity, canDeleteOthers); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageReact handles message:react events from clients.
func (h *Hub) OnMessageReact(s socketio.Conn, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(s); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" || req.Emoji == "" {
		return marshalAck(map[string]interface{}{"error": "room, message_uuid, and emoji are required"})
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(s, req.Room)
	if ackErr != "" {
		return ackErr, nil
	}

	if err := h.msgSvc.React(roomUUID, req.MessageUUID, identity, req.Emoji); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageUnreact handles message:unreact events from clients.
func (h *Hub) OnMessageUnreact(s socketio.Conn, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(s); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" || req.Emoji == "" {
		return marshalAck(map[string]interface{}{"error": "room, message_uuid, and emoji are required"})
	}

	identity := claimsIdentity(s)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(s, req.Room)
	if ackErr != "" {
		return ackErr, nil
	}

	if err := h.msgSvc.Unreact(roomUUID, req.MessageUUID, identity, req.Emoji); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}
