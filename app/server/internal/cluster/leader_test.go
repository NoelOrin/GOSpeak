package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestLeaderLockTryAcquire(t *testing.T) {
	dir := t.TempDir()
	ns, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  dir,
	})
	if err != nil {
		t.Fatalf("new nats server: %v", err)
	}
	ns.Start()
	defer ns.Shutdown()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	lock1, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("open leader lock 1: %v", err)
	}
	ok, err := lock1.TryAcquire(context.Background(), "agent-a")
	if err != nil {
		t.Fatalf("acquire leader lock: %v", err)
	}
	if !ok {
		t.Fatal("expected first agent to acquire leader lock")
	}

	lock2, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("open leader lock 2: %v", err)
	}
	ok, err = lock2.TryAcquire(context.Background(), "agent-b")
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if ok {
		t.Fatal("expected second agent to be rejected")
	}
}
