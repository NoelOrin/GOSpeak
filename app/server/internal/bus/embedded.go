package bus

import (
	"fmt"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// EmbeddedServer 进程内 nats-server 句柄。
type EmbeddedServer struct {
	ns *natsserver.Server
}

// StartEmbeddedServer 启动仅监听本机随机端口的内嵌 NATS。
// Port=-1 避免与宿主机/其他副本抢 4222。
func StartEmbeddedServer() (*EmbeddedServer, error) {
	opts := &natsserver.Options{
		Host:        "127.0.0.1",
		Port:        -1,
		NoLog:       true,
		NoSigs:      true,
		MaxPayload:  1 << 20, // 1MiB，信令 JSON 足够
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("nats embedded: new server: %w", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		return nil, fmt.Errorf("nats embedded: not ready for connections")
	}
	return &EmbeddedServer{ns: ns}, nil
}

func (e *EmbeddedServer) ClientURL() string {
	if e == nil || e.ns == nil {
		return ""
	}
	return e.ns.ClientURL()
}

func (e *EmbeddedServer) Shutdown() {
	if e == nil || e.ns == nil {
		return
	}
	e.ns.Shutdown()
	e.ns.WaitForShutdown()
}
