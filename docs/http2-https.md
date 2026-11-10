# GOSpeak HTTP/2 与 HTTPS 兼容性说明

> 状态：HTTP/2 / HTTPS 能力现状与部署方案整理。

## 1. 结论

- GOSpeak 架构上兼容 HTTP/2：Go 后端基于 Gin + 标准库 `net/http`，TLS 模式下会自动启用 HTTP/2（h2）。
- HTTP/2 与 HTTP/1.1 可以同时兼容：同一个 443 端口通过 ALPN 协商，现代客户端走 h2，旧客户端自动回退 HTTP/1.1，不需要二选一。
- 当前默认部署没有开启 HTTP/2：Go 进程使用明文 `ListenAndServe()`。需要 h2 时有两种方式：开 `edge` 档位用内置 Caddy 自动完成 TLS + h2/h3 协商，或单二进制配置 `TLS_CERT` / `TLS_KEY` 直跑 HTTPS。
- 单文件二进制：已支持通过 `TLS_CERT` / `TLS_KEY` 配置直跑 HTTPS + h2；两者均未配置时自动保持明文 HTTP/1.1。
- 二进制内容（上传/下载文件）不受 HTTP 版本影响；WebRTC 媒体走 UDP/DTLS，与 HTTP 协议无关。

## 2. 为什么兼容

Go 的 `net/http.Server` 在 TLS 模式下默认注册 h2：

- 使用 `ListenAndServeTLS(certFile, keyFile)` 后，同一端口同时提供 h2 和 HTTP/1.1。
- 客户端在 TLS 握手中通过 ALPN 声明支持的协议，服务端选择交集：支持 h2 的客户端优先 h2，其余自动回退 HTTP/1.1。
- Gin 基于 `net/http`，HTTP/2 对业务代码透明，不需要在路由、Handler 或前端做适配。

当前启动代码位于 `app/server/server/gin.go`，使用明文监听：

```go
srv := &http.Server{
	Addr:    ":" + port,
	Handler: rootHandler,
}
// ...
if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	logger.WithComponent("HTTP").Fatalf("listen error: %v", err)
}
```

## 3. 部署方式对比

| 方式 | 客户端协议 | 后端协议 | WebSocket | 说明 |
|------|-----------|---------|-----------|------|
| Compose `edge` 档位（内置 Caddy，推荐） | h2 + HTTP/1.1（ALPN），另支持 h3 | HTTP/1.1 | HTTP/1.1 Upgrade | 填 `GOSPEAK_DOMAIN` 后 Caddy 自动签发证书并启用 h2；`reverse_proxy` 自动处理 WebSocket 升级 |
| 已有反代（Nginx / Traefik / Dokploy） | h2 + HTTP/1.1（ALPN） | HTTP/1.1 | HTTP/1.1 Upgrade | 只需指向单一上游 `gospeak:8998`，由反代完成 TLS 与 ALPN |
| 单二进制直跑（无 TLS） | HTTP/1.1 | HTTP/1.1 | HTTP/1.1 Upgrade | 未配置 `TLS_CERT` / `TLS_KEY` 时保持明文，不对外提供 h2 |
| 单二进制直跑（配置 TLS） | h2 + HTTP/1.1（ALPN） | h2 + HTTP/1.1 | HTTP/1.1 Upgrade | 配置 `TLS_CERT` / `TLS_KEY` 后启用，未配置时自动 fallback HTTP/1.1 |
| 明文 h2c | 不支持浏览器 | - | - | 无实际意义，不推荐 |

## 4. 当前代码与配置现状

### 4.1 Go 服务

- 监听端口：`SERVER_PORT`（默认 `8998`）。
- 启动方式：`srv.ListenAndServe()`，明文 HTTP/1.1。
- 支持可选 HTTPS：`TLS_CERT` / `TLS_KEY` 同时配置时调用 `ListenAndServeTLS`；均未配置时保持明文 HTTP/1.1，只配置其一时启动报错。
- Cookie 的 `Secure` 属性默认是 `auto`：直跑 TLS 时看 `r.TLS`，经反向代理时看 `X-Forwarded-Proto`。Caddy / Nginx / Traefik 都会自动传这个头，因此 `auto` 在两种拓扑下都正确；只有反代故意剥离该头时才需要显式设置 `COOKIE_SECURE=true`。

### 4.2 反向代理与 HTTPS 入口

仓库不再提供独立的反代配置文件，改为两种内建方式：

- **`deploy/Caddyfile` + `edge` 档位（推荐）**：`caddy:2-alpine` 容器监听 80/443（含 `443/udp` 供 HTTP/3），填了 `GOSPEAK_DOMAIN` 就自动签发并续期证书，对浏览器提供 h2/h3 与 HTTP/1.1 的 ALPN 协商；站点地址留空时退化为 `:80`。`reverse_proxy gospeak:8998` 自动完成 WebSocket 升级，无需额外指令。
- **已有反代（Nginx / Traefik / Dokploy / 宝塔）**：不开 `edge`，把反代指向单一上游 `gospeak:8998`。`/api`、`/ws`、`/uploads`、`/livekit` 全部同源走这一个上游，**不需要为 LiveKit 单独配置路由或证书**。反代侧只需保留 `Host` 与 `X-Forwarded-Proto`、放行 WebSocket 升级。

开启 TLS 后，入口对浏览器同时提供 h2 与 HTTP/1.1 ALPN fallback；后端仍用 HTTP/1.1 通信，无需改动 Go 代码。

## 5. HTTP/1.1 与 HTTP/2 共存原理

HTTP/2 不是“替换” HTTP/1.1，而是同一 TLS 端口上的协议协商：

```text
客户端 TLS ClientHello（ALPN: h2, http/1.1）
        │
        ▼
服务端选择交集 → h2（现代浏览器 / curl）
               → http/1.1（旧客户端、不支持 h2 的工具）
```

因此开启 h2 不会破坏旧客户端；Nginx 和 Go 标准库都会自动完成该协商。

## 6. WebSocket 注意事项

- `/ws` 使用 `nhooyr.io/websocket`，依赖 HTTP/1.1 Upgrade 握手。
- Go 标准库的 HTTP/2 server 不支持 RFC 8441（WebSocket over HTTP/2）的 extended CONNECT，因此不要让 `/ws` 走 h2。
- 内置 Caddy 的 `reverse_proxy` 自动完成 Upgrade 握手；自有 Nginx 需要对 `/` 使用 `proxy_http_version 1.1` + `Upgrade` 头，与页面 h2 并存没有问题。
- 浏览器对 `wss://` 连接通常协商 HTTP/1.1，因此单二进制直跑 TLS 时，只要 WebSocket 握手走 HTTP/1.1 即可正常工作；如遇到异常，优先用 `edge` 档位的内置 Caddy（或自有反代）终结 TLS 并转发 `/ws`。

## 7. Cookie 注意事项

`COOKIE_SECURE` 默认值为 `auto`：

- Go 直跑 TLS：`r.TLS != nil`，Cookie 自动带 `Secure`。
- 经反代（Caddy 或自有反代）：请求到达 Go 时是明文，`auto` 会读 `X-Forwarded-Proto`，看到 `https` 就加 `Secure`。Caddy 的 `reverse_proxy` 与常见 Nginx 配置都会传，所以通常无需额外设置。
- 反代没传 `X-Forwarded-Proto`，或不想依赖该头时：显式设置 `COOKIE_SECURE=true`。

## 8. 二进制文件内容

- 本地上传、S3 预签名直传、头像等接口处理任意字节流（`multipart/form-data`、`application/octet-stream` 等）。
- HTTP/2 与 HTTP/1.1 对二进制 payload 没有协议级区别。
- WebRTC 音频/视频媒体本身不走 HTTP，走 LiveKit 的 UDP/DTLS 端口，不受 HTTP 版本影响。

## 9. 如何验证

### 9.1 验证服务器支持 h2

```bash
# 查看协商结果：curl 输出中 https 后带 h2 表示使用 HTTP/2
curl -k --http2 -I https://<host>/ping

# 强制 HTTP/1.1 验证 fallback
curl -k --http1.1 -I https://<host>/ping
```

### 9.2 确认 Caddy 的证书与协议

```bash
# 有域名时，看证书是否签发成功
docker compose logs caddy | grep -i "certificate obtained"

# 确认 ALPN 协商结果（应出现 h2）
curl -k --http2 -I https://<host>/ping
```

### 9.3 确认当前进程协议

```bash
curl -I http://<host>:8998/ping
# 当前未启用 TLS 时，协议为 HTTP/1.1
```

## 10. 开启步骤

### 10.1 内置 Caddy 档位（推荐）

1. 在 `deploy/.env` 填域名并追加 `edge` 档位：

```ini
COMPOSE_PROFILES=app,livekit,edge
GOSPEAK_DOMAIN=voice.example.com
```

2. 域名 A 记录指向本机公网 IP，防火墙放行 80/443。
3. 启动：

```bash
cd deploy
docker compose up -d --build
```

4. Caddy 自动申请并续期证书、把 HTTP 重定向到 HTTPS、对浏览器提供 h2（以及 h3）。反代只有一个上游 `gospeak:8998`，`/livekit` 也在其中，所以 **LiveKit 不需要自己的证书或域名**。
5. `COOKIE_SECURE` 保持默认 `auto` 即可：Caddy 会传 `X-Forwarded-Proto: https`，后端据此加 `Secure`。

已有反代的话不开 `edge`，把现有反代指向 `gospeak:8998` 即可，效果相同。

### 10.2 单二进制直跑（已支持）

1. 准备证书（`fullchain.pem` / `privkey.pem`，可由 Caddy / Let's Encrypt / 云证书提供）。
2. 配置环境变量，两者必须同时提供：

```env
TLS_CERT=/path/to/fullchain.pem
TLS_KEY=/path/to/privkey.pem
```

3. 启动服务，同一端口自动提供 h2 + HTTP/1.1 ALPN fallback：

```bash
./gospeak-linux-amd64 server -e prod
```

4. 不配置 `TLS_CERT` / `TLS_KEY` 时行为不变，继续以明文 HTTP/1.1 监听 `SERVER_PORT`。

## 11. 相关文件

- `app/server/server/gin.go`：HTTP 服务启动与 `ListenAndServe`。
- `app/server/internal/config/config.go`：环境变量配置（含 `TLS_CERT` / `TLS_KEY`）。
- `app/server/server/gin.go`：按 TLS 配置选择监听方式（`serveHTTP`）。
- `app/server/server/gin_test.go`：h2 + HTTP/1.1 fallback 测试。
- `app/server/internal/handler/authcookie.go`：Cookie `Secure` 逻辑。
- `app/server/internal/ws/upgrader.go`：WebSocket 升级与鉴权。
- `deploy/Caddyfile`：内置 Caddy 入口（`edge` 档位），自动 HTTPS + h2/h3。
- `deploy/docker-compose.yml`：`caddy` 服务的端口暴露（80/443/443udp）与 `gospeak` 上游。
- `deploy/DEPLOY.md`：三种部署场景（有域名 / 已有反代 / 局域网）。
