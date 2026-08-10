package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"GOSpeak/internal/model"
)

// PersistFromJob is called by the jobs consumer to persist a message from a "chat.persist" job.
// 幂等处理：广播失败兜底或并发 job 已落库时直接跳过创建，避免重试/重放重复写入。
func (s *MessageService) PersistFromJob(payload []byte) error {
	if !s.syncWriteAllowed {
		return errors.New("message persistence is not allowed on data-plane instance")
	}
	var data struct {
		UUID        string    `json:"uuid"`
		RoomUUID    string    `json:"room_uuid"`
		AuthorID    string    `json:"author_id"`
		AuthorUUID  string    `json:"author_uuid"`
		Content     string    `json:"content"`
		ReplyTo     string    `json:"reply_to"`
		Mentions    []string  `json:"mentions"`
		CreatedAt   time.Time `json:"created_at"`
		ClientNonce string    `json:"client_nonce"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	if _, err := s.msgRepo.GetByUUID(data.UUID); err == nil {
		return s.ensureJobMentions(data.UUID, data.Mentions)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	m := &model.Message{
		UUID: data.UUID, RoomUUID: data.RoomUUID, AuthorID: data.AuthorID,
		AuthorUUID: data.AuthorUUID,
		Content:    data.Content, ReplyTo: data.ReplyTo,
		CreatedAt: data.CreatedAt, UpdatedAt: data.CreatedAt,
	}
	if err := s.msgRepo.Create(m); err != nil {
		// 并发兜底写入时唯一键冲突：按已存在处理。
		if _, getErr := s.msgRepo.GetByUUID(data.UUID); getErr != nil {
			return err
		}
	}
	return s.ensureJobMentions(data.UUID, data.Mentions)
}

func (s *MessageService) ensureJobMentions(messageUUID string, mentions []string) error {
	if len(mentions) == 0 {
		return nil
	}
	rows := make([]model.MessageMention, 0, len(mentions))
	for _, uid := range mentions {
		rows = append(rows, model.MessageMention{MessageUUID: messageUUID, UserID: uid})
	}
	return s.msgRepo.EnsureMentions(rows)
}

// MutateFromJob is called by the jobs consumer to apply a mutation (edit/delete/react/unreact)
// from a "chat.mutate" job.
// 消息未落库时，编辑/删除携带 room_uuid/author_id 的 job 会用时间戳 upsert 占位记录，
// 避免消息创建 job 重试耗尽后变更永久丢失。
func (s *MessageService) MutateFromJob(payload []byte) error {
	if !s.syncWriteAllowed {
		return errors.New("message mutation is not allowed on data-plane instance")
	}
	var data struct {
		Action      string    `json:"action"`
		MessageUUID string    `json:"message_uuid"`
		Content     string    `json:"content,omitempty"`
		UserID      string    `json:"user_id,omitempty"`
		Emoji       string    `json:"emoji,omitempty"`
		RoomUUID    string    `json:"room_uuid,omitempty"`
		AuthorID    string    `json:"author_id,omitempty"`
		AuthorUUID  string    `json:"author_uuid,omitempty"`
		Timestamp   time.Time `json:"timestamp,omitempty"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	_, err := s.msgRepo.GetByUUID(data.MessageUUID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if data.RoomUUID == "" || data.AuthorID == "" {
			return fmt.Errorf("message not ready")
		}
		ts := data.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		msg := &model.Message{
			UUID: data.MessageUUID, RoomUUID: data.RoomUUID, AuthorID: data.AuthorID,
			AuthorUUID: data.AuthorUUID,
			Content:    data.Content, CreatedAt: ts, UpdatedAt: ts,
		}
		switch data.Action {
		case "edit":
			if err := s.msgRepo.Create(msg); err != nil {
				if _, getErr := s.msgRepo.GetByUUID(data.MessageUUID); getErr != nil {
					return err
				}
			}
			return s.msgRepo.UpdateContent(data.MessageUUID, data.Content, ts)
		case "delete":
			if err := s.msgRepo.Create(msg); err != nil {
				if _, getErr := s.msgRepo.GetByUUID(data.MessageUUID); getErr != nil {
					return err
				}
			}
			return s.msgRepo.SoftDelete(data.MessageUUID)
		default:
			return fmt.Errorf("message not ready")
		}
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
