package handler

import (
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	roomSvc   *service.RoomService
	permSvc   *service.PermissionService
	domainSvc *service.DomainService
}

func NewRoomHandler(roomSvc *service.RoomService, permSvc *service.PermissionService, domainSvc *service.DomainService) *RoomHandler {
	return &RoomHandler{roomSvc: roomSvc, permSvc: permSvc, domainSvc: domainSvc}
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
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if h.permSvc != nil && h.permSvc.HasPermission(roleFromContext(c), perm) {
		return true
	}
	if room.DomainUUID != "" && h.domainSvc != nil &&
		h.domainSvc.HasDomainRole(room.DomainUUID, currentUserUUID(c), service.DomainRoleAdmin) {
		return true
	}
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
	domainUUID := domainUUIDFromContext(c)
	if domainUUID != "" && !middleware.IsDomainMember(domainUUID, currentUserUUID(c)) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
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

	pkg.Success(c, room)
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

	if room.DomainUUID != "" && !middleware.IsDomainMember(room.DomainUUID, currentUserUUID(c)) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
		return
	}

	pkg.Success(c, room)
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
		Page       int    `json:"page"`
		PageSize   int    `json:"page_size"`
		Type       string `json:"type"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	domainUUID := domainUUIDFromContext(c)
	if domainUUID != "" && !middleware.IsDomainMember(domainUUID, currentUserUUID(c)) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
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
		"rooms": rooms,
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

	pkg.Success(c, room)
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

	pkg.Success(c, nil)
}
