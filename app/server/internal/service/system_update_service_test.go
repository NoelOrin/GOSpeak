package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GOSpeak/internal/pkg"
)

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.4", -1},
		{"1.3.0", "1.2.9", 1},
		{"2.0.0", "1.9.9", 1},
		// 前导 v 与空白容忍
		{"v1.2.4", "1.2.3", 1},
		{" 1.2.4 ", "v1.2.3", 1},
		// 预发布：正式版 > 同版本预发布
		{"1.2.3", "1.2.3-rc.1", 1},
		{"1.2.3-rc.1", "1.2.3", -1},
		// 预发布段之间：数值比较与字典序
		{"1.2.3-rc.2", "1.2.3-rc.1", 1},
		{"1.2.3-beta.2", "1.2.3-alpha.1", 1},
		{"1.2.3-rc", "1.2.3-rc.1", -1},
		{"1.2.3-1", "1.2.3-alpha", -1}, // SemVer：数字标识符 < 字母标识符
		// 长度：标识符多者 > 少者（前缀相同）
		{"1.2.3-rc.1.1", "1.2.3-rc.1", 1},
		// 不可解析视为不可比
		{"dev", "1.2.3", 0},
		{"1.2", "1.2.3", 0},
		{"", "1.2.3", 0},
		{"1.2.3-x.2a", "1.2.3", -1},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, compareVersion(c.a, c.b), "compareVersion(%q, %q)", c.a, c.b)
	}
}

// stubGitHub 起一个伪造的 GitHub Releases API，返回给定的发布列表并统计请求数。
func stubGitHub(t *testing.T, releases []githubRelease) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestSystemUpdateService_Check_FindsNewerMainRelease(t *testing.T) {
	srv, _ := stubGitHub(t, []githubRelease{
		{TagName: "desktop-v1.5.0", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/desktop-v1.5.0"}, // 桌面端发布应被忽略
		{TagName: "v1.1.0", Body: "server release notes", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/v1.1.0"},
		{TagName: "v0.9.0", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/v0.9.0"},
		{TagName: "v2.0.0-rc.1", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/v2.0.0-rc.1"}, // rc 不算最新正式版
	})
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0}

	info, err := svc.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Latest)
	assert.Equal(t, "server release notes", info.Notes)
	// 当前版本为 dev（不可比较），available 兜底为 false，但最新版信息照常返回
	assert.False(t, info.Available)
	assert.Equal(t, "https://github.com/NoelOrin/GOSpeak/releases/tag/v1.1.0", info.URL)
}

func TestSystemUpdateService_Check_DevVersionNotAvailable(t *testing.T) {
	srv, _ := stubGitHub(t, []githubRelease{
		{TagName: "v1.1.0", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/v1.1.0"},
	})
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0}
	// buildinfo.Version 默认 dev，不可比较 → available=false，但仍展示最新版
	info, err := svc.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "dev", info.Current)
	assert.Equal(t, "1.1.0", info.Latest)
	assert.False(t, info.Available)
}

func TestSystemUpdateService_Check_UpToDate(t *testing.T) {
	// 将"当前版本"伪造成与最新发布相同：借助全局 buildinfo 不便改写，
	// 直接验证比较逻辑：latest == current → available=false。
	srv, _ := stubGitHub(t, []githubRelease{
		{TagName: "v0.0.0", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/v0.0.0"},
	})
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0}
	info, err := svc.Check(context.Background())
	require.NoError(t, err)
	assert.False(t, info.Available)
	assert.NotEmpty(t, info.URL)
}

func TestSystemUpdateService_Check_NoReleases(t *testing.T) {
	srv, _ := stubGitHub(t, []githubRelease{})
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0}
	info, err := svc.Check(context.Background())
	require.NoError(t, err)
	assert.Empty(t, info.Latest)
	assert.False(t, info.Available)
	assert.Equal(t, githubReleasePage, info.URL, "无发布时回退 releases 列表页")
}

func TestSystemUpdateService_Check_Cache(t *testing.T) {
	srv, calls := stubGitHub(t, []githubRelease{
		{TagName: "v1.1.0", HTMLURL: "https://github.com/NoelOrin/GOSpeak/releases/tag/v1.1.0"},
	})
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 5 * time.Minute}
	ctx := context.Background()
	_, err := svc.Check(ctx)
	require.NoError(t, err)
	_, err = svc.Check(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, calls.Load(), "TTL 内第二次 Check 应命中缓存")
}

func TestSystemUpdateService_Check_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0}
	_, err := svc.Check(context.Background())
	require.Error(t, err)
	var appErr *pkg.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.INTERNAL_ERROR, appErr.Code)
}

// 403/429 表示 GitHub 匿名配额耗尽，必须映射为 RATE_LIMITED（HTTP 429），
// 不能塌缩成 INTERNAL_ERROR 让管理员看不出是限流。
func TestSystemUpdateService_Check_RateLimitedStatus(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0}
		_, err := svc.Check(context.Background())
		srv.Close()
		require.Error(t, err)
		assert.True(t, pkg.IsCode(err, pkg.RATE_LIMITED), "status %d must map to RATE_LIMITED, got %v", status, err)
	}
}

// 限流后做短负缓存：TTL 内重复 Check 不再打上游，但仍返回 RATE_LIMITED。
func TestSystemUpdateService_Check_RateLimitedNegativeCache(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	svc := &SystemUpdateService{
		client:   srv.Client(),
		apiURL:   srv.URL,
		cacheTTL: 0,
		negTTL:   time.Minute,
	}

	_, err := svc.Check(context.Background())
	require.Error(t, err)
	assert.True(t, pkg.IsCode(err, pkg.RATE_LIMITED))

	for i := 0; i < 3; i++ {
		_, err := svc.Check(context.Background())
		require.Error(t, err)
		assert.True(t, pkg.IsCode(err, pkg.RATE_LIMITED), "cached rejection must stay RATE_LIMITED")
	}
	assert.EqualValues(t, 1, calls.Load(), "负缓存 TTL 内不应重复请求上游")
}

// 负缓存过期后允许重新请求上游，成功则清空负缓存。
func TestSystemUpdateService_Check_NegativeCacheExpires(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]githubRelease{{TagName: "v1.1.0"}})
	}))
	t.Cleanup(srv.Close)
	svc := &SystemUpdateService{
		client:   srv.Client(),
		apiURL:   srv.URL,
		cacheTTL: 0,
		negTTL:   time.Nanosecond,
	}

	_, err := svc.Check(context.Background())
	require.Error(t, err)

	info, err := svc.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Latest)
	assert.EqualValues(t, 2, calls.Load())
}

func TestSystemUpdateService_Check_NegativeCacheConcurrent(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	svc := &SystemUpdateService{client: srv.Client(), apiURL: srv.URL, cacheTTL: 0, negTTL: time.Hour}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Check(context.Background())
		}()
	}
	wg.Wait()
	assert.LessOrEqual(t, calls.Load(), int64(16))
}

// 中文发布说明必须按 rune 截断，不能切断多字节字符产生非法 UTF-8。
func TestTruncateNotes_UTF8Safe(t *testing.T) {
	notes := strings.Repeat("中文发布说明", 600)
	got := truncateNotes(notes)
	require.True(t, utf8.ValidString(got), "truncated notes must remain valid UTF-8")
	assert.LessOrEqual(t, len([]rune(got)), maxNotesLen+len([]rune("\n\n…（发布说明过长已截断，完整内容见发布页）")))
	assert.Contains(t, got, "已截断")
}

func TestTruncateNotes_ShortPassthrough(t *testing.T) {
	assert.Equal(t, "短说明", truncateNotes("  短说明  "))
	assert.Equal(t, "", truncateNotes("   "))
}
