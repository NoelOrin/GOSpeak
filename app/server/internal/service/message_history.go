package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
)

// ListHistory returns paginated message history for a room, newest first.
// limit is clamped to [50, 200]; default 100.
// Returns items (ASC, oldest-first), hasMore, nextBefore cursor, error.
func (s *MessageService) ListHistory(roomUUID string, actor MessageActor, before string, limit int, password string) (items []MessageDTO, hasMore bool, nextBefore string, err error) {
	if limit <= 0 {
		limit = 100
	}
	if limit < 50 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	room, roomErr := s.roomRepo.GetByUUID(roomUUID)
	if roomErr != nil {
		return nil, false, "", pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if model.NormalizeRoomType(room.Type) != model.RoomTypeText {
		return nil, false, "", pkg.NewAppError(pkg.FORBIDDEN, "not a text room")
	}
	if err := s.requireRoomAccess(room, actor, password); err != nil {
		return nil, false, "", err
	}

	rows, more, repoErr := s.msgRepo.ListBefore(roomUUID, before, limit)
	if repoErr != nil {
		return nil, false, "", pkg.NewAppError(pkg.INTERNAL_ERROR, repoErr.Error())
	}

	items = make([]MessageDTO, len(rows))
	for i, m := range rows {
		deleted := m.DeletedAt.Valid
		content := m.Content
		if deleted {
			content = ""
		}
		items[i] = MessageDTO{
			UUID:       m.UUID,
			RoomUUID:   m.RoomUUID,
			AuthorID:   m.AuthorID,
			AuthorUUID: m.AuthorUUID,
			Content:    content,
			ReplyTo:    m.ReplyTo,
			EditedAt:   m.EditedAt,
			Deleted:    deleted,
			CreatedAt:  m.CreatedAt,
		}
	}

	if more && len(items) > 0 {
		nextBefore = items[0].UUID
	}
	s.enrichMentions(items)
	s.enrichAuthorInfo(items)

	return items, more, nextBefore, nil
}

// Search 返回文本房间内匹配 content 的最新消息，用于全文搜索 UI。

// Search 返回文本房间内匹配 content 的最新消息，用于全文搜索 UI。
func (s *MessageService) Search(roomUUID string, actor MessageActor, query, password string) ([]MessageDTO, error) {
	room, err := s.roomRepo.GetByUUID(roomUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "room not found")
	}
	if model.NormalizeRoomType(room.Type) != model.RoomTypeText {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a text room")
	}
	if err := s.requireRoomAccess(room, actor, password); err != nil {
		return nil, err
	}
	rows, err := s.msgRepo.Search(roomUUID, query, 100)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	items := make([]MessageDTO, 0, len(rows))
	for _, m := range rows {
		items = append(items, MessageDTO{
			UUID:       m.UUID,
			RoomUUID:   m.RoomUUID,
			AuthorID:   m.AuthorID,
			AuthorUUID: m.AuthorUUID,
			Content:    m.Content,
			ReplyTo:    m.ReplyTo,
			EditedAt:   m.EditedAt,
			Deleted:    m.DeletedAt.Valid,
			CreatedAt:  m.CreatedAt,
		})
	}
	s.enrichMentions(items)
	s.enrichAuthorInfo(items)
	return items, nil
}

// requireRoomAccess 校验房间访问权：Domain 房间走成员校验；
// 平台级私密房间要求调用者提供正确密码（创建者免密）。
func (s *MessageService) requireRoomAccess(room *model.Room, actor MessageActor, password string) error {
	if room.DomainUUID != "" {
		return s.requireDomainMembership(room, actor)
	}
	if room.Password == "" {
		return nil
	}
	if actor.UserUUID != "" && room.CreatedBy == actor.Identity {
		return nil
	}
	if pkg.VerifyPassword(room.Password, password) {
		return nil
	}
	return pkg.NewAppError(pkg.FORBIDDEN, "room password required")
}

// enrichMentions 批量回填消息 DTO 的 mentions。

// enrichMentions 批量回填消息 DTO 的 mentions。
func (s *MessageService) enrichMentions(items []MessageDTO) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.UUID)
	}
	rows, err := s.msgRepo.ListMentions(ids)
	if err != nil || len(rows) == 0 {
		return
	}
	byUUID := make(map[string][]string)
	for _, row := range rows {
		byUUID[row.MessageUUID] = append(byUUID[row.MessageUUID], row.UserID)
	}
	for i := range items {
		items[i].Mentions = byUUID[items[i].UUID]
	}
}

// PersistFromJob is called by the jobs consumer to persist a message from a "chat.persist" job.
