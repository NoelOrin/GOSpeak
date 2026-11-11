package router

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	announcementRoutes "GOSpeak/internal/router/routes/announcement"
	auditRoutes "GOSpeak/internal/router/routes/audit"
	authRoutes "GOSpeak/internal/router/routes/auth"
	conversationRoutes "GOSpeak/internal/router/routes/conversation"
	domainRoutes "GOSpeak/internal/router/routes/domain"
	emailRoutes "GOSpeak/internal/router/routes/email"
	emailConfigRoutes "GOSpeak/internal/router/routes/email_config"
	guestRoutes "GOSpeak/internal/router/routes/guest"
	inviteRoutes "GOSpeak/internal/router/routes/invite"
	messageRoutes "GOSpeak/internal/router/routes/message"
	muteRoutes "GOSpeak/internal/router/routes/mute"
	oauthRoutes "GOSpeak/internal/router/routes/oauth"
	permissionRoutes "GOSpeak/internal/router/routes/permission"
	roleRoutes "GOSpeak/internal/router/routes/role"
	roomRoutes "GOSpeak/internal/router/routes/room"
	sfuConfigRoutes "GOSpeak/internal/router/routes/sfu_config"
	signalRoutes "GOSpeak/internal/router/routes/signal"
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
	// Config 由组合根注入，路由层不再读取全局 config.Current。
	Config *config.Config
	// ReadyCheck 探测 DB 等基础依赖，nil 时 /readyz 返回不可用。
	ReadyCheck   func() error
	Auth         *handler.AuthHandler
	Guest        *handler.GuestHandler
	Invite       *handler.InviteHandler
	User         *handler.UserHandler
	Signal       *handler.SignalHandler
	UserGroup    *handler.UserGroupHandler
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
	Domain       *handler.DomainHandler
	Conversation *handler.ConversationHandler
	Audit        *handler.AuditHandler
	Announcement *handler.AnnouncementHandler
}

func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {

	var cfg *config.Config
	if h != nil {
		cfg = h.Config
	}

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if h == nil || h.ReadyCheck == nil || h.ReadyCheck() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 静态服务与 LocalProvider 共用 STORAGE_LOCAL_DIR（默认 ./uploads），保证落盘与访问同源；
	// STORAGE_PATH_PREFIX 仅用于 S3 对象键前缀，本地存储忽略，避免出现 /uploads/uploads/... 重复路径。
	uploadDir := "uploads"
	if cfg != nil && cfg.StorageLocalDir != "" {
		uploadDir = cfg.StorageLocalDir
	}
	r.GET("/uploads/*filepath", func(c *gin.Context) { serveUploads(c, uploadDir) })
	serveSPA(r, cfg)

	swaggerRoutes.Register(r)

	api := r.Group("/api/v1")
	authRoutes.Register(api.Group("/auth"), h.Auth)
	if h.Guest != nil {
		guestRoutes.Register(api.Group("/auth"), h.Guest)
	}
	emailRoutes.Register(api.Group("/email"), h.Email)
	oauthRoutes.Register(api.Group("/oauth"), h.OAuth)
	signalRoutes.Register(api.Group("/signal"), h.Signal)
	if h.Invite != nil {
		inviteRoutes.RegisterPublic(api.Group("/invite"), h.Invite)
	}

	protected := api.Group("")
	protected.Use(middleware.JWTAuth())
	protected.Use(middleware.BanCheck())
	protected.Use(middleware.GuestGuard())
	userRoutes.Register(protected.Group("/user"), h.User)
	userGroupRoutes.RegisterProtected(protected.Group("/user-group"), h.UserGroup)
	authRoutes.RegisterProtected(protected.Group("/auth"), h.Auth)
	signalRoutes.RegisterProtected(protected.Group("/signal"), h.Signal)
	oauthRoutes.RegisterAdmin(protected.Group("/oauth/admin"), h.OAuth)
	roleRoutes.RegisterProtected(protected.Group("/role"), h.Role)
	muteRoutes.RegisterProtected(protected.Group("/mute"), h.Mute)
	roomRoutes.RegisterProtected(protected.Group("/room"), h.Room)
	messageRoutes.RegisterProtected(protected.Group("/room"), h.Message)
	permissionRoutes.RegisterProtected(protected.Group("/permission"), h.Permission)
	sfuConfigRoutes.RegisterProtected(protected.Group("/sfu"), h.SFUConfig)
	storageRoutes.Register(protected.Group("/storage"), h.Storage)
	emailConfigRoutes.RegisterProtected(protected.Group("/email"), h.EmailConfig)
	if h.Conversation != nil {
		conversationRoutes.RegisterProtected(protected.Group("/conversation"), h.Conversation)
	}
	if h.Domain != nil {
		domainRoutes.Register(protected.Group("/domain"), h.Domain)
	}
	if h.Guest != nil {
		guestRoutes.RegisterProtected(protected.Group(""), h.Guest)
	}
	auditRoutes.RegisterProtected(protected.Group("/audit"), h.Audit)
	if h.Announcement != nil {
		announcementRoutes.RegisterProtected(protected.Group("/announcement"), h.Announcement)
	}
	if h.Invite != nil {
		inviteRoutes.Register(protected.Group(""), h.Invite)
	}
	systemRoutes.RegisterProtected(protected.Group("/system"), h.Monitor)

	return r
}

// serveSPA 托管前端构建产物。
// 路径优先级：STATIC_DIR > /app/static > ./static > go:embed 内嵌资源。
// 开发环境前端走 Vite；无外部目录且未嵌入前端时静默跳过。
func serveSPA(r *gin.Engine, cfg *config.Config) {
	var fileServer http.Handler
	var hasFile func(path string) bool
	var serveIndex func(c *gin.Context)

	staticDir := ""
	if cfg != nil {
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
func serveUploads(c *gin.Context, uploadDir string) {
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
