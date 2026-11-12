package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestAvatarCacheLoadsAndCaches(t *testing.T) {
	body := pngBytes(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := newAvatarCache()
	url := srv.URL + "/a.png"

	done := make(chan struct{})
	c.get("u1", url, func() { close(done) })
	<-done

	if hits.Load() != 1 {
		t.Fatalf("server hits = %d, want 1", hits.Load())
	}
	if bmp := c.lookup(url); bmp == nil {
		t.Fatal("avatar was not cached after a successful load")
	}

	c.get("u1", url, func() {})
	if hits.Load() != 1 {
		t.Fatalf("server hits after cache hit = %d, want 1", hits.Load())
	}
}

func TestAvatarCacheFailureIsCached(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newAvatarCache()
	url := srv.URL + "/missing.png"

	// A failed load calls no onLoaded, so wait for the failure to be recorded.
	c.get("u1", url, func() {})
	deadline := time.Now().Add(3 * time.Second)
	for {
		c.mu.Lock()
		failed := c.failed[url]
		c.mu.Unlock()
		if failed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a failed avatar was never recorded as failed")
		}
		time.Sleep(10 * time.Millisecond)
	}

	c.get("u1", url, func() {})
	if hits.Load() != 1 {
		t.Fatalf("server hits for a failed avatar = %d, want 1 (no retry)", hits.Load())
	}
	if bmp := c.lookup(url); bmp != nil {
		t.Fatal("a failed avatar was cached as a bitmap")
	}
}

func TestAvatarCacheEmptyURLIsSkipped(t *testing.T) {
	c := newAvatarCache()
	called := false
	c.get("u1", "", func() { called = true })
	if called {
		t.Fatal("an empty avatar URL triggered a load")
	}
}

func TestAvatarURLResolution(t *testing.T) {
	base := "http://127.0.0.1:1234"
	if got := resolveAvatarURL(base, "/uploads/a.png"); got != base+"/uploads/a.png" {
		t.Errorf("resolveAvatarURL relative = %q", got)
	}
	if got := resolveAvatarURL(base, "https://cdn.example.com/a.png"); got != "https://cdn.example.com/a.png" {
		t.Errorf("resolveAvatarURL absolute = %q", got)
	}
	if got := resolveAvatarURL(base, ""); got != "" {
		t.Errorf("resolveAvatarURL empty = %q, want empty", got)
	}
}

func TestAvatarCacheClear(t *testing.T) {
	c := newAvatarCache()
	c.failed["http://x/a.png"] = true
	c.clear()
	if len(c.failed) != 0 {
		t.Fatal("clear() did not empty the cache")
	}
}
