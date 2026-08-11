# Review P0: Domain RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复双线程 review 整合报告中域权限/RBAC 子系统的 P0 问题：域角色权限在 room/message 的 HTTP 与 WS 路径真正生效，并限制域内 admin 提权。

**Architecture:** 新增 handler 共享的“域房间只认域角色权限、平台资源回退全局权限”判定 helper，接入 room/message HTTP 与 WS 消息事件；`SetMemberRole` 限制 admin 角色仅 owner 可分配；最后做全量回归。

**Tech Stack:** Go 1.26 + Gin + GORM + NATS（app/server），TypeScript + Vitest（app/web）。

---

## 范围说明

本计划只覆盖域权限/RBAC 子系统：

- handler 共享域权限判定 helper。
- room HTTP 域权限接入。
- message HTTP 域权限接入。
- `SetMemberRole` admin 仅 owner 可分配。
- WS 消息事件域权限接入。
- 全量回归验证。

认证/会话已拆到独立计划 `2026-08-11-review-p0-auth-session.md`；NATS/WAL、SFU/媒体、Bot/前端、存储/迁移按 Scope Check 拆为后续子计划，不在本文件实现。

## 已核实的现状（避免重复实现）

- `gin.go:215-216` 已把 `domainSvc.HasDomainPermission` 注入 `HubOptions.DomainPermissionChecker`，但 `message_bridge.go` 只有 delete_others 使用，send/edit/react/unreact 的 `checkMessagePerm` 仍只看全局权限。
- `message_handler.go` 的 List/Search 已接收并透传 `password`，私密房历史读取已覆盖；Send/Edit/Delete/React/Unreact 的 password 透传属于另一子计划。
- 默认全局 `user` 角色包含 `room:create/read` 与 `message:send/read`（`model/permission.go:166-172`），因此域房间权限必须“域权限优先、不回退全局”，否则 guest 可借全局权限绕过域角色。
- `room_handler.go` 的 `canManageRoom` 目前对域房间仍回退全局权限与创建者特权；`message_handler.go` 只有 Delete 接入了 `PermMessageDeleteOthers` 的域判定。

### Task 1: handler 共享域权限判定 helper

**Files:**
- Create: `app/server/internal/handler/domain_access.go`
- Test: `app/server/internal/handler/domain_access_test.go`

背景：room/message handler 需要统一“域房间只认域角色权限，平台资源回退全局权限”的判定，避免各 handler 复制不一致逻辑。

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/handler/domain_access_test.go`：

```go
package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDomainPermissionGranted_DomainRoomUsesDomainPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("user_uuid", "member-1")
	c.Set("role", "user")

	if !domainPermissionGranted(c, "domain-a", "room:create", fakeDomainPermChecker(true), nil) {
		t.Fatal("expected domain permission to grant access")
	}
	if domainPermissionGranted(c, "domain-a", "room:create", fakeDomainPermChecker(false), nil) {
		t.Fatal("expected missing domain permission to deny access even with global role present")
	}
}

func TestDomainPermissionGranted_PlatformRoomFallsBackToGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("user_uuid", "user-1")
	c.Set("role", "admin")

	if !domainPermissionGranted(c, "", "room:read", nil, fakeGlobalPermChecker(true)) {
		t.Fatal("expected global permission to grant platform room access")
	}
	if domainPermissionGranted(c, "", "room:read", nil, fakeGlobalPermChecker(false)) {
		t.Fatal("expected missing global permission to deny")
	}
}

type fakeDomainPermSvc struct {
	allow bool
}

func (f fakeDomainPermSvc) HasDomainPermission(_, _, _ string) bool { return f.allow }

func fakeDomainPermChecker(allow bool) fakeDomainPermSvc { return fakeDomainPermSvc{allow: allow} }

type fakeGlobalPermSvc struct {
	allow bool
}

func (f fakeGlobalPermSvc) HasPermission(_, _ string) bool { return f.allow }

func fakeGlobalPermChecker(allow bool) fakeGlobalPermSvc { return fakeGlobalPermSvc{allow: allow} }
```

helper 定义会用到 `*service.DomainService` 与 `*service.PermissionService`；为让测试可编译，helper 使用小接口而不是具体类型，见 Step 3。

- [ ] **Step 2: 运行测试确认编译失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestDomainPermissionGranted -v`

Expected: FAIL，`domainPermissionGranted` 未定义。

- [ ] **Step 3: 创建 `domain_access.go`**

```go
package handler

import (
	"github.com/gin-gonic/gin"
)

// domainPermissionChecker 校验用户在指定 Domain 的角色是否具备权限码。
type domainPermissionChecker interface {
	HasDomainPermission(domainUUID, userUUID, permCode string) bool
}

// globalPermissionChecker 校验用户全局角色是否具备权限码。
type globalPermissionChecker interface {
	HasPermission(role, permCode string) bool
}

// domainPermissionGranted 判定资源权限：Domain 房间只认域角色权限；
// 平台资源（domainUUID 为空）回退全局角色权限。
func domainPermissionGranted(
	c *gin.Context,
	domainUUID, permCode string,
	domainSvc domainPermissionChecker,
	permSvc globalPermissionChecker,
) bool {
	if domainUUID != "" {
		return domainSvc != nil && domainSvc.HasDomainPermission(domainUUID, currentUserUUID(c), permCode)
	}
	return permSvc != nil && permSvc.HasPermission(roleFromContext(c), permCode)
}
```

`*service.DomainService` 与 `*service.PermissionService` 分别满足两个小接口，无需 import service。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestDomainPermissionGranted -v`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/handler/domain_access.go internal/handler/domain_access_test.go
git commit -m "feat(handler): add domain-first resource permission helper"
```

---

### Task 2: room HTTP 域权限接入

**Files:**
- Modify: `app/server/internal/handler/room_handler.go`
- Test: `app/server/internal/handler/room_handler_domain_required_test.go`

背景：Create/Get/List/Update/Delete 目前只靠全局权限 + 域成员校验，域角色对 `room:create/read/update/delete` 的控制不生效。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/room_handler_domain_required_test.go` 末尾追加：

```go
func TestRoomHandler_Create_DomainPermissionRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Chat", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "guest-1", RoleName: model.DomainRoleGuest}).Error; err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	middleware.SetDomainChecker(func(_, _ string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	h := NewRoomHandler(roomSvc, nil, domainSvc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "guest-1")
		c.Set("user_uuid", "guest-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/create", h.Create)

	body := `{"name":"lobby","domain_uuid":"` + domain.UUID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("guest must not create room: expected 1013, got %d: %s", code, resp["msg"])
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestRoomHandler_Create_DomainPermissionRequired -v`

Expected: FAIL，guest（全局 user 有 `room:create`）当前能创建成功，返回 code 0。

- [ ] **Step 3: 修改 `room_handler.go`**

在 `Create` 的成员校验（`middleware.IsDomainMember`）之后追加：

```go
	if !domainPermissionGranted(c, domainUUID, permcode.PermRoomCreate, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain room permission")
		return
	}
```

将 `Get` 中的：

```go
	if room.DomainUUID != "" && !middleware.IsDomainMember(room.DomainUUID, currentUserUUID(c)) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
		return
	}
```

替换为：

```go
	if room.DomainUUID != "" {
		if !middleware.IsDomainMember(room.DomainUUID, currentUserUUID(c)) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
			return
		}
		if !domainPermissionGranted(c, room.DomainUUID, permcode.PermRoomRead, h.domainSvc, h.permSvc) {
			pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain room permission")
			return
		}
	}
```

在 `List` 中把：

```go
	if domainUUID != "" && !middleware.IsDomainMember(domainUUID, currentUserUUID(c)) {
		pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
		return
	}
```

替换为：

```go
	if domainUUID != "" {
		if !middleware.IsDomainMember(domainUUID, currentUserUUID(c)) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this domain")
			return
		}
		if !domainPermissionGranted(c, domainUUID, permcode.PermRoomRead, h.domainSvc, h.permSvc) {
			pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain room permission")
			return
		}
	}
```

修改 `canManageRoom`：

```go
func (h *RoomHandler) canManageRoom(c *gin.Context, room *model.Room, perm string) bool {
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if room.DomainUUID != "" {
		if h.domainSvc != nil && h.domainSvc.HasDomainPermission(room.DomainUUID, currentUserUUID(c), perm) {
			return true
		}
		// 域房间创建者保留对自己房间的管理权。
		return room.CreatedBy == usernameStr
	}
	if h.permSvc != nil && h.permSvc.HasPermission(roleFromContext(c), perm) {
		return true
	}
	return room.CreatedBy == usernameStr
}
```

Update/Delete 无需再改，它们已调用 `canManageRoom`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run 'TestRoomHandler_Create_DomainPermissionRequired|TestRoomHandler_Create_RejectsDomainUUIDMismatch|TestRoomHandler_Create_UsesRequestBodyDomainUUID' -v`

Expected: 全部 PASS（新增测试与既有 domain_uuid 校验测试）。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/handler/room_handler.go internal/handler/room_handler_domain_required_test.go
git commit -m "fix(room): enforce domain role permissions on room endpoints"
```

---

### Task 3: message HTTP 域权限接入

**Files:**
- Modify: `app/server/internal/handler/message_handler.go`
- Test: `app/server/internal/handler/message_handler_domain_test.go`

背景：List/Search/Send/Edit/Delete/React/Unreact 只依赖全局 `message:*`，域角色 `message:read/send/delete_others` 不生效。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/message_handler_domain_test.go` 末尾追加：

```go
func TestMessageHandler_Send_DomainPermissionRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}, &model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domain := &model.Domain{Name: "Chat", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "guest-1", RoleName: model.DomainRoleGuest}).Error; err != nil {
		t.Fatalf("seed guest: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	msgSvc := service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc)
	h := NewMessageHandler(msgSvc, nil, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "guest-1")
		c.Set("user_uuid", "guest-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/send", h.Send)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "content": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("guest must not send: expected 1013, got %d: %s", code, resp["msg"])
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run TestMessageHandler_Send_DomainPermissionRequired -v`

Expected: FAIL，guest 发送成功（全局 user 有 `message:send`）。

- [ ] **Step 3: 修改 `message_handler.go`**

新增私有 helper（放在 `messageActorFromContext` 之后）：

```go
func (h *MessageHandler) roomOf(c *gin.Context, roomUUID string) (*model.Room, bool) {
	if h.roomSvc == nil {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "room service unavailable")
		return nil, false
	}
	room, err := h.roomSvc.GetByUUID(roomUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return nil, false
	}
	return room, true
}
```

在 `List` 中，`messageActorFromContext` 校验之后追加：

```go
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if room.DomainUUID != "" && !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageRead, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
```

在 `Search` 中同样追加（权限码同为 `PermMessageRead`）。

在 `Send` 中追加：

```go
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if room.DomainUUID != "" && !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
```

在 `Edit`、`React`、`Unreact` 中追加与 `Send` 相同的代码块（`PermMessageSend`）。

在 `Delete` 中，把 `canDeleteOthers` 计算前追加：

```go
	room, ok := h.roomOf(c, req.RoomUUID)
	if !ok {
		return
	}
	if room.DomainUUID != "" && !domainPermissionGranted(c, room.DomainUUID, permcode.PermMessageSend, h.domainSvc, h.permSvc) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain message permission")
		return
	}
```

`Delete` 中已有的 `roomSvc.GetByUUID` 查询可保留（此时已缓存，无重复副作用），但为 DRY 可改为使用上面的 `room` 变量并删除原查询。

`message_handler.go` 需要新增 import：`GOSpeak/internal/model`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/handler/ -run 'TestMessageHandler_Send_DomainPermissionRequired|TestMessageHandler_Delete_AllowsDomainRolePermission|TestMessageHandler_Delete_RejectsNonAuthor' -v`

Expected: 全部 PASS（新增测试与既有域角色删除测试）。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/handler/message_handler.go internal/handler/message_handler_domain_test.go
git commit -m "fix(message): enforce domain role permissions on message endpoints"
```

---

### Task 4: SetMemberRole 限制 admin 仅 owner 可分配

**Files:**
- Modify: `app/server/internal/service/domain_service.go:457-485`
- Test: `app/server/internal/service/domain_service_test.go`

背景：任何持有 `domain:role:manage` 的成员都能把他人提升为 admin，形成域内提权。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/service/domain_service_test.go` 末尾追加：

```go
func TestDomainService_SetMemberRole_AdminOnlyForOwner(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := seedDomainOwner(t, db, "Chat", "owner-1")
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "admin-1", RoleName: DomainRoleAdmin}).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	err := svc.SetMemberRole(domain.UUID, "admin-1", "member-1", DomainRoleAdmin)
	checkAppErrCode(t, err, pkg.FORBIDDEN)
}

func TestDomainService_SetMemberRole_OwnerCanAssignAdmin(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := seedDomainOwner(t, db, "Chat", "owner-1")
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}

	if err := svc.SetMemberRole(domain.UUID, "owner-1", "member-1", DomainRoleAdmin); err != nil {
		t.Fatalf("owner should assign admin: %v", err)
	}
}
```

两个测试统一用 `db.Create` seed 成员，避免依赖未定义方法。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestDomainService_SetMemberRole' -v`

Expected: `AdminOnlyForOwner` FAIL（当前 admin-1 可提升 member-1），`OwnerCanAssignAdmin` PASS。

- [ ] **Step 3: 修改 `domain_service.go` 的 `SetMemberRole`**

在 `SetMemberRole` 的 self/owner 校验之后追加：

```go
	if roleName == model.DomainRoleAdmin && !s.IsOwner(domainUUID, operatorUUID) {
		return pkg.NewAppError(pkg.FORBIDDEN, "only domain owner can assign admin role")
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/service/ -run 'TestDomainService_SetMemberRole' -v`

Expected: 两个测试 PASS。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/service/domain_service.go internal/service/domain_service_test.go
git commit -m "fix(domain): restrict admin assignment to domain owner"
```

---

### Task 5: WS 消息域权限接入

**Files:**
- Modify: `app/server/internal/signal/message_bridge.go`
- Test: `app/server/internal/signal/message_bridge_test.go`

背景：`checkMessagePerm` 只查全局权限，`domainPermChecker` 已注入但只有 delete_others 使用；WS 发送/编辑/删除/反应可绕过域角色。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/signal/message_bridge_test.go` 末尾追加：

```go
func TestOnMessageSend_DomainPermissionRequiredWhenCheckerWired(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{
		{UUID: "uuid-domain", Name: "text-chat", Type: model.RoomTypeText, DomainUUID: "domain-a"},
	}}
	h := newTestHub()
	h.roomStore = store
	h.permChecker = &mockPermChecker{rolePerms: map[string]map[string]bool{"user": {"message:send": true}}}
	h.domainPermChecker = func(domainUUID, userUUID, permCode string) bool {
		return domainUUID == "domain-a" && userUUID == "mod-1" && permCode == "message:send"
	}
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	conn := newMockClient("conn-1")
	conn.claims = &pkg.Claims{Username: "guest-1", UserUUID: "guest-1", Role: "user"}
	if _, err := h.OnRoomJoin(conn, `{"room":"text-chat","identity":"guest-1","domain_uuid":"domain-a"}`); err != nil {
		t.Fatalf("join: %v", err)
	}
	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","domain_uuid":"domain-a","content":"hi"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ackMap := decodeAck(t, ack)
	if ackMap["error"] != "permission denied" {
		t.Fatalf("expected permission denied, got %v", ackMap)
	}
	if msgSvc.sendCalled != 0 {
		t.Fatalf("send must not be called, got %d calls", msgSvc.sendCalled)
	}
}

func TestOnMessageSend_DomainPermissionAllowsMember(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{
		{UUID: "uuid-domain", Name: "text-chat", Type: model.RoomTypeText, DomainUUID: "domain-a"},
	}}
	h := newTestHub()
	h.roomStore = store
	h.permChecker = &mockPermChecker{rolePerms: map[string]map[string]bool{"user": {"message:send": true}}}
	h.domainPermChecker = func(domainUUID, userUUID, permCode string) bool {
		return domainUUID == "domain-a" && userUUID == "mod-1" && permCode == "message:send"
	}
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	conn := newMockClient("conn-1")
	conn.claims = &pkg.Claims{Username: "mod-1", UserUUID: "mod-1", Role: "user"}
	if _, err := h.OnRoomJoin(conn, `{"room":"text-chat","identity":"mod-1","domain_uuid":"domain-a"}`); err != nil {
		t.Fatalf("join: %v", err)
	}
	ack, err := h.OnMessageSend(conn, `{"room":"text-chat","domain_uuid":"domain-a","content":"hi"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ackMap := decodeAck(t, ack)
	if ackMap["success"] != true {
		t.Fatalf("expected success, got %v", ackMap)
	}
}
```

同时把 `TestOnMessageDelete_GlobalPermissionFallsBackWhenDomainMissing` 改名为 `TestOnMessageDelete_DomainPermissionGatesDeleteWhenCheckerWired`，并把断言改为期望 `permission denied`（域房间不再回退全局权限）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/signal/ -run 'TestOnMessageSend_DomainPermission|TestOnMessageDelete_DomainPermissionGatesDeleteWhenCheckerWired' -v`

Expected: 三个测试 FAIL（当前 `checkMessagePerm` 不看域权限，guest 发送成功；delete 全局回退仍成功）。

- [ ] **Step 3: 修改 `message_bridge.go`**

将 `checkMessagePerm` 签名与实现改为：

```go
func (h *Hub) checkMessagePerm(c ws.ClientMessenger, roomDomainUUID string) string {
	claims := c.Claims()
	if claims == nil {
		return ""
	}
	role := claims.Role
	if role == "" && h.userStore != nil {
		if user, err := h.userStore.GetByName(clientIdentity(c)); err == nil && user != nil {
			role = user.Role
		}
	}
	if roomDomainUUID != "" && claims.UserUUID != "" && h.domainPermChecker != nil {
		if !h.domainPermChecker(roomDomainUUID, claims.UserUUID, permcode.PermMessageSend) {
			return `{"error":"permission denied"}`
		}
		return ""
	}
	if !permissionGranted(claims, role, permcode.PermMessageSend, h.permChecker) {
		return `{"error":"permission denied"}`
	}
	return ""
}
```

调整 `OnMessageSend`/`OnMessageEdit`/`OnMessageReact`/`OnMessageUnreact`：先 `resolveMessageRoom` 取得 `roomDomainUUID`，再调用 `checkMessagePerm(c, roomDomainUUID)`。以 `OnMessageSend` 为例，把：

```go
	if ack := h.checkMessagePerm(c); ack != "" {
		return ack, nil
	}
```

移动到 `roomUUID, _, roomDomainUUID, ackErr := h.resolveMessageRoom(...)` 之后，并改为：

```go
	if ack := h.checkMessagePerm(c, roomDomainUUID); ack != "" {
		return ack, nil
	}
```

`OnMessageDelete` 同样把 `checkMessagePerm(c)` 移到 `resolveMessageRoom` 之后并传 `roomDomainUUID`；其 `canDeleteOthers` 保持现有域优先逻辑。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./internal/signal/ -run 'TestOnMessage' -v`

Expected: 全部 PASS，包括更新后的 delete 测试。

- [ ] **Step 5: 提交**

```bash
cd /Users/noelorin/GOSpeak/app/server
git add internal/signal/message_bridge.go internal/signal/message_bridge_test.go
git commit -m "fix(signal): enforce domain role permissions on ws message events"
```

---

### Task 6: 全量回归验证

**Files:**
- 无新增代码

- [ ] **Step 1: 运行 Go 全量测试**

Run: `cd /Users/noelorin/GOSpeak/app/server && go test ./...`

Expected: 全部 PASS。

- [ ] **Step 2: 运行前端与集成相关测试**

Run: `cd /Users/noelorin/GOSpeak && pnpm vitest run test/auth/auth.test.ts`

Expected: PASS（需本地后端运行）。

Run: `cd /Users/noelorin/GOSpeak/app/web && pnpm test`

Expected: 现有前端单测全部 PASS（本计划未改前端代码，仅做回归确认）。

- [ ] **Step 3: 静态检查**

Run: `cd /Users/noelorin/GOSpeak/app/server && gofmt -l . && go vet ./...`

Expected: 无未格式化文件，vet 无输出。

- [ ] **Step 4: 提交收尾（若有残留格式改动）**

```bash
cd /Users/noelorin/GOSpeak
git status --short
git add app/server/internal app/server/server test/auth
git commit -m "chore: finalize auth and domain rbac fixes"
```

`git status --short` 确认仅本计划涉及文件后执行；若包含用户未提交改动，只 add 本计划 Task 1-8 修改过的文件，绝不动用户改动。

---


## 后续子计划（不在本文件实现）

- Auth & Session：`2026-08-11-review-p0-auth-session.md`。
- NATS/WAL 与集群：WAL 启动重放、Truncate 截断、queue DeliverPolicy、cluster fence/leader fence、drain NodeID、heartbeat 锁、OwnerNodeID 不匹配。
- SFU/媒体：mediasoup bridge 鉴权、sfu:* 越权、dynamic provider 热切换、text room token、hub connSlots 误删、BroadcastMute nil。
- Bot/前端：ws-ticket 迁移、bot createRoom domain_uuid、botRunner 队列、chatStore/socketStore/domainStore 竞态、私密房 password 透传。
- 存储/迁移/OAuth：seed 默认口令、OAuth secret 迁移、domain_uuid NOT NULL、migrateDomainCUID2 角色表、email 验证原子性、SSRF、S3 MIME。
- 契约/CI：`test/signal/signal.test.ts` 的 ws-ticket 旧用例、`system/stream` 鉴权文档、错误码表同步。

## Self-Review

1. **Spec coverage**：域 RBAC P0 中 room/message HTTP 与 WS 路径、HasDomainPermission 接入、SetMemberRole 提权限制全部覆盖；私密房 password 透传按范围拆分到后续计划。
2. **Placeholder scan**：所有代码步骤均给出完整可编译代码；未出现 TBD/TODO/“适当处理”式占位。
3. **Type consistency**：Task 1 定义的小接口 `domainPermissionChecker`/`globalPermissionChecker` 在 Task 2/3 中被 `*service.DomainService`/`*service.PermissionService` 满足；`checkMessagePerm(c, roomDomainUUID)` 的签名在 Task 5 中所有调用点同步更新。
