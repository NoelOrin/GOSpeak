package router

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"
	authRoutes "go_rtc/internal/router/routes/auth"
	oauthRoutes "go_rtc/internal/router/routes/oauth"
	roleRoutes "go_rtc/internal/router/routes/role"
	signalRoutes "go_rtc/internal/router/routes/signal"
	swaggerRoutes "go_rtc/internal/router/routes/swagger"
	userRoutes "go_rtc/internal/router/routes/user"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"

	_ "go_rtc/docs"
)

type Handlers struct {
	Auth   *handler.AuthHandler
	User   *handler.UserHandler
	Signal *handler.SignalHandler
	OAuth  *handler.OAuthHandler
	Role   *handler.RoleHandler
}

func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	swaggerRoutes.Register(r)

	api := r.Group("/api/v1")
	authRoutes.Register(api.Group("/auth"), h.Auth)
	signalRoutes.Register(api.Group("/signal"), h.Signal)
	oauthRoutes.Register(api.Group("/oauth"), h.OAuth)

	protected := api.Group("")
	protected.Use(middleware.JWTAuth())
	protected.Use(middleware.BanCheck())
	userRoutes.Register(protected.Group("/user"), h.User)
	authRoutes.RegisterProtected(protected.Group("/auth"), h.Auth)
	oauthRoutes.RegisterAdmin(protected.Group("/oauth/admin"), h.OAuth)
	roleRoutes.RegisterProtected(protected.Group("/role"), h.Role)

	return r
}

func SetupSocketRoutes(server *socketio.Server, signalHub interface {
	SetupRoutes(*socketio.Server)
}) {
	signalHub.SetupRoutes(server)
}
