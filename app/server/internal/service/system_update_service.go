package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"GOSpeak/internal/buildinfo"
	"GOSpeak/internal/pkg"
)

// GitHub 上主体服务端的发布来源（与桌面端 desktop-v* 发布链相互独立）。
const (
	githubReleasesAPI = "https://api.github.com/repos/NoelOrin/GOSpeak/releases?per_page=30"
	githubReleasePage = "https://github.com/NoelOrin/GOSpeak/releases"
	// maxNotesLen 按 rune 截断过长的 release notes，避免响应膨胀且不切断 UTF-8。
	maxNotesLen = 2000
	// defaultNegativeCacheTTL 是上游限流（403/429）后的短负缓存时长，
	// 避免管理员每点一次就再打一次已经耗尽配额的 GitHub API。
	defaultNegativeCacheTTL = time.Minute
)

// mainTagPattern 匹配主体服务端发布 tag（v1.2.3），排除桌面端 desktop-v*。
var mainTagPattern = regexp.MustCompile(`^v[0-9]`)

// githubRelease 是 GitHub Releases API 返回条目中本服务关心的字段。
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// ServerUpdateInfo 服务端更新检查结果（仅 admin 可见的 /system/update 端点）。
type ServerUpdateInfo struct {
	// Current 运行中服务端的版本（构建注入，dev 为源码构建）。
	Current string `json:"current"`
	// Latest 最新发布的主体版本号；检查失败时为 ""。
	Latest string `json:"latest"`
	// Available 最新版本比当前版本新。
	Available bool `json:"available"`
	// Notes 最新版本的发布说明（Markdown，超长截断）。
	Notes string `json:"notes,omitempty"`
	// URL 最新版本的发布页链接（available=false 时是 releases 列表页）。
	URL string `json:"url,omitempty"`
}

// SystemUpdateService 检查主体服务端是否有新版本。
//
// 独立于桌面端更新体系：桌面端由 MyGo 内建签名更新模块走 desktop-v* 发布链，
// 这里只读主体 v* 发布的元信息，供管理员在 Web 控制台比对版本与查看说明。
// 成功结果缓存 cacheTTL；上游限流（403/429，匿名配额耗尽）另做短负缓存，
// 避免管理员反复点击继续打爆 GitHub API（匿名 60 次/小时/IP）。
type SystemUpdateService struct {
	client   *http.Client
	apiURL   string
	cacheTTL time.Duration
	negTTL   time.Duration

	mu       sync.Mutex
	cached   *ServerUpdateInfo
	cachedAt time.Time
	negUntil time.Time
}

func NewSystemUpdateService() *SystemUpdateService {
	return &SystemUpdateService{
		client:   &http.Client{Timeout: 10 * time.Second},
		apiURL:   githubReleasesAPI,
		cacheTTL: 5 * time.Minute,
		negTTL:   defaultNegativeCacheTTL,
	}
}

// negativeCacheTTL 返回生效的负缓存时长。
func (s *SystemUpdateService) negativeCacheTTL() time.Duration {
	if s.negTTL > 0 {
		return s.negTTL
	}
	return defaultNegativeCacheTTL
}

// Check 拉取最新发布并与当前版本比较。网络失败返回 AppError（5001/1017）。
func (s *SystemUpdateService) Check(ctx context.Context) (*ServerUpdateInfo, error) {
	s.mu.Lock()
	if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
		info := s.cached
		s.mu.Unlock()
		return info, nil
	}
	if time.Now().Before(s.negUntil) {
		s.mu.Unlock()
		return nil, pkg.NewAppError(pkg.RATE_LIMITED)
	}
	s.mu.Unlock()

	release, err := s.fetchLatest(ctx)
	if err != nil {
		if pkg.IsCode(err, pkg.RATE_LIMITED) {
			s.mu.Lock()
			s.negUntil = time.Now().Add(s.negativeCacheTTL())
			s.mu.Unlock()
		}
		return nil, err
	}

	info := &ServerUpdateInfo{Current: buildinfo.Version}
	if release != nil {
		info.Latest = strings.TrimPrefix(release.TagName, "v")
		info.Notes = truncateNotes(release.Body)
		info.URL = release.HTMLURL
		info.Available = compareVersion(info.Latest, info.Current) > 0
	}
	if info.URL == "" {
		info.URL = githubReleasePage
	}

	s.mu.Lock()
	s.cached = info
	s.cachedAt = time.Now()
	s.negUntil = time.Time{}
	s.mu.Unlock()
	return info, nil
}

// fetchLatest 返回最新的主体 v* 发布；没有发布时返回 nil（不视为错误）。
func (s *SystemUpdateService) fetchLatest(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "check update failed")
	}
	// GitHub API 要求显式 Accept；匿名请求也带 UA 便于配额归因。
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "GOSpeak-Server")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "check update failed")
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden:
		return nil, pkg.NewAppErrorWithCause(pkg.RATE_LIMITED,
			fmt.Errorf("github api status %d", resp.StatusCode))
	default:
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR,
			fmt.Errorf("github api status %d", resp.StatusCode), "check update failed")
	}

	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, pkg.NewAppErrorWithCause(pkg.INTERNAL_ERROR, err, "check update failed")
	}

	var latest *githubRelease
	for i := range releases {
		r := &releases[i]
		if r.Draft || !mainTagPattern.MatchString(r.TagName) {
			continue
		}
		// 稳定通道：带预发布段的版本（v2.0.0-rc.1）不作为"最新版"推荐。
		if _, pre, ok := parseVersion(strings.TrimPrefix(r.TagName, "v")); !ok || pre != "" {
			continue
		}
		if latest == nil || compareVersion(strings.TrimPrefix(r.TagName, "v"), strings.TrimPrefix(latest.TagName, "v")) > 0 {
			latest = r
		}
	}
	return latest, nil
}

// truncateNotes 按 rune 截断，避免切断 UTF-8 多字节字符产生非法文本。
func truncateNotes(notes string) string {
	notes = strings.TrimSpace(notes)
	runes := []rune(notes)
	if len(runes) <= maxNotesLen {
		return notes
	}
	return string(runes[:maxNotesLen]) + "\n\n…（发布说明过长已截断，完整内容见发布页）"
}

// compareVersion 比较「去 v 前缀」的点分版本号（1.2.3 与 1.2.3-beta.1）。
// 返回 -1/0/1；任一版本无法解析时返回 0（语义上视为"不可比"），由调用方兜底。
func compareVersion(a, b string) int {
	na, pa, okA := parseVersion(a)
	nb, pb, okB := parseVersion(b)
	if !okA || !okB {
		return 0
	}
	for i := range na {
		if na[i] != nb[i] {
			if na[i] > nb[i] {
				return 1
			}
			return -1
		}
	}
	// 数字段相同：无预发布段 > 有预发布段（1.2.3 > 1.2.3-rc.1）。
	if pa == pb {
		return 0
	}
	if pa == "" {
		return 1
	}
	if pb == "" {
		return -1
	}
	// 预发布段按点分标识符比较：数字标识符按数值，其余按字典序。
	return comparePrerelease(pa, pb)
}

// parseVersion 解析 "1.2.3-beta.1" 为数字段与预发布段。
// 宽松处理前导 v 与多余位数；主段不足三位视为解析失败。
func parseVersion(v string) ([3]int, string, bool) {
	var nums [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nums, "", false
	}
	core, pre, _ := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) < 3 {
		return nums, "", false
	}
	for i := range nums {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return nums, "", false
		}
		nums[i] = n
	}
	return nums, pre, true
}

// comparePrerelease 按 SemVer 规则比较预发布段标识符。
func comparePrerelease(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
		case aerr == nil: // 数字标识符 < 字母标识符
			return -1
		case berr == nil:
			return 1
		default:
			if as[i] != bs[i] {
				if as[i] > bs[i] {
					return 1
				}
				return -1
			}
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}
