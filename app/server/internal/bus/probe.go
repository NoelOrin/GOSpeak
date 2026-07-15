package bus

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// ProbeExternal 连接外部 NATS 验证其可用性。
// 返回 nil 表示外部 NATS 可达。
func ProbeExternal(url string) error {
	nc, err := nats.Connect(url, nats.Timeout(3*time.Second))
	if err != nil {
		return fmt.Errorf("probe external nats: %w", err)
	}
	nc.Close()
	return nil
}
