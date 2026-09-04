# GOSpeak HTTP/2 与 HTTPS 兼容性说明

> 状态：HTTP/2 / HTTPS 能力现状与部署方案整理。

## 1. 结论

- GOSpeak 架构上兼容 HTTP/2：Go 后端基于 Gin + 标准库 `net/http`，TLS 模式下会自动启用 HTTP/2（h2）。
- HTTP/2 与 HTTP/1.1 可以同时兼容：同一个 443 端口通过 ALPN 协商，现代客户端走 h2，旧客户端自动回退 HTTP/1.1，不需要二选一。
- 当前默认部署没有开启 HTTP/2：Go 进程使用明文 `ListenAndServe()`，Docker Compose 默认只暴露 80；仓库的 Nginx 配置中预留了 443 + `http2` 模板但默认注释。
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
| Nginx 反代（推荐） | h2 + HTTP/1.1（ALPN） | HTTP/1.1 | HTTP/1.1 Upgrade | 仓库已提供模板，开启 443 后即兼容两种协议 |
| 单二进制直跑（现状） | HTTP/1.1 | HTTP/1.1 | HTTP/1.1 Upgrade | 无 TLS 配置入口，不能对外提供 h2 |
| 单二进制直跑（已支持） | h2 + HTTP/1.1（ALPN） | h2 + HTTP/1.1 | HTTP/1.1 Upgrade | 配置 `TLS_CERT` / `TLS_KEY` 后启用，未配置时自动 fallback HTTP/1.1 |
| 明文 h2c | 不支持浏览器 | - | - | 无实际意义，不推荐 |

## 4. 当前代码与配置现状

### 4.1 Go 服务

- 监听端口：`SERVER_PORT`（默认 `8998`）。
- 启动方式：`srv.ListenAndServe()`，明文 HTTP/1.1。
- 支持可选 HTTPS：`TLS_CERT` / `TLS_KEY` 同时配置时调用 `ListenAndServeTLS`；均未配置时保持明文 HTTP/1.1，只配置其一时启动报错。
- Cookie 的 `Secure` 属性默认是 `auto`，仅在 `r.TLS != nil` 时启用；直跑 TLS 时会自动正确，但经反向代理时需要显式设置 `COOKIE_SECURE=true`。

### 4.2 Nginx 反代

仓库已有模板：

- `deploy/nginx-https.conf`：宿主 Nginx 完整 HTTPS + h2 模板，80 自动 301 到 443。
- `deploy/nginx-docker-https.conf`：Docker Compose 完整 HTTPS + h2 模板。
- `deploy/nginx.conf`：默认明文 80 模板，文件尾部说明 HTTPS + h2 的启用方式。
- `deploy/nginx-docker.conf` / `deploy/nginx-custom.conf`：默认监听 80，已配置 `/ws`、`/api/`、静态资源、SRS WHIP/WHEP 反代。
- `deploy/docker-compose.yml`：默认暴露 `${HTTP_PORT:-80}:80`。

开启 443 后，Nginx 对浏览器同时提供 h2 与 HTTP/1.1 ALPN fallback；后端仍用 `proxy_http_version 1.1` 通信，无需改动 Go 代码。

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
- Nginx 已对 `/ws` 使用 `proxy_http_version 1.1` + `Upgrade` 头，与页面 h2 并存没有问题。
- 浏览器对 `wss://` 连接通常协商 HTTP/1.1，因此单二进制直跑 TLS 时，只要 WebSocket 握手走 HTTP/1.1 即可正常工作；如遇到异常，优先用 Nginx 反代终结 TLS 并转发 `/ws`。

## 7. Cookie 注意事项

`COOKIE_SECURE` 默认值为 `auto`：

- Go 直跑 TLS：`r.TLS != nil`，Cookie 自动带 `Secure`。
- 经 Nginx 反代：Go 看到的请求不是 TLS，`auto` 不会自动加 `Secure`，需要显式设置 `COOKIE_SECURE=true`。

## 8. 二进制文件内容

- 本地上传、S3 预签名直传、头像等接口处理任意字节流（`multipart/form-data`、`application/octet-stream` 等）。
- HTTP/2 与 HTTP/1.1 对二进制 payload 没有协议级区别。
- WebRTC 音频/视频媒体本身不走 HTTP，走 SRS/LiveKit 的 UDP/DTLS 端口，不受 HTTP 版本影响。

## 9. 如何验证

### 9.1 验证服务器支持 h2

```bash
# 查看协商结果：curl 输出中 https 后带 h2 表示使用 HTTP/2
curl -k --http2 -I https://<host>/ping

# 强制 HTTP/1.1 验证 fallback
curl -k --http1.1 -I https://<host>/ping
```

### 9.2 查看 Nginx 是否编译了 http_v2_module

```bash
nginx -V 2>&1 | grep http_v2_module
```

### 9.3 确认当前进程协议

```bash
curl -I http://<host>:8998/ping
# 当前未启用 TLS 时，协议为 HTTP/1.1
```

## 10. 开启步骤

### 10.1 Nginx 反代方式（推荐）

1. 准备证书（`fullchain.pem` / `privkey.pem`，可由 Caddy / Let's Encrypt / 云证书提供）。
2. 宿主 Nginx：直接使用 `deploy/nginx-https.conf`，修改 `server_name` 与证书路径后加载。
3. Docker Compose：将 `nginx` 服务的配置挂载替换为 `deploy/nginx-docker-https.conf`，映射 443 并挂载证书：

```yaml
ports:
  - "${HTTP_PORT:-80}:80"
  - "${HTTPS_PORT:-443}:443"
volumes:
  - ./nginx-docker-https.conf:/etc/nginx/nginx.conf:ro
  - ./ssl/fullchain.pem:/etc/nginx/ssl/fullchain.pem:ro
  - ./ssl/privkey.pem:/etc/nginx/ssl/privkey.pem:ro
```

4. 前端使用 `https://<host>`，并将 `SRS_PUBLIC_HOST`、`CLUSTER_ENTRY_URL` 等配置为 HTTPS 地址。
5. 设置 `COOKIE_SECURE=true`。

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
- `deploy/nginx.conf`：Nginx 反代模板（含 443 + http2 注释示例）。
- `deploy/nginx-https.conf` / `deploy/nginx-docker-https.conf`：HTTPS + h2 完整模板。
- `deploy/nginx-docker.conf` / `deploy/nginx-custom.conf`：Docker 与宿主反代配置。
- `deploy/docker-compose.yml`：端口暴露。
