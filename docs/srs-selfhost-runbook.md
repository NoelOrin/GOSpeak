# SRS 自部署端到端 Runbook

注意: SRS 默认不校验 WHIP Bearer,token 安全靠网络边界。但 `SRS_SECRET` 必须非空 — 否则 token 退化为 `room:identity` 明文,房间名/用户名含中文等非 latin1 字符时,`fetch` 的 `Authorization` header 会报 `String contains non ISO-8859-1 code point`。设了 secret 即走 JWT (HS256, base64url, header 安全)。

dev 环境(浏览器与 docker 同宿主)。LAN 部署见末节。

浏览器侧 WHIP/WHEP 走同源反向代理:前端拿到的 `whipUrl` 是 `/rtc/v1/whip/`,实际由 Vite dev proxy 或 nginx 转发到 SRS HTTP API `:1985`。不要让浏览器直接连接 `http://srs:1985` 或 `http://localhost:1985`,否则生产环境会遇到跨域、HTTPS mixed content 或内网地址不可达问题。

## 注意事项（必读）

1. **信令可反代，媒体必须直连**  
   WHIP/WHEP 走同源 `/rtc/v1` → SRS `:1985`。WebRTC 媒体走 SRS `:8000`（UDP/TCP），Nginx 不能代 UDP。

2. **`SRS_SECRET` 必填**  
   空 secret 会明文 token + 中文 header 报错。生产/开发都用 `openssl rand -hex 32`。

3. **两套地址不要混**  
   - Go 管 SRS：`SRS_HOST=srs`（或 localhost）  
   - 浏览器：`SRS_PUBLIC_HOST=https://域名` + 相对 `SRS_WHIP_URL=/rtc/v1/whip/`  
   不要把 `serverUrl` 指到容器内网或浏览器不可达地址。

4. **`SRS_CANDIDATE` = 客户端可达的媒体 IP**  
   本机 `127.0.0.1`，局域网 LAN IP，公网公网 IP。填错表现为 ICE failed / 无声。

5. **backend 先于 publish**  
   SRS hooks 回调 Go；backend 挂了会 on_publish 403。

6. **当前无 TURN 下发**  
   不依赖 Coturn。公网务必放行 `8000/udp`（及 `8000/tcp` 回退）。NAT 极差环境需自建中继时再另议，不在默认部署路径。

7. **生产勿让浏览器直连 `:1985`**  
   跨域、mixed content、内网不可达都会踩坑；统一走 Nginx 同源反代。


## 1. 起 SRS

> SRS http_hooks 已配 callback 到 backend:8998,backend 必须先于 SRS publish 启动,否则 SRS fail-closed 拒所有 publish。

```bash
docker compose -f deploy/docker-compose.example.yml up -d srs
curl -s http://localhost:1985/api/v1/versions   # 期望 {"code":0,...}
```

## 2. 后端切 SRS

编辑 `app/server/.env.dev`:
- 注释 `SFU_PROVIDER="livekit"` 行
- 取消注释(或新增)`SFU_PROVIDER="srs"`
- `SRS_SECRET` 设非空值 (生成: `openssl rand -hex 32`)

server config 已有 `SRS_HOST=localhost SRS_API_PORT=1985 SRS_WHIP_PORT=1985` 默认值。`SRS_SECRET` 留空会触发明文 token 的 latin1 报错,必须设。

启动:
```bash
pnpm dev:server
```

## 3. 前端切 SRS

新建 `app/web/.env.local`(已 gitignore):
```
VITE_SFU_PROVIDER=srs
```

启动:
```bash
pnpm dev:web
```

开发环境 `app/web/vite.config.ts` 已配置 `/rtc/v1 -> http://localhost:1985` 代理。浏览器 Network 里 WHIP 请求应为同源地址:

```text
POST http://localhost:<vite端口>/rtc/v1/whip/?app=live&stream=gs-...&token=...
```

## 3.1 生产 nginx 反代

生产部署参考 `deploy/nginx.conf`,必须包含 `/rtc/v1/` 代理到 SRS HTTP API:

```nginx
upstream gospeak_srs_http_api {
    server 127.0.0.1:1985;
}

location /rtc/v1/ {
    proxy_pass http://gospeak_srs_http_api;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_request_buffering off;
    proxy_read_timeout 120s;
    proxy_send_timeout 120s;
}
```

若 nginx 与 SRS 不在同一宿主,把 upstream 改成 SRS 容器名、内网 IP 或服务发现地址。该 location 必须放在 SPA `location /` 之前。

## 4. 多流双向音频验证

多流模式下,每客户端独立 publish stream (`gs-<hash>`),不再共享 `livestream`。WHEP 订阅由 `member:joined`/`member:left` 事件自动触发。

### 4.1 基本流程

1. 浏览器开 `http://localhost:<vite端口>`(终端输出实际端口)
2. 标签 A 注册/登录,创建/加入房间 R
3. 标签 B(同浏览器不同标签或无痕)登录另一账号,加入房间 R
4. **A 说话 → B 听到;B 说话 → A 听到** (每方订阅对方的独立 stream,不订阅自身 → 无回声)
5. 标签 C (第三账号)加入房间 R
6. A 和 B 应自动收到 C 的音频 (member:joined 触发的 subscribePeers)
7. 任一方关闭标签或离场 → 其他人收到 track removed (member:left 触发的 unsubscribePeer)

### 4.2 流调试

SRS 日志可观察每客户端的独立 publish stream:

```bash
# 查看当前活跃的 WHIP publish 流 (每客户端一行)
docker logs gospeak-srs 2>&1 | grep "RTC whip publish"

# 期望输出(示例):
# [RTC] whip publish stream=gs-6ccajt9uage8, client_id=xxx
# [RTC] whip publish stream=gs-a1b2c3d4e5f6, client_id=yyy
# 注意: 每客户端 stream 不同,不再有共享 livestream

# 查看 http_hooks callback 命中情况
docker logs gospeak-srs 2>&1 | grep "on_publish\|on_play\|on_unpublish\|on_stop"

# 期望输出:
# http: on_publish ok, ... stream=gs-xxx, response={"code":0}
# http: on_play ok, ... stream=gs-yyy, response={"code":0}
# http: on_unpublish ok, ... stream=gs-xxx, response={"code":0}
```

如果 on_publish 返回 `{"code":403}`,排查顺序:
1. 确认 `SRS_SECRET` 与 backend `.env.dev` 一致
2. 确认 backend 起在 8998,`host.docker.internal` 可达
3. 检查 SRS → backend 网络:`docker exec gospeak-srs curl -s http://host.docker.internal:8998/api/v1/srs/callback` 应可达

## 5. 禁言与房间维度管理

### 5.1 禁言语义（Discord 式）

> 禁言时后端写入禁推黑名单（NATS KV / 内存），**不踢流**——被禁言成员保留订阅仍可收听；
> 其他客户端收到 `member:muted` 事件后对该成员远端音轨静音。
> SRS `on_publish` http_hook 命中黑名单时返回 `code:1` 拒绝推流，因此禁言期间断流/重连无法绕过发声。
> 解禁后黑名单删除，客户端收到 `member:unmuted` 恢复音量，可重新推流。

### 5.2 SRS 多节点 / Cluster 说明

SRS 5.0+ 的 origin-edge / origin cluster 是**流分发与故障转移**能力（edge 回源、RTMP302 重定向），
HTTP API（`/api/v1/streams`、`/api/v1/clients`）仍是节点级的，SRS **没有原生 room 维度管理 API**。
GOSpeak 的 room 维度管理（ListRooms/ListParticipants/DeleteRoom）通过直查 SRS API + stream→room
业务映射实现：

- 单节点：`SRS_HOST` 指向该节点即可，无需集群。
- 多节点部署：将 `SRS_HOST`/`SRS_WHIP_URL` 指向可聚合的入口节点（edge 或反代），
  或由部署层遍历各节点后聚合（GOSpeak 不内置集群遍历）。
- stream→room 映射跨实例不变：依赖 membership KV（NATS）与 `member:joined`/`on_publish` 登记，
  与 SRS 节点数无关。

## 排查表

| 症状 | 原因 | 修 |
|------|------|-----|
| ICE failed / 无声 | candidate 错 | `docker exec gospeak-srs printenv CANDIDATE` 应为 `127.0.0.1` |
| WHIP 404 且响应像前端页面 | `/rtc/v1/` 没进代理,被 SPA 兜底吃掉 | 检查 nginx/Vite proxy,`location /rtc/v1/` 必须在 `location /` 前 |
| WHIP 502/504 | 代理到 SRS 不通 | 确认 SRS `1985` 可达:`curl http://<srs-host>:1985/api/v1/versions` |
| WHIP 401 | (dev 不应发生)SRS auth 开了 | 确认 srs.conf 无 `http_api auth` |
| `String contains non ISO-8859-1 code point` (fetch headers) | `SRS_SECRET` 空,明文 token `room:identity` 含非 latin1 (中文房间名/用户名) | `.env.dev` 设 `SRS_SECRET=$(openssl rand -hex 32)`,重启 server |
| 前端仍连 livekit | env 没生效 | 重启 `pnpm dev:web`,确认 `.env.local` 在 app/web 下 |
| `curl /api/v1/streams` 空 | 还没人 publish | 正常,publish 后才出现 stream |
| WHEP 收不到 track | join 顺序 | 任一方 publish 后另一方才能 WHEP subscribe |
| curl callback 返回 403 但 token 正确 | `param` 值含 `&` 未 URL-encode | 用 `curl --data-urlencode 'param=...'`,不要手动拼接 `-d 'key=val&key2=val2'` |
| SRS WHIP/WHEP 被 403 拒 | callback 不可达或 streamToken 错 | 确认 backend 起在 8998,SRS→host.docker.internal 网络;`SRS_SECRET` 与 server 一致 |

## LAN 部署

改 `deploy/docker-compose.example.yml` srs 服务的 `CANDIDATE` 为宿主 LAN IP(如 `192.168.1.10`)。浏览器与 SRS 不同机时必须,否则 ICE candidate 不可达。
