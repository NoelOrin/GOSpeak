package bus

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// ProbeExternal 校验外部 NATS 是否可连接。
// 成功时立即关闭探测连接（正式连接由 NewNATSBus 建立）。
func ProbeExternal(url string, timeout time.Duration) error {
	if url == "" {
		return fmt.Errorf("nats probe: empty url")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	nc, err := nats.Connect(url,
		nats.Name("gospeak-probe"),
		nats.Timeout(timeout),
		nats.MaxReconnects(0),
		nats.DontRandomize(),
	)
	if err != nil {
		return fmt.Errorf("nats probe connect: %w", err)
	}
	defer nc.Close()
	if err := nc.FlushTimeout(timeout); err != nil {
		return fmt.Errorf("nats probe flush: %w", err)
	}
	if !nc.IsConnected() {
		return fmt.Errorf("nats probe: not connected")
	}
	return nil
}
