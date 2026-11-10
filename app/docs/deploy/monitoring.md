# 监控

想看到「现在多少人在线、开了几个房间、服务健康不健康」，就把 `observability` 档位打开。

## 打开它

```ini
COMPOSE_PROFILES=app,livekit,observability
```

```bash
docker compose up -d
```

起来之后能访问：

| 服务 | 地址 | 默认账号 |
|------|------|---------|
| Prometheus | `http://服务器IP:9090` | — |
| Grafana | `http://服务器IP:3000` | `admin` / `admin` |
| Alertmanager | `http://服务器IP:9093` | — |
| Loki | `http://服务器IP:3100` | — |

**这些端口不要直接暴露到公网。** 要么绑 127.0.0.1 走 SSH 隧道，要么在反代（Caddy / Nginx）上加 basic auth。

Grafana 第一次登录会强制改密码。

## 里面有什么

### 预置的仪表盘

Grafana 打开就有一个 GOSpeak Overview，包含：

- HTTP 请求量、延迟、错误率
- WebSocket 连接数
- 在线用户数
- 房间数和房间内人数
- 数据库连接状态

不用自己拼面板。

### 指标从哪来

应用在 `/metrics` 暴露 Prometheus 格式的指标，Prometheus 定时抓取。

指标分几类：

| 前缀 | 内容 |
|------|------|
| `gospeak_http_*` | 请求总数、耗时分布、状态码 |
| `gospeak_ws_*` | WebSocket 连接数、房间数、在线用户、掉线数 |
| `gospeak_db_*` | 数据库连接状态 |
| `go_*` / `process_*` | Go 运行时和进程自身指标 |

### 日志

Promtail 自动采集所有容器的 stdout，解析成结构化字段送进 Loki。在 Grafana 里可以按级别、容器名筛选。

前提是应用日志输出成 JSON：

```ini
LOG_FORMAT=json
```

不设的话是纯文本，Loki 里只能全文搜，没法按字段过滤。

### 告警

规则在 `deploy/observability/prometheus/rules.yml`，默认几条：

- 服务挂了（抓不到指标）
- 错误率过高
- 数据库连接失败

告警发到 Alertmanager，接收方在 `deploy/observability/alertmanager/alertmanager.yml` 里配。默认没配任何接收器，要自己填 webhook / 邮件 / Slack。

## 保护 `/metrics`

默认 `/metrics` 是公开的，任何人都能抓。生产环境建议加个令牌：

```ini
METRICS_TOKEN=<随机字符串>
```

设了之后请求必须带 `Authorization: Bearer <token>`。

更稳妥的做法是在反代或防火墙层面直接挡掉外网访问。用自有 Nginx 时：

```nginx
location /metrics {
    allow 127.0.0.1;
    allow 172.16.0.0/12;   # Docker 网络
    deny all;
    proxy_pass http://gospeak_backend;
}
```

## 不开这套东西怎么监控

不想跑四个容器的话，最低成本的方案：

```bash
# 健康检查，放个 cron 或 uptime 监控
curl -f http://localhost:8998/ping
```

`/ping` 只检查进程活着，不碰数据库。`/readyz` 会真的去 ping 一下数据库，数据库挂了返回 503——做负载均衡健康检查时用这个。

想看指标又不想跑 Prometheus，可以直接 `curl /metrics` 看原始数据，虽然不好看但能看。

## 常见问题

**Grafana 里没数据**
Prometheus 抓不到应用。进 Prometheus 的 Targets 页面看 `gospeak` 那个 target 的状态，通常是网络不通或者应用没起。

**日志面板是空的**
应用没输出 JSON 格式日志。设 `LOG_FORMAT=json` 重启。

**告警一直不发**
Alertmanager 里没配接收器，或者配了但没重启。看 `docker compose logs alertmanager`。

**磁盘被日志撑满**
Loki 默认保留期可能太长。改 `deploy/observability/loki/loki-config.yml` 里的 `retention_period`。

## 下一步

- [故障排查](/deploy/troubleshooting) — 不看监控也能查问题
- [配置项](/guide/configuration) — 日志相关环境变量
