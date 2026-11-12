package handler

import (
	"time"

	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// InviteHandler 邀请码管理端与公开预览端点。
type InviteHandler struct {
	svc       *service.InviteService
	domainSvc domainPermissionChecker
	permSvc   globalPermissionChecker
}

func NewInviteHandler(svc *service.InviteService) *InviteHandler {
	return &InviteHandler{svc: svc}
}

// SetPermissionCheckers 注入域/平台权限判决依赖（管理端点用）。
func (h *InviteHandler) SetPermissionCheckers(domainSvc domainPermissionChecker, permSvc globalPermissionChecker) {
	h.domainSvc = domainSvc
	h.permSvc = permSvc
}

func (h *InviteHandler) canManage(c *gin.Context, domainUUID string) bool {
	return domainPermissionGranted(c, domainUUID, permcode.PermDomainInviteManage, h.domainSvc, h.permSvc)
}

type createInviteRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	Label      string `json:"label"`
	ExpiresIn  int    `json:"expires_in_hours"`
	MaxUses    int    `json:"max_uses"`
}

func (h *InviteHandler) Create(c *gin.Context) {
	var req createInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &t
	}
	invite, err := h.svc.Create(req.DomainUUID, req.Label, expiresAt, req.MaxUses, currentUserUUID(c))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, invite)
}

type domainUUIDRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

func (h *InviteHandler) List(c *gin.Context) {
	var req domainUUIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	invites, err := h.svc.List(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"invites": invites})
}

type disableInviteRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	InviteID   uint   `json:"invite_id" binding:"required"`
}

func (h *InviteHandler) Disable(c *gin.Context) {
	var req disableInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	if err := h.svc.Disable(req.InviteID, currentUserUUID(c)); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type redemptionsRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	InviteID   uint   `json:"invite_id" binding:"required"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

func (h *InviteHandler) Redemptions(c *gin.Context) {
	var req redemptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	items, total, err := h.svc.ListRedemptions(req.InviteID, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total})
}

type previewInviteRequest struct {
	Code string `json:"code" binding:"required"`
}

// Preview 是公开端点：验码后返回域名/图标/描述与是否需审核。
// 失败一律不泄露域信息。
func (h *InviteHandler) Preview(c *gin.Context) {
	var req previewInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invite code is required")
		return
	}
	result, err := h.svc.Validate(req.Code)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{
		"domain": gin.H{
			"uuid":        result.Domain.UUID,
			"name":        result.Domain.Name,
			"icon_url":    result.Domain.IconURL,
			"description": result.Domain.Description,
		},
		"requires_approval": result.Domain.RequireJoinApproval,
		"remaining_uses":    result.RemainingUses,
	})
}
