package server

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/jobs"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/plugin"
	"GOSpeak/internal/plugin/builtin"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/router"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/factory"
	"GOSpeak/internal/sfu/providers/mediasoup"
	"GOSpeak/internal/signal"
	"context"
	"fmt"
	"net/http"
	"os"
	ossignal "os/signal"
	"strconv"
	"syscall"
	"time"

	"GOSpeak/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type EnvEnum string

const (
	Dev  EnvEnum = "dev"
	Prod EnvEnum = "prod"
)

func StartGin(env EnvEnum) {
	appEnv := string(env)
	if appEnv == "" {
		appEnv = string(Prod)
	}
	// 进程环境优先；文件仅填充未设置变量
	config.LoadEnvFiles(appEnv)
	if os.Getenv("APP_ENV") == "" {
		_ = os.Setenv("APP_ENV", appEnv)
	}

	cfg := config.MustLoad()

	// 统一日志初始化（先于其它组件，后续 log/fmt 可逐步迁移）
	if err := logger.Init(logger.OptionsFrom(cfg.LoggerOptions())); err != nil {
		panic(fmt.Sprintf("failed to init logger: %v", err))
	}
	logger.SetupGin()
	logger.WithComponent("Server").WithFields(logger.Fields{
		"env":      appEnv,
		"port":     cfg.ServerPort,
		"provider": cfg.SFUProvider,
	}).Info("logger ready")

	if env == Prod || env == "" || cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	if cfg.GinMode != "" {
		gin.SetMode(cfg.GinMode)
	}

	if err := repository.InitDB(cfg); err != nil {
		panic(fmt.Sprintf("failed to initialize database: %v", err))
	}

	redis.InitRedis(cfg)

	if env == Prod || cfg.IsProduction() {
		redis.SetProductionMode()
	}

	roleRepo := repository.NewRoleRepository(repository.DB)
	seedRoles(roleRepo)

	userRepo := repository.NewUserRepository(repository.DB)
	adminUUID := seedAdminUser(userRepo)
	userGroupRepo := repository.NewUserGroupRepository(repository.DB)
	if adminUUID != "" {
		if err := repository.EnsureDefaultDomain(repository.DB, adminUUID); err != nil {
			logger.WithComponent("Seed").Warnf("同步默认语音域失败: %v", err)
		}
	}
	roomRepo := repository.NewRoomRepository(repository.DB)
	oauthProviderRepo := repository.NewOAuthProviderRepository(repository.DB)
	oauthAccountRepo := repository.NewOAuthAccountRepository(repository.DB)
	emailConfigRepo := repository.NewEmailConfigRepository(repository.DB)
	emailVerificationRepo := repository.NewEmailVerificationCodeRepository(repository.DB)
	permRepo := repository.NewPermissionRepository(repository.DB)
	muteRepo := repository.NewMuteRepository(repository.DB)
	messageRepo := repository.NewMessageRepository(repository.DB)
	conversationRepo := repository.NewConversationRepository(repository.DB)
	sfuConfigRepo := repository.NewSFUConfigRepository(repository.DB)
	storageConfigRepo := repository.NewStorageConfigRepository(repository.DB)
	domainRepo := repository.NewDomainRepository(repository.DB)
	domainSvc := service.NewDomainService(domainRepo)

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
	userGroupSvc := service.NewUserGroupService(userGroupRepo)
	oauthSvc := service.NewOAuthService(oauthProviderRepo, oauthAccountRepo, userRepo)
	roomSvc := service.NewRoomService(roomRepo)
	messageSvc := service.NewMessageService(messageRepo, roomRepo, domainSvc)
	messageSvc.SetUserRepo(userRepo)
	muteSvc := service.NewMuteService(muteRepo, userRepo)
	sfuConfigSvc := service.NewSFUConfigService(sfuConfigRepo, cfg)
	if err := sfuConfigSvc.SyncFromEnv(); err != nil {
		panic(fmt.Sprintf("failed to sync sfu config from env: %v", err))
	}
	botTokenRepo := repository.NewBotTokenRepository(repository.DB)
	botSvc := service.NewBotService(userRepo, botTokenRepo)

	// 后端插件系统：多组件注册 + 可选 side server
	pluginConfigRepo := repository.NewPluginConfigRepository(repository.DB)
	pluginHost := plugin.NewHost(repository.DB, cfg, pluginConfigRepo)
	pluginReg := plugin.NewRegistry(pluginHost)
	// 注册二进制内嵌的基础插件（bot-base 等）
	if err := builtin.RegisterAll(pluginReg); err != nil {
		panic(fmt.Sprintf("register builtin plugins: %v", err))
	}
	logger.WithComponent("Plugin").WithField("embedded", builtin.EmbeddedSummary()).Info("builtin plugins registered")
	if err := pluginReg.InitAll(); err != nil {
		panic(fmt.Sprintf("init plugins: %v", err))
	}
	// 后端启动时同步启动已启用插件（bot-base 内嵌且默认启用）
	if err := pluginReg.StartEnabled(context.Background()); err != nil {
		logger.WithComponent("Plugin").Warnf("start plugins: %v", err)
	}
	pluginSvc := service.NewPluginService(pluginReg)
	for _, info := range pluginSvc.List() {
		logger.WithComponent("Plugin").Infof(
			"plugin=%s enabled=%v status=%s side_servers=%d",
			info.Name, info.Enabled, info.Status, len(info.SideServers),
		)
	}
	var sfuProvider sfu.Provider = factory.NewDynamicProvider(sfuConfigSvc.ResolveConfig)
	resolvedSFUCfg, err := sfuConfigSvc.ResolveConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to resolve sfu config: %v", err))
	}

	r := gin.New()
	r.Use(logger.GinRecovery(), logger.GinLogger())
	middleware.SetTokenVersionChecker(authSvc)

	wsFanout := ws.NewFanout()
	wsHandler := ws.NewHandlerRegistry()

	timeout, err := time.ParseDuration(cfg.NATSConnectTimeout)
	if err != nil || timeout <= 0 {
		timeout = 2 * time.Second
	}
	deliverer := bus.NewWSDeliverer(wsFanout)
	embeddedPort := 0
	if cfg.NATSEmbeddedPort != "" {
		p, err := strconv.Atoi(cfg.NATSEmbeddedPort)
		if err != nil {
			panic(fmt.Sprintf("invalid NATS_EMBEDDED_PORT %q: %v", cfg.NATSEmbeddedPort, err))
		}
		embeddedPort = p
	}
	eventBus, closeEventBus, err := bus.Init(bus.InitConfig{
		URL:            cfg.NATSURL,
		Prefix:         cfg.NATSSubjectPrefix,
		Name:           cfg.NATSName,
		ConnectTimeout: timeout,
		EmbeddedPort:   embeddedPort,
		Deliverer:      deliverer,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to init event bus: %v", err))
	}

	signalHub := signal.NewHub(roomSvc, muteSvc, userSvc, permSvc)
	signalHub.SetEventBus(eventBus)
	signalHub.SetStateNotifier(eventBus)
	permSvc.SetEventBus(eventBus)
	messageSvc.SetEventBus(eventBus)
	signalHub.SetMessageService(messageSvc)
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
		logger.WithComponent("StateStore").Warnf("unavailable mode=%s: %v", cfg.StateStore, err)
	} else if store != nil {
		signalHub.SetMembershipStore(store, instanceID)
		logger.WithComponent("StateStore").Infof("ready backend=%s instance=%s", backend, instanceID)
	} else {
		logger.WithComponent("StateStore").Info("backend=none (local membership only)")
	}

	// mute rule KV for degraded media mute (Agora kicking-rule ids): redis → nats → memory
	muteRuleStore, muteRuleBackend := bus.ResolveMuteRuleStore(bus.ResolveMuteRuleConfig{
		Mode:   cfg.StateStore,
		Prefix: cfg.NATSSubjectPrefix,
		Redis:  redisClient,
		NATS:   natsConn,
	})
	if dp, ok := sfuProvider.(*factory.DynamicProvider); ok {
		dp.SetMuteRuleStore(muteRuleStore)
	}
	logger.WithComponent("MuteRuleStore").Infof("ready backend=%s", muteRuleBackend)

	// JWT blacklist + signing key: Redis preferred; otherwise NATS KV so multi-instance logout still works.
	if !redis.IsConnected() && natsConn != nil {
		if authStore, err := bus.OpenAuthStore(bus.AuthStoreConfig{
			Prefix: cfg.NATSSubjectPrefix,
			NC:     natsConn,
		}); err != nil {
			logger.WithComponent("AuthStore").Warnf("nats unavailable: %v", err)
		} else {
			redis.SetAuthBackend(authStore)
			logger.WithComponent("AuthStore").Infof("ready backend=%s", authStore.Backend())
		}
	} else if redis.IsConnected() {
		logger.WithComponent("AuthStore").Info("ready backend=redis")
	} else {
		logger.WithComponent("AuthStore").Info("backend=none (static JWT_KEY / no multi-instance blacklist)")
	}

	var jobQueue *bus.JobQueue
	if nb, ok := eventBus.(*bus.NATSBus); ok && nb.Conn() != nil {
		q, err := bus.OpenJobQueue(bus.JobQueueConfig{
			Prefix: cfg.NATSSubjectPrefix,
			NC:     nb.Conn(),
		})
		if err != nil {
			logger.WithComponent("JobQueue").Warnf("unavailable: %v", err)
		} else {
			jobQueue = q
			signalHub.SetCleanupPublisher(q)
			messageSvc.SetJobQueue(q)
			if _, err := q.Consume(nb.InstanceID(), func(job bus.JobEnvelope) error {
				return jobs.Handle(job, signalHub, signalHub, messageSvc)
			}); err != nil {
				logger.WithComponent("JobQueue").Errorf("consume failed: %v", err)
			} else {
				logger.WithComponent("JobQueue").Infof("consumer started instance=%s", nb.InstanceID())
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
	signalHub.SetupFanout(wsFanout, wsHandler)
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
	userGroupH := handler.NewUserGroupHandler(userGroupSvc, userSvc)
	oauthH := handler.NewOAuthHandler(oauthSvc)
	roleH := handler.NewRoleHandler(roleSvc)
	clusterSvc := service.NewClusterService(repository.NewClusterNodeRepository(repository.DB), repository.NewServerAssignmentRepository(repository.DB))
	clusterSvc.SetNotifier(eventBus)
	clusterSvc.SetServerRepo(repository.NewDomainRepository(repository.DB))
	signalH.SetClusterResolver(func(domainUUID string) (string, error) {
		_, node, err := clusterSvc.ResolveServer(domainUUID)
		if err != nil {
			return "", err
		}
		return node.AdvertiseURL, nil
	})
	middleware.SetDomainChecker(domainSvc.IsMember)
	signalHub.SetDomainChecker(domainSvc.IsMember)
	roomH := handler.NewRoomHandler(roomSvc, permSvc, domainSvc)
	msgH := handler.NewMessageHandler(messageSvc, permSvc)
	permH := handler.NewPermissionHandler(permSvc)
	muteH := handler.NewMuteHandler(muteSvc, userSvc, signalHub)
	conversationSvc := service.NewConversationService(conversationRepo, messageRepo)
	conversationSvc.SetEventBus(eventBus)
	signalHub.SetConversationService(conversationSvc)
	sfuConfigH := handler.NewSFUConfigHandler(sfuConfigSvc, signalHub)
	storageH := handler.NewStorageHandler(storageSvc)
	botH := handler.NewBotHandler(botSvc)
	pluginH := handler.NewPluginHandler(pluginSvc)

	var clusterHandler *handler.ClusterHandler
	if cfg.IsAgent() {
		clusterHandler = handler.NewClusterHandler(clusterSvc)
	}
	var clusterStop func()
	localNodeUUID := ""
	switch cfg.ClusterRole {
	case "all":
		localNodeUUID, clusterStop, err = startLocalClusterRuntime(cfg, clusterSvc, signalHub, eventBus.InstanceID())
	case "worker":
		clusterStop, err = startWorkerClusterRuntime(cfg, signalHub)
	case "agent":
		clusterStop, err = startAgentClusterRuntime(cfg, clusterSvc)
	}
	if err != nil {
		panic(fmt.Sprintf("failed to start cluster runtime: %v", err))
	}

	domainH := handler.NewDomainHandler(domainSvc, permSvc)
	domainH.SetOnDomainCreated(func(serverUUID string) {
		if err := clusterSvc.EnsureServer(serverUUID, 1, localNodeUUID); err != nil {
			logger.WithComponent("Cluster").Warnf("schedule server %s failed: %v", serverUUID, err)
		}
	})
	domainH.SetOnDomainDelete(func(serverUUID string) {
		if err := clusterSvc.DeleteServer(serverUUID); err != nil {
			logger.WithComponent("Cluster").Warnf("delete server %s assignments failed: %v", serverUUID, err)
		}
		signalHub.OnDomainDelete(serverUUID)
	})
	if cfg.IsAgent() {
		domains, _, listErr := domainSvc.List(1, 1000)
		if listErr != nil {
			logger.WithComponent("Cluster").Warnf("list domains for scheduling failed: %v", listErr)
		} else {
			for _, domain := range domains {
				if err := clusterSvc.EnsureServer(domain.UUID, 1, localNodeUUID); err != nil {
					logger.WithComponent("Cluster").Warnf("ensure server %s failed: %v", domain.UUID, err)
				}
			}
		}
	}
	conversationH := handler.NewConversationHandler(conversationSvc)

	monitorH := handler.NewMonitorHandler(signalHub, cfg, eventBus)

	// 启动签名密钥轮换检查
	go redis.KeyRotationLoop()

	// WS 升级路由（JWT 鉴权由 Upgrader 内部完成）
	var wsUpgrader *ws.Upgrader
	if cfg.IsWorker() {
		wsUpgrader = ws.NewUpgrader(ws.UpgraderConfig{
			Fanout:         wsFanout,
			Handler:        wsHandler,
			AllowedOrigins: wsAllowedOrigins(cfg),
			OnConnect: func(c *ws.Client) {
				_ = signalHub.OnConnect(c)
			},
			OnDisconnect: func(c *ws.Client) {
				signalHub.OnDisconnect(c)
			},
		})
	}

	router.SetupRoutes(r, &router.Handlers{
		Auth:         authH,
		User:         userH,
		UserGroup:    userGroupH,
		Signal:       signalH,
		Cloudflare:   cfH,
		OAuth:        oauthH,
		Role:         roleH,
		Room:         roomH,
		Permission:   permH,
		Mute:         muteH,
		Message:      msgH,
		SFUConfig:    sfuConfigH,
		Storage:      storageH,
		Email:        emailH,
		EmailConfig:  emailConfigH,
		Monitor:      monitorH,
		SRSCallback:  srsCallbackH,
		Bot:          botH,
		Plugin:       pluginH,
		Domain:       domainH,
		Conversation: conversationH,
		Cluster:      clusterHandler,
		PluginHost:   pluginHost,
	})

	port := cfg.ServerPort
	if port == "" {
		port = "8998"
	}

	logger.WithComponent("Swagger").Infof("API 文档地址: http://localhost:%s/swagger/index.html", port)

	// /ws 必须绕过 Gin：nhooyr/websocket 的 hijack 与 gin.ResponseWriter 不兼容。
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if wsUpgrader != nil && req.URL.Path == "/ws" {
			wsUpgrader.ServeHTTP(w, req)
			return
		}
		r.ServeHTTP(w, req)
	})
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: rootHandler,
	}

	// 优雅关闭：监听系统信号，先关 WebSocket 连接再关 HTTP
	go func() {
		quit := make(chan os.Signal, 1)
		ossignal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.WithComponent("Server").Info("shutting down...")

		// 1) stop accepting HTTP first so in-flight handlers can still emit signal
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.WithComponent("HTTP").Errorf("shutdown error: %v", err)
		}

		pluginReg.StopAll(context.Background())
		logger.WithComponent("Plugin").Info("plugins stopped")

		if clusterStop != nil {
			clusterStop()
			logger.WithComponent("Cluster").Info("cluster runtime stopped")
		}

		// 2) drain signal fanout then close socket connections
		logger.WithComponent("Message").Info("write queue closed")
		closeEventBus()
		logger.WithComponent("EventBus").Info("closed")
		// WS connections close when the HTTP server shuts down
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.WithComponent("HTTP").Fatalf("listen error: %v", err)
	}
}

func seedPermissions(permRepo *repository.PermissionRepository) {
	// 种子权限定义
	for i := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&model.DefaultPermissions[i]); err != nil {
			logger.WithComponent("Seed").Warnf("创建权限 %s 失败: %v", model.DefaultPermissions[i].Code, err)
		}
	}

	// 种子角色-权限映射
	for roleName, codes := range model.DefaultRolePermissions {
		if err := permRepo.EnsureRolePermissions(roleName, codes); err != nil {
			logger.WithComponent("Seed").Warnf("同步角色 %s 权限失败: %v", roleName, err)
		}
	}
	logger.WithComponent("Seed").Info("权限系统初始化完成")
}

func seedRoles(roleRepo *repository.RoleRepository) {
	for i := range model.DefaultRoles {
		if err := roleRepo.CreateIfNotExists(&model.DefaultRoles[i]); err != nil {
			logger.WithComponent("Seed").Warnf("创建角色 %s 失败: %v", model.DefaultRoles[i].Name, err)
		}
	}
	roles, err := roleRepo.List()
	if err != nil {
		logger.WithComponent("Seed").Errorf("加载角色列表失败: %v", err)
		return
	}
	model.LoadRoleCache(roles)
	logger.WithComponent("Seed").Infof("已加载 %d 个角色", len(roles))
}

func wsAllowedOrigins(cfg *config.Config) []string {
	if origins := cfg.WSAllowedOriginsList(); len(origins) > 0 {
		return origins
	}
	return []string{cfg.CORSOrigin}
}

func seedAdminUser(userRepo *repository.UserRepository) string {
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(service.DefaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.WithComponent("Seed").Errorf("生成密码哈希失败: %v", err)
		return ""
	}

	existing, _ := userRepo.GetByName("admin")
	if existing != nil {
		return existing.UUID
	}

	admin := &model.User{
		Name:        "admin",
		DisplayName: "管理员",
		Password:    string(hashedPwd),
		Role:        "admin",
	}
	if err := userRepo.Create(admin); err != nil {
		logger.WithComponent("Seed").Errorf("创建管理员用户失败: %v", err)
		return ""
	}
	logger.WithComponent("Seed").Infof("已创建管理员用户: admin / %s", service.DefaultAdminPassword)
	return admin.UUID
}

func init() {
	// 日志完整初始化在 StartGin 的 config 加载之后；此处仅做最小兜底
	gin.DisableConsoleColor()
}
