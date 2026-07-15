package bus

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// InitConfig 启动 EventBus 的参数。
// URL 空 = 直接内嵌；非空 = 先探测外部，可用则 external，不可用则回退内嵌。
type InitConfig struct {
	URL            string
	Prefix         string
	Name           string
	ConnectTimeout time.Duration
	// EmbeddedPort 内嵌 NATS 监听端口；<=0 表示随机端口。
	EmbeddedPort int
	Deliverer    Deliverer
	// RemoteHook 可选：peer 事件在本地投递后回调（用于 Hub 控制面清理等）。
	RemoteHook func(event string, payload interface{})
}

// Stats 供监控面板与测试。
type Stats struct {
	Mode                 string `json:"mode"`
	Connected            bool   `json:"connected"`
	InstanceID           string `json:"instance_id"`
	FallbackFromExternal bool   `json:"fallback_from_external"`
}

// Init 创建 EventBus。
// cleanup 必须在进程退出时调用：先 Close bus，再 Shutdown 内嵌（若有）。
func Init(cfg InitConfig) (EventBus, func(), error) {
	if cfg.Deliverer == nil {
		return nil, func() {}, fmt.Errorf("bus init: nil Deliverer")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 2 * time.Second
	}

	instanceID := cfg.Name
	if instanceID == "" {
		host, _ := os.Hostname()
		instanceID = fmt.Sprintf("gospeak-%s-%d", sanitize(host), os.Getpid())
	}
	name := cfg.Name
	if name == "" {
		name = instanceID
	}

	url := strings.TrimSpace(cfg.URL)
	mode := "embedded"
	fallbackFromExternal := false
	var embedded *EmbeddedServer

	if url != "" {
		if err := ProbeExternal(url, cfg.ConnectTimeout); err != nil {
			log.Printf("[EventBus] external nats unavailable (%s): %v; fallback to embedded", url, err)
			fallbackFromExternal = true
			url = ""
		} else {
			mode = "external"
			log.Printf("[EventBus] external nats probe ok: %s instance=%s", url, instanceID)
		}
	}

	if mode == "embedded" {
		es, err := StartEmbeddedServerOnPort(cfg.EmbeddedPort)
		if err != nil {
			return nil, func() {}, fmt.Errorf("bus init embedded: %w", err)
		}
		embedded = es
		url = es.ClientURL()
		if fallbackFromExternal {
			log.Printf("[EventBus] embedded nats started (fallback): %s instance=%s", url, instanceID)
		} else {
			log.Printf("[EventBus] embedded nats started: %s instance=%s", url, instanceID)
		}
	}

	nb, err := NewNATSBus(NATSBusConfig{
		URL:                  url,
		SubjectPrefix:        cfg.Prefix,
		InstanceID:           instanceID,
		Name:                 name,
		Mode:                 mode,
		FallbackFromExternal: fallbackFromExternal,
		Deliverer:            cfg.Deliverer,
		RemoteHook:           cfg.RemoteHook,
	})
	if err != nil {
		if embedded != nil {
			embedded.Shutdown()
		}
		return nil, func() {}, err
	}

	cleanup := func() {
		_ = nb.Close()
		if embedded != nil {
			embedded.Shutdown()
			log.Printf("[EventBus] embedded nats stopped")
		}
	}
	return nb, cleanup, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	if s == "" {
		return "unknown"
	}
	return s
}

// GetStats 返回 EventBus 状态快照。
func GetStats(b EventBus) Stats {
	if b == nil {
		return Stats{Mode: "none", Connected: false}
	}
	st := Stats{
		Mode:       b.Mode(),
		Connected:  b.IsConnected(),
		InstanceID: b.InstanceID(),
	}
	if nb, ok := b.(*NATSBus); ok {
		st.FallbackFromExternal = nb.fallbackFromExternal
	}
	return st
}
