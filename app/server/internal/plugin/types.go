package plugin

import (
	"context"
	"database/sql"
	"net/http"

	"GOSpeak/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Kind 插件类型
type Kind string

const (
	KindBuiltin  Kind = "builtin"
	KindExternal Kind = "external"
)

// Meta 插件元信息，供管理端展示
type Meta struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Desc        string `json:"desc"`
	Kind        Kind   `json:"kind"`
	// ConfigSchema 可选：前端表单提示（JSON object 描述）
	ConfigSchema map[string]any `json:"config_schema,omitempty"`
}

// Status 运行态
type Status string

const (
	StatusRegistered Status = "registered"
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusStopped    Status = "stopped"
	StatusFailed     Status = "failed"
)

// Plugin 后端可挂载组件接口
type Plugin interface {
	Meta() Meta
	// Init 注入 Host，完成依赖装配（不启动服务）
	Init(host Host) error
	// Start 启动插件自身逻辑（可拉起 side server）
	Start(ctx context.Context) error
	// Stop 优雅停止
	Stop(ctx context.Context) error
}

// Configurable 可选：插件支持读写配置
type Configurable interface {
	// ValidateConfig 校验待写入配置；返回规范化后的配置
	ValidateConfig(raw map[string]any) (map[string]any, error)
	// OnConfigUpdated 配置热更新回调（可选）
	OnConfigUpdated(cfg map[string]any) error
}

// SideServer 插件自启小服务端句柄
type SideServer interface {
	Name() string
	Addr() string
	URL() string
	Stop(ctx context.Context) error
}

// Host 插件宿主能力
type Host interface {
	Logger(component string) *logrus.Entry
	DB() HostDB
	AppConfig() *config.Config
	// RegisterHTTP 在主 API 下挂载插件路由：/api/v1/plugins/:name/*
	RegisterHTTP(fn func(r *gin.RouterGroup))
	// StartSideServer 允许插件自行拉起独立 HTTP 小服务
	StartSideServer(name, addr string, handler http.Handler) (SideServer, error)
	// LoadConfig / SaveConfig 读写插件持久化配置
	LoadConfig(pluginName string) (enabled bool, cfg map[string]any, err error)
	SaveConfig(pluginName string, enabled bool, cfg map[string]any) error
}

// HostDB 是插件可用的受限数据库能力，不暴露 *gorm.DB 与任意表访问。
type HostDB interface {
	Ping() error
	Stats() sql.DBStats
}

// Info 管理端列表项
type Info struct {
	Meta
	Enabled     bool             `json:"enabled"`
	Status      Status           `json:"status"`
	Error       string           `json:"error,omitempty"`
	Config      map[string]any   `json:"config,omitempty"`
	SideServers []SideServerInfo `json:"side_servers,omitempty"`
}

// SideServerInfo 对外展示
type SideServerInfo struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
	URL  string `json:"url"`
}
