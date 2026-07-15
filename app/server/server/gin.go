package server

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/jobs"
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/mediasoup"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/router"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/factory"
	"GOSpeak/internal/signal"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
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
	if env == Prod {
		redis.SetProductionMode()
	}

	roleRepo := repository.NewRoleRepository(repository.DB)
	seedRoles(roleRepo)

	userRepo := repository.NewUserRepository(repository.DB)
	seedAdminUser(userRepo)
	roomRepo := repository.NewRoomRepository(repository.DB)
	oauthProviderRepo := repository.NewOAuthProviderRepository(repository.DB)
	oauthAccountRepo := repository.NewOAuthAccountRepository(repository.DB)
	emailConfigRepo := repository.NewEmailConfigRepository(repository.DB)
	emailVerificationRepo := repository.NewEmailVerificationCodeRepository(repository.DB)
	permRepo := repository.NewPermissionRepository(repository.DB)
	muteRepo := repository.NewMuteRepository(repository.DB)
	sfuConfigRepo := repository.NewSFUConfigRepository(repository.DB)
	storageConfigRepo := repository.NewStorageConfigRepository(repository.DB)

	// 初始化权限系统
	seedPermissions(permRepo)
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		panic(fmt.Sprintf("failed to load permission cache: %v", err))
	}
	middleware.SetPermissionChecker(permSvc)

	roleSvc := service.NewRoleService(roleRepo)
	emailConfigSvc := service.NewEmailConfigService(emailConfigRepo, cfg)
	emailSvc := service.NewEmailService(emailConfigSvc.ResolveConfig)
	emailVerificationSvc := service.NewEmailVerificationService(emailVerificationRepo, userRepo, emailSvc, emailConfigSvc.ResolveConfig)
	authSvc := service.NewAuthService(userRepo, emailConfigSvc, emailVerificationSvc)
	storageSvc := service.NewStorageService(storageConfigRepo, cfg)
	userSvc := service.NewUserService(userRepo, storageSvc)
	oauthSvc := service.NewOAuthService(oauthProviderRepo, oauthAccountRepo, userRepo)
	roomSvc := service.NewRoomService(roomRepo)
	muteSvc := service.NewMuteService(muteRepo, userRepo)
	sfuConfigSvc := service.NewSFUConfigService(sfuConfigRepo, cfg)
	if err := sfuConfigSvc.SyncFromEnv(); err != nil {
		panic(fmt.Sprintf("failed to sync sfu config from env: %v", err))
	}
	botTokenRepo := repository.NewBotTokenRepository(repository.DB)
	botSvc := service.NewBotService(userRepo, botTokenRepo)
	var sfuProvider sfu.Provider = factory.NewDynamicProvider(sfuConfigSvc.ResolveConfig)
	resolvedSFUCfg, err := sfuConfigSvc.ResolveConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to resolve sfu config: %v", err))
	}

	r := gin.Default()
	middleware.SetTokenVersionChecker(authSvc)

	wsTransport := websocket.Default
	wsTransport.CheckOrigin = func(r *http.Request) bool { return true }

	sioServer := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			polling.Default,
			wsTransport,
		},
	})

	timeout, err := time.ParseDuration(cfg.NATSConnectTimeout)
	if err != nil || timeout <= 0 {
		timeout = 2 * time.Second
	}
	deliverer := bus.NewSIODeliverer(sioServer)
	eventBus, closeEventBus, err := bus.Init(bus.InitConfig{
		URL:            cfg.NATSURL,
		Prefix:         cfg.NATSSubjectPrefix,
		Name:           cfg.NATSName,
		ConnectTimeout: timeout,
		Deliverer:      deliverer,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to init event bus: %v", err))
	}

	signalHub := signal.NewHub(roomSvc, muteSvc, userSvc, permSvc)
	signalHub.SetEventBus(eventBus)
	signalHub.SetStateNotifier(eventBus)
	permSvc.SetEventBus(eventBus)
	var natsConn *nats.Conn
	instanceID := eventBus.InstanceID()
	if nb, ok := eventBus.(*bus.NATSBus); ok {
		natsConn = nb.Conn()
		nb.SetRemoteHook(func(event string, payload interface{}) {
			if event == service.EventPermissionsInvalidated {
				permSvc.OnRemoteInvalidate(payload)
				return
			}
			signalHub.HandleRemoteEvent(event, payload)
		})
	}

	// membership KV: redis → nats → none (STATE_STORE=auto default)
	var redisClient *goredis.Client
	if redis.IsConnected() {
		redisClient = redis.Client
	}
	store, backend, err := bus.ResolveMembershipStore(bus.ResolveMembershipConfig{
		Mode:   cfg.StateStore,
		Prefix: cfg.NATSSubjectPrefix,
		Redis:  redisClient,
		NATS:   natsConn,
	})
	if err != nil {
		log.Printf("[StateStore] unavailable mode=%s: %v", cfg.StateStore, err)
	} else if store != nil {
		signalHub.SetMembershipStore(store, instanceID)
		log.Printf("[StateStore] ready backend=%s instance=%s", backend, instanceID)
	} else {
		log.Printf("[StateStore] backend=none (local membership only)")
	}

	var jobQueue *bus.JobQueue
	if nb, ok := eventBus.(*bus.NATSBus); ok && nb.Conn() != nil {
		q, err := bus.OpenJobQueue(bus.JobQueueConfig{
			Prefix: cfg.NATSSubjectPrefix,
			NC:     nb.Conn(),
		})
		if err != nil {
			log.Printf("[JobQueue] unavailable: %v", err)
		} else {
			jobQueue = q
			signalHub.SetCleanupPublisher(q)
			if _, err := q.Consume(nb.InstanceID(), func(job bus.JobEnvelope) error {
				return jobs.Handle(job, signalHub, signalHub)
			}); err != nil {
				log.Printf("[JobQueue] consume failed: %v", err)
			} else {
				log.Printf("[JobQueue] consumer started instance=%s", nb.InstanceID())
			}
		}
	}
	signalHub.SetSFU(sfuProvider)
	if snr, ok := sfuProvider.(signal.StreamNameResolver); ok {
		signalHub.SetStreamResolver(snr)
	}
	// 注入 Hub room 聚合视图给 SRS 等无原生 room 维度的 provider（pkg.RoomRegistrySetter）。
	if rs, ok := sfuProvider.(pkg.RoomRegistrySetter); ok {
		rs.SetRoomRegistry(signalHub)
	}
	if resolvedSFUCfg.SFUProvider == "mediasoup" {
		msService := mediasoup.NewService(resolvedSFUCfg)
		msSignal := mediasoup.NewMediasoupSignal(msService.Bridge, signalHub.BroadcastToRoom)
		signalHub.SetSFUSignalHandler(msSignal)
	}
	signalHub.SetupRoutes(sioServer)
	sfuSvc := service.NewSFUService(sfuProvider, signalHub)
	signalH := handler.NewSignalHandler(sfuSvc)
	if jobQueue != nil {
		signalH.SetJobs(jobQueue)
	}
	cfMediaSvc := service.NewCloudflareMediaService(sfuConfigSvc.ResolveConfig)
	cfH := handler.NewCloudflareHandler(cfMediaSvc)
	srsCallbackH := handler.NewSRSCallbackHandlerWithResolver(signalHub, func() string {
		resolved, err := sfuConfigSvc.ResolveConfig()
		if err != nil || resolved == nil {
			return cfg.SRSSecret
		}
		if resolved.SRSSecret != "" {
			return resolved.SRSSecret
		}
		return cfg.SRSSecret
	})
	if jobQueue != nil {
		srsCallbackH.SetJobs(jobQueue)
	}

	authH := handler.NewAuthHandler(authSvc)
	emailH := handler.NewEmailVerificationHandler(emailVerificationSvc)
	emailConfigH := handler.NewEmailConfigHandler(emailConfigSvc)
	userH := handler.NewUserHandler(userSvc, storageSvc)
	oauthH := handler.NewOAuthHandler(oauthSvc)
	roleH := handler.NewRoleHandler(roleSvc)
	roomH := handler.NewRoomHandler(roomSvc, permSvc)
	permH := handler.NewPermissionHandler(permSvc)
	muteH := handler.NewMuteHandler(muteSvc, userSvc, signalHub)
	sfuConfigH := handler.NewSFUConfigHandler(sfuConfigSvc, signalHub)
	storageH := handler.NewStorageHandler(storageSvc)
	botH := handler.NewBotHandler(botSvc)

	monitorH := handler.NewMonitorHandler(signalHub, cfg, eventBus)

	// 启动签名密钥轮换检查
	go redis.KeyRotationLoop()

	// 启动 Socket.IO 事件循环
	go func() {
		if err := sioServer.Serve(); err != nil {
			fmt.Printf("[Socket.IO] serve error: %v\n", err)
		}
	}()

	wsAuth := middleware.WSAuth()
	cors := middleware.CORS()
	r.GET("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
	r.POST("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))
	r.OPTIONS("/socket.io/*any", cors, wsAuth, gin.WrapH(sioServer))

	router.SetupRoutes(r, &router.Handlers{
		Auth:        authH,
		User:        userH,
		Signal:      signalH,
		Cloudflare:  cfH,
		OAuth:       oauthH,
		Role:        roleH,
		Room:        roomH,
		Permission:  permH,
		Mute:        muteH,
		SFUConfig:   sfuConfigH,
		Storage:     storageH,
		Email:       emailH,
		EmailConfig: emailConfigH,
		Monitor:     monitorH,
		SRSCallback: srsCallbackH,
		Bot:         botH,
	})

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8998"
	}

	fmt.Printf("[Swagger] API 文档地址: http://localhost:%s/swagger/index.html\n", port)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// 优雅关闭：监听系统信号，先关 Socket.IO 连接再关 HTTP
	go func() {
		quit := make(chan os.Signal, 1)
		ossignal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("[Server] shutting down...")

		if err := sioServer.Close(); err != nil {
			log.Printf("[Socket.IO] close error: %v", err)
		}
		log.Println("[Socket.IO] connections closed")

		closeEventBus()
		log.Println("[EventBus] closed")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[HTTP] shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[HTTP] listen error: %v", err)
	}
}

func loadingEnv(env EnvEnum) {
	switch env {
	case Dev:
		if err := loadEnvFile("./.env.dev"); err != nil {
			panic(err)
		}
	case Prod:
		// Docker / K8s 通过 env_file 或环境变量注入, .env.prod 可不存在
		if err := loadEnvFile("./.env.prod"); err != nil {
			log.Printf("[Config] .env.prod not loaded (%v); using process environment", err)
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
		if err := permRepo.EnsureRolePermissions(roleName, codes); err != nil {
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
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(service.DefaultAdminPassword), bcrypt.DefaultCost)
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
	fmt.Printf("[Seed] 已创建管理员用户: admin / %s\n", service.DefaultAdminPassword)
}

func init() {
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)

	gin.DisableConsoleColor()
	// f, _ := os.Create("gin.log")
	// gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
}
