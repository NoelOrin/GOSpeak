package server

import (
	"fmt"
	"go_rtc/internal/config"
	"go_rtc/internal/handler"
	"go_rtc/internal/middleware"
	"go_rtc/internal/model"
	"go_rtc/internal/sfu"
	"go_rtc/internal/redis"
	"go_rtc/internal/repository"
	"go_rtc/internal/router"
	"go_rtc/internal/service"
	"go_rtc/internal/signal"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	engineio "github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
	"github.com/googollee/go-socket.io/engineio/transport/polling"
	"github.com/googollee/go-socket.io/engineio/transport/websocket"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type EnvEnum string

const (
	Dev  EnvEnum = "dev"
	Prod EnvEnum = "prod"
)

func StartGin(env EnvEnum) {
	loadingEnv(env)

	if env == Prod || env == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := repository.InitDB(); err != nil {
		panic(fmt.Sprintf("failed to initialize database: %v", err))
	}

	redis.InitRedis()

	cfg := config.Load()
	sfuProvider, err := sfu.NewProvider(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize SFU provider: %v", err))
	}

	roleRepo := repository.NewRoleRepository(repository.DB)
	seedRoles(roleRepo)

	userRepo := repository.NewUserRepository(repository.DB)
	seedAdminUser(userRepo)
	roomRepo := repository.NewRoomRepository(repository.DB)
	oauthProviderRepo := repository.NewOAuthProviderRepository(repository.DB)
	oauthAccountRepo := repository.NewOAuthAccountRepository(repository.DB)
	permRepo := repository.NewPermissionRepository(repository.DB)

	// 初始化权限系统
	seedPermissions(permRepo)
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		panic(fmt.Sprintf("failed to load permission cache: %v", err))
	}
	middleware.SetPermissionChecker(permSvc)

	authSvc := service.NewAuthService(userRepo)
	userSvc := service.NewUserService(userRepo)
	oauthSvc := service.NewOAuthService(oauthProviderRepo, oauthAccountRepo, userRepo)
	roomSvc := service.NewRoomService(roomRepo)

	authH := handler.NewAuthHandler(authSvc)
	userH := handler.NewUserHandler(userSvc)
	oauthH := handler.NewOAuthHandler(oauthSvc)
	roleH := handler.NewRoleHandler(roleRepo)
	roomH := handler.NewRoomHandler(roomSvc, permSvc)
	permH := handler.NewPermissionHandler(permSvc)

	r := gin.Default()

	wsTransport := websocket.Default
	wsTransport.CheckOrigin = func(r *http.Request) bool { return true }

	sioServer := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			polling.Default,
			wsTransport,
		},
	})
	signalHub := signal.NewHub(roomSvc)
	signalHub.SetupRoutes(sioServer)
	signalH := handler.NewSignalHandler(sfuProvider, signalHub)

	// 启动 Socket.IO 事件循环（消费 sessions 并处理消息）
	go func() {
		if err := sioServer.Serve(); err != nil {
			fmt.Printf("[Socket.IO] serve error: %v\n", err)
		}
	}()

	r.GET("/socket.io/*any", gin.WrapH(sioServer))
	r.POST("/socket.io/*any", gin.WrapH(sioServer))
	r.OPTIONS("/socket.io/*any", gin.WrapH(sioServer))

	router.SetupRoutes(r, &router.Handlers{
		Auth:       authH,
		User:       userH,
		Signal:     signalH,
		OAuth:      oauthH,
		Role:       roleH,
		Room:       roomH,
		Permission: permH,
	})

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8998"
	}

	fmt.Printf("[Swagger] API 文档地址: http://localhost:%s/swagger/index.html\n", port)

	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

func loadingEnv(env EnvEnum) {
	switch env {
	case Dev:
		if err := loadEnvFile("./.env.dev"); err != nil {
			panic(err)
		}
	case Prod:
		if err := loadEnvFile("./.env.prod"); err != nil {
			panic(err)
		}
	}
}

func loadEnvFile(path string) error {
	return godotenv.Load(path)
}

func seedPermissions(permRepo *repository.PermissionRepository) {
	// 种子权限定义
	for i := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&model.DefaultPermissions[i]); err != nil {
			fmt.Printf("[Seed] 创建权限 %s 失败: %v\n", model.DefaultPermissions[i].Code, err)
		}
	}

	// 种子角色-权限映射
	for roleName, codes := range model.DefaultRolePermissions {
		if err := permRepo.SyncRolePermissions(roleName, codes); err != nil {
			fmt.Printf("[Seed] 同步角色 %s 权限失败: %v\n", roleName, err)
		}
	}
	fmt.Println("[Seed] 权限系统初始化完成")
}

func seedRoles(roleRepo *repository.RoleRepository) {
	for i := range model.DefaultRoles {
		if err := roleRepo.CreateIfNotExists(&model.DefaultRoles[i]); err != nil {
			fmt.Printf("[Seed] 创建角色 %s 失败: %v\n", model.DefaultRoles[i].Name, err)
		}
	}
	roles, err := roleRepo.List()
	if err != nil {
		fmt.Printf("[Seed] 加载角色列表失败: %v\n", err)
		return
	}
	model.LoadRoleCache(roles)
	fmt.Printf("[Seed] 已加载 %d 个角色\n", len(roles))
}

func seedAdminUser(userRepo *repository.UserRepository) {
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte("123123"), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("[Seed] 生成密码哈希失败: %v\n", err)
		return
	}

	existing, _ := userRepo.GetByName("admin")
	if existing != nil {
		return
	}

	admin := &model.User{
		Name:        "admin",
		DisplayName: "管理员",
		Password:    string(hashedPwd),
		Role:        "admin",
	}
	if err := userRepo.Create(admin); err != nil {
		fmt.Printf("[Seed] 创建管理员用户失败: %v\n", err)
		return
	}
	fmt.Println("[Seed] 已创建管理员用户: admin / 123123")
}

func init() {
	logFile, err := os.OpenFile("./server/logs/xxx.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("open log file failed, err:", err)
		return
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)

	gin.DisableConsoleColor()
	// f, _ := os.Create("gin.log")
	// gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
}
