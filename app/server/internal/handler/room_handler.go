package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"GOSpeak/internal/audit"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// cacheBus 发布内部事件（与 service 层同形，事件名常量定义在 service/events.go）。
type cacheBus interface {
	PublishInternal(ctx context.Context, event string, payload interface{}) error
}

type RoomHandler struct {
	roomSvc    *service.RoomService
	permSvc    *service.PermissionService
	domainSvc  *service.DomainService
	eventBus   cacheBus
	auditor    *audit.Service
}

func NewRoomHandler(roomSvc *service.RoomService, permSvc *service.PermissionService, domainSvc *service.DomainService, eventBus cacheBus, auditor *audit.Service) *RoomHandler {
	return &RoomHandler{roomSvc: roomSvc, permSvc: permSvc, domainSvc: domainSvc, eventBus: eventBus, auditor: auditor}
}


// notifyRoomListChanged 房间元数据变更后发内部事件，由 signal Hub 单点重新广播
// 房间列表（REST/WS 双通道写同一视图的旧模式已移除）。
func (h *RoomHandler) notifyRoomListChanged(domainUUID string) {
	if h.eventBus == nil {
		return
	}
	payload := map[string]string{"domain_uuid": domainUUID}
	if err := h.eventBus.PublishInternal(context.Background(), service.EventRoomListChanged, payload); err != nil {
		logger.WithComponent("Room").Warnf("publish room list changed: %v", err)
	}
}

// currentUserUUID 优先取 JWT UserUUID；访客等 UserUUID 为空的身份回退 username。
func currentUserUUID(c *gin.Context) string {
	if uuid := middleware.CurrentUserUUID(c); uuid != "" {
		return uuid
	}
	return middleware.CurrentUsername(c)
}

// domainUUIDFromContext returns the domain_uuid resolved by RequireDomainMember middleware.
func domainUUIDFromContext(c *gin.Context) string {
	if v, ok := c.Get("domain_uuid"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func domainRoomAccessAllowed(c *gin.Context, domainUUID string) bool {
	return middleware.IsDomainMember(domainUUID, currentUserUUID(c)) || middleware.IsPlatformAdmin(c)
}

func (h *RoomHandler) canManageRoom(c *gin.Context, room *model.Room, perm string) bool {
	if !domainRoomAccessAllowed(c, room.DomainUUID) {
		return false
	}
	if domainPermissionGranted(c, room.DomainUUID, perm, h.domainSvc, h.permSvc) {
		return true
	}
	// 平台房间与域房间都只保留创建者管理权，不回退到其它全局角色权限。
	return room.CreatedBy == middleware.CurrentUsername(c)
}

func roleFromContext(c *gin.Context) string {
	return middleware.CurrentRole(c)
}

type CreateRoomRequest struct {
	Name          string `json:"name" binding:"required"`
	Password      string `json:"password"`
	Description   string `json:"description"`
	Limit         uint   `json:"limit"`
	AudioOnly     *bool  `json:"audio_only"`
	AllowAudience *bool  `json:"allow_audience"`
	Type          string `json:"type"`
	DomainUUID    string `json:"domain_uuid" binding:"required"`
}

// Create
// @Summary      创建房间
// @Description  创建新房间
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      CreateRoomRequest  true  "房间信息"
// @Success      200      {object}  pkg.Response
// @Router       /room/create [post]
func (h *RoomHandler) Create(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username := middleware.CurrentUsername(c)
	if username == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	domainUUID := strings.TrimSpace(req.DomainUUID)
	if ctxDomain := domainUUIDFromContext(c); ctxDomain != "" {
		if domainUUID != "" && domainUUID != ctxDomain {
			pkg.Fail(c, pkg.FORBIDDEN, "domain_uuid does not match the request context")
			return
		}
		domainUUID = ctxDomain
	}
	if domainUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	if !domainRoomAccessAllowed(c, domainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
		return
	}
	if !domainPermissionGranted(c, domainUUID, permcode.PermRoomCreate, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain room permission")
		return
	}
	audioOnly := true
	if req.AudioOnly != nil {
		audioOnly = *req.AudioOnly
	}
	allowAudience := true
	if req.AllowAudience != nil {
		allowAudience = *req.AllowAudience
	}
	room, err := h.roomSvc.CreateRoom(
		req.Name, req.Password, req.Description,
		req.Limit, audioOnly, allowAudience,
		username, req.Type,
		domainUUID,
	)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	h.notifyRoomListChanged(domainUUID)

	pkg.Success(c, service.RoomToDTO(room))
}

// Get
// @Summary      获取房间详情
// @Description  根据 ID 获取房间信息
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint}  true  "房间 ID"
// @Success      200      {object}  pkg.Response
// @Router       /room/get [post]
func (h *RoomHandler) Get(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	room, err := h.roomSvc.GetByID(req.ID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	if room.DomainUUID != "" {
		if !domainRoomAccessAllowed(c, room.DomainUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
			return
		}
	}
	if !domainPermissionGranted(c, room.DomainUUID, permcode.PermRoomRead, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain room permission")
		return
	}

	pkg.Success(c, service.RoomToDTO(room))
}

// List
// @Summary      房间列表
// @Description  分页获取房间列表
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{page=int,page_size=int}  true  "分页参数"
// @Success      200      {object}  pkg.Response
// @Router       /room/list [post]
func (h *RoomHandler) List(c *gin.Context) {
	var req struct {
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
		Type     string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	domainUUID := domainUUIDFromContext(c)
	if domainUUID != "" {
		if !domainRoomAccessAllowed(c, domainUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
			return
		}
	}
	if !domainPermissionGranted(c, domainUUID, permcode.PermRoomRead, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain room permission")
		return
	}

	var rooms []model.Room
	var total int64
	var err error
	if domainUUID != "" {
		rooms, total, err = h.roomSvc.List(req.Page, req.PageSize, req.Type, domainUUID)
	} else {
		rooms, total, err = h.roomSvc.ListPlatform(req.Page, req.PageSize, req.Type)
	}
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"rooms": service.RoomsToDTOs(rooms),
		"total": total,
		"page":  req.Page,
		"size":  req.PageSize,
	})
}

type UpdateRoomRequest struct {
	ID            uint    `json:"id" binding:"required"`
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Limit         *uint   `json:"limit"`
	AudioOnly     *bool   `json:"audio_only"`
	AllowAudience *bool   `json:"allow_audience"`
}

// Update
// @Summary      更新房间
// @Description  根据 ID 更新房间信息
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      UpdateRoomRequest  true  "更新内容"
// @Success      200      {object}  pkg.Response
// @Router       /room/update [post]
func (h *RoomHandler) Update(c *gin.Context) {
	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	room, err := h.roomSvc.GetByID(req.ID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	if room.DomainUUID != "" && !domainRoomAccessAllowed(c, room.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
		return
	}

	if !h.canManageRoom(c, room, permcode.PermRoomUpdate) {
		pkg.Fail(c, pkg.FORBIDDEN, "没有权限编辑该房间")
		return
	}

	if req.Name != nil {
		room.Name = *req.Name
	}
	if req.Description != nil {
		room.Description = *req.Description
	}
	if req.Limit != nil {
		room.Limit = *req.Limit
	}
	if req.AudioOnly != nil {
		room.AudioOnly = *req.AudioOnly
	}
	if req.AllowAudience != nil {
		room.AllowAudience = *req.AllowAudience
	}

	if err := h.roomSvc.Update(room); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, service.RoomToDTO(room))
}

// Delete
// @Summary      删除房间
// @Description  根据 ID 删除房间
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint}  true  "房间 ID"
// @Success      200      {object}  pkg.Response
// @Router       /room/delete [post]
func (h *RoomHandler) Delete(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	room, err := h.roomSvc.GetByID(req.ID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	if room.DomainUUID != "" && !domainRoomAccessAllowed(c, room.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
		return
	}

	if !h.canManageRoom(c, room, permcode.PermRoomDelete) {
		pkg.Fail(c, pkg.FORBIDDEN, "没有权限删除该房间")
		return
	}

	if err := h.roomSvc.Delete(req.ID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.auditor != nil {
		ua, un := auditActor(c)
		h.auditor.Log(audit.Entry{
			ActorUUID:  ua,
			ActorName:  un,
			Action:     audit.ActionDeleteRoom,
			TargetType: audit.TargetRoom,
			TargetID:   fmt.Sprintf("%d", req.ID),
			Detail:     fmt.Sprintf("name=%q domain=%s", room.Name, room.DomainUUID),
			IP:         audit.AuditIP(c),
			Success:    true,
		})
	}

	h.notifyRoomListChanged(room.DomainUUID)

	pkg.Success(c, nil)
}
