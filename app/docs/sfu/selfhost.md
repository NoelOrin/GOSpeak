# 自建 LiveKit

自建 LiveKit 使数据完全自持，且无按量计费成本，代价是需自行承担运维。

**运维中最复杂的部分（域名、证书、反向代理）无需处理**：GOSpeak 应用内置同源反代 `/livekit`，浏览器信令走现有站点域名，LiveKit 不需要第二个域名，也不需要自己的证书。

## Compose 部署（推荐）

`deploy/` 已完成配置：

```bash
cd deploy
./init.sh --domain voice.example.com
docker compose up -d
```

`./init.sh --domain voice.example.com` 写入域名并将 `edge` 加入 `COMPOSE_PROFILES`（已有 `app,livekit` 时追加为 `app,livekit,edge`）。执行后直接运行 `docker compose up -d`，无需手动调整档位。

**不启用 `edge` 的场景**：已有反向代理（Nginx / Traefik / Dokploy / 宝塔），或纯局域网无域名时，**不传 `--domain`**（或手动将 `edge` 从 `COMPOSE_PROFILES` 中移除），反代指向宿主机 `8998`。

`init.sh` 执行三项与 LiveKit 相关的操作：

1. 随机生成 API Key 与 Secret，写入 `deploy/.env`
2. 归一 `livekit/livekit.yaml`，剥离硬编码密钥
3. 校验 `.env` / `livekit.yaml` / `Caddyfile` 一致性

公网 IP 由 LiveKit 通过 `rtc.use_external_ip: true` 配合 STUN 探测，`init.sh` 不处理该项。

密钥仅在 `deploy/.env` 中保存一份，由 compose 注入 LiveKit 容器，`livekit.yaml` 不含任何凭据，因此不存在「两侧 key 不一致」的问题。

## 信令连接：同源 `/livekit`

同源反代是自建 LiveKit 部署的核心简化点。

```
浏览器 ──wss://<你的域名>/livekit──► gospeak:8998 ──► livekit:7880（容器内网）
浏览器 ──WebRTC(UDP)────────────────────────────────► livekit:7882（媒体直连）
```

- 应用内置一个同源反代：`/livekit/*` 转发到内部 LiveKit 的 7880，**WebSocket 升级自动处理**。
- `LIVEKIT_PUBLIC_URL` **默认留空**。留空时，`POST /api/v1/signal/token` 返回的 `serverUrl` 由请求本身推导：
  - 浏览器打开 `https://voice.example.com` → `serverUrl = wss://voice.example.com/livekit`
  - scheme 取 `X-Forwarded-Proto`，缺省按是否 TLS 判断
  - livekit-client 会自动在这个地址后拼 `/rtc/v1`，经路径前缀反代握手正常

结论：

- **LiveKit 不需要域名。**
- **LiveKit 不需要证书。** TLS 在 Caddy 或自有反代上终结一次即可。
- **LiveKit 不需要单独的反代规则。** 反代只面对 `gospeak:8998` 一个上游。

仅以下两种情况需显式填写 `LIVEKIT_PUBLIC_URL`：LiveKit 使用独立域名对外，或使用 LiveKit Cloud。

## 配置文件说明

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

关键配置项：

| 配置 | 说明 |
|------|------|
| `port` | 信令端口，仅在容器内网使用，无需发布到宿主机 |
| `rtc.tcp_port` / `udp_port` | 媒体端口。TCP 与 UDP 使用同一数字属正常配置 |
| `use_external_ip` | 生产环境保持 `true`：由 LiveKit 用 STUN 探测公网 IP 作为 ICE 候选。云服务器存在多网卡时可能探测错误，此时设 `false` 并手动指定 `node_ip` |
| `keys` | 保持 `{}`，密钥由环境变量注入 |
| `turn` | NAT 无法打通时经其中继 |
| `webhook` | 房间事件回调至应用，用于同步成员状态 |

## 端口

| 端口 | 协议 | 用途 | 是否开放公网 |
|------|------|------|-------------|
| 7880 | TCP | 信令（WebSocket） | **禁止**，仅绑宿主机回环供本机调试 |
| 7882 | TCP | 媒体（UDP 不通时兜底） | 必须 |
| 7882 | UDP | 媒体（主通道） | 必须 |
| 3478 | UDP | TURN 中继 | 建议开放 |

**7880 / 7881 不对公网发布。** 7880 在 compose 中仅绑宿主机回环（`127.0.0.1:7880`），供本机 `pnpm dev` 调试直连；对外信令经应用同源的 `/livekit` 反代进入容器内网，公网不可见。7881 已不再发布。

**媒体端口未开放时语音必然不通。** 页面可进入、可登录、可见成员但无声音，通常由此导致。

云服务器需检查两层防火墙：系统内防火墙（ufw / firewalld）与云厂商控制台的安全组。

## 非 Compose 独立运行

单独运行 LiveKit 容器时，**禁止**将 7880 发布到公网。应用与 LiveKit 处于同一 Docker 网络时，应用使用服务名访问：

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

此时应用侧 `LIVEKIT_HOST` 填 `ws://livekit:7880`（同一网络内的服务名）。

应用运行在宿主机（`pnpm dev`）时，需增加一行将信令端口绑定到回环：`-p 127.0.0.1:7880:7880`，然后 `LIVEKIT_HOST=ws://localhost:7880`。注意必须使用 `127.0.0.1:` 前缀，**不可**写成 `-p 7880:7880`（将对公网开放）。

LiveKit 位于另一台机器时，只能将 7880 对外暴露，`LIVEKIT_HOST` 填该机地址；此情况下需自行保证该链路的 TLS（例如置于反代之后），`LIVEKIT_PUBLIC_URL` 也须显式配置。

容器内的 `localhost` 指容器自身，跨容器访问须使用服务名或宿主机 IP。

## 公网 IP 探测失败处理

云服务器常见问题：机器存在多块网卡（公网 + 内网）时，LiveKit 探测到内网地址，客户端使用该地址无法连接。

症状：可见成员列表，但音频持续无法连接，浏览器控制台报 ICE 失败。

处理方式：手动指定

```yaml
rtc:
  use_external_ip: false
  node_ip: 你的公网IP
```

修改后执行 `docker compose restart livekit`。

## 密钥生成

```bash
openssl rand -hex 32
```

生成两组，分别作为 key 与 secret。也可直接运行 `./init.sh --force` 自动更换。

密钥仅在 `deploy/.env` 一处保存，compose 注入 LiveKit 容器，无需修改 `livekit.yaml`。

## HTTPS

**无需为 LiveKit 单独配置 HTTPS。** 页面为 HTTPS 时，浏览器连接的是 `wss://<你的域名>/livekit`，该跳的 TLS 由 Caddy 或自有反代终结；应用内部到 LiveKit 为容器内网明文 `ws://livekit:7880`。

因此：

- 无需在 `livekit.yaml` 中配置 `cert_file` / `key_file`
- 无需为 LiveKit 单独申请证书
- 无需在反代中为 LiveKit 编写 `location`

仅需配置站点本身的 HTTPS：

- 有域名：`COMPOSE_PROFILES=app,livekit,edge`，Caddy 自动申请证书
- 已有反代：反代指向 `gospeak:8998` 即可

详见 [反代与 HTTPS](/deploy/nginx)。

## 扩展

单节点可支撑数十至上百人，取决于服务器规格。规模更大时有两个方向：

**垂直扩展**：增加 CPU 与带宽。音频转发的瓶颈通常在带宽而非 CPU。

**多节点**：LiveKit 支持集群，但需 Redis 做节点间状态同步。GOSpeak 的部署清单不包含此项，需按 LiveKit 文档自行配置。

## 排查

**LiveKit 已运行但应用无法连接**

```bash
docker compose logs livekit
# 从应用容器内部探（7880 只绑宿主机回环，容器内用服务名）
docker compose exec gospeak wget -qO- http://livekit:7880
```

**密钥不一致**

密钥仅在 `.env` 一处，确认注入是否生效：

```bash
grep LIVEKIT_KEY deploy/.env
docker compose config | grep -E "LIVEKIT_KEYS|LIVEKIT_WEBHOOK_API_KEY"
```

不一致时重新运行 `./init.sh --force`。

**语音断续**

带宽不足或 UDP 丢包严重。`docker stats` 查看 LiveKit 的网络 IO，接近带宽上限时需升级。

**TURN 相关报错**

NAT 类型限制过严。确认 `3478/udp` 已开放，且 `turn.enabled: true`。

**语音无法连接，页面与登录正常**

查看浏览器 Network 面板中 `/api/v1/signal/token` 返回的 `serverUrl`。正常值应为 `wss://<你的域名>/livekit`。若为 `ws://` 而页面是 HTTPS，说明反代未传递 `X-Forwarded-Proto`；若 `/livekit` 返回 404，说明反代未将该路径转发至 `8998`。

## 下一步

- [语音服务](/sfu/) — 整体架构
- [LiveKit Cloud](/sfu/cloud) — 云服务方案
- [故障排查](/deploy/troubleshooting) — 按症状排查
