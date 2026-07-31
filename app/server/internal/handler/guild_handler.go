package handler

import (
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
)

type GuildHandler struct {
	guildSvc      *service.GuildService
	permSvc       *service.PermissionService
	onGuildDelete func(string)
}

func NewGuildHandler(guildSvc *service.GuildService, permSvc *service.PermissionService) *GuildHandler {
	return &GuildHandler{guildSvc: guildSvc, permSvc: permSvc}
}

// SetOnGuildDelete 注入删除后的信令清理回调（生产环境为 signalHub.OnGuildDelete）。
func (h *GuildHandler) SetOnGuildDelete(fn func(string)) {
	h.onGuildDelete = fn
}

func (h *GuildHandler) hasPermission(c *gin.Context, code string) bool {
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

type CreateGuildRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

// Create 创建语音服务器。
func (h *GuildHandler) Create(c *gin.Context) {
	var req CreateGuildRequest
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
	guild, err := h.guildSvc.Create(req.Name, req.Description, userUUID, req.IsPublic)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, guild)
}

type UUIDRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

// Get 获取语音服务器详情。
func (h *GuildHandler) Get(c *gin.Context) {
	var req UUIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	guild, err := h.guildSvc.GetByUUID(req.UUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, guild)
}

type ListGuildRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

// List 列出所有语音服务器。
func (h *GuildHandler) List(c *gin.Context) {
	var req ListGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page = 1
		req.PageSize = 20
	}
	guilds, total, err := h.guildSvc.List(req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"guilds": guilds, "total": total})
}

// ListPublic 列出公开语音服务器。
func (h *GuildHandler) ListPublic(c *gin.Context) {
	var req ListGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page = 1
		req.PageSize = 20
	}
	guilds, total, err := h.guildSvc.ListPublic(req.Page, req.PageSize, strings.TrimSpace(req.Keyword))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"guilds": guilds, "total": total})
}

// MyGuilds 返回当前用户加入的 Guild UUID 列表。
func (h *GuildHandler) MyGuilds(c *gin.Context) {
	userUUIDVal, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	userUUID, _ := userUUIDVal.(string)
	uuids, err := h.guildSvc.ListUserGuilds(userUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"guild_uuids": uuids})
}

type UpdateGuildRequest struct {
	UUID        string  `json:"uuid" binding:"required"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IconURL     *string `json:"icon_url"`
	IsPublic    *bool   `json:"is_public"`
	MaxRooms    *uint   `json:"max_rooms"`
}

// Update 更新语音服务器信息。
func (h *GuildHandler) Update(c *gin.Context) {
	var req UpdateGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	guild, err := h.guildSvc.GetByUUID(req.UUID)
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
	if !h.hasPermission(c, permcode.PermGuildManage) && !h.guildSvc.IsOwner(req.UUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not guild owner or missing permission")
		return
	}
	if req.Name != nil {
		guild.Name = *req.Name
	}
	if req.Description != nil {
		guild.Description = *req.Description
	}
	if req.IconURL != nil {
		guild.IconURL = *req.IconURL
	}
	if req.IsPublic != nil {
		guild.IsPublic = *req.IsPublic
	}
	if req.MaxRooms != nil {
		guild.MaxRooms = *req.MaxRooms
	}
	if err := h.guildSvc.Update(guild); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, guild)
}

type DeleteGuildRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

// Delete 删除语音服务器。仅 Owner 和持有 guild:delete 权限的用户可调用。
func (h *GuildHandler) Delete(c *gin.Context) {
	var req DeleteGuildRequest
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
	permOK := h.hasPermission(c, permcode.PermGuildDelete)
	ownerOK := h.guildSvc.IsOwner(req.UUID, userUUID)
	if !permOK && !ownerOK {
		pkg.Fail(c, pkg.FORBIDDEN, "not guild owner or missing permission")
		return
	}
	if err := h.guildSvc.Delete(req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.onGuildDelete != nil {
		h.onGuildDelete(req.UUID)
	}
	pkg.Success(c, nil)
}

type JoinGuildRequest struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

// Join 通过邀请码加入语音服务器。
func (h *GuildHandler) Join(c *gin.Context) {
	var req JoinGuildRequest
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
	guild, err := h.guildSvc.Join(req.InviteCode, userUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, guild)
}

// Preview 根据邀请码或邀请链接返回 Guild 信息，供加入前确认。
func (h *GuildHandler) Preview(c *gin.Context) {
	var req JoinGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	guild, err := h.guildSvc.GetByInviteCode(strings.TrimSpace(req.InviteCode))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, guild)
}

type LeaveGuildRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

// Leave 离开语音服务器。
func (h *GuildHandler) Leave(c *gin.Context) {
	var req LeaveGuildRequest
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
	if err := h.guildSvc.Leave(req.UUID, userUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type KickGuildMemberRequest struct {
	GuildUUID string `json:"guild_uuid" binding:"required"`
	UserUUID  string `json:"user_uuid" binding:"required"`
}

// Kick 踢出成员。仅 Owner/Admin 或持有 guild:kick 权限的用户可调用。
func (h *GuildHandler) Kick(c *gin.Context) {
	var req KickGuildMemberRequest
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
	permOK := h.hasPermission(c, permcode.PermGuildKick)
	roleOK := h.guildSvc.HasGuildRole(req.GuildUUID, userUUID, service.GuildRoleAdmin)
	if !permOK && !roleOK {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient guild role or permission")
		return
	}
	if err := h.guildSvc.Kick(req.GuildUUID, req.UserUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type GuildMembersRequest struct {
	GuildUUID string `json:"guild_uuid" binding:"required"`
}

// Members 列出语音服务器成员。
func (h *GuildHandler) Members(c *gin.Context) {
	var req GuildMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	members, err := h.guildSvc.ListMembers(req.GuildUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"members": members})
}
