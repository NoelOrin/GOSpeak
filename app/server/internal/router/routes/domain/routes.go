package domain

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.DomainHandler) {
	r.POST("/create", middleware.RateLimit(10, time.Minute), middleware.RequirePermission(permcode.PermDomainCreate), h.Create)
	r.POST("/get", middleware.RequireDomainMember(), h.Get)
	r.POST("/list", middleware.RequirePermission(permcode.PermDomainRead), h.List)
	r.POST("/list-public", h.ListPublic)
	r.POST("/my-domains", h.MyDomains)
	r.POST("/update", middleware.RequireDomainMember(), h.Update)
	r.POST("/delete", middleware.RequireDomainMember(), h.Delete)
	r.POST("/join", h.Join)
	r.POST("/preview", h.Preview)
	r.POST("/leave", middleware.RequireDomainMember(), h.Leave)
	r.POST("/kick", middleware.RequireDomainMember(), h.Kick)
	r.POST("/members", middleware.RequireDomainMember(), h.Members)
	r.POST("/roles/list", middleware.RequireDomainMember(), h.ListRoles)
	r.POST("/roles/create", middleware.RequireDomainMember(), h.CreateRole)
	r.POST("/roles/update", middleware.RequireDomainMember(), h.UpdateRole)
	r.POST("/roles/delete", middleware.RequireDomainMember(), h.DeleteRole)
	r.POST("/members/update-role", middleware.RequireDomainMember(), h.UpdateMemberRole)
	r.POST("/my-permissions", middleware.RequireDomainMember(), h.MyPermissions)
}
