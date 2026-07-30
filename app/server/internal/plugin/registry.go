package plugin

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Registry 多组件注册中心
type Registry struct {
	mu        sync.RWMutex
	plugins   map[string]Plugin
	states    map[string]*runtimeState
	host      *HostImpl
	lifecycle sync.Mutex // 序列化 Init/Start/Stop 防止 WithPlugin TOCTOU
}

type runtimeState struct {
	status Status
	err    string
}

func (r *Registry) Host() *HostImpl {
	return r.host
}

func NewRegistry(host *HostImpl) *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		states:  make(map[string]*runtimeState),
		host:    host,
	}
}

// Register 注册插件（启动前调用）
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin is nil")
	}
	meta := p.Meta()
	if meta.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[meta.Name]; exists {
		return fmt.Errorf("plugin already registered: %s", meta.Name)
	}
	r.plugins[meta.Name] = p
	r.states[meta.Name] = &runtimeState{status: StatusRegistered}
	return nil
}

// Get 按名取插件
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// Names 已注册插件名（排序）
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.plugins))
	for n := range r.plugins {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// InitAll 初始化全部插件
func (r *Registry) InitAll() error {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	for _, name := range r.Names() {
		p, _ := r.Get(name)
		r.host.WithPlugin(name)
		if err := p.Init(r.host); err != nil {
			r.setState(name, StatusFailed, err.Error())
			return fmt.Errorf("init plugin %s: %w", name, err)
		}
	}
	return nil
}

// StartEnabled 启动 enabled=true 的插件
func (r *Registry) StartEnabled(ctx context.Context) error {
	for _, name := range r.Names() {
		enabled, _, err := r.host.LoadConfig(name)
		if err != nil {
			r.setState(name, StatusFailed, err.Error())
			continue
		}
		if !enabled {
			r.setState(name, StatusStopped, "")
			continue
		}
		if err := r.StartOne(ctx, name); err != nil {
			// 单个插件失败不阻塞其他插件
			continue
		}
	}
	return nil
}

// StartOne 启动指定插件
func (r *Registry) StartOne(ctx context.Context, name string) error {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	p, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	r.setState(name, StatusStarting, "")
	r.host.WithPlugin(name)
	if err := p.Start(ctx); err != nil {
		r.setState(name, StatusFailed, err.Error())
		return err
	}
	r.setState(name, StatusRunning, "")
	return nil
}

// StopOne 停止指定插件
func (r *Registry) StopOne(ctx context.Context, name string) error {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	return r.stopOne(ctx, name)
}

// stopOne 内部实现，调用方须持有 r.lifecycle
func (r *Registry) stopOne(ctx context.Context, name string) error {
	p, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	if err := p.Stop(ctx); err != nil {
		r.setState(name, StatusFailed, err.Error())
		return err
	}
	r.host.StopSideServersByPlugin(name)
	r.setState(name, StatusStopped, "")
	return nil
}

// StopAll 停止全部
func (r *Registry) StopAll(ctx context.Context) {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	for _, name := range r.Names() {
		_ = r.stopOne(ctx, name)
	}
	r.host.StopAllSideServers()
}

// List 管理端列表
func (r *Registry) List() []Info {
	names := r.Names()
	out := make([]Info, 0, len(names))
	for _, name := range names {
		p, _ := r.Get(name)
		meta := p.Meta()
		enabled, cfg, _ := r.host.LoadConfig(name)
		st := r.getState(name)
		info := Info{
			Meta:        meta,
			Enabled:     enabled,
			Status:      st.status,
			Error:       st.err,
			Config:      cfg,
			SideServers: r.host.ListSideServers(name),
		}
		out = append(out, info)
	}
	return out
}

func (r *Registry) setState(name string, status Status, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.states[name]; ok {
		s.status = status
		s.err = errMsg
	}
}

func (r *Registry) getState(name string) runtimeState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.states[name]; ok {
		return *s
	}
	return runtimeState{status: StatusRegistered}
}
