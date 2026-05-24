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

func (h *UserHandler) GetProfile(c *gin.Context) {
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	user, err := h.userService.GetByID(0)
	if err != nil {
		pkg.FailWithMsg(c, pkg.NOT_FOUND, err.Error())
		return
	}
	_ = usernameStr

	pkg.Success(c, gin.H{
		"username": user.Name,
		"uuid":     user.UUID,
	})
}

func (h *UserHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	user, err := h.userService.GetByID(uint(id))
	if err != nil {
		pkg.FailWithMsg(c, pkg.NOT_FOUND, err.Error())
		return
	}

	pkg.Success(c, user)
}

func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.userService.List(page, pageSize)
	if err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, gin.H{
		"list":  users,
		"total": total,
		"page":  page,
	})
}

func (h *UserHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.FailWithMsg(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	if err := h.userService.Delete(uint(id)); err != nil {
		pkg.FailWithMsg(c, pkg.INTERNAL_ERROR, err.Error())
		return
	}

	pkg.Success(c, nil)
}