package handler

import (
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// JoinRequestHandler 域加入申请的管理端端点。
type JoinRequestHandler struct {
	svc       *service.JoinRequestService
	domainSvc domainPermissionChecker
	permSvc   globalPermissionChecker
}

func NewJoinRequestHandler(svc *service.JoinRequestService, domainSvc domainPermissionChecker) *JoinRequestHandler {
	return &JoinRequestHandler{svc: svc, domainSvc: domainSvc}
}

// SetGlobalPermissionChecker 注入平台权限判决依赖（平台管理员跨域审核时使用）。
func (h *JoinRequestHandler) SetGlobalPermissionChecker(permSvc globalPermissionChecker) {
	h.permSvc = permSvc
}

func (h *JoinRequestHandler) canReview(c *gin.Context, domainUUID string) bool {
	return domainPermissionGranted(c, domainUUID, permcode.PermDomainMemberReview, h.domainSvc, h.permSvc)
}

func (h *JoinRequestHandler) canManage(c *gin.Context, domainUUID string) bool {
	return domainPermissionGranted(c, domainUUID, permcode.PermDomainManage, h.domainSvc, h.permSvc)
}

type listJoinRequestsRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	Status     string `json:"status"`
	Keyword    string `json:"keyword"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// List 分页查询域内加入申请，附带各状态计数。
// @Summary      加入申请列表
// @Tags         域加入审核
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  listJoinRequestsRequest  true  "查询条件"
// @Success      200  {object}  pkg.Response
// @Router       /domain/join-requests/list [post]
func (h *JoinRequestHandler) List(c *gin.Context) {
	var req listJoinRequestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canReview(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain member review permission")
		return
	}
	items, total, err := h.svc.List(req.DomainUUID, req.Status, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	counts, err := h.svc.Counts(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total, "counts": counts})
}

type reviewJoinRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	RequestID  uint   `json:"request_id" binding:"required"`
	Approve    bool   `json:"approve"`
}

// Review 通过或拒绝一条加入申请。
// @Summary      审核加入申请
// @Tags         域加入审核
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  reviewJoinRequest  true  "审核决定"
// @Success      200  {object}  pkg.Response
// @Router       /domain/join-requests/review [post]
func (h *JoinRequestHandler) Review(c *gin.Context) {
	var req reviewJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canReview(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain member review permission")
		return
	}
	updated, err := h.svc.Review(req.DomainUUID, req.RequestID, req.Approve, currentUserUUID(c))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, updated)
}

type joinApprovalConfigRequest struct {
	DomainUUID          string `json:"domain_uuid" binding:"required"`
	RequireJoinApproval bool   `json:"require_join_approval"`
}

// Config 读写域级「加入需审核」开关。
// @Summary      加入审核开关
// @Tags         域加入审核
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  joinApprovalConfigRequest  true  "开关取值"
// @Success      200  {object}  pkg.Response
// @Router       /domain/join-approval/config [post]
func (h *JoinRequestHandler) Config(c *gin.Context) {
	var req joinApprovalConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not domain owner or missing permission")
		return
	}
	domain, err := h.svc.SetRequireApproval(req.DomainUUID, req.RequireJoinApproval)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{
		"uuid":                  domain.UUID,
		"require_join_approval": domain.RequireJoinApproval,
	})
}
