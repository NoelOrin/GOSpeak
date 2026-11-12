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
	storeDir string // JetStream StoreDir
	tempDir  bool   // true = MkdirTemp 临时目录，Shutdown 时清理；false = 持久化目录，Shutdown 保留
}

// StartEmbeddedServer 启动仅监听本机的内嵌 NATS（含 JetStream），使用临时 StoreDir。
// port 固定为随机端口，避免与宿主机/其他副本抢 4222。
func StartEmbeddedServer() (*EmbeddedServer, error) {
	return StartEmbeddedServerOnPort(0)
}

// StartEmbeddedServerOnPort 按指定端口启动内嵌 NATS，使用临时 StoreDir。
// port <= 0 表示随机端口。Shutdown 会清理临时目录（保持既有测试/离线行为）。
func StartEmbeddedServerOnPort(port int) (*EmbeddedServer, error) {
	return StartEmbeddedServerWithStore(port, "")
}

// StartEmbeddedServerWithStore 按指定端口启动内嵌 NATS，并指定 JetStream StoreDir。
//
// storeDir 非空：使用固定持久化目录（自动 MkdirAll 0700），Shutdown **不会删除**，
// 下次启动同一目录会恢复 KV（JWT 签名密钥 / 黑名单 / refresh-family 等），
// 这正是让 auth 状态跨重启存活的关键。
// storeDir 为空：退回 MkdirTemp 临时目录，Shutdown 清理（兼容测试与未配置场景）。
//
// 单实例语义：固定 StoreDir 是单机部署模型，同一目录被两个进程同时打开会因
// 文件锁启动失败（fail-fast）。多副本共享状态应接外部 NATS，而不是共用本地目录。
func StartEmbeddedServerWithStore(port int, storeDir string) (*EmbeddedServer, error) {
	temp := storeDir == ""
	if temp {
		dir, err := os.MkdirTemp("", "gospeak-nats-js-*")
		if err != nil {
			return nil, fmt.Errorf("nats embedded: temp store dir: %w", err)
		}
		storeDir = dir
	} else {
		if err := os.MkdirAll(storeDir, 0o700); err != nil {
			return nil, fmt.Errorf("nats embedded: create store dir %q: %w", storeDir, err)
		}
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
		StoreDir:   storeDir,
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		if temp {
			_ = os.RemoveAll(storeDir)
		}
		return nil, fmt.Errorf("nats embedded: new server: %w", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		if temp {
			_ = os.RemoveAll(storeDir)
		}
		if port > 0 {
			return nil, fmt.Errorf("nats embedded: not ready for connections on 127.0.0.1:%d", port)
		}
		return nil, fmt.Errorf("nats embedded: not ready for connections")
	}
	return &EmbeddedServer{ns: ns, storeDir: storeDir, tempDir: temp}, nil
}

// StoreDir 返回 JetStream 数据目录；空表示未启动或临时目录已清理。供监控/测试读取。
func (e *EmbeddedServer) StoreDir() string {
	if e == nil {
		return ""
	}
	return e.storeDir
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
	// 只清理临时目录；持久化目录必须保留，否则重启又丢 KV。
	if e.tempDir && e.storeDir != "" {
		_ = os.RemoveAll(e.storeDir)
	}
	e.storeDir = ""
}
