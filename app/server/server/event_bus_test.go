package server

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"GOSpeak/internal/bus"
	"GOSpeak/internal/config"
)

type nopDeliverer struct{}

func (nopDeliverer) BroadcastToNamespace(string, interface{})    {}
func (nopDeliverer) BroadcastToRoom(string, string, interface{}) {}

func TestEventBusInitConfigWiresStoreDirFromDBPath(t *testing.T) {
	cfg := &config.Config{DBPath: filepath.Join("db", "app.db")}
	initCfg, err := eventBusInitConfig(cfg, nopDeliverer{})
	if err != nil {
		t.Fatalf("eventBusInitConfig: %v", err)
	}
	want := cfg.NATStoreDirPath()
	if want != filepath.Join("db", "nats") {
		t.Fatalf("precondition: NATStoreDirPath() = %q, want db/nats", want)
	}
	if initCfg.StoreDir != want {
		t.Fatalf("StoreDir = %q, want %q", initCfg.StoreDir, want)
	}
	if initCfg.WALPath != cfg.NATSWALPath() {
		t.Fatalf("WALPath = %q, want %q", initCfg.WALPath, cfg.NATSWALPath())
	}
}

func TestEventBusInitConfigHonorsExplicitStoreDir(t *testing.T) {
	cfg := &config.Config{DBPath: "db/app.db", NATSStoreDir: "  /data/nats-js  "}
	initCfg, err := eventBusInitConfig(cfg, nopDeliverer{})
	if err != nil {
		t.Fatalf("eventBusInitConfig: %v", err)
	}
	if initCfg.StoreDir != "/data/nats-js" {
		t.Fatalf("StoreDir = %q, want /data/nats-js", initCfg.StoreDir)
	}
}

func TestEventBusInitConfigRejectsInvalidEmbeddedPort(t *testing.T) {
	cfg := &config.Config{NATSEmbeddedPort: "not-a-port"}
	if _, err := eventBusInitConfig(cfg, nopDeliverer{}); err == nil {
		t.Fatal("expected error for invalid NATS_EMBEDDED_PORT")
	}
}

// 回归 gin.go 调用点：StoreDir 必须真正传进 bus.Init，否则内嵌 NATS 退回
// MkdirTemp 临时目录（日志 storeDir=.../gospeak-nats-js-*），auth 状态随重启丢失。
func TestEventBusInitConfigStoreDirReachesEmbeddedServer(t *testing.T) {
	root := t.TempDir()
	storeDir := filepath.Join(root, "nats")
	cfg := &config.Config{DBPath: filepath.Join(root, "app.db"), NATSStoreDir: storeDir}

	initCfg, err := eventBusInitConfig(cfg, nopDeliverer{})
	if err != nil {
		t.Fatalf("eventBusInitConfig: %v", err)
	}

	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	eventBus, cleanup, err := bus.Init(initCfg)
	log.SetOutput(prev)
	if err != nil {
		t.Fatalf("bus.Init: %v", err)
	}
	cleanup()

	got := logs.String()
	if !strings.Contains(got, "storeDir="+storeDir) {
		t.Fatalf("embedded nats did not use persistent store dir %q; logs:\n%s", storeDir, got)
	}
	if strings.Contains(got, "gospeak-nats-js-") {
		t.Fatalf("embedded nats fell back to temp store dir; logs:\n%s", got)
	}
	if _, err := os.Stat(storeDir); err != nil {
		t.Fatalf("persistent store dir %q must survive shutdown: %v", storeDir, err)
	}
	if eventBus == nil {
		t.Fatal("nil event bus")
	}
}
