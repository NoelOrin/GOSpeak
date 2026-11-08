package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestProxyPortRangeIsBelowEphemeralRange(t *testing.T) {
	if proxyPortFirst < 1024 {
		t.Fatalf("proxyPortFirst = %d, want a non-privileged port", proxyPortFirst)
	}
	if proxyPortLast >= 49152 {
		t.Fatalf("proxyPortLast = %d, want below the ephemeral range (49152)", proxyPortLast)
	}
	if proxyPortLast < proxyPortFirst {
		t.Fatalf("proxyPortLast %d < proxyPortFirst %d", proxyPortLast, proxyPortFirst)
	}
}

func TestLoadSavedPortIgnoresMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	if got := loadSavedPort(dir); got != 0 {
		t.Fatalf("loadSavedPort on empty dir = %d, want 0", got)
	}
	if err := os.WriteFile(filepath.Join(dir, proxyPortFileName), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadSavedPort(dir); got != 0 {
		t.Fatalf("loadSavedPort on corrupt file = %d, want 0", got)
	}
	// A port outside the range is treated as unset.
	if err := os.WriteFile(filepath.Join(dir, proxyPortFileName), []byte(`{"port":80}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadSavedPort(dir); got != 0 {
		t.Fatalf("loadSavedPort on out-of-range port = %d, want 0", got)
	}
}

func TestSavePortRoundTrip(t *testing.T) {
	dir := t.TempDir()
	savePort(dir, 47001)
	if got := loadSavedPort(dir); got != 47001 {
		t.Fatalf("loadSavedPort = %d, want 47001", got)
	}
}

func TestListenLoopbackPrefersSavedPort(t *testing.T) {
	dir := t.TempDir()
	want := proxyPortFirst + 7
	savePort(dir, want)

	ln, port, err := listenLoopback(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	if port != want {
		t.Fatalf("port = %d, want saved %d", port, want)
	}
}

func TestListenLoopbackSkipsPortInUse(t *testing.T) {
	dir := t.TempDir()
	// Occupy the first port of the range so the picker has to move on.
	blocked, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(proxyPortFirst))
	if err != nil {
		t.Skipf("cannot occupy %d: %v", proxyPortFirst, err)
	}
	defer func() { _ = blocked.Close() }()

	ln, port, err := listenLoopback(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	if port == proxyPortFirst {
		t.Fatalf("port = %d, want a port other than the occupied %d", port, proxyPortFirst)
	}
	if port < proxyPortFirst || port > proxyPortLast {
		t.Fatalf("port = %d, want within %d-%d", port, proxyPortFirst, proxyPortLast)
	}
	// The chosen port is persisted so the next launch reuses it.
	if got := loadSavedPort(dir); got != port {
		t.Fatalf("loadSavedPort = %d, want the chosen %d", got, port)
	}
}
