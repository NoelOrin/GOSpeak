# SRS 自部署端到端 Runbook

注意: SRS 默认不校验 WHIP Bearer,token 安全靠网络边界。但 `SRS_SECRET` 必须非空 — 否则 token 退化为 `room:identity` 明文,房间名/用户名含中文等非 latin1 字符时,`fetch` 的 `Authorization` header 会报 `String contains non ISO-8859-1 code point`。设了 secret 即走 JWT (HS256, base64url, header 安全)。

dev 环境(浏览器与 docker 同宿主)。LAN 部署见末节。

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

## 排查表

| 症状 | 原因 | 修 |
|------|------|-----|
| ICE failed / 无声 | candidate 错 | `docker exec gospeak-srs printenv CANDIDATE` 应为 `127.0.0.1` |
| WHIP 401 | (dev 不应发生)SRS auth 开了 | 确认 srs.conf 无 `http_api auth` |
| `String contains non ISO-8859-1 code point` (fetch headers) | `SRS_SECRET` 空,明文 token `room:identity` 含非 latin1 (中文房间名/用户名) | `.env.dev` 设 `SRS_SECRET=$(openssl rand -hex 32)`,重启 server |
| 前端仍连 livekit | env 没生效 | 重启 `pnpm dev:web`,确认 `.env.local` 在 app/web 下 |
| `curl /api/v1/streams` 空 | 还没人 publish | 正常,publish 后才出现 stream |
| WHEP 收不到 track | join 顺序 | 任一方 publish 后另一方才能 WHEP subscribe |
| curl callback 返回 403 但 token 正确 | `param` 值含 `&` 未 URL-encode | 用 `curl --data-urlencode 'param=...'`,不要手动拼接 `-d 'key=val&key2=val2'` |
| SRS WHIP/WHEP 被 403 拒 | callback 不可达或 streamToken 错 | 确认 backend 起在 8998,SRS→host.docker.internal 网络;`SRS_SECRET` 与 server 一致 |

## LAN 部署

改 `deploy/docker-compose.example.yml` srs 服务的 `CANDIDATE` 为宿主 LAN IP(如 `192.168.1.10`)。浏览器与 SRS 不同机时必须,否则 ICE candidate 不可达。
