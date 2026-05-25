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
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	user, err := h.userService.GetByID(0)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	_ = usernameStr

	pkg.Success(c, gin.H{
		"username": user.Name,
		"uuid":     user.UUID,
	})
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
