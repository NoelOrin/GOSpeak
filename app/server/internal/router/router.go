package router

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	authRoutes "GOSpeak/internal/router/routes/auth"
	botRoutes "GOSpeak/internal/router/routes/bot"
	emailRoutes "GOSpeak/internal/router/routes/email"
	emailConfigRoutes "GOSpeak/internal/router/routes/email_config"
	muteRoutes "GOSpeak/internal/router/routes/mute"
	oauthRoutes "GOSpeak/internal/router/routes/oauth"
	permissionRoutes "GOSpeak/internal/router/routes/permission"
	roleRoutes "GOSpeak/internal/router/routes/role"
	roomRoutes "GOSpeak/internal/router/routes/room"
	sfuConfigRoutes "GOSpeak/internal/router/routes/sfu_config"
	signalRoutes "GOSpeak/internal/router/routes/signal"
	srsRoutes "GOSpeak/internal/router/routes/srs"
	storageRoutes "GOSpeak/internal/router/routes/storage"
	swaggerRoutes "GOSpeak/internal/router/routes/swagger"
	systemRoutes "GOSpeak/internal/router/routes/system"
	userRoutes "GOSpeak/internal/router/routes/user"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"

	_ "GOSpeak/docs"
)

type Handlers struct {
	Auth        *handler.AuthHandler
	User        *handler.UserHandler
	Signal      *handler.SignalHandler
	OAuth       *handler.OAuthHandler
	Role        *handler.RoleHandler
	Room        *handler.RoomHandler
	Permission  *handler.PermissionHandler
	Mute        *handler.MuteHandler
	SFUConfig   *handler.SFUConfigHandler
	Storage     *handler.StorageHandler
	Email       *handler.EmailVerificationHandler
	EmailConfig *handler.EmailConfigHandler
	Monitor     *handler.MonitorHandler
	SRSCallback *handler.SRSCallbackHandler
	Bot         *handler.BotHandler
}

func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.Static("/uploads", "./uploads")

	swaggerRoutes.Register(r)

	api := r.Group("/api/v1")
	authRoutes.Register(api.Group("/auth"), h.Auth)
	emailRoutes.Register(api.Group("/email"), h.Email)
	signalRoutes.Register(api.Group("/signal"), h.Signal)
	oauthRoutes.Register(api.Group("/oauth"), h.OAuth)
	srsRoutes.Register(api.Group("/srs"), h.SRSCallback)
	systemRoutes.Register(api.Group("/system"), h.Monitor)

	protected := api.Group("")
	protected.Use(middleware.BanCheck())
	userRoutes.Register(protected.Group("/user"), h.User)
	authRoutes.RegisterProtected(protected.Group("/auth"), h.Auth)
	signalRoutes.RegisterProtected(protected.Group("/signal"), h.Signal)
	oauthRoutes.RegisterAdmin(protected.Group("/oauth/admin"), h.OAuth)
	roleRoutes.RegisterProtected(protected.Group("/role"), h.Role)
	muteRoutes.RegisterProtected(protected.Group("/mute"), h.Mute)
	roomRoutes.RegisterProtected(protected.Group("/room"), h.Room)
	permissionRoutes.RegisterProtected(protected.Group("/permission"), h.Permission)
	sfuConfigRoutes.RegisterProtected(protected.Group("/sfu"), h.SFUConfig)
	storageRoutes.Register(protected.Group("/storage"), h.Storage)
	emailConfigRoutes.RegisterProtected(protected.Group("/email"), h.EmailConfig)
	botRoutes.RegisterProtected(protected.Group("/bot"), h.Bot)

	return r
}

func SetupSocketRoutes(server *socketio.Server, signalHub interface {
	SetupRoutes(*socketio.Server)
}) {
	signalHub.SetupRoutes(server)
}
