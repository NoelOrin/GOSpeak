package handler

import (
	"github.com/gin-gonic/gin"

	"GOSpeak/internal/buildinfo"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
)

// SystemUpdateHandler 服务端更新检查（仅 admin）。
//
// 与桌面端更新体系独立：桌面端走 MyGo 签名更新（app/desktop/updater.go），
// 这里只提供主体服务端（v* 发布）的版本比对与发布说明查看，不涉及安装。
type SystemUpdateHandler struct {
	svc *service.SystemUpdateService
}

func NewSystemUpdateHandler(svc *service.SystemUpdateService) *SystemUpdateHandler {
	return &SystemUpdateHandler{svc: svc}
}

// requireAdmin 防御性二次校验，与 HealthStream 保持一致：路由已挂
// middleware.RequireRole("admin")，此处兜底防止误注册到公开路由泄露版本信息。
func (h *SystemUpdateHandler) requireAdmin(c *gin.Context) bool {
	roleVal, ok := c.Get("role")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST)
		return false
	}
	role, _ := roleVal.(string)
	if role != "admin" {
		pkg.Fail(c, pkg.FORBIDDEN)
		return false
	}
	return true
}

// CurrentVersion 返回运行中服务端的版本信息（无网络请求）。
func (h *SystemUpdateHandler) CurrentVersion(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	pkg.Success(c, gin.H{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
	})
}

// CheckUpdate 检查主体服务端是否有新版本（读 GitHub Releases，带缓存）。
func (h *SystemUpdateHandler) CheckUpdate(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	info, err := h.svc.Check(c.Request.Context())
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, info)
}
