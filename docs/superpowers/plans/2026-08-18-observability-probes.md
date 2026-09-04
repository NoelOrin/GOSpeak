# GOSpeak 可观测性 探针分离 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补全 K8s 风格的健康探针：新增 `/healthz` 存活探针（不依赖任何外部依赖），并让 `/readyz` 同时探活 DB 与 EventBus（NATS）。

**Architecture:** `/healthz` 只返回进程存活（200），用于 liveness；`/readyz` 复用现有 `ReadyCheck` 注入点，扩展为 DB `db.DB()` + EventBus `bus.GetStats(...).Connected` 双探活，用于 readiness。两者均为无业务逻辑的轻量 HTTP handler。

**Tech Stack:** Gin；既有 `router.Handlers.ReadyCheck`、`bus.GetStats`、`db`/`repository` 包。

---

## 背景 / 现状

- `app/server/internal/router/router.go` 已有 `r.GET("/ping", ...)` 与 `r.GET("/readyz", ...)`，且 `ReadyCheck` 已在 `app/server/server/gin.go:522` 接线（仅探 DB）。
- 缺失：`/healthz` 存活探针；`/readyz` 未探 EventBus。

---

### Task 1: 新增 /healthz 存活探针

**Files:**
- Modify: `app/server/internal/router/router.go` (SetupRoutes 内 `/ping` 之后)
- Modify: `app/server/internal/router/router_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/router/router_test.go` 追加：

```go
func TestHealthzAlwaysUp(t *testing.T) {
	r := gin.New()
	router.SetupRoutes(r, &router.Handlers{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
```

Run: `cd app/server && go test ./internal/router/ -run TestHealthzAlwaysUp -v`
Expected: FAIL（`/healthz` 未注册，404）。

- [ ] **Step 2: 实现 /healthz**

在 `app/server/internal/router/router.go` 的 `r.GET("/ping", ...)` 之后追加：

```go
// /healthz 存活探针：仅反映进程是否存活，不依赖任何外部依赖。
r.GET("/healthz", func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
})
```

- [ ] **Step 3: 跑测试确认通过**

Run: `cd app/server && go test ./internal/router/ -run TestHealthzAlwaysUp -v`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add app/server/internal/router/router.go app/server/internal/router/router_test.go
git commit -m "observability: add /healthz liveness probe"
```

---

### Task 2: /readyz 增加 EventBus 探活

**Files:**
- Modify: `app/server/server/gin.go` (ReadyCheck 闭包)
- Modify: `app/server/internal/router/router_test.go`

- [ ] **Step 1: 写基线测试**

在 `app/server/internal/router/router_test.go` 追加（验证 readiness 依赖注入点存在）：

```go
func TestReadyzUnavailableWhenNoCheck(t *testing.T) {
	r := gin.New()
	router.SetupRoutes(r, &router.Handlers{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
```

Run: `cd app/server && go test ./internal/router/ -run TestReadyzUnavailableWhenNoCheck -v`
Expected: PASS（当前 `ReadyCheck==nil` 即 503，先固化基线）。

- [ ] **Step 2: 扩展 ReadyCheck 双探活**

在 `app/server/server/gin.go` 将 `ReadyCheck` 闭包由仅探 DB 改为：

```go
ReadyCheck: func() error {
	if _, err := db.DB(); err != nil {
		return err
	}
	if !bus.GetStats(eventBus).Connected {
		return fmt.Errorf("event bus not connected")
	}
	return nil
},
```

确认 `gin.go` 已导入 `"fmt"`（现有文件已使用 `fmt.Errorf`，通常已导入；若未导入在 import 块追加 `"fmt"`）。

- [ ] **Step 3: 编译验证**

Run: `cd app/server && go build ./... && go test ./internal/router/ -run 'TestReadyz|TestHealthz' -v`
Expected: 编译通过，`TestHealthzAlwaysUp` 与 `TestReadyzUnavailableWhenNoCheck` 均 PASS。

- [ ] **Step 4: 提交**

```bash
git add app/server/server/gin.go app/server/internal/router/router_test.go
git commit -m "observability: probe EventBus in /readyz readiness check"
```

---

## 自检

1. 规格覆盖：liveness（Task 1）、readiness 扩展 EventBus（Task 2）均已覆盖。
2. 占位符扫描：无 TBD/TODO。
3. 类型一致性：`bus.GetStats(eventBus)` 返回 `bus.Stats`，`.Connected` 字段与 `internal/bus/factory.go` 定义一致；`db.DB()` 与现有 `ReadyCheck` 一致。
