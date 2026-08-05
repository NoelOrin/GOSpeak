package server

import (
	"GOSpeak/internal/bus"
	"GOSpeak/internal/cluster"
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
	"GOSpeak/internal/signal"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	ossignal "os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"GOSpeak/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
)

type EnvEnum string

const (
	Dev  EnvEnum = "dev"
	Prod EnvEnum = "prod"
)

func StartGin(env EnvEnum) error {
	appEnv := string(env)
	if appEnv == "" {
		appEnv = string(Dev)
	}
	// 进程环境优先；文件仅填充未设置变量
	config.LoadEnvFiles(appEnv)
	if os.Getenv("APP_ENV") == "" {
		_ = os.Setenv("APP_ENV", appEnv)
	}

	cfg := config.MustLoad()

	// 统一日志初始化（先于其它组件，后续 log/fmt 可逐步迁移）
	if err := logger.Init(logger.OptionsFrom(cfg.LoggerOptions())); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	logger.SetupGin()
	logger.WithComponent("Server").WithFields(logger.Fields{
		"env":      appEnv,
		"port":     cfg.ServerPort,
		"provider": cfg.SFUProvider,
	}).Info("logger ready")

	if env == Prod || cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	if cfg.GinMode != "" {
		gin.SetMode(cfg.GinMode)
	}

	if err := repository.InitDB(cfg); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	redis.InitRedis(cfg)

	if env == Prod || cfg.IsProduction() {
		redis.SetProductionMode()
	}

	roleRepo := repository.NewRoleRepository(repository.DB)
	if cfg.IsAgent() {
		seedRoles(roleRepo)
	} else {
		loadRoles(roleRepo)
	}

	userRepo := repository.NewUserRepository(repository.DB)
	var adminUUID string
	if cfg.IsAgent() {
		adminUUID = seedAdminUser(userRepo)
	}
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
	if cfg.IsAgent() {
		seedPermissions(permRepo)
	}
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		return fmt.Errorf("failed to load permission cache: %w", err)
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
	if cfg.IsAgent() {
		if err := sfuConfigSvc.SyncFromEnv(); err != nil {
			return fmt.Errorf("failed to sync sfu config from env: %w", err)
		}
	}
	botTokenRepo := repository.NewBotTokenRepository(repository.DB)
	botSvc := service.NewBotService(userRepo, botTokenRepo)

	// 后端插件系统：多组件注册 + 可选 side server
	pluginConfigRepo := repository.NewPluginConfigRepository(repository.DB)
	pluginHost := plugin.NewHost(repository.DB, cfg, pluginConfigRepo)
	pluginReg := plugin.NewRegistry(pluginHost)
	// 注册二进制内嵌的基础插件（bot-base 等）
	if err := builtin.RegisterAll(pluginReg); err != nil {
		return fmt.Errorf("register builtin plugins: %w", err)
	}
	logger.WithComponent("Plugin").WithField("embedded", builtin.EmbeddedSummary()).Info("builtin plugins registered")
	if cfg.IsAgent() {
		if err := pluginReg.InitAll(); err != nil {
			return fmt.Errorf("init plugins: %w", err)
		}
		// 后端启动时同步启动已启用插件（bot-base 内嵌且默认启用）
		if err := pluginReg.StartEnabled(context.Background()); err != nil {
			logger.WithComponent("Plugin").Warnf("start plugins: %v", err)
		}
	}
	pluginSvc := service.NewPluginService(pluginReg)
	for _, info := range pluginSvc.List() {
		logger.WithComponent("Plugin").Infof(
			"plugin=%s enabled=%v status=%s side_servers=%d",
			info.Name, info.Enabled, info.Status, len(info.SideServers),
		)
	}
	var sfuProvider sfu.Provider = factory.NewDynamicProvider(sfuConfigSvc.ResolveConfig)

	r := gin.New()

	// 只信任本机代理注入的 X-Forwarded-*，避免外部客户端伪造。
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.WithComponent("HTTP").Warnf("set trusted proxies failed: %v", err)
	}
	r.Use(logger.GinRecovery(), logger.GinLogger(), middleware.CORS(strings.Split(cfg.CORSOrigin, ",")))
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
			return fmt.Errorf("invalid NATS_EMBEDDED_PORT %q: %w", cfg.NATSEmbeddedPort, err)
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
		return fmt.Errorf("failed to init event bus: %w", err)
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
			if event == cluster.EventControlCommand {
				var cmd cluster.ControlCommand
				raw, _ := json.Marshal(payload)
				_ = json.Unmarshal(raw, &cmd)
				if cmd.NodeID == "" || cmd.NodeID == instanceID {
					_ = signalHub.HandleClusterCommand(cmd)
				}
				return
			}
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
	if store != nil {
		signalHub.StartMembershipHeartbeat()
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
		if sc, ok2 := sfuProvider.(interface{ SupportsStream() bool }); !ok2 || sc.SupportsStream() {
			signalHub.SetStreamResolver(snr)
		}
	}
	// 注入 Hub room 聚合视图给 SRS 等无原生 room 维度的 provider（pkg.RoomRegistrySetter）。
	if rs, ok := sfuProvider.(pkg.RoomRegistrySetter); ok {
		rs.SetRoomRegistry(signalHub)
	}
	// MediaSoup 专用信号初始化已禁用保留：SetSFUSignalHandler 不再调用。
	signalHub.SetupFanout(wsFanout, wsHandler)
	sfuSvc := service.NewSFUService(sfuProvider, signalHub)
	sfuSvc.SetDomainMemberChecker(domainSvc.IsMember)
	signalH := handler.NewSignalHandler(sfuSvc)
	signalH.SetLiveKitSecretResolver(func() string {
		resolved, err := sfuConfigSvc.ResolveConfig()
		if err != nil || resolved == nil {
			return cfg.LiveKitSecret
		}
		if resolved.LiveKitSecret != "" {
			return resolved.LiveKitSecret
		}
		return cfg.LiveKitSecret
	})
	if jobQueue != nil {
		signalH.SetJobs(jobQueue)
	}
	cfMediaSvc := service.NewCloudflareMediaService(sfuConfigSvc.ResolveConfig)
	if dp, ok := sfuProvider.(*factory.DynamicProvider); ok {
		cfMediaSvc.SetSessionOwnerLookup(dp.SessionOwner)
	}
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
	roomH.SetRoomListBroadcaster(signalHub)
	roomH.SetControlPublisher(clusterSvc)
	msgH := handler.NewMessageHandler(messageSvc, permSvc)
	permH := handler.NewPermissionHandler(permSvc)
	muteH := handler.NewMuteHandler(muteSvc, userSvc, signalHub)
	muteH.SetControlPublisher(clusterSvc)

	// 临时禁言过期：删除记录后走完整 unmute（广播 + SFU 恢复），
	// 若期间管理员已重新禁言则跳过，避免误解除新禁言。
	muteSvc.SetOnExpired(func(userID uint) {
		if muted, _, err := muteSvc.IsMuted(userID); err == nil && !muted {
			signalHub.BroadcastUnmute(userID)
			_ = clusterSvc.PublishControl(cluster.ControlCommand{
				Command: cluster.CommandUnmute,
				Payload: map[string]interface{}{"user_id": userID},
			})
		}
	})
	conversationSvc := service.NewConversationService(conversationRepo, messageRepo)
	conversationSvc.SetEventBus(eventBus)
	signalHub.SetConversationService(conversationSvc)
	sfuConfigH := handler.NewSFUConfigHandler(sfuConfigSvc, signalHub)
	storageH := handler.NewStorageHandler(storageSvc)
	botH := handler.NewBotHandler(botSvc)
	middleware.SetBotTokenChecker(botSvc)
	pluginH := handler.NewPluginHandler(pluginSvc)

	var clusterHandler *handler.ClusterHandler
	var clusterStop func()
	var agentLeaderLock *cluster.NATSLeaderLock
	var stopLeaderRenew func()
	localNodeUUID := ""
	degradedToWorker := false
	if cfg.IsAgent() {
		acquireCtx, cancel := context.WithTimeout(context.Background(), agentLeaderAcquireTimeout)
		leaderLock, leader, lockErr := acquireAgentLeader(acquireCtx, natsConn, cfg.NATSSubjectPrefix, instanceID)
		cancel()
		switch {
		case lockErr != nil:
			logger.WithComponent("Cluster").Warnf("agent leader lock unavailable; degraded-to-worker instance=%s role=%s err=%v", instanceID, cfg.ClusterRole, lockErr)
			degradedToWorker = true
		case !leader:
			logger.WithComponent("Cluster").Warnf("agent leader lock held by another instance; degraded-to-worker instance=%s role=%s", instanceID, cfg.ClusterRole)
			degradedToWorker = true
		default:
			agentLeaderLock = leaderLock
			renewCtx, renewCancel := context.WithCancel(context.Background())
			renewInterval := cfg.ClusterHeartbeatIntervalDuration() / 2
			if renewInterval > 2*time.Second {
				renewInterval = 2 * time.Second
			}
			renewDone := leaderLock.RenewLoop(renewCtx, instanceID, renewInterval)
			stopLeaderRenew = func() {
				renewCancel()
				<-renewDone
			}
			logger.WithComponent("Cluster").Infof("agent leader lock acquired instance=%s role=%s", instanceID, cfg.ClusterRole)
		}
	}
	if degradedToWorker {
		cfg.ClusterRole = model.ClusterRoleWorker
	}
	if cfg.IsAgent() {
		clusterHandler = handler.NewClusterHandler(clusterSvc)
	}
	switch cfg.ClusterRole {
	case "all":
		localNodeUUID, clusterStop, err = startLocalClusterRuntime(cfg, clusterSvc, signalHub, eventBus.InstanceID())
	case "worker":
		if degradedToWorker && (cfg.ClusterAgentURL == "" || cfg.ClusterAgentToken == "") {
			localNodeUUID, clusterStop, err = startDegradedLocalWorkerRuntime(cfg, signalHub, eventBus.InstanceID())
		} else {
			clusterStop, err = startWorkerClusterRuntime(cfg, signalHub)
		}
	case "agent":
		clusterStop, err = startAgentClusterRuntime(cfg, clusterSvc)
	}
	if err != nil {
		return fmt.Errorf("failed to start cluster runtime: %w", err)
	}

	domainH := handler.NewDomainHandler(domainSvc, permSvc)
	domainH.SetControlPublisher(clusterSvc)
	domainH.SetOnDomainKick(signalHub.KickUserFromDomain)
	domainH.SetOnDomainLeave(signalHub.KickUserFromDomain)
	if !degradedToWorker {
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
	}
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

	monitorH := handler.NewMonitorHandler(signalHub, cfg, eventBus, clusterSvc)

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

		// 2) stop membership heartbeat, then close websocket connections
		signalHub.StopMembershipHeartbeat()
		if wsUpgrader != nil && wsUpgrader.Fanout() != nil {
			wsUpgrader.Fanout().CloseAll()
			logger.WithComponent("WS").Info("websocket connections closed")
		}

		pluginReg.StopAll(context.Background())
		logger.WithComponent("Plugin").Info("plugins stopped")

		if clusterStop != nil {
			clusterStop()
			logger.WithComponent("Cluster").Info("cluster runtime stopped")
		}
		if stopLeaderRenew != nil {
			stopLeaderRenew()
		}
		if agentLeaderLock != nil {
			if err := agentLeaderLock.Release(instanceID); err != nil {
				logger.WithComponent("Cluster").Warnf("release agent leader lock failed: %v", err)
			}
			logger.WithComponent("Cluster").Info("agent leader lock released")
		}

		// 3) close event bus last
		logger.WithComponent("EventBus").Info("closing event bus")
		closeEventBus()
		logger.WithComponent("EventBus").Info("closed")
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.WithComponent("HTTP").Fatalf("listen error: %v", err)
	}
	return nil
}
