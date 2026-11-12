# 监控

「现在多少人在线、开了几个房间、服务健康不健康」这类运行状态，通过打开 `observability` 档位来查看。

## 启用方式

```ini
COMPOSE_PROFILES=app,livekit,observability
```

```bash
docker compose up -d
```

启动后可以访问：

| 服务 | 地址 | 默认账号 |
|------|------|---------|
| Prometheus | `http://服务器IP:9090` | — |
| Grafana | `http://服务器IP:3000` | `admin` / `admin` |
| Alertmanager | `http://服务器IP:9093` | — |
| Loki | `http://服务器IP:3100` | — |

**这些端口不要直接暴露到公网。** 建议绑定 127.0.0.1 走 SSH 隧道，或者在反代（Caddy / Nginx）上加 basic auth。

Grafana 第一次登录会强制改密码。

## 组成部分

### 预置的仪表盘

Grafana 自带一个 GOSpeak Overview 仪表盘，包含：

- HTTP 请求量、延迟、错误率
- WebSocket 连接数
- 在线用户数
- 房间数和房间内人数
- 数据库连接状态

不需要自行拼装面板。

### 指标来源

应用在 `/metrics` 暴露 Prometheus 格式的指标，由 Prometheus 定时抓取。

指标分为几类：

| 前缀 | 内容 |
|------|------|
| `gospeak_http_*` | 请求总数、耗时分布、状态码 |
| `gospeak_ws_*` | WebSocket 连接数、房间数、在线用户、掉线数 |
| `gospeak_db_*` | 数据库连接状态 |
| `go_*` / `process_*` | Go 运行时和进程自身指标 |

### 日志

Promtail 自动采集所有容器的 stdout，解析为结构化字段写入 Loki。在 Grafana 中可按级别、容器名筛选。

前提是应用日志输出为 JSON：

```ini
LOG_FORMAT=json
```

未设置时是纯文本，Loki 里只能全文搜索，无法按字段过滤。

### 告警

规则在 `deploy/observability/prometheus/rules.yml`，默认几条：

- 服务不可达（指标抓取失败）
- 错误率过高
- 数据库连接失败

告警发到 Alertmanager，接收方在 `deploy/observability/alertmanager/alertmanager.yml` 里配置。默认没有配置任何接收器，需要自行填写 webhook / 邮件 / Slack。

## 保护 `/metrics`

默认 `/metrics` 是公开的，任何人都可以抓取。生产环境建议增加令牌：

```ini
METRICS_TOKEN=<随机字符串>
```

设置之后请求必须带 `Authorization: Bearer <token>`。

更稳妥的做法是在反代或防火墙层面直接挡掉外网访问。使用自有 Nginx 时：

```nginx
location /metrics {
    allow 127.0.0.1;
    allow 172.16.0.0/12;   # Docker 网络
    deny all;
    proxy_pass http://gospeak_backend;
}
```

## 未启用时的监控方案

不运行四个容器时，最低成本的方案：

```bash
# 健康检查，放个 cron 或 uptime 监控
curl -f http://localhost:8998/ping
```

`/ping` 只检查进程存活，不碰数据库。`/readyz` 会实际 ping 一次数据库，数据库不可用时返回 503——负载均衡健康检查使用这个端点。

需要指标但不需要 Prometheus 时，可以直接 `curl /metrics` 查看原始数据，可读性一般但内容完整。

## 常见问题

**Grafana 面板无数据**
Prometheus 无法抓取应用。在 Prometheus 的 Targets 页面查看 `gospeak` 那个 target 的状态，通常是网络不通或应用未启动。

**日志面板为空**
应用未输出 JSON 格式日志。设置 `LOG_FORMAT=json` 后重启。

**告警持续未发送**
Alertmanager 里未配置接收器，或者配置了但未重启。查看 `docker compose logs alertmanager`。

**磁盘被日志撑满**
Loki 默认保留期可能太长。修改 `deploy/observability/loki/loki-config.yml` 里的 `retention_period`。

## 下一步

- [故障排查](/deploy/troubleshooting) — 脱离监控环境排查问题
- [配置项](/guide/configuration) — 日志相关环境变量
