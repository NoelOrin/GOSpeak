# 自建 LiveKit

自己跑 LiveKit，数据全在自己手里，也不按用量花钱。代价是要管运维。

好消息是**运维里最麻烦的那部分（域名、证书、反向代理）已经不需要你操心了**：GOSpeak 应用内置了同源反代 `/livekit`，浏览器信令走你现有的站点域名，LiveKit 不需要第二个域名，也不需要自己的证书。

## 用 Compose 一把梭（推荐）

`deploy/` 里已经配好了：

```bash
cd deploy
./init.sh --domain voice.example.com
docker compose up -d
```

`./init.sh --domain voice.example.com` 会写入域名并自动把 `edge` 加进 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。跑完直接 `docker compose up -d` 即可，无需手改档位。

**不开 `edge` 的场景**：已有反代（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不要传 `--domain`**（或手动把 `edge` 从 `COMPOSE_PROFILES` 里去掉），反代指向宿主机 `8998`。

`init.sh` 会做三件和 LiveKit 有关的事：

1. 随机生成 API Key 和 Secret，写进 `deploy/.env`
2. 归一 `livekit/livekit.yaml`，剥离硬编码密钥
3. 校验 `.env` / `livekit.yaml` / `Caddyfile` 一致性

公网 IP 由 LiveKit 自己用 `rtc.use_external_ip: true` + STUN 探测，`init.sh` 不再处理这一项。

密钥只在 `deploy/.env` 里有一份，由 compose 注入 LiveKit 容器，`livekit.yaml` 不含任何凭据——所以不存在「两边 key 对不上」的问题。

## 信令怎么连：同源 `/livekit`

这是自建 LiveKit 现在最省事的一点。

```
浏览器 ──wss://<你的域名>/livekit──► gospeak:8998 ──► livekit:7880（容器内网）
浏览器 ──WebRTC(UDP)────────────────────────────────► livekit:7882（媒体直连）
```

- 应用内置一个同源反代：`/livekit/*` 转发到内部 LiveKit 的 7880，**WebSocket 升级自动处理**。
- `LIVEKIT_PUBLIC_URL` **默认留空**。留空时，`POST /api/v1/signal/token` 返回的 `serverUrl` 由请求本身推导：
  - 浏览器打开 `https://voice.example.com` → `serverUrl = wss://voice.example.com/livekit`
  - scheme 取 `X-Forwarded-Proto`，缺省按是否 TLS 判断
  - livekit-client 会自动在这个地址后拼 `/rtc/v1`，经路径前缀反代握手正常

所以：

- **LiveKit 不需要域名。**
- **LiveKit 不需要证书。** TLS 在 Caddy 或你自己的反代上终结一次就够了。
- **LiveKit 不需要单独的反代规则。** 反代只面对 `gospeak:8998` 一个上游。

只有这两种情况才显式填 `LIVEKIT_PUBLIC_URL`：LiveKit 用独立域名对外，或者用 LiveKit Cloud。

## 配置文件长什么样

`deploy/livekit/livekit.yaml`：

```yaml
port: 7880

rtc:
  tcp_port: 7882
  udp_port: 7882
  # 生产必须为 true：让 LiveKit 用 STUN 探测到的公网 IP 作为 ICE 候选，
  # 否则容器内网 IP 会发给浏览器，NAT 后连不上。
  use_external_ip: true
  stun_servers:
    - stun.l.google.com:19302

keys: {}

turn:
  enabled: true
  udp_port: 3478

webhook:
  urls:
    - http://gospeak:8998/api/v1/signal/webhook
```

几个关键项：

| 配置 | 说明 |
|------|------|
| `port` | 信令端口，只在容器内网使用，不需要发布到宿主机 |
| `rtc.tcp_port` / `udp_port` | 媒体端口。TCP 和 UDP 用同一个数字是正常的 |
| `use_external_ip` | 生产保持 `true`：由 LiveKit 用 STUN 探测公网 IP 作为 ICE 候选。云服务器有多网卡时容易探错，那就设 `false` 并手动指定 `node_ip` |
| `keys` | 保持 `{}`，密钥由环境变量注入 |
| `turn` | NAT 打不通时靠它中继 |
| `webhook` | 房间事件回调到应用，用于同步成员状态 |

## 端口

| 端口 | 协议 | 用途 | 要开公网吗 |
|------|------|------|-----------|
| 7880 | TCP | 信令（WebSocket） | **不要**，只绑宿主机回环供本机调试 |
| 7882 | TCP | 媒体（UDP 不通时兜底） | 要 |
| 7882 | UDP | 媒体（主要走这个） | 要 |
| 3478 | UDP | TURN 中继 | 建议开 |

**7880 / 7881 不对公网发布。** 7880 在 compose 里只绑宿主机回环（`127.0.0.1:7880`），供本机 `pnpm dev` 调试直连；对外信令经应用同源的 `/livekit` 反代进入容器内网，公网看不到它。7881 不再发布。

**媒体端口不开，语音一定不通。** 页面能进、能登录、能看到成员，就是没声音——基本都是这个原因。

云服务器记得检查两层防火墙：系统内的（ufw / firewalld）和云厂商控制台的安全组。

## 不用 Compose 单独跑

单独跑 LiveKit 容器时，**不要**把 7880 发布到公网。让应用和 LiveKit 在同一个 Docker 网络里，应用用服务名访问它：

```bash
docker network create gospeak

docker run -d \
  --name livekit \
  --network gospeak \
  --restart unless-stopped \
  -p 7882:7882 \
  -p 7882:7882/udp \
  -p 3478:3478/udp \
  -v $(pwd)/livekit.yaml:/etc/livekit.yaml \
  livekit/livekit-server:latest \
  --config /etc/livekit.yaml
```

然后应用的 `LIVEKIT_HOST` 填 `ws://livekit:7880`（同一网络内的服务名）。

如果应用跑在宿主机上（`pnpm dev`），需要再加一行把信令端口绑到回环：`-p 127.0.0.1:7880:7880`，然后 `LIVEKIT_HOST=ws://localhost:7880`。注意是 `127.0.0.1:` 前缀，**不要**写成 `-p 7880:7880`（那会对公网开放）。

如果 LiveKit 在另一台机器上，那就只能把 7880 暴露出来，此时 `LIVEKIT_HOST` 填那台的地址；这种情况下你还需要自己保证这段链路的 TLS（例如在反代后面），`LIVEKIT_PUBLIC_URL` 也要显式配置。

注意容器里的 `localhost` 是容器自己，跨容器访问要用服务名或宿主机 IP。

## 公网 IP 探测出错怎么办

云服务器常见问题：机器有多块网卡（公网 + 内网），LiveKit 探到了内网地址，客户端拿到这个地址后连不上。

症状是：能看到成员列表，但音频一直连不上，浏览器控制台报 ICE 失败。

解决：手动指定

```yaml
rtc:
  use_external_ip: false
  node_ip: 你的公网IP
```

改完 `docker compose restart livekit`。

## 密钥怎么生成

```bash
openssl rand -hex 32
```

生成两个，一个当 key 一个当 secret。或者直接跑 `./init.sh --force` 让它自动换。

密钥只在 `deploy/.env` 一处，compose 会注入 LiveKit 容器，不需要改 `livekit.yaml`。

## HTTPS

**不用为 LiveKit 单独做 HTTPS。** 页面是 HTTPS 的时候，浏览器连的是 `wss://<你的域名>/livekit`，这一跳的 TLS 由 Caddy 或你自己的反代终结；应用内部到 LiveKit 是容器内网的明文 `ws://livekit:7880`。

所以：

- 不需要在 `livekit.yaml` 里配 `cert_file` / `key_file`
- 不需要给 LiveKit 单独申请证书
- 不需要在反代里给 LiveKit 写 `location`

你要做的只是把站点本身的 HTTPS 配好：

- 有域名：`COMPOSE_PROFILES=app,livekit,edge`，Caddy 自动申请证书
- 已有反代：反代指向 `gospeak:8998` 即可

详见 [反代与 HTTPS](/deploy/nginx)。

## 扩展

单节点能撑几十到上百人，具体看服务器。人再多有两个方向：

**垂直扩展**：加 CPU 和带宽。音频转发的瓶颈通常是带宽不是 CPU。

**多节点**：LiveKit 支持集群，但需要 Redis 做节点间状态同步。这个 GOSpeak 的部署清单不管，得自己按 LiveKit 文档配。

## 排查

**LiveKit 起来了但应用连不上**

```bash
docker compose logs livekit
# 从应用容器内部探（7880 只绑宿主机回环，容器内用服务名）
docker compose exec gospeak wget -qO- http://livekit:7880
```

**密钥不一致**

密钥只在 `.env` 一处，确认注入是否生效：

```bash
grep LIVEKIT_KEY deploy/.env
docker compose config | grep -E "LIVEKIT_KEYS|LIVEKIT_WEBHOOK_API_KEY"
```

对不上就重新跑 `./init.sh --force`。

**语音断断续续**

带宽不够，或者 UDP 丢包严重。`docker stats` 看 LiveKit 的网络 IO，接近带宽上限就该升级了。

**TURN 相关报错**

NAT 类型太严格。确认 `3478/udp` 开了，并且 `turn.enabled: true`。

**语音连不上，但页面和登录都正常**

看浏览器 Network 面板里 `/api/v1/signal/token` 返回的 `serverUrl`。正常应是 `wss://<你的域名>/livekit`。如果是 `ws://` 而页面是 HTTPS，说明反代没传 `X-Forwarded-Proto`；如果连 `/livekit` 都 404，说明反代没把这个路径转到 `8998`。

## 下一步

- [语音服务](/sfu/) — 整体架构
- [LiveKit Cloud](/sfu/cloud) — 不想运维的话
- [故障排查](/deploy/troubleshooting) — 按症状查
