package repository

import (
	"strings"
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DomainJoinRequestRepository 是 domain_join_requests 的数据访问层。
type DomainJoinRequestRepository struct {
	db *gorm.DB
}

func NewDomainJoinRequestRepository(db *gorm.DB) *DomainJoinRequestRepository {
	return &DomainJoinRequestRepository{db: db}
}

// Submit 幂等提交申请：无记录则插入 pending；已有记录则原样返回，
// 由服务层按当前状态决定是否 Reopen。并发同用户提交由唯一索引兜底。
func (r *DomainJoinRequestRepository) Submit(domainUUID, userUUID string, inviteID uint) (*model.DomainJoinRequest, error) {
	req := &model.DomainJoinRequest{
		DomainUUID: domainUUID,
		UserUUID:   userUUID,
		InviteID:   inviteID,
		Status:     model.JoinRequestPending,
	}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(req).Error; err != nil {
		return nil, err
	}
	return r.GetByDomainUser(domainUUID, userUUID)
}

func (r *DomainJoinRequestRepository) GetByID(id uint) (*model.DomainJoinRequest, error) {
	var req model.DomainJoinRequest
	err := r.db.Where("id = ?", id).First(&req).Error
	return &req, err
}

func (r *DomainJoinRequestRepository) GetByDomainUser(domainUUID, userUUID string) (*model.DomainJoinRequest, error) {
	var req model.DomainJoinRequest
	err := r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).First(&req).Error
	return &req, err
}

// Reopen 把已处理（rejected / approved）的申请打回 pending 并刷新邀请码。
// 条件更新保证并发下不会覆盖他人刚写入的审核结果。
func (r *DomainJoinRequestRepository) Reopen(id uint, inviteID uint) error {
	return r.db.Model(&model.DomainJoinRequest{}).
		Where("id = ? AND status <> ?", id, model.JoinRequestPending).
		Updates(map[string]interface{}{
			"status":      model.JoinRequestPending,
			"invite_id":   inviteID,
			"reviewed_by": "",
			"reviewed_at": nil,
		}).Error
}

// Review 条件审核：仅当仍为 pending 时生效。
// 返回 false 表示申请已被他人处理，调用方应回滚事务并报错。
func (r *DomainJoinRequestRepository) Review(id uint, approve bool, reviewerUUID string) (bool, error) {
	status := model.JoinRequestRejected
	if approve {
		status = model.JoinRequestApproved
	}
	now := time.Now()
	res := r.db.Model(&model.DomainJoinRequest{}).
		Where("id = ? AND status = ?", id, model.JoinRequestPending).
		Updates(map[string]interface{}{
			"status":      status,
			"reviewed_by": reviewerUUID,
			"reviewed_at": &now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteByDomainUser 物理删除申请（撤销语义，不保留 cancelled 状态）。
func (r *DomainJoinRequestRepository) DeleteByDomainUser(domainUUID, userUUID string) error {
	return r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Delete(&model.DomainJoinRequest{}).Error
}

// ListByDomain 分页查询域内申请，可按状态过滤、按申请人名字搜索。
// items 以空切片初始化：空结果序列化为 [] 而非 null，前端列表组件直接取 length。
func (r *DomainJoinRequestRepository) ListByDomain(domainUUID, status, keyword string, page, pageSize int) ([]model.DomainJoinRequestInfo, int64, error) {
	items := make([]model.DomainJoinRequestInfo, 0)
	var total int64

	q := r.db.Table("domain_join_requests").
		Joins("LEFT JOIN users ON users.uuid = domain_join_requests.user_uuid").
		Where("domain_join_requests.domain_uuid = ?", domainUUID)
	if status != "" {
		q = q.Where("domain_join_requests.status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("users.name LIKE ? OR users.display_name LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Select(`domain_join_requests.id, domain_join_requests.domain_uuid,
		domain_join_requests.user_uuid, domain_join_requests.invite_id,
		domain_join_requests.status, domain_join_requests.reviewed_by,
		domain_join_requests.reviewed_at, domain_join_requests.created_at,
		domain_join_requests.updated_at,
		COALESCE(users.name, '') AS name,
		COALESCE(users.display_name, '') AS display_name`).
		Order("domain_join_requests.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// CountByStatus 返回域内各状态的申请数，供管理页分区标题使用。
func (r *DomainJoinRequestRepository) CountByStatus(domainUUID string) (map[string]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	err := r.db.Model(&model.DomainJoinRequest{}).
		Select("status, COUNT(*) AS total").
		Where("domain_uuid = ?", domainUUID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{
		model.JoinRequestPending:  0,
		model.JoinRequestApproved: 0,
		model.JoinRequestRejected: 0,
	}
	for _, item := range rows {
		out[item.Status] = item.Total
	}
	return out, nil
}
