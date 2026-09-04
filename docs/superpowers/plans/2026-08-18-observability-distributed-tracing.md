# GOSpeak 可观测性 分布式链路追踪 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 接入 OpenTelemetry，覆盖 HTTP 请求（Gin）与关键服务调用，span 经 OTLP 导出到 Tempo，Grafana 增加 Tempo 数据源，形成「请求 → 业务 → SFU/NATS」的端到端追踪视图。

**Architecture:** 新增 `internal/trace` 包封装 TracerProvider 初始化/关闭（disabled 时返回 noop）；Gin 用 `otelgin` 中间件自动为每个 HTTP 请求建 span，span context 经 W3C `traceparent` 在进程内通过 `context.Context` 传播。跨实例 WS/NATS 的 trace 关联不在本计划范围（需后续在 signal hub / bus 投递处注入 traceparent，属独立子任务）。

**Tech Stack:** go.opentelemetry.io/otel (+ sdk、otlptracegrpc、contrib otelgin)、Grafana Tempo、Gin。

---

## 背景 / 现状

- `go.mod` 已间接依赖 `go.opentelemetry.io/otel v1.43.0`（`app/server/go.mod:164`），需提升为直接依赖并补充 sdk / exporter / otelgin。
- 当前无 tracing 代码，排障只能靠日志手动 correlate。
- Grafana 数据源仅有 Prometheus 与 Loki，缺 Tempo。

---

### Task 1: 引入 OTel 依赖

**Files:**
- Modify: `app/server/go.mod`（通过 `go get`）

- [ ] **Step 1: 拉取依赖并 tidy**

Run:
```bash
cd app/server && go get \
  go.opentelemetry.io/otel/sdk@v1.43.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.43.0 \
  go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin@latest \
  go.opentelemetry.io/otel/runtime@latest \
  && go mod tidy
```

- [ ] **Step 2: 校验编译**

Run: `cd app/server && go build ./...`
Expected: 编译通过，otel 相关包进入 `require`（直接依赖）。

- [ ] **Step 3: 提交**

```bash
git add app/server/go.mod app/server/go.sum
git commit -m "observability: add OpenTelemetry tracing dependencies"
```

---

### Task 2: 配置字段

**Files:**
- Modify: `app/server/internal/config/config.go`（Config 结构体，放在 `MetricsToken` 附近）

- [ ] **Step 1: 新增 OTel 配置项**

在 `app/server/internal/config/config.go` 的 `MetricsToken string` 字段之后追加：

```go
	OTelEnabled     bool    `env:"OTEL_ENABLED" envDefault:"false"`
	OTelEndpoint    string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"tempo:4317"`
	OTelSampleRatio float64 `env:"OTEL_TRACES_SAMPLER_ARG" envDefault:"0.1"`
```

- [ ] **Step 2: 编译验证**

Run: `cd app/server && go build ./...`
Expected: 编译通过，`cfg.OTelEnabled` / `cfg.OTelEndpoint` / `cfg.OTelSampleRatio` 可访问。

- [ ] **Step 3: 提交**

```bash
git add app/server/internal/config/config.go
git commit -m "observability: add OTel config fields"
```

---

### Task 3: trace 包 Init / Shutdown

**Files:**
- Create: `app/server/internal/trace/trace.go`
- Create: `app/server/internal/trace/trace_test.go`

- [ ] **Step 1: 写失败测试**

`app/server/internal/trace/trace_test.go`：

```go
package trace

import (
	"context"
	"testing"
)

func TestInitDisabledReturnsNoop(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd app/server && go test ./internal/trace/ -run TestInitDisabledReturnsNoop -v`
Expected: FAIL（`trace` 包不存在）。

- [ ] **Step 3: 实现 trace 包**

`app/server/internal/trace/trace.go`：

```go
package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Config 汇聚 OTel 初始化参数。
type Config struct {
	Enabled     bool
	Endpoint    string
	SampleRatio float64
	ServiceName string
}

// Init 初始化全局 TracerProvider；disabled 时返回 noop provider 与空 shutdown。
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRatio)),
		sdktrace.WithResource(resource.Default()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

// Tracer 返回命名 tracer，便于在 service 层创建 span。
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd app/server && go test ./internal/trace/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add app/server/internal/trace/
git commit -m "observability: add trace init/shutdown package"
```

---

### Task 4: Gin 中间件接线

**Files:**
- Modify: `app/server/server/gin.go`（指标接线附近）
- Modify: `app/server/server/gin.go` import 块

- [ ] **Step 1: 在 import 块追加**

```go
	"GOSpeak/internal/trace"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
```

- [ ] **Step 2: 在 metrics 接线之后初始化并挂载中间件**

在 `app/server/server/gin.go` 的 `r.GET("/metrics", gin.WrapH(metricsHandler))` 之后追加：

```go
// OpenTelemetry：enabled 时初始化 TracerProvider 并挂载 Gin 中间件。
shutdownTrace, err := trace.Init(context.Background(), trace.Config{
	Enabled:     cfg.OTelEnabled,
	Endpoint:    cfg.OTelEndpoint,
	SampleRatio: cfg.OTelSampleRatio,
	ServiceName: "gospeak",
})
if err != nil {
	logger.WithComponent("Trace").Warnf("otel init failed: %v", err)
} else {
	defer shutdownTrace(context.Background())
}
if cfg.OTelEnabled {
	r.Use(otelgin.Middleware("gospeak"))
}
```

`context` 包应已在 `gin.go` 导入（启动流程大量使用 `context.Background()`）；若未导入则在 import 块追加 `"context"`。

- [ ] **Step 3: 编译验证**

Run: `cd app/server && go build ./...`
Expected: 编译通过；`OTEL_ENABLED=true` 时启动会初始化 TracerProvider。

- [ ] **Step 4: 提交**

```bash
git add app/server/server/gin.go
git commit -m "observability: wire OTel Gin middleware"
```

---

### Task 5: Tempo 数据源 + 部署

**Files:**
- Modify: `deploy/observability/grafana/provisioning/datasources/datasource.yml`
- Create: `deploy/observability/tempo/tempo.yaml`
- Modify: `deploy/docker-compose.yml`（observability profile 增加 tempo）
- Modify: `deploy/DEPLOY.md`

- [ ] **Step 1: Grafana 增加 Tempo 数据源**

在 `datasource.yml` 的 `datasources:` 列表追加：

```yaml
  - name: Tempo
    type: tempo
    access: proxy
    url: http://tempo:3200
    editable: true
```

- [ ] **Step 2: 创建 tempo.yaml**

`deploy/observability/tempo/tempo.yaml`：

```yaml
auth_enabled: false

server:
  http_listen_port: 3200
  grpc_listen_port: 9095

distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317

storage:
  trace:
    backend: local
    local:
      path: /tmp/tempo/blocks
    wal:
      path: /tmp/tempo/wal

overrides:
  metrics_generator:
    storage:
      path: /tmp/tempo/generators
```

- [ ] **Step 3: docker-compose 增加 tempo 服务**

在 `deploy/docker-compose.yml` 的 `promtail:` 服务之前追加（与其他 observability 服务同级，使用 `<<: *restart` 与 `profiles: ["observability"]`）：

```yaml
  tempo:
    <<: *restart
    profiles: ["observability"]
    image: grafana/tempo:2.7.1
    container_name: gospeak-tempo
    command: ["-config.file=/etc/tempo/tempo.yaml"]
    volumes:
      - ./observability/tempo/tempo.yaml:/etc/tempo/tempo.yaml:ro
    ports:
      - "${TEMPO_PORT:-3200}:3200"
```

- [ ] **Step 4: DEPLOY.md 标注**

在 Observability 小节追加：

```markdown
#### 分布式追踪
- 设置 `OTEL_ENABLED=true` 开启 OpenTelemetry；`OTEL_EXPORTER_OTLP_ENDPOINT` 默认 `tempo:4317`。
- `OTEL_TRACES_SAMPLER_ARG` 控制采样率（默认 0.1）。
- 启动 observability profile 会拉起 Tempo，Grafana 已配置 Tempo 数据源，可在 Trace 页按 `gospeak` service 查询。
```

- [ ] **Step 5: 校验 compose 合法**

Run: `docker compose -f deploy/docker-compose.yml --profile observability config >/dev/null && echo OK`
Expected: 打印 `OK`。

- [ ] **Step 6: 提交**

```bash
git add deploy/observability/grafana/provisioning/datasources/datasource.yml deploy/observability/tempo/ deploy/docker-compose.yml deploy/DEPLOY.md
git commit -m "observability: add Tempo datasource and deployment"
```

---

## 自检

1. 规格覆盖：依赖（Task 1）、配置（Task 2）、trace 包（Task 3）、Gin 接线（Task 4）、Tempo 部署（Task 5）均已覆盖。
2. 占位符扫描：无 TBD/TODO；跨实例 WS/NATS trace 关联在架构说明中明确为范围外，非步骤占位。
3. 类型一致性：`trace.Config` 字段与 `config.go` 新增字段一一对应；`otelgin.Middleware("gospeak")` 与 `trace.Init` 同属 otel 生态。
