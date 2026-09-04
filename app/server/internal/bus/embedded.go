package bus

import (
	"fmt"
	"os"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// EmbeddedServer 进程内 nats-server 句柄。
type EmbeddedServer struct {
	ns       *natsserver.Server
	storeDir string // JetStream StoreDir，Shutdown 时清理
}

// StartEmbeddedServer 启动仅监听本机的内嵌 NATS（含 JetStream）。
// port <= 0 时使用随机端口（Port=-1），避免与宿主机/其他副本抢 4222。
// port > 0 时固定监听 127.0.0.1:port。
func StartEmbeddedServer() (*EmbeddedServer, error) {
	return StartEmbeddedServerOnPort(0)
}

// StartEmbeddedServerOnPort 按指定端口启动内嵌 NATS。
// port <= 0 表示随机端口。
func StartEmbeddedServerOnPort(port int) (*EmbeddedServer, error) {
	dir, err := os.MkdirTemp("", "gospeak-nats-js-*")
	if err != nil {
		return nil, fmt.Errorf("nats embedded: temp store dir: %w", err)
	}

	listenPort := port
	if listenPort <= 0 {
		listenPort = -1
	}

	opts := &natsserver.Options{
		Host:       "127.0.0.1",
		Port:       listenPort,
		NoLog:      true,
		NoSigs:     true,
		MaxPayload: 1 << 20, // 1MiB，信令 JSON 足够
		JetStream:  true,
		StoreDir:   dir,
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("nats embedded: new server: %w", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		_ = os.RemoveAll(dir)
		if port > 0 {
			return nil, fmt.Errorf("nats embedded: not ready for connections on 127.0.0.1:%d", port)
		}
		return nil, fmt.Errorf("nats embedded: not ready for connections")
	}
	return &EmbeddedServer{ns: ns, storeDir: dir}, nil
}

func (e *EmbeddedServer) ClientURL() string {
	if e == nil || e.ns == nil {
		return ""
	}
	return e.ns.ClientURL()
}

func (e *EmbeddedServer) Shutdown() {
	if e == nil {
		return
	}
	if e.ns != nil {
		e.ns.Shutdown()
		e.ns.WaitForShutdown()
	}
	if e.storeDir != "" {
		_ = os.RemoveAll(e.storeDir)
		e.storeDir = ""
	}
}
