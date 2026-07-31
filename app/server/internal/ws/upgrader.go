package ws

import (
	"log"
	"net/http"
	"strings"

	nhooyrws "nhooyr.io/websocket"

	"GOSpeak/internal/middleware"
	"GOSpeak/internal/pkg"
)

// UpgraderConfig 控制 WebSocket 升级行为。
type UpgraderConfig struct {
	// Fanout 用于注册/注销客户端连接。
	Fanout Broadcaster
	// Handler 是事件分发注册表。
	Handler *HandlerRegistry
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

// extractToken 从请求中提取 JWT token：Authorization header > cookie > query。
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if tokenStr := strings.TrimSpace(authHeader); tokenStr != "" {
		return tokenStr
	}
	if cookie, err := r.Cookie("gospeak_token"); err == nil {
		return cookie.Value
	}
	return r.URL.Query().Get("token")
}

// ServeHTTP 实现 http.Handler，一次完成升级→鉴权→注册→读取循环。
// 应该在 Gin 路由中通过 `r.GET("/ws", gin.WrapH(upgrader))` 挂载。
func (u *Upgrader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractToken(r)
	if tokenStr == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	claims, code := middleware.VerifyToken(tokenStr)
	if code != pkg.SUCCESS {
		log.Printf("[ws] upgrade rejected: code=%s client=%s", code.String(), r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := nhooyrws.Accept(w, r, &nhooyrws.AcceptOptions{
		InsecureSkipVerify: true, // Origin check 由 Gin CORS 中间件处理
	})
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	clientID := claims.UserUUID
	if clientID == "" {
		clientID = claims.Username
	}
	client := NewClient(conn, clientID, claims)

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
				u.cfg.Fanout.Remove(clientID)
			}
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
