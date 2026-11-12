package bus

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartEmbeddedServer_ClientCanConnect(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatalf("StartEmbeddedServer: %v", err)
	}
	defer es.Shutdown()

	url := es.ClientURL()
	if url == "" {
		t.Fatal("empty ClientURL")
	}
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect embedded: %v", err)
	}
	defer nc.Close()
	if !nc.IsConnected() {
		t.Fatal("expected connected")
	}
}

func TestStartEmbeddedServer_JetStreamAvailable(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatalf("StartEmbeddedServer: %v", err)
	}
	defer es.Shutdown()

	nc, err := nats.Connect(es.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream context: %v", err)
	}
	info, err := js.AccountInfo()
	if err != nil {
		t.Fatalf("AccountInfo: %v", err)
	}
	if info == nil {
		t.Fatal("AccountInfo returned nil")
	}
}

func TestStartEmbeddedServerOnPort_Fixed(t *testing.T) {
	es, err := StartEmbeddedServerOnPort(0)
	if err != nil {
		t.Fatalf("bootstrap free port server: %v", err)
	}
	// Borrow a free port by starting random, then rebind via second start after shutdown
	// using OS-assigned free port discovery from first ClientURL is flaky; instead use port 0
	// and validate fixed path with a known free TCP port.
	es.Shutdown()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	es, err = StartEmbeddedServerOnPort(port)
	if err != nil {
		t.Fatalf("StartEmbeddedServerOnPort(%d): %v", port, err)
	}
	defer es.Shutdown()

	want := fmt.Sprintf("nats://127.0.0.1:%d", port)
	if es.ClientURL() != want {
		t.Fatalf("ClientURL = %q, want %q", es.ClientURL(), want)
	}
	nc, err := nats.Connect(es.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect fixed port: %v", err)
	}
	defer nc.Close()
}

func TestStartEmbeddedServerWithStore_PersistsAcrossRestart(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "nats-js")
	const bucket = "auth"
	const key = "jwt.active"
	const want = "a-persisted-signing-key"

	// 第一次启动：建 bucket 写入一个模拟签名密钥，然后正常 Shutdown。
	es1, err := StartEmbeddedServerWithStore(0, storeDir)
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	if got := es1.StoreDir(); got != storeDir {
		t.Fatalf("StoreDir = %q, want %q", got, storeDir)
	}
	writeKV(t, es1.ClientURL(), bucket, key, want)
	es1.Shutdown()

	// 关键保证 1：持久化目录在 Shutdown 后必须还在（临时目录才会被删）。
	if _, err := os.Stat(storeDir); err != nil {
		t.Fatalf("persistent store dir was removed on shutdown: %v", err)
	}

	// 关键保证 2：换一个随机端口重启，同一目录的 KV 必须能读回旧值。
	es2, err := StartEmbeddedServerWithStore(0, storeDir)
	if err != nil {
		t.Fatalf("restart with same store dir: %v", err)
	}
	defer es2.Shutdown()
	if got := readKV(t, es2.ClientURL(), bucket, key); got != want {
		t.Fatalf("KV after restart = %q, want %q", got, want)
	}
}

func TestStartEmbeddedServer_TempDirRemovedOnShutdown(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatalf("StartEmbeddedServer: %v", err)
	}
	dir := es.StoreDir()
	if dir == "" {
		t.Fatal("expected a temp store dir")
	}
	es.Shutdown()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temp dir should be removed on shutdown, stat err = %v", err)
	}
}

func writeKV(t *testing.T, url, bucket, key, value string) {
	t.Helper()
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream: %v", err)
	}
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: bucket})
	if err != nil {
		t.Fatalf("CreateKeyValue: %v", err)
	}
	if _, err := kv.Put(key, []byte(value)); err != nil {
		t.Fatalf("kv.Put: %v", err)
	}
}

func readKV(t *testing.T, url, bucket, key string) string {
	t.Helper()
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream: %v", err)
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		t.Fatalf("KeyValue(%q): %v", bucket, err)
	}
	entry, err := kv.Get(key)
	if err != nil {
		t.Fatalf("kv.Get(%q): %v", key, err)
	}
	return string(entry.Value())
}

func TestInit_EmbeddedPortFixed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	b, cleanup, err := Init(InitConfig{
		Prefix:       "gospeak",
		EmbeddedPort: port,
		Deliverer:    nopDeliverer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b.Mode() != "embedded" {
		t.Fatalf("mode = %q, want embedded", b.Mode())
	}
	nb, ok := b.(*NATSBus)
	if !ok {
		t.Fatal("expected *NATSBus")
	}
	want := fmt.Sprintf("nats://127.0.0.1:%d", port)
	if nb.Conn() == nil || nb.Conn().ConnectedUrl() != want {
		got := ""
		if nb.Conn() != nil {
			got = nb.Conn().ConnectedUrl()
		}
		t.Fatalf("ConnectedUrl = %q, want %q", got, want)
	}
}
