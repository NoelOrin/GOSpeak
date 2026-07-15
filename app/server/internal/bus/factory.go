package bus

import (
	"fmt"
	"log"
)

// InitConfig 是 Init 的参数。
type InitConfig struct {
	InstanceID    string
	SubjectPrefix string
	NATSURL       string
	Deliverer     Deliverer
}

// Stats 暴露 NATSBus 内部状态，用于监控和测试。
type Stats struct {
	Mode                 string
	Connected            bool
	FallbackFromExternal bool
	URL                  string
}

// GetStats 返回 NATSBus 的状态快照。
func GetStats(b *NATSBus) Stats {
	return Stats{
		Mode:                 b.mode,
		Connected:            b.connected.Load(),
		FallbackFromExternal: b.fallbackFromExternal,
		URL:                  b.url,
	}
}

// Init 探测外部 NATS 可用性后决定内嵌/外部。
//
// 决策逻辑:
//
//	if NATSURL == "":
//	    startEmbedded(); connect(ClientURL); mode=embedded
//	else:
//	    if probe(NATSURL) OK:
//	        connect external; mode=external; no embed
//	    else:
//	        log warning; startEmbedded; mode=embedded; FallbackFromExternal=true
func Init(cfg InitConfig) (nb *NATSBus, embed *EmbeddedServer, err error) {
	// 情况 1: 未配置外部 NATS → 直接起内嵌
	if cfg.NATSURL == "" {
		es, startErr := StartEmbeddedServer()
		if startErr != nil {
			return nil, nil, fmt.Errorf("bus init: start embedded: %w", startErr)
		}
		nb, connErr := NewNATSBus(NATSBusConfig{
			InstanceID:    cfg.InstanceID,
			SubjectPrefix: cfg.SubjectPrefix,
			URL:           es.ClientURL(),
			Deliverer:     cfg.Deliverer,
		})
		if connErr != nil {
			es.Shutdown()
			return nil, nil, fmt.Errorf("bus init: connect embedded: %w", connErr)
		}
		return nb, es, nil
	}

	// 情况 2: 配置了外部 NATS — 先探测
	probeErr := ProbeExternal(cfg.NATSURL)
	if probeErr == nil {
		// 外部可达
		nb, connErr := NewNATSBus(NATSBusConfig{
			InstanceID:    cfg.InstanceID,
			SubjectPrefix: cfg.SubjectPrefix,
			URL:           cfg.NATSURL,
			Mode:          "external",
			Deliverer:     cfg.Deliverer,
		})
		if connErr != nil {
			return nil, nil, fmt.Errorf("bus init: connect external: %w", connErr)
		}
		return nb, nil, nil
	}

	// 外部不可达 — 降级到内嵌
	log.Printf("warn: probe external nats %s failed (%v), falling back to embedded", cfg.NATSURL, probeErr)
	es, startErr := StartEmbeddedServer()
	if startErr != nil {
		return nil, nil, fmt.Errorf("bus init: probe fail (%v) and start embedded: %w", probeErr, startErr)
	}
	nb, connErr := NewNATSBus(NATSBusConfig{
		InstanceID:           cfg.InstanceID,
		SubjectPrefix:        cfg.SubjectPrefix,
		URL:                  es.ClientURL(),
		Mode:                 "embedded",
		FallbackFromExternal: true,
		Deliverer:            cfg.Deliverer,
	})
	if connErr != nil {
		es.Shutdown()
		return nil, nil, fmt.Errorf("bus init: probe fail (%v) and connect embedded: %w", probeErr, connErr)
	}
	return nb, es, nil
}
