package signal

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/protocol"
	message "GOSpeak/internal/pkg/message"
	"GOSpeak/internal/ws"
)

// messageSender abstracts message operations for the signal layer.
// Satisfied by *service.MessageService.
type messageSender interface {
	Send(roomUUID string, actor message.Actor, content, replyTo, clientNonce string, mentions []string) (*message.DTO, error)
	Edit(roomUUID, messageUUID string, actor message.Actor, content string) (*message.DTO, error)
	Delete(roomUUID, messageUUID string, actor message.Actor, canDeleteOthers bool) error
	React(roomUUID, messageUUID string, actor message.Actor, emoji string) error
	Unreact(roomUUID, messageUUID string, actor message.Actor, emoji string) error
}

// 上行 payload 定义已收编至 protocol（MessageSendPayload/MessageMutatePayload）。
type messageSendPayload = protocol.MessageSendPayload
type messageMutatePayload = protocol.MessageMutatePayload

// ACK 约定（protocol 单一事实源，规格 §3.3）：
// 成功 return "", nil（wire ACK 无 data）；失败 return "", *AppError
// （dispatcher 转 SendErrorACK）。禁止把错误塞进成功 ACK 的 data。

// resolveMessageRoom looks up a DB room by name, validates the conn slot,
// and returns (roomUUID, roomType, roomDomainUUID) or an error.
// roomDomainUUID 来自 DB，是权威域上下文。
//
// 访问策略不在这里复查：ws.HandlerRegistry 的 PolicyGuard 已对每条上行消息
// 做过一次判决（handler 层是通用入口），这里再评估一次无信息量。
func (h *Hub) resolveMessageRoom(c ws.ClientMessenger, roomName, domainUUID string) (roomUUID, roomType, roomDomainUUID string, err error) {
	identity := clientIdentity(c)
	if identity == "" {
		return "", "", "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}
	if c == nil || c.Claims() == nil || c.Claims().UserUUID == "" {
		return "", "", "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	if h.roomStore == nil {
		return "", "", "", pkg.NewAppError(pkg.INTERNAL_ERROR, "room store unavailable")
	}
	var dbRoom *model.Room
	var lookupErr error
	if domainUUID != "" {
		dbRoom, lookupErr = h.roomStore.GetByDomainAndName(domainUUID, roomName)
	} else {
		dbRoom, lookupErr = h.roomStore.GetByName(roomName)
	}
	if lookupErr != nil || dbRoom == nil {
		return "", "", "", pkg.NewAppError(pkg.NOT_FOUND, "room not found: "+roomName)
	}
	roomUUID = dbRoom.UUID
	roomType = model.NormalizeRoomType(dbRoom.Type)
	roomDomainUUID = dbRoom.DomainUUID
	if domainUUID == "" && roomDomainUUID != "" {
		domainUUID = roomDomainUUID
	}

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
		return "", "", "", pkg.NewAppError(pkg.INVALID_PARAMS, "not in room: "+roomName)
	}

	return roomUUID, roomType, roomDomainUUID, nil
}

// checkMessagePerm returns an error if the connection lacks message:send.
// 域房间只认域角色权限；平台房间 token 显式权限优先，否则回退全局角色。
func (h *Hub) checkMessagePerm(c ws.ClientMessenger, roomDomainUUID string) error {
	claims := c.Claims()
	if claims == nil {
		return pkg.NewAppError(pkg.FORBIDDEN, "permission denied")
	}
	role := claims.Role
	if role == "" && h.userStore != nil {
		if user, err := h.userStore.GetByName(clientIdentity(c)); err == nil && user != nil {
			role = user.Role
		}
	}
	if roomDomainUUID != "" {
		if h.domainPermChecker == nil || claims.UserUUID == "" {
			return pkg.NewAppError(pkg.FORBIDDEN, "permission denied")
		}
		if !h.domainPermChecker(roomDomainUUID, claims.UserUUID, permcode.PermMessageSend) {
			return pkg.NewAppError(pkg.FORBIDDEN, "permission denied")
		}
		return nil
	}
	if !permissionGranted(claims, role, permcode.PermMessageSend, h.permChecker) {
		return pkg.NewAppError(pkg.FORBIDDEN, "permission denied")
	}
	return nil
}

func messageActorFromConn(c ws.ClientMessenger) (message.Actor, bool) {
	identity := clientIdentity(c)
	if identity == "" || c == nil || c.Claims() == nil || c.Claims().UserUUID == "" {
		return message.Actor{}, false
	}
	return message.Actor{Identity: identity, UserUUID: c.Claims().UserUUID}, true
}

// OnMessageSend handles message:send events from clients.
func (h *Hub) OnMessageSend(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "message service unavailable")
	}

	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "room is required")
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	roomUUID, _, roomDomainUUID, err := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if err != nil {
		return "", err
	}
	if err := h.checkMessagePerm(c, roomDomainUUID); err != nil {
		return "", err
	}

	if _, err := h.msgSvc.Send(roomUUID, actor, req.Content, req.ReplyTo, req.ClientNonce, req.Mentions); err != nil {
		return "", err
	}
	return "", nil
}

// OnMessageEdit handles message:edit events from clients.
func (h *Hub) OnMessageEdit(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "message service unavailable")
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "room and message_uuid are required")
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	roomUUID, _, roomDomainUUID, err := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if err != nil {
		return "", err
	}
	if err := h.checkMessagePerm(c, roomDomainUUID); err != nil {
		return "", err
	}

	if _, err := h.msgSvc.Edit(roomUUID, req.MessageUUID, actor, req.Content); err != nil {
		return "", err
	}
	return "", nil
}

// OnMessageDelete handles message:delete events from clients.
func (h *Hub) OnMessageDelete(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "message service unavailable")
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "room and message_uuid are required")
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	roomUUID, _, roomDomainUUID, err := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if err != nil {
		return "", err
	}
	if err := h.checkMessagePerm(c, roomDomainUUID); err != nil {
		return "", err
	}

	// 域房间只认域角色 delete_others；平台房间 token 显式权限优先，否则全局角色。
	canDeleteOthers := false
	if claims := c.Claims(); claims != nil {
		if roomDomainUUID != "" {
			if h.domainPermChecker != nil && claims.UserUUID != "" {
				canDeleteOthers = h.domainPermChecker(roomDomainUUID, claims.UserUUID, permcode.PermMessageDeleteOthers)
			}
		} else {
			canDeleteOthers = permissionGranted(claims, claims.Role, permcode.PermMessageDeleteOthers, h.permChecker)
		}
	}

	if err := h.msgSvc.Delete(roomUUID, req.MessageUUID, actor, canDeleteOthers); err != nil {
		return "", err
	}
	return "", nil
}

// OnMessageReact handles message:react events from clients.
func (h *Hub) OnMessageReact(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "message service unavailable")
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" || req.Emoji == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "room, message_uuid, and emoji are required")
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	roomUUID, _, roomDomainUUID, err := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if err != nil {
		return "", err
	}
	if err := h.checkMessagePerm(c, roomDomainUUID); err != nil {
		return "", err
	}

	if err := h.msgSvc.React(roomUUID, req.MessageUUID, actor, req.Emoji); err != nil {
		return "", err
	}
	return "", nil
}

// OnMessageUnreact handles message:unreact events from clients.
func (h *Hub) OnMessageUnreact(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "message service unavailable")
	}

	var req messageMutatePayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" || req.MessageUUID == "" || req.Emoji == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "room, message_uuid, and emoji are required")
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	roomUUID, _, roomDomainUUID, err := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if err != nil {
		return "", err
	}
	if err := h.checkMessagePerm(c, roomDomainUUID); err != nil {
		return "", err
	}

	if err := h.msgSvc.Unreact(roomUUID, req.MessageUUID, actor, req.Emoji); err != nil {
		return "", err
	}
	return "", nil
}
