package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartMuteExpiryScanner_TriggersScan(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartMuteExpiryScanner(ctx, func() { calls.Add(1) }, 20*time.Millisecond)
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected scanner to invoke scan at least twice within 2s, got %d calls", calls.Load())
}

func TestStartMuteExpiryScanner_StopStopsScanning(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := StartMuteExpiryScanner(ctx, func() { calls.Add(1) }, 10*time.Millisecond)

	waitForCalls(t, &calls, 2)
	stop()
	stop() // stop 必须幂等，重复调用不得 panic
	atStop := calls.Load()
	time.Sleep(80 * time.Millisecond)
	if got := calls.Load(); got > atStop+1 {
		t.Fatalf("scanner kept scanning after stop: before=%d after=%d", atStop, got)
	}
}

func TestStartMuteExpiryScanner_CtxCancelStopsScanning(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	stop := StartMuteExpiryScanner(ctx, func() { calls.Add(1) }, 10*time.Millisecond)
	defer stop()

	waitForCalls(t, &calls, 2)
	cancel()
	atCancel := calls.Load()
	time.Sleep(80 * time.Millisecond)
	if got := calls.Load(); got > atCancel+1 {
		t.Fatalf("scanner kept scanning after ctx cancel: before=%d after=%d", atCancel, got)
	}
}

func waitForCalls(t *testing.T, calls *atomic.Int32, min int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= min {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected at least %d calls, got %d", min, calls.Load())
}
