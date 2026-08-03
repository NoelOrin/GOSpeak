package signal

import (
	"fmt"

	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/service"
	"GOSpeak/internal/ws"
)

// messageSender abstracts message operations for the signal layer.
// Satisfied by *service.MessageService.
type messageSender interface {
	Send(roomUUID string, actor service.MessageActor, content, replyTo, clientNonce string, mentions []string) (*service.MessageDTO, error)
	Edit(roomUUID, messageUUID string, actor service.MessageActor, content string) (*service.MessageDTO, error)
	Delete(roomUUID, messageUUID string, actor service.MessageActor, canDeleteOthers bool) error
	React(roomUUID, messageUUID string, actor service.MessageActor, emoji string) error
	Unreact(roomUUID, messageUUID string, actor service.MessageActor, emoji string) error
}

// messageSendPayload is the client->server payload for message:send.
type messageSendPayload struct {
	Room        string   `json:"room"`
	DomainUUID  string   `json:"domain_uuid,omitempty"`
	Content     string   `json:"content"`
	ReplyTo     string   `json:"reply_to,omitempty"`
	Mentions    []string `json:"mentions,omitempty"`
	ClientNonce string   `json:"client_nonce,omitempty"`
}

// messageMutatePayload is the client->server payload for message operations
// targeting an existing message (edit/delete/react/unreact).
type messageMutatePayload struct {
	Room        string `json:"room"`
	DomainUUID  string `json:"domain_uuid,omitempty"`
	MessageUUID string `json:"message_uuid"`
	Content     string `json:"content,omitempty"`
	Emoji       string `json:"emoji,omitempty"`
}

// resolveMessageRoom looks up a DB room by name, validates the conn slot,
// and returns (roomUUID, roomType) or an ack error string.
func (h *Hub) resolveMessageRoom(c ws.ClientMessenger, roomName, domainUUID string) (roomUUID, roomType string, ackErr string) {
	identity := clientIdentity(c)
	if identity == "" {
		return "", "", `{"error":"unauthorized"}`
	}
	if c == nil || c.Claims() == nil || c.Claims().UserUUID == "" {
		return "", "", `{"error":"unauthorized"}`
	}

	if h.roomStore == nil {
		return "", "", `{"error":"room store unavailable"}`
	}
	var dbRoom *model.Room
	var err error
	if domainUUID != "" {
		dbRoom, err = h.roomStore.GetByDomainAndName(domainUUID, roomName)
	} else {
		dbRoom, err = h.roomStore.GetByName(roomName)
	}
	if err != nil || dbRoom == nil {
		return "", "", fmt.Sprintf(`{"error":"room not found: %s"}`, roomName)
	}
	roomUUID = dbRoom.UUID
	roomType = model.NormalizeRoomType(dbRoom.Type)

	// Validate conn slot: must have joined this room
	h.mu.RLock()
	slots := h.connSlots[c.ID()]
	allowed := false
	if slots != nil {
		if roomType == model.RoomTypeText {
			allowed = slots.TextRoom == roomKey(domainUUID, roomName)
		} else {
			allowed = slots.VoiceRoom == roomKey(domainUUID, roomName)
		}
	}
	h.mu.RUnlock()
	if !allowed {
		return "", "", fmt.Sprintf(`{"error":"not in room: %s"}`, roomName)
	}

	return roomUUID, roomType, ""
}

// checkMessagePerm returns an ack error string if the connection lacks message:send.
func (h *Hub) checkMessagePerm(c ws.ClientMessenger) string {
	claims := c.Claims()
	if claims == nil {
		return ""
	}
	role := claims.Role
	if role == "" && h.userStore != nil {
		if user, err := h.userStore.GetByName(clientIdentity(c)); err == nil && user != nil {
			role = user.Role
		}
	}
	if !middleware.PermissionGranted(claims, role, permcode.PermMessageSend, h.permChecker) {
		return `{"error":"permission denied"}`
	}
	return ""
}

func messageActorFromConn(c ws.ClientMessenger) (service.MessageActor, bool) {
	identity := clientIdentity(c)
	if identity == "" || c == nil || c.Claims() == nil || c.Claims().UserUUID == "" {
		return service.MessageActor{}, false
	}
	return service.MessageActor{Identity: identity, UserUUID: c.Claims().UserUUID}, true
}

// OnMessageSend handles message:send events from clients.
func (h *Hub) OnMessageSend(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(c); ack != "" {
		return ack, nil
	}

	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return marshalAck(map[string]interface{}{"error": "room is required"})
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if ackErr != "" {
		return ackErr, nil
	}

	if _, err := h.msgSvc.Send(roomUUID, actor, req.Content, req.ReplyTo, req.ClientNonce, req.Mentions); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageEdit handles message:edit events from clients.
func (h *Hub) OnMessageEdit(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(c); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" {
		return marshalAck(map[string]interface{}{"error": "room and message_uuid are required"})
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if ackErr != "" {
		return ackErr, nil
	}

	if _, err := h.msgSvc.Edit(roomUUID, req.MessageUUID, actor, req.Content); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageDelete handles message:delete events from clients.
func (h *Hub) OnMessageDelete(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(c); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" {
		return marshalAck(map[string]interface{}{"error": "room and message_uuid are required"})
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if ackErr != "" {
		return ackErr, nil
	}

	// canDeleteOthers: check permission via permChecker
	canDeleteOthers := false
	if claims := c.Claims(); claims != nil && h.permChecker != nil {
		canDeleteOthers = h.permChecker.HasPermission(claims.Role, permcode.PermMessageDeleteOthers)
	}

	if err := h.msgSvc.Delete(roomUUID, req.MessageUUID, actor, canDeleteOthers); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageReact handles message:react events from clients.
func (h *Hub) OnMessageReact(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(c); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" || req.Emoji == "" {
		return marshalAck(map[string]interface{}{"error": "room, message_uuid, and emoji are required"})
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if ackErr != "" {
		return ackErr, nil
	}

	if err := h.msgSvc.React(roomUUID, req.MessageUUID, actor, req.Emoji); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}

// OnMessageUnreact handles message:unreact events from clients.
func (h *Hub) OnMessageUnreact(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return marshalAck(map[string]interface{}{"error": "message service unavailable"})
	}
	if ack := h.checkMessagePerm(c); ack != "" {
		return ack, nil
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" || req.Emoji == "" {
		return marshalAck(map[string]interface{}{"error": "room, message_uuid, and emoji are required"})
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	roomUUID, _, ackErr := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if ackErr != "" {
		return ackErr, nil
	}

	if err := h.msgSvc.Unreact(roomUUID, req.MessageUUID, actor, req.Emoji); err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}
	return marshalAck(map[string]interface{}{"success": true})
}
