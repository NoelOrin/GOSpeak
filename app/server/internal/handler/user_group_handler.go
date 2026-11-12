package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type UserGroupHandler struct {
	groupSvc *service.UserGroupService
	userSvc  *service.UserService
}

func NewUserGroupHandler(groupSvc *service.UserGroupService, userSvc *service.UserService) *UserGroupHandler {
	return &UserGroupHandler{groupSvc: groupSvc, userSvc: userSvc}
}

// List
// @Summary      获取当前用户分组
// @Description  获取当前登录用户创建的用户分组列表
// @Tags         用户分组
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /user-group/list [post]
func (h *UserGroupHandler) List(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	groups, err := h.groupSvc.List(userID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"groups": groups})
}

// Create
// @Summary      创建用户分组
// @Description  为当前用户创建一个命名分组
// @Tags         用户分组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{group_name=string}  true  "分组名"
// @Success      200      {object}  pkg.Response
// @Router       /user-group/create [post]
func (h *UserGroupHandler) Create(c *gin.Context) {
	var req struct {
		GroupName string `json:"group_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	group, err := h.groupSvc.Create(userID, req.GroupName)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, group)
}

// Update
// @Summary      重命名用户分组
// @Description  重命名当前用户的一个分组
// @Tags         用户分组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint,group_name=string}  true  "分组 ID 和新名称"
// @Success      200      {object}  pkg.Response
// @Router       /user-group/update [post]
func (h *UserGroupHandler) Update(c *gin.Context) {
	var req struct {
		ID        uint   `json:"id" binding:"required"`
		GroupName string `json:"group_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	if err := h.groupSvc.Rename(req.ID, userID, req.GroupName); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// Delete
// @Summary      删除用户分组
// @Description  删除当前用户的一个分组
// @Tags         用户分组
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint}  true  "分组 ID"
// @Success      200      {object}  pkg.Response
// @Router       /user-group/delete [post]
func (h *UserGroupHandler) Delete(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userID, ok := h.currentUserID(c)
	if !ok {
		return
	}
	if err := h.groupSvc.Delete(req.ID, userID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *UserGroupHandler) currentUserID(c *gin.Context) (uint, bool) {
	userUUID, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return 0, false
	}
	uuidStr, _ := userUUID.(string)
	if uuidStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return 0, false
	}
	user, err := h.userSvc.GetByUUID(uuidStr)
	if err != nil {
		pkg.HandleError(c, err)
		return 0, false
	}
	return user.ID, true
}
