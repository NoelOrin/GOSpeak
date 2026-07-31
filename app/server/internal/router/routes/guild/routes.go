package guild

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.RouterGroup, h *handler.GuildHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermGuildCreate), h.Create)
	r.POST("/get", middleware.RequireGuildMember(), h.Get)
	r.POST("/list", middleware.RequirePermission(permcode.PermGuildRead), h.List)
	r.POST("/list-public", h.ListPublic)
	r.POST("/my-guilds", h.MyGuilds)
	r.POST("/update", middleware.RequireGuildMember(), h.Update)
	r.POST("/delete", middleware.RequireGuildMember(), h.Delete)
	r.POST("/join", h.Join)
	r.POST("/preview", h.Preview)
	r.POST("/leave", middleware.RequireGuildMember(), h.Leave)
	r.POST("/kick", middleware.RequireGuildMember(), h.Kick)
	r.POST("/members", middleware.RequireGuildMember(), h.Members)
}
