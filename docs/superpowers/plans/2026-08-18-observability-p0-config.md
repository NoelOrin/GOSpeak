# GOSpeak 可观测性 P0 配置快赢 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐可观测性的两个剩余 critical 缺口：告警通知落地（Alertmanager 接 webhook）与日志结构化解析（Promtail JSON pipeline_stages），并把 `METRICS_TOKEN` 暴露到主 env 且生产环境为空时打印告警。

**Architecture:** 纯配置改动 + 一处启动期日志告警（无业务代码改动）。Alertmanager 用静态 webhook receiver（运维按部署替换 URL）；Promtail 用 `pipeline_stages` 把 logrus JSON 日志解析出 `level`/`component`/`time` 成 Loki label；`METRICS_TOKEN` 已在 `config.go` 存在，只需在 env 模板暴露并在 release 模式空值时告警。

**Tech Stack:** Promtail 3.4 / Loki / Alertmanager 0.28（部署配置）；Go + logrus（JSON 日志）；Gin（启动告警）。

---

## 背景 / 现状（来自可观测性评估）

- `deploy/observability/alertmanager/alertmanager.yml` 的 `default` receiver 只有注释示例，告警触发后无人接收。
- `deploy/observability/promtail/promtail-config.yml` 仅有 docker service discovery，无 `pipeline_stages`，logrus 的 JSON 日志未被结构化，Loki 只能全文检索。
- `METRICS_TOKEN` 配置字段已在 `config.go:124` 定义，但只出现在 `deploy/env/app.livekit.env.example:40` 与 `app.srs.env.example:47`（注释态），主 env 流程未暴露；`/metrics` 默认公开可读。

---

### Task 1: Alertmanager 接入通知通道

**Files:**
- Modify: `deploy/observability/alertmanager/alertmanager.yml`
- Create: `deploy/observability/alertmanager/templates/feishu.tmpl`
- Modify: `deploy/DEPLOY.md`

- [ ] **Step 1: 写告警路由 + webhook receiver（含飞书卡片模板）**

`deploy/observability/alertmanager/alertmanager.yml` 全文替换为：

```yaml
global:
  resolve_timeout: 5m

route:
  group_by: ['alertname', 'instance']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: default

receivers:
  - name: default
    webhook_configs:
      # 替换为你的飞书/钉钉/Slack/Discord 入站 webhook URL
      - url: 'https://example.com/hooks/gospeak'
        send_resolved: true
        http_config:
          # 需要鉴权时在 deploy/env 中管理 secret，不要提交明文
          basic_auth:
            username: gospeak
            password: '__ALERT_WEBHOOK_TOKEN__'
    # 飞书富文本卡片（可选）：解开下面这段并挂载 templates 目录
    # webhook_configs:
    #   - url: 'https://open.feishu.cn/open-apis/bot/v2/hook/__FEISHU_BOT_ID__'
    #     send_resolved: true
    # templates:
    #   - /etc/alertmanager/templates/feishu.tmpl
```

`deploy/observability/alertmanager/templates/feishu.tmpl`：

```text
{{ define "feishu.card" }}
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": { "tag": "plain_text", "content": "GOSpeak 告警: {{ .CommonLabels.severity }}" }
    },
    "elements": [
      { "tag": "div", "text": { "tag": "lark_md", "content": "**{{ .CommonLabels.alertname }}**\n{{ .CommonAnnotations.description }}" } }
    ]
  }
}
{{ end }}
```

- [ ] **Step 2: 在 DEPLOY.md 标注告警配置**

在 `deploy/DEPLOY.md` 的 Observability 小节追加：

```markdown
#### 告警通知
- Alertmanager 默认 receiver 为 `default`，webhook URL 写在 `deploy/observability/alertmanager/alertmanager.yml`。
- 生产部署请把 `https://example.com/hooks/gospeak` 替换为真实入站 webhook（飞书/钉钉/Slack/Discord），不要提交 `basic_auth.password` 明文，改用 secret 挂载。
- 现有 `rules.yml` 已含进程宕机 / 5xx / p99 延迟 / EventBus / DB / 副本延迟 / 集群离线 / 磁盘 共 9 条规则。
```

- [ ] **Step 3: 校验 YAML 合法**

Run: `docker compose -f deploy/docker-compose.yml --profile observability config >/dev/null && echo OK`
Expected: 打印 `OK`（无 YAML 解析错误）。

- [ ] **Step 4: 提交**

```bash
git add deploy/observability/alertmanager/ deploy/DEPLOY.md
git commit -m "observability: wire Alertmanager webhook receiver + Feishu template"
```

---

### Task 2: Promtail JSON pipeline_stages

**Files:**
- Modify: `deploy/observability/promtail/promtail-config.yml`

- [ ] **Step 1: 给 docker scrape job 增加结构化解析**

`deploy/observability/promtail/promtail-config.yml` 全文替换为（`pipeline_stages` 解析 logrus JSON 的 `level`/`component`/`time`/`msg`）：

```yaml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: docker
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
    relabel_configs:
      - source_labels: ["__meta_docker_container_name"]
        regex: "/(.*)"
        target_label: container
      - source_labels: ["__meta_docker_container_log_stream"]
        target_label: stream
      - source_labels: ["__meta_docker_container_label_com_docker_compose_service"]
        target_label: compose_service
      - source_labels: ["__meta_docker_container_label_com_docker_compose_project"]
        target_label: compose_project
    pipeline_stages:
      # 解析 docker 日志信封，得到应用原始日志行
      - docker: {}
      # 解析 logrus JSON：level/time/component/msg 为顶层字段
      - json:
          expressions:
            level: level
            component: component
            time: time
            msg: msg
      # 把 level / component 提升为 Loki label，可按 level 过滤聚合
      - labels:
          level:
          component:
      # 用应用自身时间戳作为日志时间，避免容器时间漂移
      - timestamp:
          source: time
          format: RFC3339Nano
```

- [ ] **Step 2: 校验日志可被结构化解析**

Run: `docker compose -f deploy/docker-compose.yml --profile observability up -d promtail loki && sleep 5`
Expected: `docker compose logs promtail | grep -i "level=info"` 无 `pipeline_stages` 解析报错；Grafana 中 `{compose_service=~"gospeak.*"} | json` 能解析出 `level` label。

- [ ] **Step 3: 提交**

```bash
git add deploy/observability/promtail/promtail-config.yml
git commit -m "observability: parse logrus JSON logs in Promtail pipeline_stages"
```

---

### Task 3: METRICS_TOKEN 暴露与生产告警

**Files:**
- Modify: `deploy/env/app.livekit.env.example:40`
- Modify: `deploy/env/app.srs.env.example:47`
- Modify: `app/server/server/gin.go` (指标接线之后)
- Modify: `deploy/DEPLOY.md`

- [ ] **Step 1: 在 env 模板暴露 METRICS_TOKEN**

`deploy/env/app.livekit.env.example` 第 40 行由注释改为：

```bash
# /metrics 端点 Bearer 鉴权；生产必须设置，否则指标公开可读
METRICS_TOKEN=
```

`deploy/env/app.srs.env.example` 第 47 行同样改为上面内容。

- [ ] **Step 2: release 模式空 token 时打印告警**

在 `app/server/server/gin.go` 的 `r.GET("/metrics", gin.WrapH(metricsHandler))` 之后追加：

```go
// 生产环境 /metrics 默认公开，必须在 release 模式强制设置 METRICS_TOKEN。
if cfg.GinMode == "release" && cfg.MetricsToken == "" {
	logger.WithComponent("Metrics").Warn("METRICS_TOKEN is empty: /metrics endpoint is publicly readable; set METRICS_TOKEN in production")
}
```

- [ ] **Step 3: DEPLOY.md 标注**

在 `deploy/DEPLOY.md` Observability 小节追加：

```markdown
#### 指标鉴权
- 设置 `METRICS_TOKEN` 后，`/metrics` 需 `Authorization: Bearer <token>` 才能访问。
- release 模式未设置该值时启动会打印告警日志。
```

- [ ] **Step 4: 编译验证**

Run: `cd app/server && go build ./...`
Expected: 编译通过，无报错。

- [ ] **Step 5: 提交**

```bash
git add deploy/env/app.livekit.env.example deploy/env/app.srs.env.example app/server/server/gin.go deploy/DEPLOY.md
git commit -m "observability: expose METRICS_TOKEN and warn when unset in release"
```

---

## 自检

1. 规格覆盖：告警通知（Task 1）、日志结构化（Task 2）、指标鉴权（Task 3）均已覆盖；均为评估中 Yellow/Red 项。
2. 占位符扫描：无 TBD/TODO；webhook URL 与 password 明确标注为部署期替换项（配置值本身即部署相关，非代码占位）。
3. 类型一致性：复用既有 `cfg.MetricsToken`、`cfg.GinMode` 字段，与 `config.go` 定义一致。
