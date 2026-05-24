package router

import (
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
)

type Handlers struct {
	Auth   *handler.AuthHandler
	User   *handler.UserHandler
	Signal *handler.SignalHandler
}

func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", h.Auth.Login)
			auth.POST("/register", h.Auth.Register)
			auth.POST("/refresh_token", h.Auth.GetRefreshToken)
		}

		signal := api.Group("/signal")
		{
			signal.POST("/token", h.Signal.GetJoinToken)
			signal.POST("/signal", h.Signal.Signal)
			signal.GET("/rooms", h.Signal.ListRooms)
			signal.GET("/participants", h.Signal.ListParticipants)
		}

		protected := api.Group("")
		protected.Use(middleware.JWTAuth())
		{
			user := protected.Group("/user")
			{
				user.GET("/profile", h.User.GetProfile)
				user.GET("/list", h.User.List)
				user.GET("/:id", h.User.GetByID)
				user.DELETE("/:id", h.User.Delete)
			}

			auth.POST("/logout", h.Auth.Logout)
			auth.POST("/refresh", h.Auth.RefreshToken)
		}
	}

	return r
}

func SetupSocketRoutes(server *socketio.Server, signalHub interface {
	SetupRoutes(*socketio.Server)
}) {
	signalHub.SetupRoutes(server)
}