package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"GOSpeak/internal/audit"
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	roomSvc             *service.RoomService
	permSvc             *service.PermissionService
	domainSvc           *service.DomainService
	controlPublisher    ControlPublisher
	roomListBroadcaster roomListBroadcaster
	auditor             *audit.Service
}

func NewRoomHandler(roomSvc *service.RoomService, permSvc *service.PermissionService, domainSvc *service.DomainService) *RoomHandler {
	return &RoomHandler{roomSvc: roomSvc, permSvc: permSvc, domainSvc: domainSvc}
}

// SetAuditor 注入审计服务，用于记录删除房间等敏感操作。
func (h *RoomHandler) SetAuditor(a *audit.Service) { h.auditor = a }

// SetControlPublisher 注入集群控制命令发布器。
func (h *RoomHandler) SetControlPublisher(p ControlPublisher) {
	h.controlPublisher = p
}

// roomListBroadcaster 注入信号层的房间列表广播器（Hub.BroadcastRoomList）。
type roomListBroadcaster interface {
	BroadcastRoomList(domainUUID string)
}

// SetRoomListBroadcaster 注入 Hub.BroadcastRoomList，供 Create/Update 触发房间列表广播。
func (h *RoomHandler) SetRoomListBroadcaster(b roomListBroadcaster) {
	h.roomListBroadcaster = b
}

func currentUserUUID(c *gin.Context) string {
	if v, ok := c.Get("user_uuid"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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

func (h *RoomHandler) canManageRoom(c *gin.Context, room *model.Room, perm string) bool {
	if domainPermissionGranted(c, room.DomainUUID, perm, h.domainSvc, h.permSvc) {
		return true
	}
	// 平台房间与域房间都只保留创建者管理权，不回退到其它全局角色权限。
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	return room.CreatedBy == usernameStr
}

func roleFromContext(c *gin.Context) string {
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	return roleStr
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

	usernameVal, ok := c.Get("username")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	username, _ := usernameVal.(string)
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
	if !middleware.IsDomainMember(domainUUID, currentUserUUID(c)) {
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

	if h.roomListBroadcaster != nil {
		h.roomListBroadcaster.BroadcastRoomList(domainUUID)
	}

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
		if !middleware.IsDomainMember(room.DomainUUID, currentUserUUID(c)) {
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
		if !middleware.IsDomainMember(domainUUID, currentUserUUID(c)) {
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

	if room.DomainUUID != "" && !middleware.IsDomainMember(room.DomainUUID, currentUserUUID(c)) {
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

	if room.DomainUUID != "" && !middleware.IsDomainMember(room.DomainUUID, currentUserUUID(c)) {
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
			IP:         c.ClientIP(),
			Success:    true,
		})
	}

	if h.roomListBroadcaster != nil {
		h.roomListBroadcaster.BroadcastRoomList(room.DomainUUID)
	}

	// DB 已删除；control 失败只告警，由 KV/房间列表最终一致清理，不再返回误导性 5xx。
	if h.controlPublisher != nil {
		if err := h.controlPublisher.PublishControl(cluster.ControlCommand{
			Command:    cluster.CommandDeleteRoom,
			DomainUUID: room.DomainUUID,
			Room:       room.Name,
		}); err != nil {
			log.Printf("[Room] publish delete control failed id=%d room=%q err=%v", req.ID, room.Name, err)
		}
	}

	pkg.Success(c, nil)
}
