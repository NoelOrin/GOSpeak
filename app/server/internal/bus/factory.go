package bus

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// InitConfig 启动 EventBus 的参数。总线固定使用进程内嵌 NATS。
type InitConfig struct {
	Prefix         string
	Name           string
	ConnectTimeout time.Duration
	// EmbeddedPort 内嵌 NATS 监听端口；<=0 表示随机端口。
	EmbeddedPort int
	// StoreDir JetStream 持久化目录；非空时跨重启保留 KV（签名密钥/黑名单），Shutdown 不删除。
	// 为空则退化为临时目录（兼容测试与未配置场景）。
	StoreDir  string
	Deliverer Deliverer
	// RemoteHook 可选：peer 事件在本地投递后回调（用于 Hub 控制面清理等）。
	RemoteHook func(event string, payload interface{})
	// WALPath 非空时启用断线事件磁盘持久化；为空保持纯内存行为（默认）。
	WALPath string
}

// Stats 供监控面板与测试。
type Stats struct {
	Mode           string `json:"mode"`
	Connected      bool   `json:"connected"`
	InstanceID     string `json:"instance_id"`
	DroppedPublish uint64 `json:"dropped_publish"`
}

// Init 创建 EventBus（内嵌 NATS）。
// cleanup 必须在进程退出时调用：先 Close bus，再 Shutdown 内嵌。
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

	es, err := StartEmbeddedServerWithStore(cfg.EmbeddedPort, cfg.StoreDir)
	if err != nil {
		return nil, func() {}, fmt.Errorf("bus init embedded: %w", err)
	}
	url := es.ClientURL()
	log.Printf("[EventBus] embedded nats started: %s instance=%s storeDir=%s", url, instanceID, es.StoreDir())

	nb, err := NewNATSBus(NATSBusConfig{
		URL:           url,
		SubjectPrefix: cfg.Prefix,
		InstanceID:    instanceID,
		Name:          name,
		Deliverer:     cfg.Deliverer,
		RemoteHook:    cfg.RemoteHook,
		WALPath:       cfg.WALPath,
	})
	if err != nil {
		es.Shutdown()
		return nil, func() {}, err
	}

	cleanup := func() {
		_ = nb.Close()
		es.Shutdown()
		log.Printf("[EventBus] embedded nats stopped")
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
		st.DroppedPublish = nb.DroppedPublishCount()
	}
	return st
}
