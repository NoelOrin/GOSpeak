package bus

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// InitConfig 启动 EventBus 的参数。
// URL 空 = 内嵌；非空 = 先探测外部，成功则 external，失败直接返回错误（不回退内嵌）。
type InitConfig struct {
	URL            string
	Prefix         string
	Name           string
	ConnectTimeout time.Duration
	// EmbeddedPort 内嵌 NATS 监听端口；<=0 表示随机端口。
	// 仅当 URL 为空时生效。
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
	DroppedPublish       uint64 `json:"dropped_publish"`
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
	var embedded *EmbeddedServer

	if url != "" {
		if err := ProbeExternal(url, cfg.ConnectTimeout); err != nil {
			// 配置了外部 URL：探测失败直接失败，不回退内嵌。
			// 上层 StartGin 对 Init 错误会 panic，进程退出。
			return nil, func() {}, fmt.Errorf("bus init external nats probe failed (%s): %w", url, err)
		}
		mode = "external"
		log.Printf("[EventBus] external nats probe ok: %s instance=%s", url, instanceID)
	} else {
		es, err := StartEmbeddedServerOnPort(cfg.EmbeddedPort)
		if err != nil {
			return nil, func() {}, fmt.Errorf("bus init embedded: %w", err)
		}
		embedded = es
		url = es.ClientURL()
		log.Printf("[EventBus] embedded nats started: %s instance=%s", url, instanceID)
	}

	nb, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		SubjectPrefix: cfg.Prefix,
		InstanceID:    instanceID,
		Name:          name,
		Mode:          mode,
		Deliverer:     cfg.Deliverer,
		RemoteHook:    cfg.RemoteHook,
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
		st.DroppedPublish = nb.DroppedPublishCount()
	}
	return st
}
