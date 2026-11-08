# 自建 LiveKit

自己跑 LiveKit，数据全在自己手里，也不按用量花钱。代价是要管运维。

## 用 Compose 一把梭（推荐）

`deploy/` 里已经配好了：

```bash
cd deploy
./init.sh
docker compose up -d
```

`init.sh` 会做三件和 LiveKit 有关的事：

1. 随机生成 API Key 和 Secret
2. 探测公网 IP，写进 candidate 配置
3. 把密钥同时写进 `deploy/.env` 和 `livekit/livekit.yaml`

第 3 步不能省——应用的密钥和 LiveKit 的密钥必须一致，否则 webhook 验签失败、令牌也签不对。

## 配置文件长什么样

`deploy/livekit/livekit.yaml`：

```yaml
port: 7880

rtc:
  tcp_port: 7882
  udp_port: 7882
  use_external_ip: false
  stun_servers:
    - stun.l.google.com:19302

keys:
  你的key: 你的secret

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
| `port` | 信令端口，客户端连这个 |
| `rtc.tcp_port` / `udp_port` | 媒体端口。TCP 和 UDP 用同一个数字是正常的 |
| `use_external_ip` | 自动探测公网 IP。云服务器有多网卡时容易探错，那就设 `false` 手动指定 `node_ip` |
| `keys` | 必须和应用里的 `LIVEKIT_KEY` / `LIVEKIT_SECRET` 一致 |
| `turn` | NAT 打不通时靠它中继 |
| `webhook` | 房间事件回调到应用，用于同步成员状态 |

## 端口

| 端口 | 协议 | 用途 | 要开公网吗 |
|------|------|------|-----------|
| 7880 | TCP | 信令（WebSocket） | 要 |
| 7881 | TCP | 媒体 | 要 |
| 7882 | TCP | 媒体 | 要 |
| 7882 | UDP | 媒体（主要走这个） | 要 |
| 3478 | UDP | TURN 中继 | 建议开 |

**媒体端口不开，语音一定不通。** 页面能进、能登录、能看到成员，就是没声音——基本都是这个原因。

云服务器记得检查两层防火墙：系统内的（ufw / firewalld）和云厂商控制台的安全组。

## 不用 Compose 单独跑

```bash
docker run -d \
  --name livekit \
  --restart unless-stopped \
  -p 7880:7880 \
  -p 7881:7881 \
  -p 7882:7882 \
  -p 7882:7882/udp \
  -p 3478:3478/udp \
  -v $(pwd)/livekit.yaml:/etc/livekit.yaml \
  livekit/livekit-server:latest \
  --config /etc/livekit.yaml
```

然后应用的 `LIVEKIT_HOST` 指向它。

注意容器里的 `localhost` 是容器自己，跨容器访问要用宿主机 IP 或者 Docker 网络里的服务名。

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

改完两边都要改——`deploy/.env` 和 `livekit/livekit.yaml`。

## HTTPS

页面是 HTTPS 的时候，`LIVEKIT_HOST` 必须是 `wss://`。

两种做法：

**LiveKit 自己带证书**

```yaml
cert_file: /etc/livekit/cert.pem
key_file: /etc/livekit/key.pem
```

**Nginx 反代 7880**

```nginx
location /livekit/ {
    proxy_pass http://livekit:7880/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

然后 `LIVEKIT_HOST=wss://voice.example.com/livekit`。

注意这只加密了信令，媒体还是 UDP 直连。

## 扩展

单节点能撑几十到上百人，具体看服务器。人再多有两个方向：

**垂直扩展**：加 CPU 和带宽。音频转发的瓶颈通常是带宽不是 CPU。

**多节点**：LiveKit 支持集群，但需要 Redis 做节点间状态同步。这个 GOSpeak 的部署清单不管，得自己按 LiveKit 文档配。

## 排查

**LiveKit 起来了但应用连不上**

```bash
docker compose logs livekit
curl -s http://localhost:7880
```

从应用容器里测：

```bash
docker compose exec gospeak wget -qO- http://livekit:7880
```

**密钥不一致**

```bash
grep LIVEKIT deploy/.env
grep -A3 "^keys:" deploy/livekit/livekit.yaml
```

两边对不上就重新跑 `./init.sh --force`。

**语音断断续续**

带宽不够，或者 UDP 丢包严重。`docker stats` 看 LiveKit 的网络 IO，接近带宽上限就该升级了。

**TURN 相关报错**

NAT 类型太严格。确认 `3478/udp` 开了，并且 `turn.enabled: true`。

## 下一步

- [语音服务](/sfu/) — 整体架构
- [LiveKit Cloud](/sfu/cloud) — 不想运维的话
- [故障排查](/deploy/troubleshooting) — 按症状查
