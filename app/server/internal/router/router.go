package router

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	authRoutes "GOSpeak/internal/router/routes/auth"
	botRoutes "GOSpeak/internal/router/routes/bot"
	clusterRoutes "GOSpeak/internal/router/routes/cluster"
	conversationRoutes "GOSpeak/internal/router/routes/conversation"
	domainRoutes "GOSpeak/internal/router/routes/domain"
	emailRoutes "GOSpeak/internal/router/routes/email"
	emailConfigRoutes "GOSpeak/internal/router/routes/email_config"
	messageRoutes "GOSpeak/internal/router/routes/message"
	muteRoutes "GOSpeak/internal/router/routes/mute"
	oauthRoutes "GOSpeak/internal/router/routes/oauth"
	permissionRoutes "GOSpeak/internal/router/routes/permission"
	pluginRoutes "GOSpeak/internal/router/routes/plugin"
	roleRoutes "GOSpeak/internal/router/routes/role"
	roomRoutes "GOSpeak/internal/router/routes/room"
	sfuConfigRoutes "GOSpeak/internal/router/routes/sfu_config"
	signalRoutes "GOSpeak/internal/router/routes/signal"
	srsRoutes "GOSpeak/internal/router/routes/srs"
	storageRoutes "GOSpeak/internal/router/routes/storage"
	swaggerRoutes "GOSpeak/internal/router/routes/swagger"
	systemRoutes "GOSpeak/internal/router/routes/system"
	userRoutes "GOSpeak/internal/router/routes/user"
	userGroupRoutes "GOSpeak/internal/router/routes/user_group"

	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"GOSpeak/internal/webui"

	_ "GOSpeak/docs"
)

type Handlers struct {
	Auth         *handler.AuthHandler
	User         *handler.UserHandler
	Signal       *handler.SignalHandler
	UserGroup    *handler.UserGroupHandler
	Cloudflare   *handler.CloudflareHandler
	OAuth        *handler.OAuthHandler
	Role         *handler.RoleHandler
	Room         *handler.RoomHandler
	Permission   *handler.PermissionHandler
	Mute         *handler.MuteHandler
	Message      *handler.MessageHandler
	SFUConfig    *handler.SFUConfigHandler
	Storage      *handler.StorageHandler
	Email        *handler.EmailVerificationHandler
	EmailConfig  *handler.EmailConfigHandler
	Monitor      *handler.MonitorHandler
	SRSCallback  *handler.SRSCallbackHandler
	Bot          *handler.BotHandler
	Plugin       *handler.PluginHandler
	Domain       *handler.DomainHandler
	Conversation *handler.ConversationHandler
	Cluster      *handler.ClusterHandler
	// PluginHost 用于挂载插件自定义路由
	PluginHost interface{ MountRoutes(*gin.RouterGroup) }
}

func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if repository.DB == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		if _, err := repository.DB.DB(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/uploads/*filepath", serveUploads)
	serveSPA(r)

	swaggerRoutes.Register(r)

	api := r.Group("/api/v1")
	role := ""
	if cfg := config.Current(); cfg != nil {
		role = cfg.ClusterRole
	}
	isWorker := role == "worker"
	if !isWorker {
		authRoutes.Register(api.Group("/auth"), h.Auth)
		emailRoutes.Register(api.Group("/email"), h.Email)
		oauthRoutes.Register(api.Group("/oauth"), h.OAuth)
	}
	signalRoutes.Register(api.Group("/signal"), h.Signal)
	srsRoutes.Register(api.Group("/srs"), h.SRSCallback)

	if isWorker {
		workerProtected := api.Group("")
		workerProtected.Use(middleware.JWTAuth())
		workerProtected.Use(middleware.BanCheck())
		signalRoutes.RegisterProtected(workerProtected.Group("/signal"), h.Signal, h.Cloudflare)
		systemRoutes.RegisterProtected(workerProtected.Group("/system"), h.Monitor)
		return r
	}

	protected := api.Group("")
	protected.Use(middleware.JWTAuth())
	protected.Use(middleware.BanCheck())
	userRoutes.Register(protected.Group("/user"), h.User)
	userGroupRoutes.RegisterProtected(protected.Group("/user-group"), h.UserGroup)
	authRoutes.RegisterProtected(protected.Group("/auth"), h.Auth)
	signalRoutes.RegisterProtected(protected.Group("/signal"), h.Signal, h.Cloudflare)
	oauthRoutes.RegisterAdmin(protected.Group("/oauth/admin"), h.OAuth)
	roleRoutes.RegisterProtected(protected.Group("/role"), h.Role)
	muteRoutes.RegisterProtected(protected.Group("/mute"), h.Mute)
	roomRoutes.RegisterProtected(protected.Group("/room"), h.Room)
	messageRoutes.RegisterProtected(protected.Group("/room"), h.Message)
	permissionRoutes.RegisterProtected(protected.Group("/permission"), h.Permission)
	sfuConfigRoutes.RegisterProtected(protected.Group("/sfu"), h.SFUConfig)
	storageRoutes.Register(protected.Group("/storage"), h.Storage)
	emailConfigRoutes.RegisterProtected(protected.Group("/email"), h.EmailConfig)
	botRoutes.RegisterProtected(protected.Group("/bot"), h.Bot)
	if h.Plugin != nil {
		pluginRoutes.RegisterProtected(protected.Group("/plugins"), h.Plugin)
	}
	if h.Conversation != nil {
		conversationRoutes.RegisterProtected(protected.Group("/conversation"), h.Conversation)
	}
	if h.PluginHost != nil {
		// route registry: /api/v1/plugins/:name/*
		pluginGroup := protected.Group("/plugins")
		pluginGroup.Use(middleware.RequirePermission(permcode.PermPluginManage))
		h.PluginHost.MountRoutes(pluginGroup)
	}
	if h.Domain != nil {
		domainRoutes.Register(protected.Group("/domain"), h.Domain)
	}
	if h.Cluster != nil {
		clusterRoutes.RegisterProtected(protected.Group("/cluster"), h.Cluster)
	}
	systemRoutes.RegisterProtected(protected.Group("/system"), h.Monitor)

	return r
}

// workerModeAPIFallback 拦截未注册的 worker API 路径：写方法 403，读方法 404。
func workerModeAPIFallback(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		pkg.Fail(c, pkg.FORBIDDEN, "worker mode does not accept business writes")
		c.Abort()
	default:
		c.Status(http.StatusNotFound)
	}
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

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") || strings.HasPrefix(path, "/swagger") {
			if strings.HasPrefix(path, "/api/") {
				if cfg := config.Current(); cfg != nil && cfg.ClusterRole == "worker" {
					workerModeAPIFallback(c)
					return
				}
			}
			c.Status(http.StatusNotFound)
			return
		}
		if fileServer != nil && hasFile(path) {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		if serveIndex != nil {
			serveIndex(c)
			return
		}
		c.Status(http.StatusNotFound)
	})
}

// serveUploads 以安全响应头提供上传文件，并拒绝路径穿越。
func serveUploads(c *gin.Context) {
	uploadDir := "uploads"
	if cfg := config.Current(); cfg != nil && cfg.StoragePathPrefix != "" {
		uploadDir = strings.TrimSuffix(cfg.StoragePathPrefix, "/")
	}
	clean := filepath.Clean("/" + strings.TrimPrefix(c.Param("filepath"), "/"))
	full := filepath.Join(uploadDir, strings.TrimPrefix(clean, "/"))
	abs, err := filepath.Abs(full)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	base, err := filepath.Abs(uploadDir)
	if err != nil || (abs != base && !strings.HasPrefix(abs, base+string(filepath.Separator))) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		c.Header("Content-Disposition", "inline")
	default:
		c.Header("Content-Disposition", "attachment")
	}
	http.ServeFile(c.Writer, c.Request, abs)
}
