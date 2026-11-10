package service

import (
	"context"
	"log"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/notify"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

const (
	// EventAnnouncementNew 是新公告的推送事件名，前端在 WS 上订阅。
	EventAnnouncementNew = "announcement:new"

	announcementDefaultPageSize = 20
	announcementMaxPageSize     = 100
	announcementMaxTitleRunes   = 200
	announcementMaxContentRunes = 20000
)

type AnnouncementDTO struct {
	UUID          string `json:"uuid"`
	ScopeType     string `json:"scope_type"`
	ScopeID       string `json:"scope_id,omitempty"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         string `json:"level"`
	Pinned        bool   `json:"pinned"`
	PublishedAt   *int64 `json:"published_at,omitempty"`
	ExpiresAt     *int64 `json:"expires_at,omitempty"`
	CreatedByUUID string `json:"created_by_uuid,omitempty"`
	CreatedByName string `json:"created_by_name"`
	IsRead        bool   `json:"is_read"`
	CreatedAt     int64  `json:"created_at"`
}

type AnnouncementActor struct {
	UserUUID        string
	Name            string
	IsPlatformAdmin bool
}

type AnnouncementInput struct {
	ScopeType string
	ScopeID   string
	Title     string
	Content   string
	Level     string
	Pinned    bool
	Publish   *bool
	ExpiresAt *int64
}

type AnnouncementEventBus interface {
	PublishRoom(ctx context.Context, room, event string, payload interface{}) error
	PublishNamespace(ctx context.Context, event string, payload interface{}) error
}

type AnnouncementService struct {
	repo       *repository.AnnouncementRepository
	domainRepo *repository.DomainRepository
	db         *gorm.DB
	bus        AnnouncementEventBus
	// domainPerm 判定域内权限码，由 DomainService.HasDomainPermission 注入。
	// 不硬编码「角色名 == owner/admin」，否则自定义域角色拿到 domain:manage 也进不来。
	domainPerm func(domainUUID, userUUID, permCode string) bool
}

func NewAnnouncementService(repo *repository.AnnouncementRepository, domainRepo *repository.DomainRepository, db *gorm.DB) *AnnouncementService {
	return &AnnouncementService{repo: repo, domainRepo: domainRepo, db: db}
}

func (s *AnnouncementService) SetEventBus(b AnnouncementEventBus) { s.bus = b }

func (s *AnnouncementService) SetDomainPermissionChecker(fn func(domainUUID, userUUID, permCode string) bool) {
	s.domainPerm = fn
}

func toAnnouncementDTO(a *model.Announcement, isRead bool) AnnouncementDTO {
	dto := AnnouncementDTO{
		UUID:          a.UUID,
		ScopeType:     a.ScopeType,
		ScopeID:       a.ScopeID,
		Title:         a.Title,
		Content:       a.Content,
		Level:         a.Level,
		Pinned:        a.Pinned,
		CreatedByUUID: a.CreatedByUUID,
		CreatedByName: a.CreatedByName,
		IsRead:        isRead,
		CreatedAt:     a.CreatedAt.UnixMilli(),
	}
	if a.PublishedAt != nil {
		ts := a.PublishedAt.UnixMilli()
		dto.PublishedAt = &ts
	}
	if a.ExpiresAt != nil {
		ts := a.ExpiresAt.UnixMilli()
		dto.ExpiresAt = &ts
	}
	return dto
}

// CanManage 按操作对象的 scope 分叉判定，避免平台权限与域权限互相放大。
func (s *AnnouncementService) CanManage(actor AnnouncementActor, scopeType, scopeID string) bool {
	if actor.UserUUID == "" {
		return false
	}
	if scopeType == model.AnnouncementScopePlatform {
		return actor.IsPlatformAdmin
	}
	if scopeID == "" {
		return false
	}
	if actor.IsPlatformAdmin {
		return true
	}
	if s.domainPerm != nil {
		return s.domainPerm(scopeID, actor.UserUUID, permcode.PermDomainManage)
	}
	member, err := s.domainRepo.GetMember(scopeID, actor.UserUUID)
	if err != nil || member == nil {
		return false
	}
	return member.RoleName == model.DomainRoleOwner || member.RoleName == model.DomainRoleAdmin
}

func (s *AnnouncementService) userDomainUUIDs(userUUID string) ([]string, error) {
	if userUUID == "" {
		return nil, nil
	}
	return s.domainRepo.ListUserDomains(userUUID)
}

func (s *AnnouncementService) Create(actor AnnouncementActor, in AnnouncementInput) (*AnnouncementDTO, error) {
	if in.ScopeType == "" {
		in.ScopeType = model.AnnouncementScopePlatform
	}
	if !s.CanManage(actor, in.ScopeType, in.ScopeID) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "insufficient announcement permission")
	}
	if in.Title == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "title is required")
	}
	if len([]rune(in.Title)) > announcementMaxTitleRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "title too long")
	}
	if in.Content == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content is required")
	}
	if len([]rune(in.Content)) > announcementMaxContentRunes {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
	}
	if in.Level == "" {
		in.Level = model.AnnouncementLevelInfo
	}

	a := &model.Announcement{
		ScopeType:     in.ScopeType,
		ScopeID:       in.ScopeID,
		Title:         in.Title,
		Content:       in.Content,
		Level:         in.Level,
		Pinned:        in.Pinned,
		CreatedByUUID: actor.UserUUID,
		CreatedByName: actor.Name,
	}
	if in.Publish != nil && *in.Publish {
		now := time.Now()
		a.PublishedAt = &now
	}
	if in.ExpiresAt != nil && *in.ExpiresAt > 0 {
		t := time.UnixMilli(*in.ExpiresAt)
		a.ExpiresAt = &t
	}

	if err := s.repo.Create(a); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "create announcement failed")
	}

	dto := toAnnouncementDTO(a, true)
	if a.PublishedAt != nil {
		s.deliver(a, dto)
	}
	return &dto, nil
}

func (s *AnnouncementService) Update(actor AnnouncementActor, uuid string, in AnnouncementInput) (*AnnouncementDTO, error) {
	a, err := s.repo.GetByUUID(uuid)
	if err != nil || a == nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "announcement not found")
	}
	if !s.CanManage(actor, a.ScopeType, a.ScopeID) {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "insufficient announcement permission")
	}
	if in.Title != "" {
		if len([]rune(in.Title)) > announcementMaxTitleRunes {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "title too long")
		}
		a.Title = in.Title
	}
	if in.Content != "" {
		if len([]rune(in.Content)) > announcementMaxContentRunes {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "content too long")
		}
		a.Content = in.Content
	}
	if in.Level != "" {
		a.Level = in.Level
	}
	a.Pinned = in.Pinned
	if in.ExpiresAt != nil {
		if *in.ExpiresAt > 0 {
			t := time.UnixMilli(*in.ExpiresAt)
			a.ExpiresAt = &t
		} else {
			a.ExpiresAt = nil
		}
	}

	wasPublished := a.PublishedAt != nil
	if in.Publish != nil && *in.Publish && !wasPublished {
		now := time.Now()
		a.PublishedAt = &now
	}

	if err := s.repo.Save(a); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "update announcement failed")
	}

	dto := toAnnouncementDTO(a, false)
	if !wasPublished && a.PublishedAt != nil {
		s.deliver(a, dto)
	}
	return &dto, nil
}

func (s *AnnouncementService) Delete(actor AnnouncementActor, uuid string) error {
	a, err := s.repo.GetByUUID(uuid)
	if err != nil || a == nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "announcement not found")
	}
	if !s.CanManage(actor, a.ScopeType, a.ScopeID) {
		return pkg.NewAppError(pkg.FORBIDDEN, "insufficient announcement permission")
	}
	if err := s.repo.Delete(uuid); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "delete announcement failed")
	}
	return nil
}

func (s *AnnouncementService) List(userUUID string, onlyUnread bool, page, pageSize int) ([]AnnouncementDTO, int64, error) {
	page, pageSize = normalizeAnnouncementPaging(page, pageSize)
	domains, err := s.userDomainUUIDs(userUUID)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
	}
	items, total, err := s.repo.ListVisible(userUUID, domains, onlyUnread, page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "list announcements failed")
	}

	uuids := make([]string, 0, len(items))
	for i := range items {
		uuids = append(uuids, items[i].UUID)
	}
	readSet, err := s.repo.ReadUUIDs(userUUID, uuids)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load read states failed")
	}

	out := make([]AnnouncementDTO, 0, len(items))
	for i := range items {
		_, read := readSet[items[i].UUID]
		out = append(out, toAnnouncementDTO(&items[i], read))
	}
	return out, total, nil
}

func (s *AnnouncementService) UnreadCount(userUUID string) (int64, error) {
	domains, err := s.userDomainUUIDs(userUUID)
	if err != nil {
		return 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
	}
	count, err := s.repo.CountUnread(userUUID, domains)
	if err != nil {
		return 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "count unread failed")
	}
	return count, nil
}

func (s *AnnouncementService) MarkRead(userUUID, announcementUUID string) error {
	if announcementUUID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "uuid is required")
	}
	domains, err := s.userDomainUUIDs(userUUID)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
	}
	visible, err := s.repo.IsVisible(announcementUUID, domains)
	if err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "check visibility failed")
	}
	if !visible {
		return pkg.NewAppError(pkg.NOT_FOUND, "announcement not found")
	}
	if err := s.repo.MarkRead(announcementUUID, userUUID); err != nil {
		return pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "mark read failed")
	}
	return nil
}

func (s *AnnouncementService) ListAdmin(actor AnnouncementActor, page, pageSize int) ([]AnnouncementDTO, int64, error) {
	page, pageSize = normalizeAnnouncementPaging(page, pageSize)
	var domains []string
	if !actor.IsPlatformAdmin {
		owned, err := s.userDomainUUIDs(actor.UserUUID)
		if err != nil {
			return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "load user domains failed")
		}
		managed := make([]string, 0, len(owned))
		for _, d := range owned {
			if s.CanManage(actor, model.AnnouncementScopeDomain, d) {
				managed = append(managed, d)
			}
		}
		domains = managed
	}

	items, total, err := s.repo.ListAdmin(actor.UserUUID, domains, actor.IsPlatformAdmin, page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "list announcements failed")
	}
	out := make([]AnnouncementDTO, 0, len(items))
	for i := range items {
		out = append(out, toAnnouncementDTO(&items[i], true))
	}
	return out, total, nil
}

func normalizeAnnouncementPaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = announcementDefaultPageSize
	}
	if pageSize > announcementMaxPageSize {
		pageSize = announcementMaxPageSize
	}
	return page, pageSize
}

// deliver 只做在线推送；失败只记日志。公告已落库，前端重连后靠 REST 兜底。
func (s *AnnouncementService) deliver(a *model.Announcement, dto AnnouncementDTO) {
	if s.bus == nil {
		return
	}
	ctx := context.Background()
	if a.ScopeType == model.AnnouncementScopePlatform {
		if err := notify.NotifyAll(ctx, s.bus, EventAnnouncementNew, dto); err != nil {
			log.Printf("[Announcement] broadcast failed: %v", err)
		}
		return
	}
	members, err := s.domainRepo.ListMembers(a.ScopeID)
	if err != nil {
		log.Printf("[Announcement] list domain members failed scope=%s: %v", a.ScopeID, err)
		return
	}
	for i := range members {
		if err := notify.NotifyUser(ctx, s.bus, members[i].UserUUID, EventAnnouncementNew, dto); err != nil {
			log.Printf("[Announcement] notify user %s failed: %v", members[i].UserUUID, err)
		}
	}
}
