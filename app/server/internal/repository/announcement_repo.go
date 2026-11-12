package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnnouncementRepository struct {
	db *gorm.DB
}

func NewAnnouncementRepository(db *gorm.DB) *AnnouncementRepository {
	return &AnnouncementRepository{db: db}
}

func (r *AnnouncementRepository) Create(a *model.Announcement) error {
	return r.db.Create(a).Error
}

func (r *AnnouncementRepository) Save(a *model.Announcement) error {
	return r.db.Save(a).Error
}

func (r *AnnouncementRepository) GetByUUID(uuid string) (*model.Announcement, error) {
	var a model.Announcement
	err := r.db.Where("uuid = ?", uuid).First(&a).Error
	return &a, err
}

func (r *AnnouncementRepository) Delete(uuid string) error {
	return r.db.Where("uuid = ?", uuid).Delete(&model.Announcement{}).Error
}

// visibleQuery 是「用户可见公告」的唯一事实来源：已发布 + 未过期 + scope 命中。
func (r *AnnouncementRepository) visibleQuery(domainUUIDs []string) *gorm.DB {
	now := time.Now()
	q := r.db.Model(&model.Announcement{}).
		Where("published_at IS NOT NULL AND published_at <= ?", now).
		Where("(expires_at IS NULL OR expires_at > ?)", now)

	if len(domainUUIDs) == 0 {
		return q.Where("scope_type = ?", model.AnnouncementScopePlatform)
	}
	return q.Where(
		"((scope_type = ? AND (scope_id = '' OR scope_id IS NULL)) OR (scope_type = ? AND scope_id IN ?))",
		model.AnnouncementScopePlatform, model.AnnouncementScopeDomain, domainUUIDs,
	)
}

func (r *AnnouncementRepository) ListVisible(userUUID string, domainUUIDs []string, onlyUnread bool, page, pageSize int) ([]model.Announcement, int64, error) {
	q := r.visibleQuery(domainUUIDs)
	if onlyUnread {
		q = q.Where(
			"NOT EXISTS (SELECT 1 FROM announcement_reads ar WHERE ar.announcement_uuid = announcements.uuid AND ar.user_uuid = ?)",
			userUUID,
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Announcement
	err := q.Order("pinned DESC, published_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

func (r *AnnouncementRepository) CountUnread(userUUID string, domainUUIDs []string) (int64, error) {
	var total int64
	err := r.visibleQuery(domainUUIDs).
		Where(
			"NOT EXISTS (SELECT 1 FROM announcement_reads ar WHERE ar.announcement_uuid = announcements.uuid AND ar.user_uuid = ?)",
			userUUID,
		).
		Count(&total).Error
	return total, err
}

// ReadUUIDs 返回给定公告集合中该用户已读的 uuid 集合。
func (r *AnnouncementRepository) ReadUUIDs(userUUID string, announcementUUIDs []string) (map[string]struct{}, error) {
	if len(announcementUUIDs) == 0 {
		return map[string]struct{}{}, nil
	}
	var uuids []string
	err := r.db.Model(&model.AnnouncementRead{}).
		Where("user_uuid = ? AND announcement_uuid IN ?", userUUID, announcementUUIDs).
		Pluck("announcement_uuid", &uuids).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(uuids))
	for _, u := range uuids {
		out[u] = struct{}{}
	}
	return out, nil
}

// IsVisible 判断单条公告是否对该用户可见，供 MarkRead 做权限校验，避免翻页扫描。
func (r *AnnouncementRepository) IsVisible(announcementUUID string, domainUUIDs []string) (bool, error) {
	var count int64
	err := r.visibleQuery(domainUUIDs).
		Where("uuid = ?", announcementUUID).
		Count(&count).Error
	return count > 0, err
}

// MarkRead 幂等写入已读记录；重复调用不产生错误也不更新 read_at。
func (r *AnnouncementRepository) MarkRead(announcementUUID, userUUID string) error {
	row := model.AnnouncementRead{
		AnnouncementUUID: announcementUUID,
		UserUUID:         userUUID,
		ReadAt:           time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

// ListAdmin 返回管理端列表（含草稿与已过期）。
// platformAdmin 为 true 时看全部；否则只看自己可管理的域公告 + 自己创建的全站草稿。
func (r *AnnouncementRepository) ListAdmin(userUUID string, domainUUIDs []string, platformAdmin bool, page, pageSize int) ([]model.Announcement, int64, error) {
	q := r.db.Model(&model.Announcement{})
	if !platformAdmin {
		if len(domainUUIDs) == 0 {
			q = q.Where("scope_type = ? AND created_by_uuid = ?", model.AnnouncementScopePlatform, userUUID)
		} else {
			q = q.Where(
				"(scope_type = ? AND scope_id IN ?) OR (scope_type = ? AND created_by_uuid = ?)",
				model.AnnouncementScopeDomain, domainUUIDs, model.AnnouncementScopePlatform, userUUID,
			)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Announcement
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}
