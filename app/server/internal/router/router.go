package router

import (
	"GOSpeak/internal/config"
	guildRoutes "GOSpeak/internal/router/routes/guild"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	authRoutes "GOSpeak/internal/router/routes/auth"
	botRoutes "GOSpeak/internal/router/routes/bot"
	emailRoutes "GOSpeak/internal/router/routes/email"
	emailConfigRoutes "GOSpeak/internal/router/routes/email_config"
	messageRoutes "GOSpeak/internal/router/routes/message"
	muteRoutes "GOSpeak/internal/router/routes/mute"
	oauthRoutes "GOSpeak/internal/router/routes/oauth"
	pluginRoutes "GOSpeak/internal/router/routes/plugin"
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

	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"

	"GOSpeak/internal/webui"

	_ "GOSpeak/docs"
)

type Handlers struct {
	Auth        *handler.AuthHandler
	User        *handler.UserHandler
	Signal      *handler.SignalHandler
	Cloudflare  *handler.CloudflareHandler
	OAuth       *handler.OAuthHandler
	Role        *handler.RoleHandler
	Room        *handler.RoomHandler
	Permission  *handler.PermissionHandler
	Mute        *handler.MuteHandler
	Message     *handler.MessageHandler
	SFUConfig   *handler.SFUConfigHandler
	Storage     *handler.StorageHandler
	Email       *handler.EmailVerificationHandler
	EmailConfig *handler.EmailConfigHandler
	Monitor     *handler.MonitorHandler
	SRSCallback *handler.SRSCallbackHandler
	Bot         *handler.BotHandler
	Plugin      *handler.PluginHandler
	Guild       *handler.GuildHandler
	// PluginHost 用于挂载插件自定义路由
	PluginHost  interface{ MountRoutes(*gin.RouterGroup) }
}

func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {
	r.Use(middleware.CORS())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.Static("/uploads", "./uploads")
	serveSPA(r)

	swaggerRoutes.Register(r)

	api := r.Group("/api/v1")
	authRoutes.Register(api.Group("/auth"), h.Auth)
	emailRoutes.Register(api.Group("/email"), h.Email)
	signalRoutes.Register(api.Group("/signal"), h.Signal)
	oauthRoutes.Register(api.Group("/oauth"), h.OAuth)
	srsRoutes.Register(api.Group("/srs"), h.SRSCallback)
	systemRoutes.Register(api.Group("/system"), h.Monitor)

	protected := api.Group("")
	protected.Use(middleware.JWTAuth())
	protected.Use(middleware.BanCheck())
	userRoutes.Register(protected.Group("/user"), h.User)
	authRoutes.RegisterProtected(protected.Group("/auth"), h.Auth)
	signalRoutes.RegisterProtected(protected.Group("/signal"), h.Signal, h.Cloudflare)
	oauthRoutes.RegisterAdmin(protected.Group("/oauth/admin"), h.OAuth)
	roleRoutes.RegisterProtected(protected.Group("/role"), h.Role)
	muteRoutes.RegisterProtected(protected.Group("/mute"), h.Mute)
	roomRoutes.RegisterProtected(protected.Group("/room"), h.Room)
	if h.Message != nil {
		messageRoutes.RegisterProtected(protected.Group("/message"), h.Message)
	}
	permissionRoutes.RegisterProtected(protected.Group("/permission"), h.Permission)
	sfuConfigRoutes.RegisterProtected(protected.Group("/sfu"), h.SFUConfig)
	storageRoutes.Register(protected.Group("/storage"), h.Storage)
	emailConfigRoutes.RegisterProtected(protected.Group("/email"), h.EmailConfig)
	botRoutes.RegisterProtected(protected.Group("/bot"), h.Bot)
	if h.Plugin != nil {
		pluginRoutes.RegisterProtected(protected.Group("/plugins"), h.Plugin)
	}
	if h.PluginHost != nil {
		// 插件自注册路由：/api/v1/plugins/:name/*
		h.PluginHost.MountRoutes(protected.Group("/plugins"))
	}
	if h.Guild != nil {
		guildRoutes.Register(protected.Group("/guild"), h.Guild)
	}

	return r
}

func SetupSocketRoutes(server *socketio.Server, signalHub interface {
	SetupRoutes(*socketio.Server)
}) {
	signalHub.SetupRoutes(server)
}

// serveSPA 托管前端构建产物。
// 路径优先级：STATIC_DIR > /app/static > ./static > go:embed 内嵌资源。
// 开发环境前端走 Vite；无外部目录且未嵌入前端时静默跳过。
func serveSPA(r *gin.Engine) {
	var fileServer http.Handler
	var hasFile func(path string) bool
	var serveIndex func(c *gin.Context)

	staticDir := ""
	if cfg := config.Current(); cfg != nil {
		staticDir = cfg.StaticDir
	}
	if staticDir == "" {
		for _, candidate := range []string{"/app/static", "./static"} {
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				staticDir = candidate
				break
			}
		}
	}

	if staticDir != "" {
		index := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			fsys := http.Dir(staticDir)
			fileServer = http.FileServer(fsys)
			hasFile = func(path string) bool {
				full := filepath.Join(staticDir, path)
				st, err := os.Stat(full)
				return err == nil && !st.IsDir()
			}
			serveIndex = func(c *gin.Context) { c.File(index) }
		}
	}

	// 外部目录不可用时，回退到编译期嵌入的前端资源
	if fileServer == nil {
		if fsys := webui.FS(); fsys != nil {
			fileServer = http.FileServer(fsys)
			hasFile = func(path string) bool {
				f, err := fsys.Open(strings.TrimPrefix(path, "/"))
				if err != nil {
					return false
				}
				defer f.Close()
				st, err := f.Stat()
				return err == nil && !st.IsDir()
			}
			serveIndex = func(c *gin.Context) {
				c.Request.URL.Path = "/"
				fileServer.ServeHTTP(c.Writer, c.Request)
			}
		}
	}

	if fileServer == nil {
		return
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/socket.io") || strings.HasPrefix(path, "/swagger") {
			c.Status(http.StatusNotFound)
			return
		}
		if hasFile(path) {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		serveIndex(c)
	})
}
