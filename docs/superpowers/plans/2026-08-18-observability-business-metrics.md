# GOSpeak 可观测性 业务指标 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 `internal/metrics` 包增加业务级计数器（房间创建、消息发送、鉴权失败、OAuth 失败、SFU 错误），在对应 handler 关键路径上报，并在 Grafana 面板追加查询，补齐「事件密集区」的可观测信号。

**Architecture:** 复用 `internal/metrics.Server`：新增 5 个 `CounterVec` 并在 `New()` 中注册；`New()` 同时把实例存入包级单例 `defaultServer`，handler 通过 `metrics.Default().IncXxx()` 上报（避免改动所有 handler 构造签名）。Grafana 在 `gospeak-overview.json` 增加一个 Business Events 面板。

**Tech Stack:** prometheus/client_golang（已依赖）；Gin handlers。

---

## 背景 / 现状

- `app/server/internal/metrics/metrics.go` 已暴露基础设施 + WS 概要指标（`gospeak_ws_*`、`gospeak_db_*` 等），但无业务事件计数。
- handler 不持有 `metrics.Server` 实例，需在 `gin.go` 的 `metrics.New(...)` 处建立单例供 handler 调用。
- Grafana `gospeak-overview.json` 已有 HTTP / P99 / 语音会话 / 集群 / 依赖 / 磁盘 / 日志面板，缺业务事件。

---

### Task 1: 指标定义与单例

**Files:**
- Modify: `app/server/internal/metrics/metrics.go`

- [ ] **Step 1: 在 Server 结构体追加计数器字段**

在 `Server` 结构体的 `cpuPercent prometheus.Gauge` 字段之后追加：

```go
	roomCreated   *prometheus.CounterVec
	messageSent   *prometheus.CounterVec
	authFailure   *prometheus.CounterVec
	oauthFailure  *prometheus.CounterVec
	sfuErrors     *prometheus.CounterVec
```

- [ ] **Step 2: 在 New() 中构造并注册**

在 `New()` 的 `cpuPercent: prometheus.NewGauge(...)` 之后追加：

```go
		roomCreated: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gospeak_room_created_total",
			Help: "Rooms created.",
		}, []string{}),
		messageSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gospeak_message_sent_total",
			Help: "Text messages sent.",
		}, []string{}),
		authFailure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gospeak_auth_failure_total",
			Help: "Auth failures by reason.",
		}, []string{"reason"}),
		oauthFailure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gospeak_oauth_failure_total",
			Help: "OAuth callback failures by provider.",
		}, []string{"provider"}),
		sfuErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gospeak_sfu_error_total",
			Help: "SFU operation errors by provider and op.",
		}, []string{"provider", "op"}),
```

在 `s.MustRegister(...)` 调用中追加这 5 个计数器：

```go
		s.roomCreated,
		s.messageSent,
		s.authFailure,
		s.oauthFailure,
		s.sfuErrors,
```

- [ ] **Step 3: 增加单例与 Inc 方法**

在 `New()` 函数返回前、文件末尾或 `New()` 上方增加包级单例（放在 `New` 之前）：

```go
// defaultServer 缓存最近一次 New 创建的指标服务，供 handler 在非构造路径上上报业务指标。
var defaultServer *Server

// Default 返回全局指标服务单例；未初始化时返回 nil，调用方需 nil 保护。
func Default() *Server { return defaultServer }
```

在 `New()` 内 `s.registry.MustRegister(...)` 之后、`return s` 之前设置单例：

```go
	defaultServer = s
```

在 `Server` 的 `Middleware()` 方法附近追加上报方法（nil 安全）：

```go
func (s *Server) IncRoomCreated() {
	if s == nil {
		return
	}
	s.roomCreated.WithLabelValues().Inc()
}

func (s *Server) IncMessageSent() {
	if s == nil {
		return
	}
	s.messageSent.WithLabelValues().Inc()
}

func (s *Server) IncAuthFailure(reason string) {
	if s == nil {
		return
	}
	s.authFailure.WithLabelValues(reason).Inc()
}

func (s *Server) IncOAuthFailure(provider string) {
	if s == nil {
		return
	}
	s.oauthFailure.WithLabelValues(provider).Inc()
}

func (s *Server) IncSFUError(provider, op string) {
	if s == nil {
		return
	}
	s.sfuErrors.WithLabelValues(provider, op).Inc()
}
```

- [ ] **Step 4: 编译验证**

Run: `cd app/server && go build ./...`
Expected: 编译通过，5 个计数器注册到同一 registry。

- [ ] **Step 5: 提交**

```bash
git add app/server/internal/metrics/metrics.go
git commit -m "observability: add business metric counters + singleton"
```

---

### Task 2: handler 接线

**Files:**
- Modify: `app/server/internal/handler/room_handler.go` (Create 成功路径)
- Modify: `app/server/internal/handler/message_handler.go` (Send 成功路径)
- Modify: `app/server/internal/handler/auth_handler.go` (Login 失败路径)
- Modify: `app/server/internal/handler/oauth_handler.go` (Callback 失败路径)
- Modify: `app/server/internal/handler/sfu_config_handler.go` (SwitchProvider/Update 失败路径)

- [ ] **Step 1: 房间创建成功上报**

在 `RoomHandler.Create` 成功 `pkg.Success(c, room)` 之前追加：

```go
metrics.Default().IncRoomCreated()
```

- [ ] **Step 2: 消息发送成功上报**

在 `MessageHandler.Send` 成功 `pkg.Success(c, msg)` 之前追加：

```go
metrics.Default().IncMessageSent()
```

- [ ] **Step 3: 鉴权失败上报**

在 `AuthHandler.Login` 的失败分支（`pkg.HandleError(c, err)` 或返回错误处）追加：

```go
metrics.Default().IncAuthFailure("login")
```

- [ ] **Step 4: OAuth 失败上报**

在 `OAuthHandler.Callback` 的失败分支追加：

```go
metrics.Default().IncOAuthFailure(provider)
```

- [ ] **Step 5: SFU 错误上报**

在 `SFUConfigHandler.SwitchProvider` 与 `Update` 的失败分支（`pkg.HandleError` 之前）追加：

```go
metrics.Default().IncSFUError(provider, "switch")
// 或 "update" 对应 Update 方法
```

- [ ] **Step 6: 编译验证**

Run: `cd app/server && go build ./...`
Expected: 编译通过；各 handler 调用 `metrics.Default().IncXxx()`（函数已在 Task 1 定义）。

- [ ] **Step 7: 提交**

```bash
git add app/server/internal/handler/room_handler.go app/server/internal/handler/message_handler.go app/server/internal/handler/auth_handler.go app/server/internal/handler/oauth_handler.go app/server/internal/handler/sfu_config_handler.go
git commit -m "observability: emit business metrics from handlers"
```

---

### Task 3: Grafana 业务事件面板

**Files:**
- Modify: `deploy/observability/grafana/dashboards/gospeak-overview.json`

- [ ] **Step 1: 追加 Business Events 面板**

在 `gospeak-overview.json` 的 `panels` 数组内追加一个面板对象（放在 `Application Logs` 面板之前或之后均可）：

```json
{
  "title": "Business Events",
  "type": "timeseries",
  "datasource": { "type": "prometheus", "uid": "prometheus" },
  "gridPos": { "h": 8, "w": 24, "x": 0, "y": 24 },
  "targets": [
    { "expr": "sum(rate(gospeak_room_created_total[5m]))", "legendFormat": "rooms/s" },
    { "expr": "sum(rate(gospeak_message_sent_total[5m]))", "legendFormat": "msgs/s" },
    { "expr": "sum(rate(gospeak_auth_failure_total[5m]))", "legendFormat": "auth fail/s" },
    { "expr": "sum(rate(gospeak_oauth_failure_total[5m]))", "legendFormat": "oauth fail/s" },
    { "expr": "sum(rate(gospeak_sfu_error_total[5m])) by (provider)", "legendFormat": "{{provider}} sfu err/s" }
  ]
}
```

- [ ] **Step 2: 校验 JSON 合法**

Run: `python3 -c "import json,sys; json.load(open('deploy/observability/grafana/dashboards/gospeak-overview.json')); print('OK')"`
Expected: 打印 `OK`。

- [ ] **Step 3: 提交**

```bash
git add deploy/observability/grafana/dashboards/gospeak-overview.json
git commit -m "observability: add business events panel to Grafana"
```

---

### Task 4: 指标测试

**Files:**
- Modify: `app/server/internal/metrics/metrics_test.go`

- [ ] **Step 1: 写计数器测试**

在 `app/server/internal/metrics/metrics_test.go` 追加（同包可访问未导出字段）：

```go
func TestBusinessCounters(t *testing.T) {
	srv := New(nil)
	srv.IncRoomCreated()
	srv.IncAuthFailure("login")

	if v := testutil.ToFloat64(srv.roomCreated.WithLabelValues()); v != 1 {
		t.Fatalf("room_created expected 1, got %v", v)
	}
	if v := testutil.ToFloat64(srv.authFailure.WithLabelValues("login")); v != 1 {
		t.Fatalf("auth_failure expected 1, got %v", v)
	}
}
```

- [ ] **Step 2: 运行确认通过**

Run: `cd app/server && go test ./internal/metrics/ -run TestBusinessCounters -v`
Expected: PASS。

- [ ] **Step 3: 提交**

```bash
git add app/server/internal/metrics/metrics_test.go
git commit -m "observability: test business metric counters"
```

---

## 自检

1. 规格覆盖：指标定义（Task 1）、handler 接线（Task 2）、Grafana 面板（Task 3）、测试（Task 4）均已覆盖。
2. 占位符扫描：无 TBD/TODO；handler 接线复用 Task 1 定义的 `metrics.Default().IncXxx()`，函数已给出完整实现。
3. 类型一致性：`IncXxx` 方法的 label 维度（`reason`/`provider`/`provider,op`）与 `CounterVec` 构造时的 `[]string{...}` 完全一致；`Default()` 返回 `*Server` 与 `IncXxx` 接收者一致。
