package livekitproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// PathPrefix 是应用内同源反代的挂载前缀。
// 浏览器拿到的 LiveKit 地址形如 wss://<域名>/livekit，livekit-client 会在其后
// 拼接 /rtc/v1，因此转发时需要剥掉该前缀。
const PathPrefix = "/livekit"

// Resolver 返回后端访问 LiveKit 的地址（LIVEKIT_HOST，如 ws://livekit:7880）。
// 返回空字符串表示未配置。
type Resolver func() string

type targetKey struct{}

// NewHandler 构造同源反代。目标地址按请求实时解析，便于运行期切换 SFU 配置。
func NewHandler(resolve Resolver) http.Handler {
	h := &handler{resolve: resolve}
	h.proxy = &httputil.ReverseProxy{
		Director: h.direct,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeBadGateway(w, err)
		},
		FlushInterval: -1,
	}
	return h
}

type handler struct {
	resolve Resolver
	proxy   *httputil.ReverseProxy
}

// allowedPrefixes 限制可转发的路径：只放行浏览器信令用到的 /rtc/*，
// 其余（admin twirp、/debug/pprof、/whip 等）一律 404，
// 避免把 LiveKit 的管理面顺带暴露到公网。
var allowedPrefixes = []string{PathPrefix + "/rtc/"}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowedPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	target, err := parseTarget(h.resolve())
	if err != nil {
		writeBadGateway(w, err)
		return
	}
	h.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), targetKey{}, target)))
}

// direct 把请求改写到解析出的 LiveKit 地址，并剥掉 /livekit 前缀。
// WebSocket 升级由 httputil.ReverseProxy 自行处理。
func (h *handler) direct(req *http.Request) {
	target, ok := req.Context().Value(targetKey{}).(*url.URL)
	if !ok || target == nil {
		return
	}
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host

	path := strings.TrimPrefix(req.URL.Path, PathPrefix)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if base := strings.TrimSuffix(target.Path, "/"); base != "" {
		path = base + path
	}
	req.URL.Path = path
}

func allowedPath(path string) bool {
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// parseTarget 把 LIVEKIT_HOST 归一为 ReverseProxy 可用的 http(s) 地址。
func parseTarget(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errNotConfigured
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errInvalidTarget
	}
	switch u.Scheme {
	case "ws", "http":
		u.Scheme = "http"
	case "wss", "https":
		u.Scheme = "https"
	default:
		return nil, errInvalidTarget
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u, nil
}

// ServerURLForRequest 推导下发给浏览器的同源 LiveKit 地址。
// host 优先取 X-Forwarded-Host（反代或桌面端回环代理改写 Host 时仍能拿到
// 用户实际访问的地址），否则用 r.Host；scheme 取 X-Forwarded-Proto
// （Caddy/Nginx/Traefik 都会注入），缺省回退到 TLS 判定。
func ServerURLForRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	scheme := "ws"
	switch strings.ToLower(firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))) {
	case "https":
		scheme = "wss"
	case "http":
		scheme = "ws"
	default:
		if r.TLS != nil {
			scheme = "wss"
		}
	}
	return scheme + "://" + host + PathPrefix
}

// firstHeaderValue 取逗号分隔多值头的第一段（多级反代会追加而非覆盖）。
func firstHeaderValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	return raw
}

func writeBadGateway(w http.ResponseWriter, err error) {
	msg := "livekit unreachable"
	if err == errNotConfigured {
		msg = "livekit not configured"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 6001,
		"msg":  msg,
		"data": nil,
	})
}

type targetError string

func (e targetError) Error() string { return string(e) }

const (
	errNotConfigured targetError = "livekit target not configured"
	errInvalidTarget targetError = "livekit target invalid"
)

// Cached 给解析结果加一层 TTL 缓存：握手与信令都是高频路径，
// 每次解析都会打到 SFU 配置服务（可能触发 DB 查询）。
func Cached(resolve Resolver, ttl time.Duration) Resolver {
	var mu sync.RWMutex
	var cached string
	var storedAt time.Time
	var filled bool

	return func() string {
		mu.RLock()
		if filled && time.Since(storedAt) < ttl {
			value := cached
			mu.RUnlock()
			return value
		}
		mu.RUnlock()

		value := resolve()
		mu.Lock()
		cached, storedAt, filled = value, time.Now(), true
		mu.Unlock()
		return value
	}
}
