package handler

import (
	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"go_rtc/internal/repository"
	"strconv"

	"github.com/gin-gonic/gin"
)

type RoleHandler struct {
	roleRepo *repository.RoleRepository
}

func NewRoleHandler(roleRepo *repository.RoleRepository) *RoleHandler {
	return &RoleHandler{roleRepo: roleRepo}
}

// List
// @Summary      获取角色列表
// @Description  获取所有角色
// @Tags         角色
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /role/list [get]
func (h *RoleHandler) List(c *gin.Context) {
	roles, err := h.roleRepo.List()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, roles)
}

// Create
// @Summary      创建角色
// @Description  创建新角色
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{name=string}  true  "角色名称"
// @Success      200      {object}  pkg.Response
// @Router       /role/create [post]
func (h *RoleHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	role := &model.Role{Name: req.Name}
	if err := h.roleRepo.Create(role); err != nil {
		pkg.Fail(c, pkg.ALREADY_EXISTS, "role already exists")
		return
	}

	// 重新加载角色缓存
	if roles, err := h.roleRepo.List(); err == nil {
		model.LoadRoleCache(roles)
	}

	pkg.Success(c, role)
}

// Delete
// @Summary      删除角色
// @Description  根据角色 ID 删除角色
// @Tags         角色
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "角色 ID"
// @Success      200  {object}  pkg.Response
// @Router       /role/{id} [delete]
func (h *RoleHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	if err := h.roleRepo.Delete(uint(id)); err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 重新加载角色缓存
	if roles, err := h.roleRepo.List(); err == nil {
		model.LoadRoleCache(roles)
	}

	pkg.Success(c, nil)
}
