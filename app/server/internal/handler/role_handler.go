package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type RoleHandler struct {
	roleSvc *service.RoleService
}

func NewRoleHandler(roleSvc *service.RoleService) *RoleHandler {
	return &RoleHandler{roleSvc: roleSvc}
}

// List
// @Summary      获取角色列表
// @Description  获取所有角色
// @Tags         角色
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /role/list [post]
func (h *RoleHandler) List(c *gin.Context) {
	roles, err := h.roleSvc.List()
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

	role, err := h.roleSvc.Create(req.Name)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, role)
}

// Update
// @Summary      更新角色
// @Description  更新角色名称
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint,name=string}  true  "角色 ID 和名称"
// @Success      200      {object}  pkg.Response
// @Router       /role/update [post]
func (h *RoleHandler) Update(c *gin.Context) {
	var req struct {
		ID   uint   `json:"id" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	role, err := h.roleSvc.Update(req.ID, req.Name)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, role)
}

// Delete
// @Summary      删除角色
// @Description  根据角色 ID 删除角色
// @Tags         角色
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{id=uint}  true  "角色 ID"
// @Success      200  {object}  pkg.Response
// @Router       /role/delete [post]
func (h *RoleHandler) Delete(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.roleSvc.Delete(req.ID); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
