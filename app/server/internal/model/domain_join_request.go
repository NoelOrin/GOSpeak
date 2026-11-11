package model

import "time"

// 加入申请状态。
const (
	JoinRequestPending  = "pending"
	JoinRequestApproved = "approved"
	JoinRequestRejected = "rejected"
)

// DomainJoinRequest 是一个用户对一个域的加入申请。
// (domain_uuid, user_uuid) 唯一：一个用户对一个域仅一条记录。
// 撤销申请是物理删除，不引入 cancelled 状态。
type DomainJoinRequest struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DomainUUID string     `gorm:"size:32;uniqueIndex:uniq_join_req_domain_user,priority:1;not null" json:"domain_uuid"`
	UserUUID   string     `gorm:"size:36;uniqueIndex:uniq_join_req_domain_user,priority:2;not null" json:"user_uuid"`
	InviteID   uint       `gorm:"default:0" json:"invite_id"`
	Status     string     `gorm:"size:16;default:pending;index" json:"status"`
	ReviewedBy string     `gorm:"size:36" json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (DomainJoinRequest) TableName() string { return "domain_join_requests" }

// IsPending 报告申请是否仍在待审状态。
func (r *DomainJoinRequest) IsPending() bool {
	return r != nil && r.Status == JoinRequestPending
}

// DomainJoinRequestInfo 是管理端审核列表视图，附带申请人全局名称。
type DomainJoinRequestInfo struct {
	ID          uint       `json:"id"`
	DomainUUID  string     `json:"domain_uuid"`
	UserUUID    string     `json:"user_uuid"`
	InviteID    uint       `json:"invite_id"`
	Status      string     `json:"status"`
	ReviewedBy  string     `json:"reviewed_by"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name"`
}
