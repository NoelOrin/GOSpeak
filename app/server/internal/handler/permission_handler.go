package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	permSvc *service.PermissionService
}

func NewPermissionHandler(permSvc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{permSvc: permSvc}
}

// ListPermissions
// @Summary      获取所有权限定义
// @Description  列出系统中所有可用的权限码
// @Tags         权限
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /permission/list [post]
func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	perms, err := h.permSvc.ListPermissions()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, perms)
}

// GetRolePermissions
// @Summary      获取角色权限
// @Description  获取指定角色的权限码列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{role=string}  true  "角色名称"
// @Success      200   {object}  pkg.Response
// @Router       /permission/role [post]
func (h *PermissionHandler) GetRolePermissions(c *gin.Context) {
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	codes := h.permSvc.GetRolePermissions(req.Role)
	pkg.Success(c, gin.H{
		"role":        req.Role,
		"permissions": codes,
	})
}

type SyncRolePermissionsRequest struct {
	Role        string   `json:"role" binding:"required"`
	Permissions []string `json:"permissions" binding:"required"`
}

// SyncRolePermissions
// @Summary      同步角色权限
// @Description  全量覆盖指定角色的权限列表
// @Tags         权限
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      SyncRolePermissionsRequest  true  "角色和权限码列表"
// @Success      200      {object}  pkg.Response
// @Router       /permission/sync [post]
func (h *PermissionHandler) SyncRolePermissions(c *gin.Context) {
	var req SyncRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.permSvc.SyncRolePermissions(req.Role, req.Permissions); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"role":        req.Role,
		"permissions": req.Permissions,
	})
}
