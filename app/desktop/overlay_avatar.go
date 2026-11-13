package main

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	avatarCacheLimit = 64
	avatarFetchLimit = 512 * 1024
)

// avatarCache keeps the decoded avatars the HUD shows, by URL. A failed load is
// remembered too, so a broken picture is not requested again on every state
// relay.
type avatarCache struct {
	mu         sync.Mutex
	images     map[string]*ui.Bitmap
	failed     map[string]bool
	loading    map[string]bool
	generation uint64
	client     *http.Client
	// wg tracks in-flight fetches so tests can wait for a generation-scoped
	// result to settle instead of racing the goroutine.
	wg sync.WaitGroup
}

func newAvatarCache() *avatarCache {
	return &avatarCache{
		images:  make(map[string]*ui.Bitmap),
		failed:  make(map[string]bool),
		loading: make(map[string]bool),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// overlayAvatars is the cache the HUD reads. A server switch clears it: its
// URLs point at the previous server.
var overlayAvatars = newAvatarCache()

// resolveAvatarURL turns a member's avatar into a URL the host can fetch.
// Relative paths go through the same origin the page uses — the loopback proxy
// in release, the dev server in dev mode — which forwards /uploads to the
// backend. Absolute URLs (S3, R2) are used as they are.
func resolveAvatarURL(base, avatar string) string {
	avatar = strings.TrimSpace(avatar)
	if avatar == "" {
		return ""
	}
	if strings.HasPrefix(avatar, "http://") || strings.HasPrefix(avatar, "https://") {
		return avatar
	}
	base = strings.TrimRight(base, "/")
	if strings.HasPrefix(avatar, "/") {
		return base + avatar
	}
	return base + "/" + avatar
}

func (c *avatarCache) lookup(url string) *ui.Bitmap {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.images[url]
}

// wait blocks until every fetch started before it returns has settled. Tests
// and shutdown use it; the HUD never calls it (fetches stay non-blocking).
func (c *avatarCache) wait() { c.wg.Wait() }

// clear forgets every avatar, as a switch of the server they came from. The
// generation bump makes in-flight fetches from the old server discard their
// result instead of writing a stale bitmap into the fresh cache.
func (c *avatarCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.images = make(map[string]*ui.Bitmap)
	c.failed = make(map[string]bool)
	c.loading = make(map[string]bool)
}

// get starts loading url unless it is cached, failed or already loading, and
// calls onLoaded once a bitmap arrives. It never blocks: the fetch runs in a
// goroutine and the caller redraws when it lands.
func (c *avatarCache) get(url string, onLoaded func()) {
	if url == "" {
		return
	}
	c.mu.Lock()
	if _, ok := c.images[url]; ok {
		c.mu.Unlock()
		return
	}
	if c.failed[url] || c.loading[url] {
		c.mu.Unlock()
		return
	}
	c.loading[url] = true
	generation := c.generation
	c.wg.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.wg.Done()
		bmp := c.fetch(url)
		c.mu.Lock()
		if generation != c.generation {
			c.mu.Unlock()
			return // server switched while this fetch was in flight
		}
		delete(c.loading, url)
		if bmp == nil {
			c.failed[url] = true
			c.mu.Unlock()
			return
		}
		if len(c.images) >= avatarCacheLimit {
			c.images = make(map[string]*ui.Bitmap)
		}
		c.images[url] = bmp
		c.mu.Unlock()
		if onLoaded != nil {
			onLoaded()
		}
	}()
}

func (c *avatarCache) fetch(url string) *ui.Bitmap {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, avatarFetchLimit))
	if err != nil {
		return nil
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bmp
}
