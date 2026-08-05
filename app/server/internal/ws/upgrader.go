package ws

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	nhooyrws "nhooyr.io/websocket"

	"github.com/google/uuid"

	"GOSpeak/internal/middleware"
	"GOSpeak/internal/pkg"
)

// UpgraderConfig 控制 WebSocket 升级行为。
type UpgraderConfig struct {
	// Fanout 用于注册/注销客户端连接。
	Fanout Broadcaster
	// Handler 是事件分发注册表。
	Handler *HandlerRegistry
	// AllowedOrigins 是握手阶段允许的 Origin 白名单。
	// 为空时默认只允许与请求 Host 同源；包含 "*" 表示允许任意来源。
	AllowedOrigins []string
	// OnConnect 在连接建立后、读取循环开始前调用（Hub 用于设置 OnClose）。
	OnConnect func(c *Client)
	// OnDisconnect 在连接关闭并注销 Fanout 后调用（Hub 用于清理房间状态）。
	OnDisconnect func(c *Client)
}

// Upgrader 封装 HTTP→WS 升级、鉴权、生命周期管理。
type Upgrader struct {
	cfg UpgraderConfig
}

// NewUpgrader 创建一个 Upgrader。
func NewUpgrader(cfg UpgraderConfig) *Upgrader {
	return &Upgrader{cfg: cfg}
}

// extractToken 从 Sec-WebSocket-Protocol 提取短时 WS ticket。
func extractToken(r *http.Request) (string, bool) {
	for _, protocol := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		protocol = strings.TrimSpace(protocol)
		if protocol != "" && protocol != "gospeak" {
			return protocol, true
		}
	}
	return "", false
}

// originAllowed 校验 WS 握手 Origin。Gin CORS 不覆盖 WS 升级，因此必须在这里独立校验。
func (u *Upgrader) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器客户端（bot/CLI）通常不发送 Origin，按合法客户端放行。
		return true
	}
	for _, allowed := range u.cfg.AllowedOrigins {
		if allowed == "*" || strings.EqualFold(strings.TrimRight(allowed, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	if len(u.cfg.AllowedOrigins) == 0 {
		parsed, err := url.Parse(origin)
		if err != nil {
			return false
		}
		switch parsed.Scheme {
		case "http", "https", "ws", "wss":
		default:
			return false
		}
		return parsed.Host == r.Host
	}
	return false
}

// ServeHTTP 实现 http.Handler，一次完成升级→鉴权→注册→读取循环。
// 应该在 Gin 路由中通过 `r.GET("/ws", gin.WrapH(upgrader))` 挂载。
// newConnID 生成连接级唯一 ID：保留用户可读前缀，追加随机后缀区分多连接。
func newConnID(userUUID, username string) string {
	clientID := userUUID
	if clientID == "" {
		clientID = username
	}
	return clientID + "-" + uuid.NewString()[:8]
}

func (u *Upgrader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !u.originAllowed(r) {
		log.Printf("[ws] upgrade rejected: origin=%q client=%s", r.Header.Get("Origin"), r.RemoteAddr)
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}

	tokenStr, fromSubprotocol := extractToken(r)
	if tokenStr == "" || !fromSubprotocol {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	claims, code := middleware.VerifyWSTicket(tokenStr)
	if code != pkg.SUCCESS {
		log.Printf("[ws] upgrade rejected: code=%s client=%s", code.String(), r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := nhooyrws.Accept(w, r, &nhooyrws.AcceptOptions{
		Subprotocols:       []string{"gospeak"},
		InsecureSkipVerify: true, // Origin 已由 Upgrader.originAllowed 校验
	})
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	clientID := claims.UserUUID
	if clientID == "" {
		clientID = claims.Username
	}
	// 追加随机后缀生成连接级唯一 ID：同一用户多连接（多标签页/断线重连）
	// 不再共享同一个 Fanout/Hub key，旧连接断开不会误清理新连接。
	client := NewClient(conn, newConnID(claims.UserUUID, claims.Username), claims)

	// 注册到 Fanout
	if u.cfg.Fanout != nil {
		u.cfg.Fanout.Add(client)
	}

	// 生命周期回调（Hub 设置 OnClose 以从 Fanout 注销 + 清理 Hub 状态）
	client.OnClose = func(id string) {
		if u.cfg.Fanout != nil {
			u.cfg.Fanout.Remove(id)
		}
		if u.cfg.OnDisconnect != nil {
			u.cfg.OnDisconnect(client)
		}
	}

	// 确保 OnConnect panic 不泄漏 client 到 Fanout
	var readLoopStarted bool
	defer func() {
		if !readLoopStarted {
			if u.cfg.Fanout != nil {
				// 使用连接级唯一 ID 清理，避免裸 user ID 无法命中注册 key。
				u.cfg.Fanout.Remove(client.ID())
			}
			// OnConnect panic 时已 Accept 的连接不会进入读取循环，必须主动关闭。
			client.Close()
		}
	}()

	if u.cfg.OnConnect != nil {
		u.cfg.OnConnect(client)
	}

	log.Printf("[ws] client connected: %s (%s) ip=%s", clientID, claims.Username, r.RemoteAddr)

	// 阻塞读取循环 — ServeHTTP 在此期间阻塞
	readLoopStarted = true
	if u.cfg.Handler != nil {
		client.StartReadLoop(func(c ClientMessenger, msg Message) {
			u.cfg.Handler.Dispatch(c, msg)
		})
	} else {
		client.StartReadLoop(func(c ClientMessenger, msg Message) {})
	}

	log.Printf("[ws] client disconnected: %s (%s)", clientID, claims.Username)
}
