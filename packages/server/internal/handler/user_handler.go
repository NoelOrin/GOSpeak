package handler

import (
	"go_rtc/internal/pkg"
	"go_rtc/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// GetProfile
// @Summary      获取用户资料
// @Description  获取当前已认证用户的资料信息
// @Tags         用户
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /user/profile [get]
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

	pkg.Success(c, user)
}

// GetByID
// @Summary      根据 ID 获取用户
// @Description  通过用户 ID 获取用户信息
// @Tags         用户
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Router       /user/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	user, err := h.userService.GetByID(uint(id))
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
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int  false  "页码（默认 1）"
// @Param        page_size query  int  false  "每页条数（默认 20，最大 100）"
// @Success      200       {object}  pkg.Response
// @Router       /user/list [get]
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.userService.List(page, pageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"list":  users,
		"total": total,
		"page":  page,
	})
}

// Delete
// @Summary      删除用户
// @Description  根据用户 ID 删除用户
// @Tags         用户
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "用户 ID"
// @Success      200  {object}  pkg.Response
// @Router       /user/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	if err := h.userService.Delete(uint(id)); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}

// UpdateRole
// @Summary      更新用户角色
// @Description  管理员更新用户角色（admin/user）
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int     true  "用户 ID"
// @Param        role  body      object  true  "角色信息"
// @Success      200   {object}  pkg.Response
// @Router       /user/{id}/role [put]
func (h *UserHandler) UpdateRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.userService.UpdateRole(uint(id), req.Role); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
