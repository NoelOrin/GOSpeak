package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

// Edit updates a message's content, broadcasts the change, then enqueues async mutate.
// Only the original author may edit.

// isMessageAuthor 优先按稳定的 AuthorUUID 校验；历史消息未回填 UUID 时退回用户名比较。
func isMessageAuthor(msg *model.Message, actor MessageActor) bool {
	if msg.AuthorUUID != "" {
		return msg.AuthorUUID == actor.UserUUID
	}
	return msg.AuthorID == actor.Identity
}

func (s *MessageService) Edit(roomUUID, messageUUID string, actor MessageActor, content string) (*MessageDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}

	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if model.NormalizeRoomType(room.Type) != model.RoomTypeText {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a text room")
	}
	if err := s.requireDomainMembership(room, actor); err != nil {
		return nil, err
	}
	if !isMessageAuthor(msg, actor) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not your message")
	}

	now := time.Now().UTC()
	dto := &MessageDTO{
		UUID:       messageUUID,
		RoomUUID:   roomUUID,
		AuthorID:   actor.Identity,
		AuthorUUID: msg.AuthorUUID,
		Content:    content,
		ReplyTo:    msg.ReplyTo,
		EditedAt:   &now,
		Deleted:    false,
		CreatedAt:  msg.CreatedAt,
	}

	// 1) broadcast first
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), broadcastRoomKey(room.DomainUUID, room.Name), "message:updated", dto)
	}

	// 2) enqueue mutate
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "edit",
		"message_uuid": messageUUID,
		"content":      content,
		"room_uuid":    roomUUID,
		"author_id":    actor.Identity,
		"author_uuid":  actor.UserUUID,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-edit-" + fmt.Sprintf("%d", now.UnixNano()),
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.UpdateContent(messageUUID, content, now); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return dto, nil
}

// Delete soft-deletes a message, broadcasts the deletion, then enqueues async mutate.
// canDeleteOthers allows moderators to delete other users' messages.

// Delete soft-deletes a message, broadcasts the deletion, then enqueues async mutate.
// canDeleteOthers allows moderators to delete other users' messages.
func (s *MessageService) Delete(roomUUID, messageUUID string, actor MessageActor, canDeleteOthers bool) error {
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if err := s.requireDomainMembership(room, actor); err != nil {
		return err
	}
	if !isMessageAuthor(msg, actor) && !canDeleteOthers {
		return pkg.NewAppError(pkg.FORBIDDEN, "not your message")
	}

	// 1) broadcast first
	dto := &MessageDTO{
		UUID:       messageUUID,
		RoomUUID:   roomUUID,
		AuthorID:   msg.AuthorID,
		AuthorUUID: msg.AuthorUUID,
		Content:    "",
		Deleted:    true,
		CreatedAt:  msg.CreatedAt,
	}
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), broadcastRoomKey(room.DomainUUID, room.Name), "message:deleted", dto)
	}

	// 2) enqueue mutate
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "delete",
		"message_uuid": messageUUID,
		"room_uuid":    roomUUID,
		"author_id":    actor.Identity,
		"author_uuid":  actor.UserUUID,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-del-" + fmt.Sprintf("%d", now.UnixNano()),
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.SoftDelete(messageUUID); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// React adds a reaction emoji to a message, broadcasts, then enqueues async mutate.

// React adds a reaction emoji to a message, broadcasts, then enqueues async mutate.
func (s *MessageService) React(roomUUID, messageUUID string, actor MessageActor, emoji string) error {
	if emoji == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "emoji is required")
	}
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if err := s.requireDomainMembership(room, actor); err != nil {
		return err
	}

	// 1) broadcast first
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), broadcastRoomKey(room.DomainUUID, room.Name), "message:reaction", map[string]interface{}{
			"action":       "added",
			"message_uuid": messageUUID,
			"user_id":      actor.Identity,
			"emoji":        emoji,
		})
	}

	// 2) enqueue mutate
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "react",
		"message_uuid": messageUUID,
		"user_id":      actor.Identity,
		"emoji":        emoji,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-react-" + actor.Identity + "-" + emoji,
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.AddReaction(&model.MessageReaction{
			MessageUUID: messageUUID,
			UserID:      actor.Identity,
			Emoji:       emoji,
		}); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// Unreact removes a reaction emoji from a message, broadcasts, then enqueues async mutate.

// Unreact removes a reaction emoji from a message, broadcasts, then enqueues async mutate.
func (s *MessageService) Unreact(roomUUID, messageUUID string, actor MessageActor, emoji string) error {
	if emoji == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "emoji is required")
	}
	msg, err := s.msgRepo.GetByUUID(messageUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}
	if msg.RoomUUID != roomUUID {
		return pkg.NewAppError(pkg.NOT_FOUND, "message not found")
	}

	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if err := s.requireDomainMembership(room, actor); err != nil {
		return err
	}

	// 1) broadcast first
	if s.bus != nil {
		_ = s.bus.PublishRoom(context.Background(), broadcastRoomKey(room.DomainUUID, room.Name), "message:reaction", map[string]interface{}{
			"action":       "removed",
			"message_uuid": messageUUID,
			"user_id":      actor.Identity,
			"emoji":        emoji,
		})
	}

	// 2) enqueue mutate
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "unreact",
		"message_uuid": messageUUID,
		"user_id":      actor.Identity,
		"emoji":        emoji,
		"timestamp":    now,
	})
	enqueued := false
	if s.queue != nil {
		if err := s.queue.Publish(context.Background(), bus.JobEnvelope{
			ID:      messageUUID + "-unreact-" + actor.Identity + "-" + emoji,
			Type:    "chat.mutate",
			Payload: payload,
		}); err == nil {
			enqueued = true
		}
	}
	if !enqueued {
		if err := s.msgRepo.RemoveReaction(messageUUID, actor.Identity, emoji); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

// ListHistory returns paginated message history for a room, newest first.
// limit is clamped to [50, 200]; default 100.
// Returns items (ASC, oldest-first), hasMore, nextBefore cursor, error.
