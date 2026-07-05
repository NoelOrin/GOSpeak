# SRS 自部署端到端 Runbook

注意: SRS 默认不校验 WHIP Bearer,token 为装饰性 JWT,安全靠网络边界。

dev 环境(浏览器与 docker 同宿主)。LAN 部署见末节。

## 1. 起 SRS

```bash
docker compose -f deploy/docker-compose.example.yml up -d srs
curl -s http://localhost:1985/api/v1/versions   # 期望 {"code":0,...}
```

## 2. 后端切 SRS

编辑 `app/server/.env.dev`:
- 注释 `SFU_PROVIDER="livekit"` 行
- 取消注释(或新增)`SFU_PROVIDER="srs"`

server config 已有 `SRS_HOST=localhost SRS_API_PORT=1985 SRS_WHIP_PORT=1985 SRS_SECRET=` 默认值,无需另设。

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

## 4. 双标签双向音频验证

1. 浏览器开 `http://localhost:<vite端口>`(终端输出实际端口)
2. 标签 A 注册/登录,创建/加入房间 R
3. 标签 B(同浏览器不同标签或无痕)登录另一账号,加入房间 R
4. A 说话 → B 听到;B 说话 → A 听到
5. 任一方离场 → 另一方收到 track removed

## 排查表

| 症状 | 原因 | 修 |
|------|------|-----|
| ICE failed / 无声 | candidate 错 | `docker exec gospeak-srs printenv CANDIDATE` 应为 `127.0.0.1` |
| WHIP 401 | (dev 不应发生)SRS auth 开了 | 确认 srs.conf 无 `http_api auth` |
| 前端仍连 livekit | env 没生效 | 重启 `pnpm dev:web`,确认 `.env.local` 在 app/web 下 |
| `curl /api/v1/streams` 空 | 还没人 publish | 正常,publish 后才出现 stream |
| WHEP 收不到 track | join 顺序 | 任一方 publish 后另一方才能 WHEP subscribe |

## LAN 部署

改 `deploy/docker-compose.example.yml` srs 服务的 `CANDIDATE` 为宿主 LAN IP(如 `192.168.1.10`)。浏览器与 SRS 不同机时必须,否则 ICE candidate 不可达。
