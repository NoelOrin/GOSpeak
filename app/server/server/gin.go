package server

import (
	"GOSpeak/internal/audit"
	"GOSpeak/internal/authstate"
	"GOSpeak/internal/bus"
	"GOSpeak/internal/config"
	"GOSpeak/internal/handler"
	"GOSpeak/internal/jobs"
	"GOSpeak/internal/livekitproxy"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/metrics"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/policy"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/router"
	"GOSpeak/internal/service"
	"GOSpeak/internal/sfu"
	"GOSpeak/internal/sfu/factory"
	"GOSpeak/internal/signal"
	"GOSpeak/internal/storage"
	"context"
	"fmt"
	"net/http"
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
	announcementRepo := repository.NewAnnouncementRepository(db)
	domainSvc := service.NewDomainService(domainRepo, domainRoleRepo)

	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.UseCasbin(repository.NewCasbinAdapter(db)); err != nil {
		return fmt.Errorf("failed to init casbin: %w", err)
	}
	if err := domainSvc.UseCasbin(repository.NewDomainCasbinAdapter(db)); err != nil {
		return fmt.Errorf("failed to init domain casbin: %w", err)
	}

	roleSvc := service.NewRoleService(roleRepo)
	// 角色改名/删除后连锁重载权限缓存与 Casbin 策略（角色名是两者共用的 key）。
	roleSvc.SetOnRoleChanged(func() {
		if err := permSvc.LoadCache(); err != nil {
			logger.WithComponent("Permission").WithError(err).Warn("role change: permission cache reload failed")
		}
	})
	roleSvc.SetOnRoleRenamed(func(oldName, newName string) error {
		return permSvc.RenameRole(oldName, newName)
	})
	emailConfigSvc := service.NewEmailConfigService(emailConfigRepo, cfg)
	emailSvc := service.NewEmailService(emailConfigSvc.ResolveConfig)
	emailVerificationSvc := service.NewEmailVerificationService(emailVerificationRepo, userRepo, emailSvc, emailConfigSvc.ResolveConfig)
	authSvc := service.NewAuthService(userRepo, emailConfigSvc, emailVerificationSvc)
	service.SetBcryptCost(cfg.BcryptCost)
	pkg.AccessTokenTTL = cfg.JWTAccessTTLDuration()
	pkg.RefreshTokenTTL = cfg.JWTRefreshTTLDuration()
	inviteSvc := service.NewInviteService(db, repository.NewDomainInviteRepository(db), domainRepo, cfg.StorageEncryptKey, cfg.JWTKey)
	joinRequestSvc := service.NewJoinRequestService(
		db,
		repository.NewDomainJoinRequestRepository(db),
		domainRepo,
		userRepo,
	)
	domainSvc.SetInviteCreator(inviteSvc)
	storageSvc := service.NewStorageService(storageConfigRepo, cfg)
	userSvc := service.NewUserService(userRepo)
	userGroupSvc := service.NewUserGroupService(userGroupRepo)
	oauthSvc := service.NewOAuthService(oauthProviderRepo, oauthAccountRepo, userRepo)
	oauthSvc.SetAutoCreateUser(cfg.OAuthAutoCreateUser)
	roomSvc := service.NewRoomService(roomRepo)
	messageSvc := service.NewMessageService(messageRepo, roomRepo, domainSvc)
	messageSvc.SetUserRepo(userRepo)
	muteSvc := service.NewMuteService(muteRepo, userRepo)
	sfuConfigSvc := service.NewSFUConfigService(sfuConfigRepo, cfg)
	var sfuProvider sfu.Provider = factory.NewDynamicProvider(sfuConfigSvc.ResolveConfig)

	r := gin.New()

	// 只信任配置的代理网段注入的 X-Forwarded-*，避免外部客户端伪造。
	// TRUSTED_PROXIES 为逗号分隔的 IP/CIDR 列表；为空时回退本机回环地址。
	trustedProxies := cfg.TrustedProxiesList()
	if len(trustedProxies) == 0 {
		trustedProxies = []string{"127.0.0.1", "::1"}
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
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
		Prefix:         cfg.NATSSubjectPrefix,
		Name:           cfg.NATSName,
		ConnectTimeout: timeout,
		EmbeddedPort:   embeddedPort,
		Deliverer:      deliverer,
		WALPath:        cfg.NATSWALPath(),
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

	// membership KV: 强制 NATS（内嵌或外部 URL，bus.Init 已保证可用）。
	store, backend, err := bus.ResolveMembershipStore(bus.ResolveMembershipConfig{
		Prefix: cfg.NATSSubjectPrefix,
		NATS:   natsConn,
	})
	if err != nil {
		return fmt.Errorf("failed to init membership store: %w", err)
	}
	logger.WithComponent("StateStore").Infof("ready backend=%s instance=%s", backend, instanceID)

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
	signalHub.StartMembershipHeartbeat()
	if nb, ok := eventBus.(*bus.NATSBus); ok {
		nb.SetRemoteHook(func(event string, payload interface{}) {
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

	// JWT blacklist + signing key: 强制 NATS KV（bus.Init 保证内嵌或外部 NATS 可用）。
	if natsConn == nil {
		return fmt.Errorf("failed to init auth store: nats connection required")
	}
	authStore, err := bus.OpenAuthStore(bus.AuthStoreConfig{
		Prefix: cfg.NATSSubjectPrefix,
		NC:     natsConn,
	})
	if err != nil {
		return fmt.Errorf("failed to init auth store: %w", err)
	}
	authstate.SetBackend(authStore)
	logger.WithComponent("AuthStore").Infof("ready backend=%s", authStore.Backend())

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
	signalH.SetLiveKitKeyResolver(func() string {
		resolved, err := sfuConfigSvc.ResolveConfig()
		if err != nil || resolved == nil {
			return cfg.LiveKitKey
		}
		if resolved.LiveKitKey != "" {
			return resolved.LiveKitKey
		}
		return cfg.LiveKitKey
	})
	if jobQueue != nil {
		signalH.SetJobs(jobQueue)
	}

	authCookieCfg := handler.NewAuthCookieConfig(cfg)
	authH := handler.NewAuthHandler(authSvc, authCookieCfg)
	joinRequestSvc.SetMemberAddedHook(domainSvc.OnMemberAdded)
	inviteJoinSvc := service.NewInviteJoinService(
		db, inviteSvc, joinRequestSvc,
		repository.NewDomainInviteRepository(db), domainRepo, userRepo,
		emailConfigSvc, emailVerificationSvc,
	)
	inviteH := handler.NewInviteHandler(inviteSvc)
	inviteJoinH := handler.NewInviteJoinHandler(inviteJoinSvc, authCookieCfg)
	joinRequestH := handler.NewJoinRequestHandler(joinRequestSvc, domainSvc)
	joinRequestH.SetGlobalPermissionChecker(permSvc)
	inviteH.SetPermissionCheckers(domainSvc, permSvc)
	emailH := handler.NewEmailVerificationHandler(emailVerificationSvc)
	emailConfigH := handler.NewEmailConfigHandler(emailConfigSvc)
	userH := handler.NewUserHandler(userSvc, permSvc)
	auditSvc := audit.NewService(db)
	userH.SetAuditor(auditSvc)
	userGroupH := handler.NewUserGroupHandler(userGroupSvc, userSvc)
	oauthH := handler.NewOAuthHandler(oauthSvc, authCookieCfg)
	roleH := handler.NewRoleHandler(roleSvc)
	roomH := handler.NewRoomHandler(roomSvc, permSvc, domainSvc)
	roomH.SetRoomListBroadcaster(signalHub)
	roomH.SetAuditor(auditSvc)
	msgH := handler.NewMessageHandler(messageSvc, permSvc, roomSvc, domainSvc)
	permH := handler.NewPermissionHandler(permSvc)
	muteH := handler.NewMuteHandler(muteSvc, userSvc, signalHub)
	muteH.SetAuditor(auditSvc)

	// 临时禁言过期：删除记录后走完整 unmute（广播 + SFU 恢复），
	// 若期间管理员已重新禁言则跳过，避免误解除新禁言。
	muteSvc.SetOnExpired(func(userID uint) {
		if muted, _, err := muteSvc.IsMuted(userID); err == nil && !muted {
			signalHub.BroadcastUnmute(userID)
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
	middleware.Configure(middleware.Dependencies{
		PermissionChecker:   permSvc,
		TokenVersionChecker: authSvc,
		DomainChecker:       domainSvc.IsMember,
		BlacklistChecker:    authstate.IsBlacklistedErr,
		AuthCookieName:      cfg.AccessCookieName,
	})

	if err := bootstrapControlPlane(db, roleRepo, userRepo, permRepo, sfuConfigSvc, permSvc); err != nil {
		return fmt.Errorf("bootstrap control plane: %w", err)
	}
	startJobConsumers(jobQueue, eventBus.InstanceID(), signalHub, messageSvc, conversationSvc)

	domainH := handler.NewDomainHandler(domainSvc, permSvc)
	domainH.SetOnDomainKick(signalHub.KickUserFromDomain)
	domainH.SetAuditor(auditSvc)
	auditH := handler.NewAuditHandler(auditSvc)
	announcementSvc := service.NewAnnouncementService(announcementRepo, domainRepo, db)
	announcementSvc.SetEventBus(eventBus)
	announcementSvc.SetDomainPermissionChecker(domainSvc.HasDomainPermission)
	announcementH := handler.NewAnnouncementHandler(announcementSvc, permSvc)
	domainH.SetOnDomainLeave(signalHub.KickUserFromDomain)
	domainH.SetOnDomainDelete(func(serverUUID string) {
		signalHub.OnDomainDelete(serverUUID)
	})
	conversationH := handler.NewConversationHandler(conversationSvc)

	monitorH := handler.NewMonitorHandler(signalHub, cfg, newServerInfraStats(eventBus))
	systemUpdateH := handler.NewSystemUpdateHandler(service.NewSystemUpdateService())

	// Prometheus /metrics：业务指标快照 + HTTP 请求指标，可选用 METRICS_TOKEN 保护。
	metricsSrv := metrics.New(func() metrics.Snapshot { return toMetricsSnapshot(monitorH.Collect()) })
	r.Use(metricsSrv.Middleware())
	metricsHandler := metrics.RequireToken(metricsSrv.Handler(), cfg.MetricsToken)
	r.GET("/metrics", gin.WrapH(metricsHandler))

	// 启动签名密钥轮换检查
	authstate.StartKeyRotationLoop()

	// WS 升级路由：JWT 鉴权由 Upgrader 内部完成，BanCheck 等价策略
	// 由 installWSPolicy 注入（/ws 绕过 Gin，不能依赖中间件链）。
	wsUpgraderCfg := ws.UpgraderConfig{
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
	}
	policyEngine := policy.NewEngine()
	installWSPolicy(policyEngine, wsHandler, &wsUpgraderCfg, signalHub)
	wsUpgrader := ws.NewUpgrader(wsUpgraderCfg)

	router.SetupRoutes(r, &router.Handlers{
		Config: cfg,
		ReadyCheck: func() error {
			if _, err := db.DB(); err != nil {
				return err
			}
			return nil
		},
		Auth:         authH,
		Invite:       inviteH,
		InviteJoin:   inviteJoinH,
		JoinRequest:  joinRequestH,
		User:         userH,
		UserGroup:    userGroupH,
		Signal:       signalH,
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
		SystemUpdate: systemUpdateH,
		Domain:       domainH,
		Conversation: conversationH,
		Audit:        auditH,
		Announcement: announcementH,
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

	// 同源 LiveKit 反代：浏览器侧 serverUrl 为 wss://<域名>/livekit 时，
	// 由应用把 /livekit/* 转发到 LIVEKIT_HOST（含 WebSocket 升级）。
	// 这样 LiveKit 不需要独立域名、证书或反代规则，反代层只面对应用一个上游。
	livekitProxy := livekitproxy.NewHandler(livekitproxy.Cached(func() string {
		resolved, err := sfuConfigSvc.ResolveConfig()
		if err == nil && resolved != nil && strings.TrimSpace(resolved.LiveKitHost) != "" {
			return resolved.LiveKitHost
		}
		return cfg.LiveKitHost
	}, 2*time.Second))

	// /ws 必须绕过 Gin：nhooyr/websocket 的 hijack 与 gin.ResponseWriter 不兼容。
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if wsUpgrader != nil && req.URL.Path == "/ws" {
			wsUpgrader.ServeHTTP(w, req)
			return
		}
		if req.URL.Path == livekitproxy.PathPrefix || strings.HasPrefix(req.URL.Path, livekitproxy.PathPrefix+"/") {
			livekitProxy.ServeHTTP(w, req)
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
		srv:           srv,
		signalHub:     signalHub,
		wsUpgrader:    wsUpgrader,
		auditSvc:      auditSvc,
		closeEventBus: closeEventBus,
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
