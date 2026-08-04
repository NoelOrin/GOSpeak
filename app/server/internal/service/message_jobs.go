package service

import (
	"encoding/json"
	"fmt"
	"time"

	"GOSpeak/internal/model"
)

// PersistFromJob is called by the jobs consumer to persist a message from a "chat.persist" job.
func (s *MessageService) PersistFromJob(payload []byte) error {
	var data struct {
		UUID        string    `json:"uuid"`
		RoomUUID    string    `json:"room_uuid"`
		AuthorID    string    `json:"author_id"`
		Content     string    `json:"content"`
		ReplyTo     string    `json:"reply_to"`
		Mentions    []string  `json:"mentions"`
		CreatedAt   time.Time `json:"created_at"`
		ClientNonce string    `json:"client_nonce"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	m := &model.Message{
		UUID: data.UUID, RoomUUID: data.RoomUUID, AuthorID: data.AuthorID,
		Content: data.Content, ReplyTo: data.ReplyTo,
		CreatedAt: data.CreatedAt, UpdatedAt: data.CreatedAt,
	}
	if err := s.msgRepo.Create(m); err != nil {
		return err
	}

	if len(data.Mentions) > 0 {
		var rows []model.MessageMention
		for _, uid := range data.Mentions {
			rows = append(rows, model.MessageMention{MessageUUID: data.UUID, UserID: uid})
		}
		return s.msgRepo.CreateMentions(rows)
	}
	return nil
}

// MutateFromJob is called by the jobs consumer to apply a mutation (edit/delete/react/unreact)
// from a "chat.mutate" job.

// MutateFromJob is called by the jobs consumer to apply a mutation (edit/delete/react/unreact)
// from a "chat.mutate" job.
func (s *MessageService) MutateFromJob(payload []byte) error {
	var data struct {
		Action      string    `json:"action"`
		MessageUUID string    `json:"message_uuid"`
		Content     string    `json:"content,omitempty"`
		UserID      string    `json:"user_id,omitempty"`
		Emoji       string    `json:"emoji,omitempty"`
		Timestamp   time.Time `json:"timestamp,omitempty"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	if _, err := s.msgRepo.GetByUUID(data.MessageUUID); err != nil {
		return fmt.Errorf("message not ready")
	}

	ts := data.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	switch data.Action {
	case "edit":
		return s.msgRepo.UpdateContent(data.MessageUUID, data.Content, ts)
	case "delete":
		return s.msgRepo.SoftDelete(data.MessageUUID)
	case "react":
		return s.msgRepo.AddReaction(&model.MessageReaction{
			MessageUUID: data.MessageUUID,
			UserID:      data.UserID,
			Emoji:       data.Emoji,
		})
	case "unreact":
		return s.msgRepo.RemoveReaction(data.MessageUUID, data.UserID, data.Emoji)
	default:
		return fmt.Errorf("unknown mutate action: %s", data.Action)
	}
}
