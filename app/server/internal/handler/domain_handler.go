package handler

import (
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
)

type DomainHandler struct {
	domainSvc      *service.DomainService
	permSvc       *service.PermissionService
	onDomainDelete func(string)
	onDomainCreated func(string)
}

func NewDomainHandler(domainSvc *service.DomainService, permSvc *service.PermissionService) *DomainHandler {
	return &DomainHandler{domainSvc: domainSvc, permSvc: permSvc}
}

// SetOnDomainCreated 注入创建 Server 后的集群调度回调。
func (h *DomainHandler) SetOnDomainCreated(fn func(string)) {
	h.onDomainCreated = fn
}

// SetOnDomainDelete 注入删除后的信令清理回调（生产环境为 signalHub.OnDomainDelete）。
func (h *DomainHandler) SetOnDomainDelete(fn func(string)) {
	h.onDomainDelete = fn
}


func (h *DomainHandler) hasPermission(c *gin.Context, code string) bool {
	if h.permSvc == nil {
		return false
	}
	roleVal, ok := c.Get("role")
	if !ok {
		return false
	}
	role, _ := roleVal.(string)
	return h.permSvc.HasPermission(role, code)
}

type CreateDomainRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

// Create 创建语音域。
func (h *DomainHandler) Create(c *gin.Context) {
	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	domain, err := h.domainSvc.Create(req.Name, req.Description, userUUID, req.IsPublic)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.onDomainCreated != nil {
		h.onDomainCreated(domain.UUID)
	}
	pkg.Success(c, domain)
}

type UUIDRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

// Get 获取语音域详情。
func (h *DomainHandler) Get(c *gin.Context) {
	var req UUIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domain, err := h.domainSvc.GetByUUID(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, domain)
}

type ListDomainRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

// List 列出所有语音域。
func (h *DomainHandler) List(c *gin.Context) {
	var req ListDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page = 1
		req.PageSize = 20
	}
	domains, total, err := h.domainSvc.List(req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"domains": domains, "total": total})
}

// ListPublic 列出公开语音域。
func (h *DomainHandler) ListPublic(c *gin.Context) {
	var req ListDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page = 1
		req.PageSize = 20
	}
	domains, total, err := h.domainSvc.ListPublic(req.Page, req.PageSize, strings.TrimSpace(req.Keyword))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"domains": domains, "total": total})
}

// MyDomains 返回当前用户加入的 Domain UUID 列表。
func (h *DomainHandler) MyDomains(c *gin.Context) {
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	uuids, err := h.domainSvc.ListUserDomains(userUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"domain_uuids": uuids})
}

type UpdateDomainRequest struct {
	DomainUUID  string  `json:"domain_uuid" binding:"required"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IconURL     *string `json:"icon_url"`
	IsPublic    *bool   `json:"is_public"`
	MaxRooms    *uint   `json:"max_rooms"`
}

// Update 更新语音域信息。
func (h *DomainHandler) Update(c *gin.Context) {
	var req UpdateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domain, err := h.domainSvc.GetByUUID(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	if !h.hasPermission(c, permcode.PermDomainManage) && !h.domainSvc.IsOwner(req.DomainUUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not domain owner or missing permission")
		return
	}
	if req.Name != nil {
		domain.Name = *req.Name
	}
	if req.Description != nil {
		domain.Description = *req.Description
	}
	if req.IconURL != nil {
		domain.IconURL = *req.IconURL
	}
	if req.IsPublic != nil {
		domain.IsPublic = *req.IsPublic
	}
	if req.MaxRooms != nil {
		domain.MaxRooms = *req.MaxRooms
	}
	if err := h.domainSvc.Update(domain); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, domain)
}

type DeleteDomainRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

// Delete 删除语音域。仅 Owner 和持有 domain:delete 权限的用户可调用。
func (h *DomainHandler) Delete(c *gin.Context) {
	var req DeleteDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	permOK := h.hasPermission(c, permcode.PermDomainDelete)
	ownerOK := h.domainSvc.IsOwner(req.DomainUUID, userUUID)
	if !permOK && !ownerOK {
		pkg.Fail(c, pkg.FORBIDDEN, "not domain owner or missing permission")
		return
	}
	if err := h.domainSvc.Delete(req.DomainUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.onDomainDelete != nil {
		h.onDomainDelete(req.DomainUUID)
	}
	pkg.Success(c, nil)
}

type JoinDomainRequest struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

// Join 通过邀请码加入语音域。
func (h *DomainHandler) Join(c *gin.Context) {
	var req JoinDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	domain, err := h.domainSvc.Join(req.InviteCode, userUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, domain)
}

// Preview 根据邀请码或邀请链接返回 Domain 信息，供加入前确认。
func (h *DomainHandler) Preview(c *gin.Context) {
	var req JoinDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domain, err := h.domainSvc.GetByInviteCode(strings.TrimSpace(req.InviteCode))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, domain)
}

type LeaveDomainRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

// Leave 离开语音域。
func (h *DomainHandler) Leave(c *gin.Context) {
	var req LeaveDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	if err := h.domainSvc.Leave(req.DomainUUID, userUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type KickDomainMemberRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	UserUUID  string `json:"user_uuid" binding:"required"`
}

// Kick 踢出成员。仅 Owner/Admin 或持有 domain:kick 权限的用户可调用。
func (h *DomainHandler) Kick(c *gin.Context) {
	var req KickDomainMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	permOK := h.hasPermission(c, permcode.PermDomainKick)
	roleOK := h.domainSvc.HasDomainRole(req.DomainUUID, userUUID, service.DomainRoleAdmin)
	if !permOK && !roleOK {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role or permission")
		return
	}
	if err := h.domainSvc.Kick(req.DomainUUID, req.UserUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type DomainMembersRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

// Members 列出语音域成员。
func (h *DomainHandler) Members(c *gin.Context) {
	var req DomainMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	members, err := h.domainSvc.ListMembers(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"members": members})
}
