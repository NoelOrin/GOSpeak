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
// @Summary      Get user profile
// @Description  Get the profile of the currently authenticated user
// @Tags         User
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /user/profile [get]
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

// GetByID
// @Summary      Get user by ID
// @Description  Retrieve a user by their ID
// @Tags         User
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /user/{id} [get]
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

// List
// @Summary      List users
// @Description  Get a paginated list of users
// @Tags         User
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int  false  "Page number (default 1)"
// @Param        page_size query  int  false  "Items per page (default 20, max 100)"
// @Success      200       {object}  pkg.Response
// @Failure      500       {object}  pkg.Response
// @Router       /user/list [get]
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

// Delete
// @Summary      Delete user
// @Description  Delete a user by their ID
// @Tags         User
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  pkg.Response
// @Failure      500  {object}  pkg.Response
// @Router       /user/{id} [delete]
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