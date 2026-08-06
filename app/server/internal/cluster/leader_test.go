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

func newTestJetStream(t *testing.T) nats.JetStreamContext {
	t.Helper()
	ns, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new nats server: %v", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		t.Fatal("nats server not ready")
	}
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(func() {
		nc.Close()
		ns.Shutdown()
	})
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	return js
}

func TestLeaderLock_ReleaseAllowsReacquire(t *testing.T) {
	js := newTestJetStream(t)
	lock, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("OpenLeaderLock: %v", err)
	}
	ok, err := lock.TryAcquire(context.Background(), "node-a")
	if err != nil || !ok {
		t.Fatalf("TryAcquire node-a: ok=%v err=%v", ok, err)
	}
	if err := lock.Release("node-a"); err != nil {
		t.Fatalf("Release node-a: %v", err)
	}
	ok, err = lock.TryAcquire(context.Background(), "node-b")
	if err != nil {
		t.Fatalf("TryAcquire node-b: %v", err)
	}
	if !ok {
		t.Fatal("node-b must acquire after release")
	}
	// node-b 已持有锁，node-a 重新抢占应失败。
	ok, err = lock.TryAcquire(context.Background(), "node-a")
	if err != nil {
		t.Fatalf("TryAcquire node-a after node-b: %v", err)
	}
	if ok {
		t.Fatal("node-a must not reacquire while node-b holds")
	}
}

func TestLeaderLock_RenewKeepsLock(t *testing.T) {
	js := newTestJetStream(t)
	lock, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("OpenLeaderLock: %v", err)
	}
	ok, err := lock.TryAcquire(context.Background(), "node-a")
	if err != nil || !ok {
		t.Fatalf("TryAcquire: ok=%v err=%v", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done, _ := lock.RenewLoop(ctx, "node-a", 100*time.Millisecond)
	defer func() { cancel(); <-done }()

	time.Sleep(300 * time.Millisecond)
	ok2, err := lock.TryAcquire(context.Background(), "node-b")
	if err != nil {
		t.Fatalf("TryAcquire node-b: %v", err)
	}
	if ok2 {
		t.Fatal("node-b must not acquire while node-a renews")
	}
}

func TestLeaderLock_RenewReportsLost(t *testing.T) {
	js := newTestJetStream(t)
	lock, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("OpenLeaderLock: %v", err)
	}
	ok, err := lock.TryAcquire(context.Background(), "node-a")
	if err != nil || !ok {
		t.Fatalf("TryAcquire node-a: ok=%v err=%v", ok, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done, _ := lock.RenewLoop(ctx, "node-a", 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	// 等 TTL 过期后让 node-b 抢占，再恢复 node-a 的续租循环检测丢失。
	lockB, err := OpenLeaderLock(js, "test")
	if err != nil {
		t.Fatalf("OpenLeaderLock B: %v", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		okB, err := lockB.TryAcquire(context.Background(), "node-b")
		if err != nil {
			t.Fatalf("TryAcquire node-b: %v", err)
		}
		if okB {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("node-b never acquired after TTL")
		}
		time.Sleep(200 * time.Millisecond)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	done2, lost := lock.RenewLoop(ctx2, "node-a", 50*time.Millisecond)
	defer func() { cancel2(); <-done2 }()
	select {
	case <-lost:
	case <-time.After(3 * time.Second):
		t.Fatal("lost channel not reported after lock was taken over")
	}
}
