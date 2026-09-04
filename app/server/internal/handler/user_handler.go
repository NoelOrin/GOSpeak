package handler

import (
	"fmt"
	"strings"

	"GOSpeak/internal/audit"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
	permSvc     *service.PermissionService
	auditor     *audit.Service
}

func NewUserHandler(userService *service.UserService, permSvc *service.PermissionService, _ ...*service.StorageService) *UserHandler {
	// storage 已注入 UserService；保留可选参数兼容旧 DI 调用
	return &UserHandler{userService: userService, permSvc: permSvc}
}

// SetAuditor 注入审计服务，用于记录删除用户等敏感操作。
func (h *UserHandler) SetAuditor(a *audit.Service) { h.auditor = a }

var presetAvatarPaths = []string{
	"/presets/avatar-1.svg",
	"/presets/avatar-2.svg",
	"/presets/avatar-3.svg",
	"/presets/avatar-4.svg",
	"/presets/avatar-5.svg",
	"/presets/avatar-6.svg",
	"/presets/avatar-7.svg",
	"/presets/avatar-8.svg",
}

// PresetAvatars
// @Summary      获取预设头像列表
// @Description  获取个人资料页可选的本地预设头像 URL
// @Tags         用户
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /user/preset-avatars [get]
func (h *UserHandler) PresetAvatars(c *gin.Context) {
	pkg.Success(c, gin.H{"avatars": presetAvatarPaths})
}

// GetProfile
// @Summary      获取用户资料
// @Description  获取当前已认证用户的资料信息
// @Tags         用户
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /user/profile [post]
func (h *UserHandler) GetProfile(c *gin.Context) {
	userUUID, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	uuidStr, ok := userUUID.(string)
	if !ok || uuidStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	user, err := h.userService.GetByUUID(uuidStr)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	if h.permSvc != nil {
		user.Permissions = h.permSvc.GetRolePermissions(user.Role)
	}
	if len(user.Permissions) == 0 {
		user.Permissions = model.DefaultRolePermissions[user.Role]
	}
	pkg.Success(c, user)
}

// GetByID
// @Summary      根据 ID 获取用户
// @Description  通过用户 ID 获取用户信息
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint}  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Router       /user/get [post]
func (h *UserHandler) GetByID(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	user, err := h.userService.GetByID(req.ID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, user)
}

// List
// @Summary      获取用户列表
// @Description  分页获取用户列表
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{page=int,page_size=int}  true  "分页参数"
// @Success      200       {object}  pkg.Response
// @Router       /user/list [post]
func (h *UserHandler) List(c *gin.Context) {
	var req struct {
		Page        int    `json:"page"`
		PageSize    int    `json:"page_size"`
		ExcludeBots bool   `json:"exclude_bots"`
		Keyword     string `json:"keyword"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	users, total, err := h.userService.List(req.Page, req.PageSize, req.ExcludeBots, strings.TrimSpace(req.Keyword))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"list":  users,
		"total": total,
		"page":  req.Page,
	})
}

// Delete
// @Summary      删除用户
// @Description  根据用户 ID 删除用户
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint}  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Router       /user/delete [post]
func (h *UserHandler) Delete(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.userService.Delete(req.ID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.auditor != nil {
		ua, un := auditActor(c)
		h.auditor.Log(audit.Entry{
			ActorUUID:  ua,
			ActorName:  un,
			Action:     audit.ActionDeleteUser,
			TargetType: audit.TargetUser,
			TargetID:   fmt.Sprintf("%d", req.ID),
			IP:         audit.AuditIP(c),
			Success:    true,
		})
	}

	pkg.Success(c, nil)
}

// GetByName
// @Summary      根据用户名获取用户
// @Description  通过用户名 (identity) 获取用户信息，用于房间成员资料查询
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{identity=string}  true  "用户名"
// @Success      200  {object}  pkg.Response
// @Router       /user/info [post]
func (h *UserHandler) GetByName(c *gin.Context) {
	var req struct {
		Identity string `json:"identity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	user, err := h.userService.GetByName(req.Identity)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, user)
}

// UpdateProfile
// @Summary      更新当前用户资料
// @Description  更新当前已认证用户的显示名称和头像
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      object  true  "用户资料"
// @Success      200   {object}  pkg.Response
// @Router       /user/update-profile [post]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userUUID, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	uuidStr, ok := userUUID.(string)
	if !ok || uuidStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Avatar      string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	user, err := h.userService.UpdateProfile(uuidStr, req.DisplayName, req.Avatar)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, user)
}

// UploadAvatar
// @Summary      上传头像
// @Description  上传当前用户的头像图片文件
// @Tags         用户
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        avatar  formData  file  true  "头像文件（支持 jpeg、png、gif、webp，最大 5MB）"
// @Success      200     {object}  pkg.Response
// @Router       /user/upload-avatar [post]
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	userUUID, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	uuidStr, ok := userUUID.(string)
	if !ok || uuidStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "avatar file is required")
		return
	}

	src, err := file.Open()
	if err != nil {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "failed to open uploaded file")
		return
	}
	defer src.Close()

	avatarURL, user, err := h.userService.UploadAvatar(
		uuidStr,
		file.Filename,
		file.Header.Get("Content-Type"),
		file.Size,
		src,
	)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{"avatar": avatarURL, "user": user})
}

// UpdateRole
// @Summary      更新用户角色
// @Description  管理员更新用户角色（admin/user）
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint,role=string}  true  "用户 ID 和角色"
// @Success      200   {object}  pkg.Response
// @Router       /user/update-role [post]
func (h *UserHandler) UpdateRole(c *gin.Context) {
	var req struct {
		ID   uint   `json:"id" binding:"required"`
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.userService.UpdateRole(req.ID, req.Role); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
