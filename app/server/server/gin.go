package server

import (
	"GOSpeak/internal/authstate"
	"GOSpeak/internal/bus"
	"GOSpeak/internal/cluster"
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/jobs"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/metrics"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/plugin"
	"GOSpeak/internal/plugin/builtin"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/router"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/factory"
	"GOSpeak/internal/signal"
	"GOSpeak/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
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
	authstate.Configure(cfg)

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

	db, err := repository.InitDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	storage.InitCipher(cfg.StorageEncryptKey, cfg.IsProduction())

	if env == Prod || cfg.IsProduction() {
		authstate.SetProductionMode()
	}

	roleRepo := repository.NewRoleRepository(db)
	if !cfg.IsAgent() {
		loadRoles(roleRepo)
	}

	userRepo := repository.NewUserRepository(db)
	userGroupRepo := repository.NewUserGroupRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	oauthProviderRepo := repository.NewOAuthProviderRepository(db)
	oauthAccountRepo := repository.NewOAuthAccountRepository(db)
	emailConfigRepo := repository.NewEmailConfigRepository(db)
	emailVerificationRepo := repository.NewEmailVerificationCodeRepository(db)
	permRepo := repository.NewPermissionRepository(db)
	muteRepo := repository.NewMuteRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	conversationRepo := repository.NewConversationRepository(db)
	sfuConfigRepo := repository.NewSFUConfigRepository(db)
	storageConfigRepo := repository.NewStorageConfigRepository(db)
	domainRepo := repository.NewDomainRepository(db)
	domainRoleRepo := repository.NewDomainRoleRepository(db)
	domainSvc := service.NewDomainService(domainRepo, domainRoleRepo)

	permSvc := service.NewPermissionService(permRepo)

	roleSvc := service.NewRoleService(roleRepo)
	emailConfigSvc := service.NewEmailConfigService(emailConfigRepo, cfg)
	emailSvc := service.NewEmailService(emailConfigSvc.ResolveConfig)
	emailVerificationSvc := service.NewEmailVerificationService(emailVerificationRepo, userRepo, emailSvc, emailConfigSvc.ResolveConfig)
	authSvc := service.NewAuthService(userRepo, emailConfigSvc, emailVerificationSvc)
	storageSvc := service.NewStorageService(storageConfigRepo, cfg)
	userSvc := service.NewUserService(userRepo, storageSvc)
	userGroupSvc := service.NewUserGroupService(userGroupRepo)
	oauthSvc := service.NewOAuthService(oauthProviderRepo, oauthAccountRepo, userRepo)
	oauthSvc.SetAutoCreateUser(cfg.OAuthAutoCreateUser)
	roomSvc := service.NewRoomService(roomRepo)
	messageSvc := service.NewMessageService(messageRepo, roomRepo, domainSvc)
	messageSvc.SetUserRepo(userRepo)
	muteSvc := service.NewMuteService(muteRepo, userRepo)
	sfuConfigSvc := service.NewSFUConfigService(sfuConfigRepo, cfg)
	botTokenRepo := repository.NewBotTokenRepository(db)
	botSvc := service.NewBotService(userRepo, botTokenRepo)

	// 后端插件系统：多组件注册 + 可选 side server
	pluginConfigRepo := repository.NewPluginConfigRepository(db)
	pluginHost := plugin.NewHost(db, cfg, pluginConfigRepo)
	pluginReg := plugin.NewRegistry(pluginHost)
	// 注册二进制内嵌的基础插件（bot-base 等）
	if err := builtin.RegisterAll(pluginReg); err != nil {
		return fmt.Errorf("register builtin plugins: %w", err)
	}
	logger.WithComponent("Plugin").WithField("embedded", builtin.EmbeddedSummary()).Info("builtin plugins registered")
	var sfuProvider sfu.Provider = factory.NewDynamicProvider(sfuConfigSvc.ResolveConfig)

	r := gin.New()

	// 只信任本机代理注入的 X-Forwarded-*，避免外部客户端伪造。
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.WithComponent("HTTP").Warnf("set trusted proxies failed: %v", err)
	}
	r.Use(middleware.RequestID(), logger.GinRecovery(), logger.GinLogger(), middleware.CORS(strings.Split(cfg.CORSOrigin, ",")))

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

	permSvc.SetEventBus(eventBus)
	domainSvc.SetEventBus(eventBus)
	muteSvc.SetEventBus(eventBus)
	messageSvc.SetEventBus(eventBus)
	var natsConn *nats.Conn
	instanceID := eventBus.InstanceID()
	if nb, ok := eventBus.(*bus.NATSBus); ok {
		natsConn = nb.Conn()
	}

	// membership KV: nats → none (STATE_STORE=auto default)
	store, backend, err := bus.ResolveMembershipStore(bus.ResolveMembershipConfig{
		Mode:   cfg.StateStore,
		Prefix: cfg.NATSSubjectPrefix,
		NATS:   natsConn,
	})
	if err != nil {
		logger.WithComponent("StateStore").Warnf("unavailable mode=%s: %v", cfg.StateStore, err)
	} else if store != nil {
		logger.WithComponent("StateStore").Infof("ready backend=%s instance=%s", backend, instanceID)
	} else {
		logger.WithComponent("StateStore").Info("backend=none (local membership only)")
	}

	var streamResolver signal.StreamNameResolver
	if snr, ok := sfuProvider.(signal.StreamNameResolver); ok {
		if sc, ok2 := sfuProvider.(interface{ SupportsStream() bool }); !ok2 || sc.SupportsStream() {
			streamResolver = snr
		}
	}
	conversationSvc := service.NewConversationService(conversationRepo, messageRepo)
	conversationSvc.SetEventBus(eventBus)
	signalHub := signal.NewHubWithOptions(roomSvc, muteSvc, userSvc, permSvc, signal.HubOptions{
		Fanout:                  wsFanout,
		EventBus:                eventBus,
		SFUProvider:             sfuProvider,
		StreamResolver:          streamResolver,
		MessageSender:           messageSvc,
		ConversationSender:      conversationSvc,
		DomainChecker:           domainSvc.IsMember,
		DomainPermissionChecker: domainSvc.HasDomainPermission,
		MembershipStore:         store,
		InstanceID:              instanceID,
		StateNotifier:           eventBus,
	})
	if store != nil {
		signalHub.StartMembershipHeartbeat()
	}
	if nb, ok := eventBus.(*bus.NATSBus); ok {
		nb.SetRemoteHook(func(event string, payload interface{}) {
			if event == cluster.EventControlCommand {
				var cmd cluster.ControlCommand
				raw, _ := json.Marshal(payload)
				if err := json.Unmarshal(raw, &cmd); err != nil {
					logger.WithComponent("Cluster").Warnf("decode control command failed: %v", err)
					return
				}
				if cmd.NodeID == "" || cmd.NodeID == instanceID {
					if err := signalHub.HandleClusterCommand(cmd); err != nil {
						logger.WithComponent("Cluster").Warnf("handle cluster command failed: %v", err)
					}
				}
				return
			}
			if event == service.EventPermissionsInvalidated {
				permSvc.OnRemoteInvalidate(payload)
				return
			}
			if event == service.EventDomainMembershipChanged {
				domainSvc.OnRemoteInvalidate(payload)
				return
			}
			if event == service.EventMuteChanged {
				muteSvc.OnRemoteInvalidate(payload)
				return
			}
			signalHub.HandleRemoteEvent(event, payload)
		})
	}

	// mute rule KV for degraded media mute (Agora kicking-rule ids): nats → memory
	muteRuleStore, muteRuleBackend := bus.ResolveMuteRuleStore(bus.ResolveMuteRuleConfig{
		Mode:   cfg.StateStore,
		Prefix: cfg.NATSSubjectPrefix,
		NATS:   natsConn,
	})
	if dp, ok := sfuProvider.(*factory.DynamicProvider); ok {
		dp.SetMuteRuleStore(muteRuleStore)
	}
	logger.WithComponent("MuteRuleStore").Infof("ready backend=%s", muteRuleBackend)

	// JWT blacklist + signing key: NATS KV so multi-instance logout still works.
	if natsConn != nil {
		if authStore, err := bus.OpenAuthStore(bus.AuthStoreConfig{
			Prefix: cfg.NATSSubjectPrefix,
			NC:     natsConn,
		}); err != nil {
			logger.WithComponent("AuthStore").Warnf("nats unavailable: %v", err)
		} else {
			authstate.SetBackend(authStore)
			logger.WithComponent("AuthStore").Infof("ready backend=%s", authStore.Backend())
		}
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
		}
	}
	// 注入 Hub room 聚合视图给 SRS 等无原生 room 维度的 provider（pkg.RoomRegistrySetter）。
	if rs, ok := sfuProvider.(pkg.RoomRegistrySetter); ok {
		rs.SetRoomRegistry(signalHub)
	}
	// 注入 stream→room 反查给 SRS 等 provider（SRS API 直查后映射回 GOSpeak room）。
	if rs, ok := sfuProvider.(pkg.StreamRoomResolverSetter); ok {
		rs.SetStreamRoomResolver(signalHub)
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
		cfMediaSvc.SetSessionDomainLookup(dp.SessionDomain)
		cfMediaSvc.SetDomainMemberChecker(domainSvc.IsMember)
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
	srsCallbackH.SetMuteRuleStore(muteRuleStore)

	authCookieCfg := handler.NewAuthCookieConfig(cfg)
	authH := handler.NewAuthHandler(authSvc, authCookieCfg)
	emailH := handler.NewEmailVerificationHandler(emailVerificationSvc)
	emailConfigH := handler.NewEmailConfigHandler(emailConfigSvc)
	userH := handler.NewUserHandler(userSvc, permSvc, storageSvc)
	userGroupH := handler.NewUserGroupHandler(userGroupSvc, userSvc)
	oauthH := handler.NewOAuthHandler(oauthSvc, authCookieCfg)
	roleH := handler.NewRoleHandler(roleSvc)
	clusterSvc := service.NewClusterService(repository.NewClusterNodeRepository(db), repository.NewServerAssignmentRepository(db))
	clusterSvc.SetNotifier(eventBus)
	clusterSvc.SetServerRepo(repository.NewDomainRepository(db))
	if rs, ok := store.(service.RoomMetaReader); ok {
		clusterSvc.SetRoomMetaStore(rs)
	}
	signalH.SetClusterResolver(func(domainUUID string) (string, error) {
		_, node, err := clusterSvc.ResolveServer(domainUUID)
		if err != nil {
			return "", err
		}
		if entry := strings.TrimSpace(cfg.ClusterEntryURL); entry != "" {
			return strings.TrimRight(entry, "/") + "/ws?worker=" + url.QueryEscape(node.UUID), nil
		}
		return node.AdvertiseURL, nil
	})
	signalH.SetRoomResolver(func(domainUUID, room string) (string, error) {
		_, node, err := clusterSvc.ResolveRoom(domainUUID, room)
		if err != nil {
			return "", err
		}
		if entry := strings.TrimSpace(cfg.ClusterEntryURL); entry != "" {
			return strings.TrimRight(entry, "/") + "/ws?worker=" + url.QueryEscape(node.UUID), nil
		}
		return node.AdvertiseURL, nil
	})
	roomH := handler.NewRoomHandler(roomSvc, permSvc, domainSvc)
	roomH.SetRoomListBroadcaster(signalHub)
	roomH.SetControlPublisher(clusterSvc)
	msgH := handler.NewMessageHandler(messageSvc, permSvc, roomSvc, domainSvc)
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
	// 临时禁言到期扫描：ListActiveMutes 会清理过期记录并触发 onExpired（广播 + SFU 恢复）。
	// 之前的到期清理依赖查询路径惰性触发，无定时任务时到期后订阅端会残留静音状态。
	_ = jobs.StartMuteExpiryScanner(context.Background(), func() {
		if _, err := muteSvc.ListActiveMutes(); err != nil {
			logger.WithComponent("MuteExpiry").Warnf("scan expired mutes: %v", err)
		}
	}, time.Minute)
	sfuConfigH := handler.NewSFUConfigHandler(sfuConfigSvc, signalHub)
	storageH := handler.NewStorageHandler(storageSvc)
	botH := handler.NewBotHandler(botSvc)
	middleware.Configure(middleware.Dependencies{
		PermissionChecker:   permSvc,
		TokenVersionChecker: authSvc,
		BotTokenChecker:     botSvc,
		DomainChecker:       domainSvc.IsMember,
		BlacklistChecker:    authstate.IsBlacklistedErr,
		AuthCookieName:      cfg.AccessCookieName,
	})

	var clusterHandler *handler.ClusterHandler
	var clusterStop func()
	var agentLeaderLock *cluster.NATSLeaderLock
	var stopLeaderRenew func()
	var leaderFence *service.LeaderFenceService
	var localNodeUUID string
	var degradedToWorker bool
	clusterHandler, clusterStop, agentLeaderLock, stopLeaderRenew, leaderFence, localNodeUUID, degradedToWorker, err = startClusterRuntimes(
		cfg, db, natsConn, instanceID, clusterSvc, signalHub, sfuProvider,
	)
	if err != nil {
		return fmt.Errorf("failed to start cluster runtime: %w", err)
	}
	// Agent 主锁抢到后才允许控制面 seed/插件写库；备机降级为 Worker 只读路径。
	if leaderFence != nil {
		if err := bootstrapAgentControlPlane(db, roleRepo, userRepo, permRepo, sfuConfigSvc, pluginReg, permSvc); err != nil {
			return fmt.Errorf("bootstrap agent control plane: %w", err)
		}
	} else {
		loadRoles(roleRepo)
		if err := permSvc.LoadCache(); err != nil {
			return fmt.Errorf("failed to load permission cache: %w", err)
		}
	}
	pluginSvc := service.NewPluginService(pluginReg)
	for _, info := range pluginSvc.List() {
		logger.WithComponent("Plugin").Infof(
			"plugin=%s enabled=%v status=%s side_servers=%d",
			info.Name, info.Enabled, info.Status, len(info.SideServers),
		)
	}
	pluginH := handler.NewPluginHandler(pluginSvc)
	localClusterNodeID := localNodeUUID

	startJobConsumers(cfg, jobQueue, eventBus.InstanceID(), signalHub, messageSvc, conversationSvc, clusterSvc, localClusterNodeID)

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

	monitorH := handler.NewMonitorHandler(signalHub, cfg, newServerInfraStats(eventBus, cfg), clusterSvc)

	// Prometheus /metrics：业务指标快照 + HTTP 请求指标，可选用 METRICS_TOKEN 保护。
	metricsSrv := metrics.New(func() metrics.Snapshot { return toMetricsSnapshot(monitorH.Collect()) })
	r.Use(metricsSrv.Middleware())
	metricsHandler := metrics.RequireToken(metricsSrv.Handler(), cfg.MetricsToken)
	r.GET("/metrics", gin.WrapH(metricsHandler))

	// 启动签名密钥轮换检查
	authstate.StartKeyRotationLoop()

	// WS 升级路由（JWT 鉴权由 Upgrader 内部完成）
	var wsUpgrader *ws.Upgrader
	if cfg.IsWorker() {
		wsUpgrader = ws.NewUpgrader(ws.UpgraderConfig{
			Fanout:         wsFanout,
			Handler:        wsHandler,
			AllowedOrigins: wsAllowedOrigins(cfg),
			AuthCookieName: cfg.AccessCookieName,
			OnConnect: func(c *ws.Client) {
				if err := signalHub.OnConnect(c); err != nil {
					logger.WithComponent("WS").Warnf("on connect failed: %v", err)
					c.Close()
				}
			},
			OnDisconnect: func(c *ws.Client) {
				signalHub.OnDisconnect(c)
			},
		})
	}

	router.SetupRoutes(r, &router.Handlers{
		Config: cfg,
		FenceCheck: func() error {
			if leaderFence == nil {
				return nil
			}
			return leaderFence.Verify()
		},
		ReadyCheck: func() error {
			if _, err := db.DB(); err != nil {
				return err
			}
			return nil
		},
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

	scheme := "http"
	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		scheme = "https"
	}
	logger.WithComponent("Swagger").Infof("API 文档地址: %s://localhost:%s/swagger/index.html", scheme, port)

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
	go runGracefulShutdown(shutdownDeps{
		srv:             srv,
		signalHub:       signalHub,
		wsUpgrader:      wsUpgrader,
		pluginReg:       pluginReg,
		clusterStop:     clusterStop,
		stopLeaderRenew: stopLeaderRenew,
		agentLeaderLock: agentLeaderLock,
		leaderFence:     leaderFence,
		instanceID:      instanceID,
		closeEventBus:   closeEventBus,
	})

	if err := serveHTTP(srv, cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && err != http.ErrServerClosed {
		logger.WithComponent("HTTP").Fatalf("listen error: %v", err)
	}
	return nil
}

// serveHTTP 按 TLS 配置选择监听方式：两者均未配置时保持明文 HTTP/1.1；
// 配置证书后启用 HTTPS，Go 标准库会自动提供 h2 + HTTP/1.1 ALPN fallback。
func serveHTTP(srv *http.Server, tlsCert, tlsKey string) error {
	if tlsCert == "" && tlsKey == "" {
		return srv.ListenAndServe()
	}
	if tlsCert == "" || tlsKey == "" {
		return fmt.Errorf("TLS_CERT and TLS_KEY must be configured together")
	}
	return srv.ListenAndServeTLS(tlsCert, tlsKey)
}
