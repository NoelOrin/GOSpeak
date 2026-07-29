package guild

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.GuildHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermGuildCreate), h.Create)
	r.POST("/get", middleware.RequirePermission(permcode.PermGuildRead), h.Get)
	r.POST("/list", middleware.RequirePermission(permcode.PermGuildRead), h.List)
	r.POST("/list-public", h.ListPublic)
	r.POST("/my-guilds", h.MyGuilds)
	r.POST("/update", middleware.RequirePermission(permcode.PermGuildManage), h.Update)
	r.POST("/delete", middleware.RequirePermission(permcode.PermGuildDelete), h.Delete)
	r.POST("/join", h.Join)
	r.POST("/leave", h.Leave)
	r.POST("/kick", middleware.RequirePermission(permcode.PermGuildKick), h.Kick)
	r.POST("/members", middleware.RequirePermission(permcode.PermGuildRead), h.Members)
}
