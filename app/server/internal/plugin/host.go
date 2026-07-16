package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// HostImpl 宿主实现
type HostImpl struct {
	db     *gorm.DB
	cfg    *config.Config
	repo   *repository.PluginConfigRepository

	mu           sync.Mutex
	routeFns     map[string][]func(*gin.RouterGroup) // pluginName -> route registrars
	currentPlugin string
	sideServers  map[string]*sideServer // key = pluginName/serverName
}

type sideServer struct {
	pluginName string
	name       string
	addr       string
	url        string
	server     *http.Server
}

func (s *sideServer) Name() string { return s.name }
func (s *sideServer) Addr() string { return s.addr }
func (s *sideServer) URL() string  { return s.url }
func (s *sideServer) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func NewHost(db *gorm.DB, cfg *config.Config, repo *repository.PluginConfigRepository) *HostImpl {
	return &HostImpl{
		db:          db,
		cfg:         cfg,
		repo:        repo,
		routeFns:    make(map[string][]func(*gin.RouterGroup)),
		sideServers: make(map[string]*sideServer),
	}
}

// WithPlugin 设置当前插件上下文（Init/Start 时调用）
func (h *HostImpl) WithPlugin(name string) *HostImpl {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.currentPlugin = name
	return h
}

func (h *HostImpl) Logger(component string) *logrus.Entry {
	return logger.WithComponent("Plugin/" + component)
}

func (h *HostImpl) DB() *gorm.DB { return h.db }

func (h *HostImpl) AppConfig() *config.Config { return h.cfg }

func (h *HostImpl) RegisterHTTP(fn func(r *gin.RouterGroup)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	name := h.currentPlugin
	if name == "" {
		name = "_unknown"
	}
	h.routeFns[name] = append(h.routeFns[name], fn)
}

// MountRoutes 把各插件路由挂到 /api/v1/plugins/:name
func (h *HostImpl) MountRoutes(pluginsGroup *gin.RouterGroup) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for name, fns := range h.routeFns {
		g := pluginsGroup.Group("/" + name)
		for _, fn := range fns {
			fn(g)
		}
	}
}

func (h *HostImpl) StartSideServer(name, addr string, handler http.Handler) (SideServer, error) {
	h.mu.Lock()
	pluginName := h.currentPlugin
	h.mu.Unlock()
	if pluginName == "" {
		return nil, fmt.Errorf("StartSideServer requires plugin context")
	}
	if name == "" {
		return nil, fmt.Errorf("side server name is required")
	}
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	bound := ln.Addr().String()
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ss := &sideServer{
		pluginName: pluginName,
		name:       name,
		addr:       bound,
		url:        "http://" + bound,
		server:     srv,
	}
	key := pluginName + "/" + name
	h.mu.Lock()
	// 若已存在同名，先停旧的
	if old, ok := h.sideServers[key]; ok {
		_ = old.Stop(context.Background())
	}
	h.sideServers[key] = ss
	h.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			h.Logger(pluginName).Warnf("side server %s stopped: %v", name, err)
		}
	}()
	h.Logger(pluginName).Infof("side server started: %s -> %s", name, ss.url)
	return ss, nil
}

func (h *HostImpl) LoadConfig(pluginName string) (bool, map[string]any, error) {
	row, err := h.repo.Get(pluginName)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 默认启用 builtin，配置为空
			return true, map[string]any{}, nil
		}
		return false, nil, err
	}
	cfg := map[string]any{}
	if strings.TrimSpace(row.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(row.ConfigJSON), &cfg); err != nil {
			return row.Enabled, nil, fmt.Errorf("invalid config json for %s: %w", pluginName, err)
		}
	}
	return row.Enabled, cfg, nil
}

func (h *HostImpl) SaveConfig(pluginName string, enabled bool, cfg map[string]any) error {
	if cfg == nil {
		cfg = map[string]any{}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return h.repo.Upsert(&model.PluginConfig{
		Name:       pluginName,
		Enabled:    enabled,
		ConfigJSON: string(raw),
	})
}

func (h *HostImpl) ListSideServers(pluginName string) []SideServerInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]SideServerInfo, 0)
	prefix := pluginName + "/"
	for k, s := range h.sideServers {
		if strings.HasPrefix(k, prefix) {
			out = append(out, SideServerInfo{Name: s.name, Addr: s.addr, URL: s.url})
		}
	}
	return out
}

func (h *HostImpl) StopSideServersByPlugin(pluginName string) {
	h.mu.Lock()
	keys := make([]string, 0)
	for k, s := range h.sideServers {
		if s.pluginName == pluginName {
			keys = append(keys, k)
		}
	}
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, k := range keys {
		h.mu.Lock()
		s := h.sideServers[k]
		delete(h.sideServers, k)
		h.mu.Unlock()
		if s != nil {
			_ = s.Stop(ctx)
		}
	}
}

func (h *HostImpl) StopAllSideServers() {
	h.mu.Lock()
	all := make([]*sideServer, 0, len(h.sideServers))
	for _, s := range h.sideServers {
		all = append(all, s)
	}
	h.sideServers = make(map[string]*sideServer)
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, s := range all {
		_ = s.Stop(ctx)
	}
}
