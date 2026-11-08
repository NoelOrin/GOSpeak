# 访客访问与权限 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 允许未注册访客通过邀请码或公开 Domain 进入语音房间，听/说/发消息三项能力由 Domain 独立配置（默认 听开/说开/消息关），并提供封禁、限流、在线上限治理能力。

**架构：** guest = `users` 表带 `is_guest=true` 的真实用户行 + `DomainMember(role=guest)`；签发标准 JWT、HttpOnly Cookie 会话；鉴权、Casbin、消息归属、SFU 全链路复用。能力开关落 `domains` 表字段（不走 Casbin），封禁走新表 `domain_guest_bans`，鉴权后由新增 guest 守卫中间件统一拦截。

**技术栈：** Go (Gin + GORM + Casbin)、SolidJS + Vite + TanStack Router、nhooyr.io/websocket。

**规格：** `docs/superpowers/specs/2026-08-25-guest-access-permissions-design.md`（先读它）

---

## 文件结构

**新建：**
- `app/server/internal/model/guest_ban.go` — DomainGuestBan 模型
- `app/server/internal/repository/guest_ban_repo.go` — 封禁 DAO
- `app/server/internal/service/guest_service.go` — 访客签发/换域/治理业务逻辑
- `app/server/internal/handler/guest_handler.go` — 访客相关 HTTP handler
- `app/server/internal/router/routes/guest/routes.go` — 路由注册
- `app/server/internal/middleware/guest.go` — guest 守卫中间件
- `app/web/src/stores/guestStore.ts` — 访客状态 store
- `app/web/src/api/guest.ts` — 访客 API 客户端
- `app/web/src/pages/guest/index.tsx` — 访客入口页（公开路由）
- `app/web/src/components/domain/GuestAccessSettings.tsx` — 管理端访客配置分区
- `app/web/src/components/domain/GuestManageTable.tsx` — 在线访客/封禁列表

**修改：**
- `app/server/internal/model/user.go` — 加 `IsGuest` 字段
- `app/server/internal/model/domain.go` — Domain 加 5 个字段
- `app/server/internal/repository/db_migrations.go:238` — AutoMigrate 注册新模型
- `app/server/internal/middleware/routes 白名单常量`（在 guest.go 内）
- `app/server/internal/handler/signal_handler.go` / `signal` service — 听/说执行点
- `app/server/internal/handler/message_handler.go` — 发消息执行点
- `app/server/internal/signal/hub.go` — JoinRoom 封禁与上限检查
- `app/server/server/gin.go` — 装配 guestService / guestHandler / 守卫依赖
- `app/web/src/stores/userStore.ts` — 暴露 `isGuest`
- `app/web/src/pages/login/index.tsx` — 访客登录按钮
- `app/web/src/pages/(app)/route.tsx` — 新增 /guest 公开路由（注意现有 invite 页是登录态的，访客入口放独立公开路由）

---

## 任务 1：数据模型与迁移

**文件：**
- 修改：`app/server/internal/model/user.go`
- 修改：`app/server/internal/model/domain.go:11-22`
- 创建：`app/server/internal/model/guest_ban.go`
- 修改：`app/server/internal/repository/db_migrations.go:238`（AutoMigrate 列表）
- 测试：`app/server/internal/model/guest_ban_test.go`、`app/server/internal/repository/guest_ban_repo.go`（含测试）、`app/server/internal/repository/guest_ban_repo_test.go`

- [ ] **步骤 1：编写失败的模型测试** `app/server/internal/model/guest_ban_test.go`

```go
func TestDomainGuestBan_IsActive(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	if !(&model.DomainGuestBan{ExpiresAt: nil}).IsActive() {
		t.Fatal("nil ExpiresAt must be active (permanent)")
	}
	if !(&model.DomainGuestBan{ExpiresAt: &future}).IsActive() {
		t.Fatal("future expiry must be active")
	}
	if (&model.DomainGuestBan{ExpiresAt: &past}).IsActive() {
		t.Fatal("past expiry must be inactive")
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`cd app/server && go test ./internal/model/ -run TestDomainGuestBan -v`
预期：FAIL，编译错误 `model.DomainGuestBan undefined`

- [ ] **步骤 3：实现模型**

`app/server/internal/model/guest_ban.go`：

```go
package model

import "time"

// DomainGuestBan 域名下的访客封禁记录；ExpiredAt 为 nil 表示永久封禁。
type DomainGuestBan struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DomainUUID string     `gorm:"size:32;index:idx_guest_ban_domain_user,priority:1;not null" json:"domain_uuid"`
	UserUUID   string     `gorm:"type:uuid;index:idx_guest_ban_domain_user,priority:2;not null" json:"user_uuid"`
	Reason     string     `gorm:"size:255" json:"reason"`
	BannedBy   string     `gorm:"type:uuid" json:"banned_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

func (DomainGuestBan) TableName() string { return "domain_guest_bans" }

func (b *DomainGuestBan) IsActive() bool {
	return b != nil && (b.ExpiresAt == nil || b.ExpiresAt.After(time.Now()))
}
```

`user.go` 的 `User` struct 加字段（放在 `IsBot` 后）：

```go
	IsGuest bool `gorm:"default:false;index" json:"is_guest"`
```

`domain.go` 的 `Domain` struct 加字段（放在 `IsPublic` 后）：

```go
	AllowGuest      bool `gorm:"default:false" json:"allow_guest"`
	GuestCanListen  bool `gorm:"default:true" json:"guest_can_listen"`
	GuestCanSpeak   bool `gorm:"default:true" json:"guest_can_speak"`
	GuestCanMessage bool `gorm:"default:false" json:"guest_can_message"`
	GuestLimit      int  `gorm:"default:50" json:"guest_limit"`
```

注意：`db_migrations.go` 会把表重建为「无危险 default」结构（SQLite 兼容），新增带 default 的列若无异常则仅追加到 AutoMigrate 列表即可。

- [ ] **步骤 4：运行测试确认通过** + 注册迁移

运行：`go test ./internal/model/ -run TestDomainGuestBan -v`
预期：PASS

`db_migrations.go:238` 的 AutoMigrate 列表追加 `&model.DomainGuestBan{}`（users/domains 新列由 AutoMigrate 自动补列；SQLite 的带 default 加列走 ALTER TABLE ADD COLUMN，GORM 支持）。

- [ ] **步骤 5：编写失败 Repo 测试** `guest_ban_repo_test.go`

```go
func TestGuestBanRepo_FindActive(t *testing.T) {
	db := testGuestBanDB(t) // sqlite :memory: + AutoMigrate(&model.DomainGuestBan{})
	repo := repository.NewGuestBanRepo(db)
	mustCreate := func(b *model.DomainGuestBan) {
		if err := repo.Create(b); err != nil { t.Fatal(err) }
	}
	mustCreate(&model.DomainGuestBan{DomainUUID: "d1", UserUUID: "u1"})           // 永久
	past := time.Now().Add(-time.Hour)
	mustCreate(&model.DomainGuestBan{DomainUUID: "d1", UserUUID: "u2", ExpiresAt: &past})

	if got := repo.FindActive("d1", "u1"); got == nil { t.Fatal("u1 must be banned") }
	if got := repo.FindActive("d1", "u2"); got != nil { t.Fatal("expired ban must not be active") }
	if got := repo.FindActive("d1", "u3"); got != nil { t.Fatal("unknown user must be unbanned") }
}
```

- [ ] **步骤 6：实现 Repo**

```go
type GuestBanRepo struct{ db *gorm.DB }

func NewGuestBanRepo(db *gorm.DB) *GuestBanRepo { return &GuestBanRepo{db: db} }

func (r *GuestBanRepo) Create(b *model.DomainGuestBan) error { return r.db.Create(b).Error }

func (r *GuestBanRepo) FindActive(domainUUID, userUUID string) *model.DomainGuestBan {
	var b model.DomainGuestBan
	err := r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("id DESC").First(&b).Error
	if err != nil { return nil }
	return &b
}

func (r *GuestBanRepo) ListByDomain(domainUUID string) ([]model.DomainGuestBan, error) {
	var list []model.DomainGuestBan
	err := r.db.Where("domain_uuid = ?", domainUUID).Order("id DESC").Find(&list).Error
	return list, err
}

// Delete 物理删除 = 解封（记录可后续进审计表，本期不留痕）。
func (r *GuestBanRepo) Delete(domainUUID, userUUID string) error {
	return r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Delete(&model.DomainGuestBan{}).Error
}
```

- [ ] **步骤 7：跑测试、Commit**

运行：`go test ./internal/repository/ -run TestGuestBanRepo -v && go build ./...`

```bash
git add app/server/internal/model app/server/internal/repository
git commit -m "feat(guest): add is_guest/domain guest fields, DomainGuestBan model + repo"
```

---

## 任务 2：GuestService（签发 / 换域 / 治理）

**文件：**
- 创建：`app/server/internal/service/guest_service.go`
- 测试：`app/server/internal/service/guest_service_test.go`

- [ ] **步骤 1：编写失败的 Service 测试**（覆盖：正常签发、未开放访客、公开域免邀请码、邀请码错误、昵称过长、已达上限被拒绝）

```go
func TestGuestService_Join(t *testing.T) {
	db := newGuestTestDB(t) // AutoMigrate users/domains/domain_members/domain_roles/role perms
	svc := newGuestService(t, db)

	t.Run("invite code join", func(t *testing.T) {
		d := seedDomain(t, db, true) // allow_guest=true
		resp, err := svc.Join(&service.GuestJoinRequest{Nickname: "玩家甲", InviteCode: d.InviteCode})
		if err != nil || resp.User == nil || !resp.User.IsGuest { t.Fatalf("expect guest: %v %v", resp, err) }
	})
	t.Run("public domain no invite code", func(t *testing.T) {
		d := seedDomain(t, db, true)
		d.IsPublic = true
		db.Save(d)
		_, err := svc.Join(&service.GuestJoinRequest{Nickname: "玩家乙", DomainUUID: d.UUID})
		if err != nil { t.Fatalf("public domain must accept without code: %v", err) }
	})
	t.Run("disabled domain rejected", func(t *testing.T) {
		d := seedDomain(t, db, false)
		_, err := svc.Join(&service.GuestJoinRequest{Nickname: "x", InviteCode: d.InviteCode})
		assertAppCode(t, err, pkg.FORBIDDEN)
	})
	t.Run("nickname over 24 chars rejected", func(t *testing.T) {
		d := seedDomain(t, db, true)
		_, err := svc.Join(&service.GuestJoinRequest{Nickname: strings.Repeat("很", 25), InviteCode: d.InviteCode})
		assertAppCode(t, err, pkg.INVALID_PARAMS)
	})
}
```

- [ ] **步骤 2：运行确认失败**（编译错误：GuestService 不存在）

- [ ] **步骤 3：实现 GuestService**

```go
package service

type GuestJoinRequest struct {
	Nickname   string `json:"nickname" binding:"required,max=24"`
	InviteCode string `json:"invite_code"`
	DomainUUID string `json:"domain_uuid"`
}

type GuestJoinResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         *model.User `json:"user"`
	Domain       *model.Domain `json:"domain"`
}

type GuestService struct {
	db          *gorm.DB
	userRepo    *repository.UserRepo
	domainRepo  *repository.DomainRepo
	banRepo     *repository.GuestBanRepo
	auth        *AuthService
	onlineCount func(domainUUID string) int // 由 gin.go 注入 WS Hub 在线 guest 统计，测试注入桩
}

// Join 签发访客身份并加入 Domain。事务：users 行 + domain_members(role=guest)。
func (s *GuestService) Join(req *GuestJoinRequest) (*GuestJoinResponse, error) {
	if req.InviteCode == "" && req.DomainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite_code or domain_uuid required")
	}
	domain, err := s.resolveDomain(req)
	if err != nil { return nil, err }
	if !domain.AllowGuest {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "guest access disabled")
	}
	if domain.GuestLimit > 0 && s.onlineCount != nil && s.onlineCount(domain.UUID) >= domain.GuestLimit {
		return nil, pkg.NewAppError(pkg.RATE_LIMITED, "guest limit reached")
	}
	user := &model.User{
		Name:        "guest_" + uuid.NewString()[:12],
		DisplayName: req.Nickname,
		IsGuest:     true,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil { return err }
		member := &model.DomainMember{
			DomainUUID: domain.UUID, UserUUID: user.UUID,
			Nickname: req.Nickname, RoleName: model.DomainRoleGuest,
		}
		return tx.Create(member).Error
	})
	if err != nil { return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error()) }
	tokens, err := s.auth.issueTokens(user) // 提取 AuthService 现有签发逻辑为小方法（无行为变更重构）
	if err != nil { return nil, err }
	return &GuestJoinResponse{AccessToken: tokens.Access, RefreshToken: tokens.Refresh, User: user, Domain: domain}, nil
}
```

`resolveDomain`：有 invite_code 按 code 查；否则按 domain_uuid 查且要求 `IsPublic`。

注意：**`AuthService.issueTokens` 提取**——把 `Login`/`Register` 里重复的 `GenerateToken + refresh` 拼包逻辑抽成小方法，两个调用点同步替换，属无行为变更重构，单独一个 commit 前先做这步。

- [ ] **步骤 4：运行测试确认通过**

运行：`go test ./internal/service/ -run TestGuestService -v`
预期：PASS

- [ ] **步骤 5：实现 Ban/Unban/ListBans**

```go
func (s *GuestService) Ban(domainUUID, operatorUUID, guestUUID, reason string, durationHours int) error {
	// 1) 校验目标是该 Domain 的 guest 成员（查 domain_members 且 user.IsGuest）
	// 2) 幂等：已存在活跃封禁 → 更新 reason/expires_at
	// 3) expires_at = nil（durationHours<=0）或 now+duration
	// 4) (可选钩子) 通知 Hub 踢下线 —— 本期由 handler 层调用现有 kick 路径完成
}

func (s *GuestService) Unban(domainUUID, guestUUID string) error {
	return s.banRepo.Delete(domainUUID, guestUUID)
}
```

- [ ] **步骤 6：补 Ban/Unban 测试并跑全量**

运行：`go test ./internal/service/ -run TestGuest -v`

- [ ] **步骤 7：Commit**

```bash
git add app/server/internal/service/guest_service.go app/server/internal/service/guest_service_test.go app/server/internal/service/auth_service.go
git commit -m "feat(guest): GuestService join/renew/ban with domain guest switches"
```

---

## 任务 3：访客 HTTP API（Handler + 路由 + 限流）

**文件：**
- 创建：`app/server/internal/handler/guest_handler.go`
- 创建：`app/server/internal/router/routes/guest/routes.go`
- 修改：`app/server/server/gin.go`（装配 + 挂路由）
- 测试：`app/server/internal/handler/guest_handler_test.go`

- [ ] **步骤 1：编写失败的 Handler 集成测试**（仿 `auth_handler_test.go` 风格，httptest + gin）

```go
func TestGuestHandler_Join(t *testing.T) {
	env := newGuestHandlerEnv(t) // 内存 sqlite + 真 service + gin engine
	d := env.seedDomain(true)

	body := `{"nickname":"路人甲","invite_code":"` + d.InviteCode + `"}`
	w := env.post("/api/v1/auth/guest", body)
	if w.Code != 200 { t.Fatalf("expect 200: %s", w.Body) }
	resp := decodeResp(t, w)
	data := resp["data"].(map[string]interface{})
	if data["access_token"] == "" { t.Fatal("missing access_token") }
	if w.Header().Get("Set-Cookie") == "" { t.Fatal("must set auth cookie like login") }
}
```

- [ ] **步骤 2：实现 Handler**

```go
type GuestHandler struct {
	guestSvc *service.GuestService
	cookie   *AuthCookieConfig
}

// Join 公开接口，发 token + cookie（与 Login 同一 cookie 约定）。
func (h *GuestHandler) Join(c *gin.Context) {
	var req service.GuestJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.guestSvc.Join(&req)
	if err != nil { pkg.HandleError(c, err); return }
	h.cookie.Set(c, resp.AccessToken, resp.RefreshToken)
	pkg.Success(c, resp)
}
```

- [ ] **步骤 3：注册路由**（`router/routes/guest/routes.go`）

```go
func Register(r *gin.RouterGroup, h *handler.GuestHandler) {
	public := r.Group("")
	public.Use(middleware.RateLimit(10, time.Hour)) // 单 IP 每小时 10 次签发
	public.POST("/auth/guest", h.Join)
}
```

`guest/renew`、四个管理接口的路由在任务 5 注册到 protected + 权限中间件。

- [ ] **步骤 4：gin.go 装配**

在 `server/gin.go` DI 组装处：`guestBanRepo := repository.NewGuestBanRepo(db)` → `guestSvc := service.NewGuestService(db, userRepo, domainRepo, guestBanRepo, authSvc, hubOnlineGuestCounter)` → `guestHandler := handler.NewGuestHandler(guestSvc, authCookieCfg)` → `guestroutes.Register(api, guestHandler)`。

`hubOnlineGuestCounter`：本期先注入一个返回 0 的桩函数 + TODO 注释指向任务 6（Hub 内存计数器在任务 6 落地后替换），避免跨任务循环依赖。

- [ ] **步骤 5：跑测试确认通过、再补限流测试**（第 11 次请求 1017）

运行：`go test ./internal/handler/ -run TestGuestHandler -v`

- [ ] **步骤 6：Commit**

```bash
git commit -m "feat(guest): POST /api/v1/auth/guest with rate limit and cookie session"
```

---

## 任务 4：Guest 守卫中间件

**文件：**
- 创建：`app/server/internal/middleware/guest.go`
- 修改：`app/server/server/gin.go`（注入 checker）
- 测试：`app/server/internal/middleware/guest_test.go`

- [ ] **步骤 1：编写失败测试**

```go
func TestGuestGuard(t *testing.T) {
	// 用户是 guest（checker 返回 true），请求域外接口 /api/v1/user/list → 1013
	// 用户是 guest，请求白名单接口 /api/v1/signal/token → 放行到下游
	// 用户是 guest 且 banned(checker.IsBanned true) → 1013
	// 非 guest → 无条件放行
}
```

- [ ] **步骤 2：实现中间件**

```go
package middleware

// guestChecker 由 gin.go 注入：判定用户是否 guest / 是否在某 Domain 被封。
type guestChecker interface {
	IsGuest(userUUID string) bool
	IsGuestBanned(domainUUID, userUUID string) bool
}

// guestAllowPrefixes 访客可调用的业务前缀（信号/房间/消息/自助资料/域查询）。
var guestAllowPrefixes = []string{
	"/api/v1/signal/",
	"/api/v1/domain/guest", // 仅读类在 handler 内再校验；管理类由权限码挡
	"/api/v1/domain/list-public",
	"/api/v1/domain/preview",
	"/api/v1/room/",
	"/api/v1/user/profile",
}

// GuestGuard 在 JWTAuth 之后执行：guest 身份 + 封禁检查 + 路由白名单。
func GuestGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := currentClaims(c) // 复用 JWTAuth 已写入的 claims
		if claims == nil { c.Next(); return }
		checker := currentGuestChecker()
		if checker == nil || !checker.IsGuest(claims.UserUUID) { c.Next(); return }
		// 封禁：domain_uuid 由请求体/查询参数解析；WS 路径在 hub 层另有检查
		if dom := guestDomainOf(c); dom != "" && checker.IsGuestBanned(dom, claims.UserUUID) {
			pkg.Fail(c, pkg.FORBIDDEN, "guest has been banned")
			c.Abort()
			return
		}
		if !guestPathAllowed(c.Request.URL.Path) {
			pkg.Fail(c, pkg.FORBIDDEN, "guest access not allowed")
			c.Abort()
			return
		}
		c.Next()
	}
}
```

`guestDomainOf(c)`：先查 `c.GetHeader("X-Domain-UUID")`，再查 form/query `domain_uuid`；POST JSON 体由 gin 的 `c.GetRawData` 缓存后解析（注意要回写 `c.Request.Body`，避免下游 ShouldBindJSON 读到空）。

- [ ] **步骤 3：gin.go 接线**

`middleware.SetGuestChecker(guestGuardAdapter{userRepo, guestBanRepo})`，并在 protected group 的 `JWTAuth()` 之后追加 `GuestGuard()`。

- [ ] **步骤 4：跑测试 + 全量回归**

运行：`go test ./internal/middleware/ -v && go test ./internal/handler/ -run 'TestAuth|TestRoom|TestSignal' -v`

- [ ] **步骤 5：Commit**

```bash
git commit -m "feat(guest): guest guard middleware with route allowlist and ban check"
```

---

## 任务 5：管理端 API（guest 配置 + ban 管理 + 清理）

**文件：**
- 修改：`app/server/internal/service/guest_service.go`（加 `UpdateConfig/ListOnlineGuests/Cleanup`）
- 修改：`app/server/internal/handler/guest_handler.go`
- 修改：`app/server/internal/router/routes/guest/routes.go`
- 测试：`guest_service_test.go` / `guest_handler_test.go` 增量

- [ ] **步骤 1：接口表（全部 POST，protected）**

| 路径 | 权限 | 请求/响应 |
|------|------|----------|
| `/api/v1/domain/guest/config` | domain:manage（域权限判决，复用 `domainPermissionGranted` 模式） | `{domain_uuid}` GET 语义读；带字段写 |
| `/api/v1/domain/guest/ban` | domain:kick | `{domain_uuid, user_uuid, reason, duration_hours}` |
| `/api/v1/domain/guest/unban` | domain:kick | `{domain_uuid, user_uuid}` |
| `/api/v1/domain/guest/ban-list` | domain:manage 读 | `{domain_uuid}` → 数组 |
| `/api/v1/domain/guest/cleanup` | domain:manage | `{domain_uuid, days}` 删 N 天未活跃 guest 的 users+members 行（消息保留） |
| `/api/v1/auth/guest/renew` | JWT(guest) | `{invite_code 或 domain_uuid}` 同一身份换域：插新 member 行、不新建 user |

- [ ] **步骤 2-7：TDD 循环同前任务**（每接口：失败测试 → 实现 → 通过）

权限判决关键代码：

```go
if !domainPermissionGranted(c, req.DomainUUID, permcode.PermDomainKick, h.domainSvc, h.permSvc) {
	pkg.Fail(c, pkg.FORBIDDEN)
	return
}
```

注意需要给 GuestHandler 注入 `domainSvc` + `permSvc`（仿 `message_handler.go` 的构造）。

- [ ] **步骤 8：Commit**

```bash
git commit -m "feat(guest): admin APIs for guest config, ban management, cleanup, renew"
```

---

## 任务 6：执行点（听 / 说 / 发消息 / Hub 拦截）

**文件：**
- 修改：`app/server/internal/service/guest_service.go`（`GuestCaps(domainUUID)` 读开关）
- 修改：signal service 的 `GetJoinToken` 实现（`app/server/internal/service/signal_service*.go`，按现状定位）
- 修改：`app/server/internal/handler/message_handler.go:167` 等 5 处 Send 判决点
- 修改：`app/server/internal/signal/hub.go`（JoinRoom）
- 测试：对应 handler/service 测试增量

- [ ] **步骤 1：GuestCaps 读取**

```go
// GuestCaps 读域开关；非 guest 调用方不会走到这里。
func (s *GuestService) GuestCaps(domainUUID string) (listen, speak, message bool) {
	d, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil || d == nil { return false, false, false }
	return d.GuestCanListen, d.GuestCanSpeak, d.GuestCanMessage
}
```

- [ ] **步骤 2：听——GetJoinToken 拦截**

在 signal service 拼 token 前：若当前用户 IsGuest 且 `!GuestCaps(domain).listen` → `pkg.NewAppError(pkg.FORBIDDEN, "guest listening disabled")`。同时把 `speak` 传入 SFU token 生成：LiveKit 走 `CanPublish: speak` grant（在 GenerateToken 增加 publish 参数或调用处包一层 `GenerateGuestToken`）；SRS/Agora/Cloudflare 按现有 mute 机制：speak=false 时签发后立即 `MuteParticipant`。先实现 LiveKit 的 CanPublish；其他 provider 用「进房即 mute」降级实现并注释说明。

- [ ] **步骤 3：说——WS 二次校验**

`hub.go` 处理发布音轨/发言状态事件处：guest 且 !speak → 忽略并 `MuteParticipant`（运行时开关被管理员关闭时兜底）。

- [ ] **步骤 4：发消息——message handler 拦截**

`message_handler.go` 的 Send（约 :167）：在 `domainPermissionGranted` 判定前加 guest 早退：

```go
if h.guestSvc != nil && h.guestSvc.IsGuest(currentUserUUID(c)) {
	if _, _, msg := h.guestSvc.GuestCaps(room.DomainUUID); !msg {
		pkg.Fail(c, pkg.FORBIDDEN, "guest messaging disabled")
		return
	}
}
```

- [ ] **步骤 5：Hub JoinRoom 封禁 + 上限**

`hub.go` JoinRoom：guest 且 `IsGuestBanned(domain, uuid)` → 关闭连接 + 广播 member_left；在线 guest 计数器（Hub 维护 `map[domainUUID]map[uuid]bool` 在 Join/Disconnect 增减）超 `GuestLimit` → 拒绝。替换任务 3 的 0 桩注入。

- [ ] **步骤 6：全量测试回归**

运行：`go test ./... -count=1`（app/server 全量）

- [ ] **步骤 7：Commit**

```bash
git commit -m "feat(guest): enforce listen/speak/message switches at signal, sfu and message paths"
```

---

## 任务 7：前端 guestStore + API 客户端

**文件：**
- 创建：`app/web/src/api/guest.ts`
- 创建：`app/web/src/stores/guestStore.ts`
- 修改：`app/web/src/stores/userStore.ts`（UserInfo 加 `is_guest?: boolean`，导出 `isGuest()` 判断）
- 测试：`app/web/src/stores/guestStore.test.ts`

- [ ] **步骤 1：API 客户端**

```ts
import { post } from "./http"; // 复用现有封装

export interface GuestJoinResp {
	access_token: string;
	refresh_token: string;
	user: { uuid: string; display_name: string; is_guest: boolean };
	domain: { uuid: string; name: string };
}

export function guestJoin(payload: { nickname: string; invite_code?: string; domain_uuid?: string }) {
	return post<GuestJoinResp>("/auth/guest", payload);
}
// 其余 5 个管理 API 同型导出
```

- [ ] **步骤 2：guestStore（仿 userStore 的模块级 createSignal 模式）**

```ts
import { createSignal } from "solid-js";

export interface GuestCaps { listen: boolean; speak: boolean; message: boolean }

const [caps, setCaps] = createSignal<GuestCaps>({ listen: true, speak: true, message: false });

export function isGuest(): boolean {
	return !!user()?.is_guest;
}
export function guestCaps(): GuestCaps { return caps(); }
export function setGuestCaps(c: GuestCaps) { setCaps(c); }
```

- [ ] **步骤 3：单测 + Commit**

```bash
git commit -m "feat(web): guest store and api client"
```

---

## 任务 8：访客入口页 + 登录页按钮

**文件：**
- 创建：`app/web/src/pages/guest/index.tsx`
- 修改：路由根（`app/web/src/pages/__root.tsx` 或 `route.tsx`，公开路由，不进 `(app)` 组）
- 修改：`app/web/src/pages/login/index.tsx`
- 测试：组件 smoke 测试

要点：
- 入口页表单：昵称 input（maxLength=24）+ Domain 信息卡片（名称/图标/成员数）；URL 支持 `?code=`（邀请链接）或 `?domain=`（公开域）
- 成功后 `loginAction(resp.user)` 复用 userStore 的登录态（cookie 已由后端种下），跳转 `/` 
- 登录页「访客登录」按钮：调 `list-public` 接口判断是否有 `allow_guest` 的公开域才显示；点击进入 `/guest`
- 已登录访客打开 `/guest`：显示「以 @display_name 继续」直进

评估：先过视觉走查（frontend-design 规范后续 polish）。

- [ ] TDD/组件测试 → 实现 → 截图自查 → Commit `feat(web): guest entry page and login guest button`

---

## 任务 9：访客侧视图门控

**文件：**
- 修改：`app/web/src/pages/(app)/` 下房间/消息相关组件（按实际文件定位：麦克风按钮、消息输入框、侧边栏 nav）

要点：
- 麦克风按钮：`isGuest() && !guestCaps().speak` → disabled + title="该域未开放访客发言"
- 消息输入框：`isGuest() && !guestCaps().message` → disabled + 占位文案「该域访客不可发言」
- 侧边栏：`isGuest()` 时隐藏 管理/Bot/邀请/Domain 设置 nav 项
- caps 来源：进 Domain 后随 domain detail 接口下发（后端在 domain detail DTO 里加 `guest_caps` 字段，避免多一次请求；如是域外房间则默认全关）

后端配套：domain detail 返回结构给 guest 附带三项开关（Domain 已有 dto 组装点，加 3 字段）。

- [ ] TDD → 实现 → Commit `feat(web): gate guest UI by domain caps`

---

## 任务 10：管理端访客配置与治理 UI

**文件：**
- 创建：`app/web/src/components/domain/GuestAccessSettings.tsx`（总开关 + 3 子开关 + 上限输入）
- 创建：`app/web/src/components/domain/GuestManageTable.tsx`（在线 guest / 封禁列表 / 踢出 / 封禁 / 解封）
- 修改：Domain 设置页接入 GuestAccessSettings、域管理 Tab 接入 GuestManageTable

要点：
- 总开关关闭时子开关灰显（`disabled`）
- 封禁弹层：reason 输入 + 时长选择（永久/1h/24h/7d）
- 复用现有表格/弹窗组件约定

- [ ] 实现 → 手工走查（本地起 dev server）→ Commit `feat(web): admin guest settings and moderation UI`

---

## 任务 11：端到端冒烟 + 文档

- [ ] 本地起服务，手工跑通：开 Domain → 开访客 → 管理端调开关 → 新浏览器以访客进入 → 听/说/消息分别验证 → 封禁 → 再进被拒
- [ ] `go test ./... -count=1` 全量 + `pnpm --filter web test` 前端全量
- [ ] 更新 `AGENTS.md` 路由表追加 6 条新路由 + 访客章节链接到规格文档
- [ ] Commit `docs: document guest access routes`

---

## 自检记录

- 规格 §2 数据模型 → 任务 1 ✅；§3 身份流转 → 任务 2/3 ✅；§4 API → 任务 3/5 ✅；§5 守卫 → 任务 4 ✅；§6 WS/SFU → 任务 6 ✅；§7 前端 → 任务 7-10 ✅；§8 治理 → 任务 5/6 ✅；§9 测试 → 每任务内嵌 TDD ✅；§10 范围外不做 ✅
- 已修正的跨层事实：token 走 HttpOnly Cookie 而非 localStorage；前端为 SolidJS（非 React/Zustand）；权限判决复用 `domainPermissionGranted`/`domainPermissionChecker` 模式而非新造。
