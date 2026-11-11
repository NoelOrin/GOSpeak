# 内联注册与域级加入审核 实现计划（第二部分）

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 把邀请链接落地页从「访客直达」改造成「内联注册 → 按域策略直接入域或提交审核 → 管理员审批」的完整链路：新增 `domains.require_join_approval` 开关与 `domain_join_requests` 状态机表，新增 `InviteJoinService` / `JoinRequestService` 两个服务、公开注册与加入端点、管理端审核端点，并重写 `/invite/d/$code` 落地页与域管理页审核分区。

**架构：** 注册与入域放在**同一个数据库事务**里——`INSERT users` → 条件 UPDATE 消耗邀请码次数 → 写 `domain_invite_redemptions` → 按 `domains.require_join_approval` 分支写 `domain_members`（直接入域）或 `domain_join_requests(status=pending)`（待审）。审核用条件 UPDATE `WHERE status='pending'` 防重复处理，`RowsAffected=0` 即报错。审核开关只有域级一个，邀请码、公开域免码加入、内联注册三条路径读同一字段，不存在绕过。

**技术栈：** Go 1.26 + Gin + GORM（SQLite/PostgreSQL/MySQL）、React 19 + TypeScript + zustand + TanStack Router/Query、vitest。

**规格来源：** `docs/superpowers/specs/2026-11-10-invite-registration-approval-design.md` §4.3、§4.4、§5、§6.1、§6.2、§6.4、§6.5、§7、§8

---

## 前置依赖（必须先满足）

本计划**假设第一部分 `2026-11-10-remove-guest-and-invite-codes.md` 的任务 1–5 已完成**，下列符号已存在，第二部分**只复用、不重复定义**：

| 符号 | 位置 |
|------|------|
| `DeriveInviteKey` / `DomainPrefixOf` / `EncodeInviteCode` / `DecodeInviteCode` / `InviteCode` / `ErrInviteCodeMalformed` / `ErrInviteCodeInvalid` / `ErrInviteCodeExpired` / `InviteMaxUsesLimit` / `inviteCodeLength` | `internal/service/invite_code.go` |
| `DomainInvite`（含 `IsDisabled()` / `IsExhausted()`）、`DomainInviteRedemption`、`InviteFail*` | `internal/model/domain_invite.go` |
| `NewInviteService(db, inviteRepo, domainRepo, storageEncryptKey, jwtKey)`、`Create`、`CreateDefaultForDomain`、`Validate(raw) (*InviteValidation, error)`、`Consume`、`Disable`、`List`、`ListRedemptions`、`InviteValidation{Invite, Domain, RemainingUses}` | `internal/service/invite_service.go` |
| `DomainInviteRepository`（`Create/GetByID/ConsumeUse/Disable/UpdateCode/ListByDomain/CreateRedemption/ListRedemptions`） | `internal/repository/domain_invite_repo.go` |
| `pkg.IsCode(err, code)` | `internal/pkg/errors.go` |
| `PermDomainInviteManage = "domain:invite:manage"`、`PermDomainMemberReview = "domain:member:review"`（已进 `DefaultPermissions`、`DefaultRolePermissions["admin"]`、`AssignableDomainPermissions`、`DefaultDomainRolePermissions[admin]`） | `internal/permcode/domain_permcode.go`、`internal/model/permission.go`、`internal/model/domain_role.go` |
| `InviteHandler`（`Create/List/Disable/Redemptions/Preview`）、`routes/invite`（`Register`/`RegisterPublic`） | `internal/handler/invite_handler.go`、`internal/router/routes/invite/routes.go` |
| `gin.go` 中的 `inviteSvc`、`inviteH` 与 `router.Handlers.Invite` | `server/gin.go`、`internal/router/router.go` |

若上述任一项缺失，先回去完成第一部分，不要在本计划里重新定义。

---

## 文件结构

### 后端（新建）

| 文件 | 职责 |
|------|------|
| `app/server/internal/model/domain_join_request.go` | `DomainJoinRequest` 表 + `DomainJoinRequestInfo` 列表视图 + 状态常量 |
| `app/server/internal/model/domain_join_request_test.go` | 状态常量、`IsPending`、表名、`RequireJoinApproval` 默认值 |
| `app/server/internal/repository/domain_join_request_repo.go` | 提交（幂等 upsert）、按域+人查、物理删除、条件审核更新、分页搜索、按状态计数 |
| `app/server/internal/repository/domain_join_request_repo_test.go` | 仓库单测（内存 SQLite，含并发审核用例） |
| `app/server/internal/service/join_request_service.go` | 提交/幂等/撤销/审核/列表/域级开关读写 |
| `app/server/internal/service/join_request_service_test.go` | 服务单测：重复提交幂等、通过/拒绝、重复审核、被拒后重申、开关读写 |
| `app/server/internal/service/invite_join_service.go` | `Preview` / `RegisterAndJoin` / `Join` / `MyRequest` / `CancelRequest`，注册与入域同事务 |
| `app/server/internal/service/invite_join_service_test.go` | 服务单测：注册即入域、注册即待审、过期码、次数耗尽、公开域免码、私有域拒入 |
| `app/server/internal/handler/invite_join_handler.go` | 4 个端点：`/invite/register`、`/invite/join`、`/invite/my-request`、`/invite/cancel-request` |
| `app/server/internal/handler/invite_join_handler_test.go` | 参数校验、错误码映射、Cookie 签发 |
| `app/server/internal/handler/join_request_handler.go` | 3 个端点：`/domain/join-requests/list`、`/domain/join-requests/review`、`/domain/join-approval/config` |
| `app/server/internal/handler/join_request_handler_test.go` | 权限校验、分页、搜索 |
| `app/server/internal/router/routes/invite_join/routes.go` | 公开/认证/管理三组路由注册 |
| `test/invite/invite.test.ts` | 集成测试：建码→注册→审核→进域、免审核直达、过期码、次数上限、跨域隔离、公开域免码 |

### 后端（修改）

| 文件 | 改动 |
|------|------|
| `app/server/internal/model/domain.go` | 新增 `RequireJoinApproval bool` 字段（默认 false） |
| `app/server/internal/repository/db_migrations.go` | `autoMigrate()` 列表追加 `&model.DomainJoinRequest{}` |
| `app/server/internal/service/domain_service.go` | 新增 `OnMemberAdded(domainUUID, userUUID)`，供审核通过后清成员缓存并广播失效 |
| `app/server/internal/router/routes/invite/routes.go` | `RegisterPublic` 的 `/preview` 加 `middleware.RateLimit(20, time.Hour)`（preview 端点已由第一部分建好，本部分只补限流，**不改响应结构**） |
| `app/server/internal/router/router.go` | `Handlers` 加 `InviteJoin` / `JoinRequest`；注册公开、认证、管理三组路由 |
| `app/server/server/gin.go` | 构造 `joinRequestSvc` / `inviteJoinSvc` / `InviteJoinHandler` / `JoinRequestHandler`，塞进 `Handlers` |

### 前端（新建）

| 文件 | 职责 |
|------|------|
| `app/web/src/api/inviteJoin.ts` | preview/register/join/my-request/cancel-request + 审核管理 9 个 API 函数与 DTO |
| `app/web/src/api/inviteJoin.spec.ts` | API 层单测：URL、请求体、skipErrorToast |
| `app/web/src/components/invite/InviteRegisterForm.tsx` | 内联注册表单（用户名/密码/昵称/邮箱验证码） |
| `app/web/src/components/invite/JoinRequestStatusCard.tsx` | 申请状态卡片（审核中/未通过 + 撤销/重新申请） |
| `app/web/src/components/domain/JoinRequestTable.tsx` | 域管理页「加入申请」表格（分页 + 搜索 + 通过/拒绝） |
| `app/web/src/components/domain/JoinRequestTable.test.tsx` | 表格组件单测（空态/行渲染/已审行无操作/错误态） |
| `app/web/src/utils/joinTarget.ts` | 纯函数：决定 `/invite/join` 用 `code` 还是 `domain_uuid` |
| `app/web/src/utils/joinTarget.spec.ts` | 上述纯函数单测 |
| `app/web/src/components/invite/inviteComponents.test.tsx` | 注册表单与状态卡片渲染单测 |
| `app/web/src/pages/invite/__tests__/inviteLanding.test.tsx` | 五种身份分支渲染单测 |

### 前端（修改）

| 文件 | 改动 |
|------|------|
| `app/web/src/pages/invite/d/$code.tsx` | 整体重写：五种身份分流 + 内联注册 + 状态卡片 |
| `app/web/src/api/domain.ts` | `Domain` 加 `require_join_approval?: boolean`；`updateDomain` 入参加该字段；收尾任务删除 `previewDomainInvite` / `joinDomain` |
| `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx` | 新增「加入申请」分区与审核开关 state/handler |
| `app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainSettingsForm.tsx` | 加 `requireJoinApproval` / `setRequireJoinApproval` props 与开关行 |
| `app/web/src/pages/(app)/discover.tsx` | 公开域加入改走 `/invite/join {domain_uuid}`；加载自己的待审域并在卡片上显示「申请审核中」；收尾任务去掉 `previewDomainInvite` / `joinDomain` |
| `app/web/src/components/discover/DiscoverDomainGrid.tsx` | 新增 `pendingUUIDs` prop，卡片按钮按「申请审核中 / 进入域 / 加入」三态渲染 |
| `app/web/src/utils/errorMessages.ts` | 补邀请链接与审核相关中文词条 |
| `app/web/src/utils/permissions.ts` | admin 兜底表补 `domain:invite:manage`、`domain:member:review`（第一部分若已加则跳过） |

### 前端（删除）

本部分不新增删除项；`previewDomainInvite` / `joinDomain` / `/domain/join` / `/domain/preview` 的删除集中在任务 15。

---

## 任务 1：数据模型与迁移

**文件：**
- 创建：`app/server/internal/model/domain_join_request.go`
- 修改：`app/server/internal/model/domain.go:11-27`（`Domain` 结构体加 `RequireJoinApproval`）
- 修改：`app/server/internal/repository/db_migrations.go:237-268`（`autoMigrate()` 列表）
- 测试：`app/server/internal/model/domain_join_request_test.go`

> 说明：`Domain.RequireJoinApproval` 可能已在第一部分收尾时被顺手加入（当前工作区 `model/domain.go` 已有该字段）。若已存在，步骤 3 的改动改为「确认字段与 tag 逐字一致，不做修改」。

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/model/domain_join_request_test.go`：

```go
package model

import (
	"testing"
	"time"
)

func TestDomainJoinRequest_StatusConstants(t *testing.T) {
	cases := map[string]string{
		JoinRequestPending:  "pending",
		JoinRequestApproved: "approved",
		JoinRequestRejected: "rejected",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("status constant = %q, want %q", got, want)
		}
	}
}

func TestDomainJoinRequest_IsPending(t *testing.T) {
	if !(&DomainJoinRequest{Status: JoinRequestPending}).IsPending() {
		t.Fatal("pending request must report pending")
	}
	if (&DomainJoinRequest{Status: JoinRequestApproved}).IsPending() {
		t.Fatal("approved request must not report pending")
	}
	if (*DomainJoinRequest)(nil).IsPending() {
		t.Fatal("nil request must not report pending")
	}
}

func TestDomainJoinRequest_TableName(t *testing.T) {
	if got := (DomainJoinRequest{}).TableName(); got != "domain_join_requests" {
		t.Fatalf("table name = %q, want domain_join_requests", got)
	}
}

func TestDomainJoinRequestInfo_EmbedsStatus(t *testing.T) {
	now := time.Now()
	info := DomainJoinRequestInfo{
		ID:         1,
		DomainUUID: "d1",
		UserUUID:   "u1",
		Status:     JoinRequestPending,
		CreatedAt:  now,
		Name:       "申请人",
	}
	if info.Status != JoinRequestPending || info.Name != "申请人" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestDomain_RequireJoinApprovalDefaultsFalse(t *testing.T) {
	d := &Domain{Name: "realm", OwnerUUID: "owner-1"}
	if d.RequireJoinApproval {
		t.Fatal("require_join_approval must default to false")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/model/ -run 'TestDomainJoinRequest|TestDomain_RequireJoinApproval' -v`
预期：FAIL，编译错误 `undefined: JoinRequestPending`（及 `DomainJoinRequest`、`DomainJoinRequestInfo`）

- [ ] **步骤 3：新增域字段**

修改 `app/server/internal/model/domain.go` 的 `Domain` 结构体，在 `IsPublic` 之后插入一行（字段顺序与 tag 逐字照抄）：

```go
	IsPublic            bool      `gorm:"default:false" json:"is_public"`
	RequireJoinApproval bool      `gorm:"default:false" json:"require_join_approval"`
```

- [ ] **步骤 4：创建加入申请表模型**

创建 `app/server/internal/model/domain_join_request.go`：

```go
package model

import "time"

// 加入申请状态。
const (
	JoinRequestPending  = "pending"
	JoinRequestApproved = "approved"
	JoinRequestRejected = "rejected"
)

// DomainJoinRequest 是一个用户对一个域的加入申请。
// (domain_uuid, user_uuid) 唯一：一个用户对一个域仅一条记录。
// 撤销申请是物理删除，不引入 cancelled 状态。
type DomainJoinRequest struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DomainUUID string     `gorm:"size:32;uniqueIndex:uniq_join_req_domain_user,priority:1;not null" json:"domain_uuid"`
	UserUUID   string     `gorm:"size:36;uniqueIndex:uniq_join_req_domain_user,priority:2;not null" json:"user_uuid"`
	InviteID   uint       `gorm:"default:0" json:"invite_id"`
	Status     string     `gorm:"size:16;default:pending;index" json:"status"`
	ReviewedBy string     `gorm:"size:36" json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (DomainJoinRequest) TableName() string { return "domain_join_requests" }

// IsPending 报告申请是否仍在待审状态。
func (r *DomainJoinRequest) IsPending() bool {
	return r != nil && r.Status == JoinRequestPending
}

// DomainJoinRequestInfo 是管理端审核列表视图，附带申请人全局名称。
type DomainJoinRequestInfo struct {
	ID          uint       `json:"id"`
	DomainUUID  string     `json:"domain_uuid"`
	UserUUID    string     `json:"user_uuid"`
	InviteID    uint       `json:"invite_id"`
	Status      string     `json:"status"`
	ReviewedBy  string     `json:"reviewed_by"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name"`
}
```

- [ ] **步骤 5：注册 AutoMigrate**

修改 `app/server/internal/repository/db_migrations.go` 的 `autoMigrate()`，在 `&model.DomainRolePermission{},` 之后追加一行：

```go
		&model.DomainRolePermission{},
		&model.DomainJoinRequest{},
		&model.AuditLog{},
```

- [ ] **步骤 6：运行测试验证通过**

运行：`cd app/server && go test ./internal/model/ -run 'TestDomainJoinRequest|TestDomain_RequireJoinApproval' -v && go build ./...`
预期：5 个测试函数 PASS；`go build ./...` 无输出

- [ ] **步骤 7：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/model/domain.go app/server/internal/model/domain_join_request.go app/server/internal/model/domain_join_request_test.go app/server/internal/repository/db_migrations.go
git commit -m "feat(domain): 加入审核开关字段与加入申请数据模型"
```

---

## 任务 2：加入申请仓库层

**文件：**
- 创建：`app/server/internal/repository/domain_join_request_repo.go`
- 测试：`app/server/internal/repository/domain_join_request_repo_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/repository/domain_join_request_repo_test.go`：

```go
package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupJoinRequestRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.DomainJoinRequest{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedJoinApplicant(t *testing.T, db *gorm.DB, name, displayName string) *model.User {
	t.Helper()
	u := &model.User{Name: name, DisplayName: displayName}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestDomainJoinRequestRepository_SubmitCreatesPending(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	got, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got.Status != model.JoinRequestPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}
	if got.InviteID != 7 {
		t.Fatalf("invite id = %d, want 7", got.InviteID)
	}
	if got.ID == 0 {
		t.Fatal("submit must assign primary key")
	}
}

func TestDomainJoinRequestRepository_SubmitIsIdempotent(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	first, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := repo.Submit("domain-1", "user-1", 9)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("second submit created a new row: %d != %d", second.ID, first.ID)
	}
	var count int64
	db.Model(&model.DomainJoinRequest{}).Count(&count)
	if count != 1 {
		t.Fatalf("rows = %d, want 1 (unique index must hold)", count)
	}
}

func TestDomainJoinRequestRepository_GetByDomainUser_NotFound(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	if _, err := repo.GetByDomainUser("d", "u"); err != gorm.ErrRecordNotFound {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestDomainJoinRequestRepository_Reopen(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	req, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := repo.Review(req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := repo.Reopen(req.ID, 42); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := repo.GetByID(req.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.JoinRequestPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}
	if got.InviteID != 42 {
		t.Fatalf("invite id = %d, want 42 (must refresh)", got.InviteID)
	}
	if got.ReviewedBy != "" || got.ReviewedAt != nil {
		t.Fatalf("reopen must clear review fields: %+v", got)
	}
}

func TestDomainJoinRequestRepository_ReviewApprove(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	req, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	ok, err := repo.Review(req.ID, true, "admin-1")
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !ok {
		t.Fatal("first review must report affected")
	}
	got, err := repo.GetByID(req.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.JoinRequestApproved {
		t.Fatalf("status = %q, want approved", got.Status)
	}
	if got.ReviewedBy != "admin-1" || got.ReviewedAt == nil {
		t.Fatalf("review fields not written: %+v", got)
	}
}

func TestDomainJoinRequestRepository_ReviewTwiceFails(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	req, err := repo.Submit("domain-1", "user-1", 7)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := repo.Review(req.ID, true, "admin-1"); err != nil {
		t.Fatalf("first review: %v", err)
	}
	ok, err := repo.Review(req.ID, false, "admin-2")
	if err != nil {
		t.Fatalf("second review: %v", err)
	}
	if ok {
		t.Fatal("second review must not affect any row")
	}
	got, _ := repo.GetByID(req.ID)
	if got.Status != model.JoinRequestApproved || got.ReviewedBy != "admin-1" {
		t.Fatalf("second review must not overwrite: %+v", got)
	}
}

func TestDomainJoinRequestRepository_DeleteByDomainUser(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	if _, err := repo.Submit("domain-1", "user-1", 7); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := repo.DeleteByDomainUser("domain-1", "user-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByDomainUser("domain-1", "user-1"); err != gorm.ErrRecordNotFound {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound (physical delete)", err)
	}
}

func TestDomainJoinRequestRepository_ListByDomain_FiltersAndSearches(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	alice := seedJoinApplicant(t, db, "alice", "爱丽丝")
	bob := seedJoinApplicant(t, db, "bob", "鲍勃")
	if _, err := repo.Submit("domain-1", alice.UUID, 1); err != nil {
		t.Fatalf("submit alice: %v", err)
	}
	bobReq, err := repo.Submit("domain-1", bob.UUID, 1)
	if err != nil {
		t.Fatalf("submit bob: %v", err)
	}
	if _, err := repo.Submit("domain-2", bob.UUID, 1); err != nil {
		t.Fatalf("submit bob other domain: %v", err)
	}
	if _, err := repo.Review(bobReq.ID, true, "admin-1"); err != nil {
		t.Fatalf("approve bob: %v", err)
	}

	all, total, err := repo.ListByDomain("domain-1", "", "", 1, 20)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("all = %d/%d, want 2/2", len(all), total)
	}

	pending, total, err := repo.ListByDomain("domain-1", model.JoinRequestPending, "", 1, 20)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if total != 1 || pending[0].UserUUID != alice.UUID {
		t.Fatalf("pending = %+v total=%d", pending, total)
	}

	hit, total, err := repo.ListByDomain("domain-1", "", "爱丽", 1, 20)
	if err != nil {
		t.Fatalf("list keyword: %v", err)
	}
	if total != 1 || hit[0].Name != "alice" || hit[0].DisplayName != "爱丽丝" {
		t.Fatalf("keyword search = %+v total=%d", hit, total)
	}
}

func TestDomainJoinRequestRepository_CountByStatus(t *testing.T) {
	db := setupJoinRequestRepoDB(t)
	repo := NewDomainJoinRequestRepository(db)
	a := seedJoinApplicant(t, db, "a", "A")
	b := seedJoinApplicant(t, db, "b", "B")
	if _, err := repo.Submit("domain-1", a.UUID, 1); err != nil {
		t.Fatalf("submit a: %v", err)
	}
	req, err := repo.Submit("domain-1", b.UUID, 1)
	if err != nil {
		t.Fatalf("submit b: %v", err)
	}
	if _, err := repo.Review(req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject b: %v", err)
	}
	counts, err := repo.CountByStatus("domain-1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if counts[model.JoinRequestPending] != 1 || counts[model.JoinRequestRejected] != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if counts[model.JoinRequestApproved] != 0 {
		t.Fatalf("approved count = %d, want 0", counts[model.JoinRequestApproved])
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run TestDomainJoinRequestRepository -v`
预期：FAIL，编译错误 `undefined: NewDomainJoinRequestRepository`

- [ ] **步骤 3：编写仓库实现**

创建 `app/server/internal/repository/domain_join_request_repo.go`：

```go
package repository

import (
	"strings"
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DomainJoinRequestRepository 是 domain_join_requests 的数据访问层。
type DomainJoinRequestRepository struct {
	db *gorm.DB
}

func NewDomainJoinRequestRepository(db *gorm.DB) *DomainJoinRequestRepository {
	return &DomainJoinRequestRepository{db: db}
}

// Submit 幂等提交申请：无记录则插入 pending；已有记录则原样返回，
// 由服务层按当前状态决定是否 Reopen。并发同用户提交由唯一索引兜底。
func (r *DomainJoinRequestRepository) Submit(domainUUID, userUUID string, inviteID uint) (*model.DomainJoinRequest, error) {
	req := &model.DomainJoinRequest{
		DomainUUID: domainUUID,
		UserUUID:   userUUID,
		InviteID:   inviteID,
		Status:     model.JoinRequestPending,
	}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(req).Error; err != nil {
		return nil, err
	}
	return r.GetByDomainUser(domainUUID, userUUID)
}

func (r *DomainJoinRequestRepository) GetByID(id uint) (*model.DomainJoinRequest, error) {
	var req model.DomainJoinRequest
	err := r.db.Where("id = ?", id).First(&req).Error
	return &req, err
}

func (r *DomainJoinRequestRepository) GetByDomainUser(domainUUID, userUUID string) (*model.DomainJoinRequest, error) {
	var req model.DomainJoinRequest
	err := r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).First(&req).Error
	return &req, err
}

// Reopen 把已处理（rejected / approved）的申请打回 pending 并刷新邀请码。
// 条件更新保证并发下不会覆盖他人刚写入的审核结果。
func (r *DomainJoinRequestRepository) Reopen(id uint, inviteID uint) error {
	return r.db.Model(&model.DomainJoinRequest{}).
		Where("id = ? AND status <> ?", id, model.JoinRequestPending).
		Updates(map[string]interface{}{
			"status":      model.JoinRequestPending,
			"invite_id":   inviteID,
			"reviewed_by": "",
			"reviewed_at": nil,
		}).Error
}

// Review 条件审核：仅当仍为 pending 时生效。
// 返回 false 表示申请已被他人处理，调用方应回滚事务并报错。
func (r *DomainJoinRequestRepository) Review(id uint, approve bool, reviewerUUID string) (bool, error) {
	status := model.JoinRequestRejected
	if approve {
		status = model.JoinRequestApproved
	}
	now := time.Now()
	res := r.db.Model(&model.DomainJoinRequest{}).
		Where("id = ? AND status = ?", id, model.JoinRequestPending).
		Updates(map[string]interface{}{
			"status":      status,
			"reviewed_by": reviewerUUID,
			"reviewed_at": &now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteByDomainUser 物理删除申请（撤销语义，不保留 cancelled 状态）。
func (r *DomainJoinRequestRepository) DeleteByDomainUser(domainUUID, userUUID string) error {
	return r.db.Where("domain_uuid = ? AND user_uuid = ?", domainUUID, userUUID).
		Delete(&model.DomainJoinRequest{}).Error
}

// ListByDomain 分页查询域内申请，可按状态过滤、按申请人名字搜索。
func (r *DomainJoinRequestRepository) ListByDomain(domainUUID, status, keyword string, page, pageSize int) ([]model.DomainJoinRequestInfo, int64, error) {
	var items []model.DomainJoinRequestInfo
	var total int64

	q := r.db.Table("domain_join_requests").
		Joins("LEFT JOIN users ON users.uuid = domain_join_requests.user_uuid").
		Where("domain_join_requests.domain_uuid = ?", domainUUID)
	if status != "" {
		q = q.Where("domain_join_requests.status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("users.name LIKE ? OR users.display_name LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Select(`domain_join_requests.id, domain_join_requests.domain_uuid,
		domain_join_requests.user_uuid, domain_join_requests.invite_id,
		domain_join_requests.status, domain_join_requests.reviewed_by,
		domain_join_requests.reviewed_at, domain_join_requests.created_at,
		domain_join_requests.updated_at,
		COALESCE(users.name, '') AS name,
		COALESCE(users.display_name, '') AS display_name`).
		Order("domain_join_requests.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// CountByStatus 返回域内各状态的申请数，供管理页分区标题使用。
func (r *DomainJoinRequestRepository) CountByStatus(domainUUID string) (map[string]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	err := r.db.Model(&model.DomainJoinRequest{}).
		Select("status, COUNT(*) AS total").
		Where("domain_uuid = ?", domainUUID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{
		model.JoinRequestPending:  0,
		model.JoinRequestApproved: 0,
		model.JoinRequestRejected: 0,
	}
	for _, item := range rows {
		out[item.Status] = item.Total
	}
	return out, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run TestDomainJoinRequestRepository -v`
预期：全部 PASS（10 个测试函数）

- [ ] **步骤 5：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/repository/domain_join_request_repo.go app/server/internal/repository/domain_join_request_repo_test.go
git commit -m "feat(domain): 加入申请仓库（幂等提交/条件审核/分页搜索）"
```

---

## 任务 3：加入申请服务层

**文件：**
- 创建：`app/server/internal/service/join_request_service.go`
- 测试：`app/server/internal/service/join_request_service_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/service/join_request_service_test.go`：

```go
package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupJoinRequestService(t *testing.T) (*JoinRequestService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewJoinRequestService(
		db,
		repository.NewDomainJoinRequestRepository(db),
		repository.NewDomainRepository(db),
		repository.NewUserRepository(db),
	)
	return svc, db
}

func seedJoinDomain(t *testing.T, db *gorm.DB, requireApproval bool) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "JoinRealm", OwnerUUID: "owner-1", RequireJoinApproval: requireApproval}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func seedJoinUser(t *testing.T, db *gorm.DB, name string) *model.User {
	t.Helper()
	u := &model.User{Name: name, DisplayName: name}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func TestJoinRequestService_SubmitCreatesPending(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if req.Status != model.JoinRequestPending || req.InviteID != 11 {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestJoinRequestService_SubmitTwiceIsIdempotent(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	first, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := svc.Submit(d.UUID, u.UUID, 12)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.ID != first.ID || second.InviteID != 11 {
		t.Fatalf("repeat submit must be idempotent: first=%+v second=%+v", first, second)
	}
}

func TestJoinRequestService_SubmitRejectsMember(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "member")
	if err := db.Create(&model.DomainMember{DomainUUID: d.UUID, UserUUID: u.UUID, RoleName: model.DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if _, err := svc.Submit(d.UUID, u.UUID, 0); err != ErrAlreadyMember {
		t.Fatalf("err = %v, want ErrAlreadyMember", err)
	}
}

func TestJoinRequestService_SubmitAfterRejectReopens(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(d.UUID, req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	reopened, err := svc.Submit(d.UUID, u.UUID, 22)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if reopened.Status != model.JoinRequestPending || reopened.InviteID != 22 {
		t.Fatalf("resubmit must reopen with fresh invite: %+v", reopened)
	}
}

func TestJoinRequestService_SubmitAfterApprovedAndLeftReopens(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(d.UUID, req.ID, true, "admin-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// 曾离开域：成员关系被删除，再次申请必须回到 pending。
	if err := db.Where("domain_uuid = ? AND user_uuid = ?", d.UUID, u.UUID).Delete(&model.DomainMember{}).Error; err != nil {
		t.Fatalf("remove member: %v", err)
	}
	reopened, err := svc.Submit(d.UUID, u.UUID, 33)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if reopened.Status != model.JoinRequestPending {
		t.Fatalf("status = %q, want pending", reopened.Status)
	}
}

func TestJoinRequestService_CancelDeletesRecord(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	if _, err := svc.Submit(d.UUID, u.UUID, 11); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := svc.Cancel(d.UUID, u.UUID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	found, err := svc.Find(d.UUID, u.UUID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found != nil {
		t.Fatalf("cancel must physically delete the record: %+v", found)
	}
}

func TestJoinRequestService_CancelRejectsReviewed(t *testing.T) {
	svc, db := setupJoinRequestService(t)
	d := seedJoinDomain(t, db, true)
	u := seedJoinUser(t, db, "applicant")

	req, err := svc.Submit(d.UUID, u.UUID, 11)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(d.UUID, req.ID, false, "admin-1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := svc.Cancel(d.UUID, u.UUID); err != ErrJoinRequestNotPending {
		t.Fatalf("err = %v, want ErrJoinRequestNotPending", err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestJoinRequestService -v`
预期：FAIL，编译错误 `undefined: NewJoinRequestService`

- [ ] **步骤 3：编写实现**

创建 `app/server/internal/service/join_request_service.go`：

```go
package service

import (
	"errors"
	"strings"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

var (
	// ErrJoinRequestNotFound 申请不存在或不属于该域。
	ErrJoinRequestNotFound = pkg.NewAppError(pkg.NOT_FOUND, "join request not found")
	// ErrJoinRequestAlreadyReviewed 申请已被他人处理（条件更新未命中）。
	ErrJoinRequestAlreadyReviewed = pkg.NewAppError(pkg.ALREADY_EXISTS, "join request has already been reviewed")
	// ErrJoinRequestNotPending 申请不是待审状态，无法撤销。
	ErrJoinRequestNotPending = pkg.NewAppError(pkg.INVALID_PARAMS, "join request is not pending")
)

// JoinRequestService 管理域加入申请的提交、撤销、审核与查询。
type JoinRequestService struct {
	db            *gorm.DB
	requestRepo   *repository.DomainJoinRequestRepository
	domainRepo    *repository.DomainRepository
	userRepo      *repository.UserRepository
	onMemberAdded func(domainUUID, userUUID string)
}

func NewJoinRequestService(
	db *gorm.DB,
	requestRepo *repository.DomainJoinRequestRepository,
	domainRepo *repository.DomainRepository,
	userRepo *repository.UserRepository,
) *JoinRequestService {
	return &JoinRequestService{db: db, requestRepo: requestRepo, domainRepo: domainRepo, userRepo: userRepo}
}

// SetMemberAddedHook 注入「新成员已写入」回调，用于清理域成员缓存并广播失效。
func (s *JoinRequestService) SetMemberAddedHook(fn func(domainUUID, userUUID string)) {
	s.onMemberAdded = fn
}

// Submit 幂等提交申请。已是成员返回 ErrAlreadyMember；
// pending 直接返回原记录；rejected / approved（但已不是成员）回到 pending。
func (s *JoinRequestService) Submit(domainUUID, userUUID string, inviteID uint) (*model.DomainJoinRequest, error) {
	if domainUUID == "" || userUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid and user_uuid are required")
	}
	if _, err := s.domainRepo.GetByUUID(domainUUID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if _, err := s.domainRepo.GetMember(domainUUID, userUUID); err == nil {
		return nil, ErrAlreadyMember
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	req, err := s.requestRepo.Submit(domainUUID, userUUID, inviteID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if req.IsPending() {
		return req, nil
	}
	if err := s.requestRepo.Reopen(req.ID, inviteID); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.requestRepo.GetByDomainUser(domainUUID, userUUID)
}

// Find 返回用户在该域的申请；不存在返回 (nil, nil)。
func (s *JoinRequestService) Find(domainUUID, userUUID string) (*model.DomainJoinRequest, error) {
	req, err := s.requestRepo.GetByDomainUser(domainUUID, userUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return req, nil
}

// Cancel 撤销待审申请（物理删除）。
func (s *JoinRequestService) Cancel(domainUUID, userUUID string) error {
	req, err := s.Find(domainUUID, userUUID)
	if err != nil {
		return err
	}
	if req == nil {
		return ErrJoinRequestNotFound
	}
	if !req.IsPending() {
		return ErrJoinRequestNotPending
	}
	if err := s.requestRepo.DeleteByDomainUser(domainUUID, userUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// Review 审核申请。approve=true 时在同一事务内写入 domain_members。
// 条件更新 RowsAffected=0 表示已被他人处理，返回 ErrJoinRequestAlreadyReviewed。
func (s *JoinRequestService) Review(domainUUID string, requestID uint, approve bool, reviewerUUID string) (*model.DomainJoinRequest, error) {
	if domainUUID == "" || requestID == 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid and request_id are required")
	}
	req, err := s.requestRepo.GetByID(requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJoinRequestNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if req.DomainUUID != domainUUID {
		return nil, ErrJoinRequestNotFound
	}
	if !req.IsPending() {
		return nil, ErrJoinRequestAlreadyReviewed
	}
	// 申请人已被删除时不允许通过：先落 rejected，再报错。
	if approve {
		if _, err := s.userRepo.GetByUUID(req.UserUUID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_, _ = s.requestRepo.Review(requestID, false, reviewerUUID)
				return nil, pkg.NewAppError(pkg.NOT_FOUND, "applicant no longer exists")
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDomainJoinRequestRepository(tx)
		ok, err := repo.Review(requestID, approve, reviewerUUID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrJoinRequestAlreadyReviewed
		}
		if !approve {
			return nil
		}
		domainRepo := repository.NewDomainRepository(tx)
		if _, err := domainRepo.GetMember(domainUUID, req.UserUUID); err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return domainRepo.AddMember(&model.DomainMember{
			DomainUUID: domainUUID,
			UserUUID:   req.UserUUID,
			RoleName:   model.DomainRoleMember,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}
	if approve && s.onMemberAdded != nil {
		s.onMemberAdded(domainUUID, req.UserUUID)
	}
	return s.requestRepo.GetByID(requestID)
}

// List 分页查询域内申请，可按状态过滤、按申请人名字搜索。
func (s *JoinRequestService) List(domainUUID, status, keyword string, page, pageSize int) ([]model.DomainJoinRequestInfo, int64, error) {
	if domainUUID == "" {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	if status != "" && status != model.JoinRequestPending &&
		status != model.JoinRequestApproved && status != model.JoinRequestRejected {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "status must be pending, approved or rejected")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := s.requestRepo.ListByDomain(domainUUID, status, strings.TrimSpace(keyword), page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, total, nil
}

// Counts 返回域内各状态的申请数。
func (s *JoinRequestService) Counts(domainUUID string) (map[string]int64, error) {
	if domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	counts, err := s.requestRepo.CountByStatus(domainUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return counts, nil
}

// SetRequireApproval 读写域级「加入需审核」开关。
func (s *JoinRequestService) SetRequireApproval(domainUUID string, require bool) (*model.Domain, error) {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	domain.RequireJoinApproval = require
	if err := s.domainRepo.Update(domain); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestJoinRequestService -v`
预期：全部 PASS（8 个测试函数）

- [ ] **步骤 5：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/service/join_request_service.go app/server/internal/service/join_request_service_test.go
git commit -m "feat(domain): 加入申请服务（幂等提交/撤销/条件审核/开关）"
```

---

## 任务 4：内联注册与加入服务

**文件：**
- 创建：`app/server/internal/service/invite_join_service.go`
- 测试：`app/server/internal/service/invite_join_service_test.go`

> **接口约定：** `Preview` 不新建服务方法——它继续由第一部分的 `InviteService.Validate` + `InviteHandler.Preview` 承担，且响应结构**不变**（任务 5 只给该端点补 20/h/IP 限流）。本任务只实现 `RegisterAndJoin` / `Join` / `MyRequest` / `CancelRequest`。
>
> **测试辅助复用：** `testStorageKey` 常量由第一部分的 `service/invite_service_test.go` 定义（同包）。若该常量不存在，在本测试文件里改用字面量 `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`，**不要重复定义**（同包重复声明会编译失败）。
>
> **为什么不复用 `InviteService.Consume`：** `Consume` 内部自带 `s.db.Transaction(...)`，无法并入「建用户 + 消耗码 + 入域/建申请」这一层事务。本任务在事务内直接用 `repository.NewDomainInviteRepository(tx).ConsumeUse(...)` 完成同一条条件更新，语义与 `Consume` 完全一致。

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/service/invite_join_service_test.go`：

```go
package service

import (
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupInviteJoinService(t *testing.T) (*InviteJoinService, *InviteService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainInvite{}, &model.DomainInviteRedemption{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	inviteRepo := repository.NewDomainInviteRepository(db)
	inviteSvc := NewInviteService(db, inviteRepo, domainRepo, testStorageKey, "jwt-key")
	joinReqSvc := NewJoinRequestService(db, repository.NewDomainJoinRequestRepository(db), domainRepo, repository.NewUserRepository(db))
	svc := NewInviteJoinService(
		db, inviteSvc, joinReqSvc, inviteRepo,
		domainRepo, repository.NewUserRepository(db),
		nil, nil,
	)
	return svc, inviteSvc, db
}

func seedJoinInviteDomain(t *testing.T, db *gorm.DB, requireApproval bool) (*model.Domain, *model.DomainInvite) {
	t.Helper()
	d := &model.Domain{Name: "JoinRealm", OwnerUUID: "owner-1", RequireJoinApproval: requireApproval}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	key := DeriveInviteKey(testStorageKey, "jwt-key")
	invite := &model.DomainInvite{DomainUUID: d.UUID, Label: "测试", MaxUses: 0, CreatedBy: "owner-1"}
	if err := db.Create(invite).Error; err != nil {
		t.Fatalf("seed invite: %v", err)
	}
	code, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf(d.UUID), InviteID: uint32(invite.ID),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	invite.Code = code
	if err := db.Save(invite).Error; err != nil {
		t.Fatalf("save code: %v", err)
	}
	return d, invite
}

func TestInviteJoinService_RegisterAndJoin_DirectJoin(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, false)

	res, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "newbie", Password: "password123", Nickname: "小明",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Status != JoinStatusJoined {
		t.Fatalf("status = %q, want joined", res.Status)
	}
	if res.Token == "" || res.RefreshToken == "" {
		t.Fatal("tokens must be issued")
	}
	member, err := repository.NewDomainRepository(db).GetMember(d.UUID, res.User.UUID)
	if err != nil {
		t.Fatalf("member missing: %v", err)
	}
	if member.RoleName != model.DomainRoleMember || member.Nickname != "小明" {
		t.Fatalf("unexpected member: %+v", member)
	}
}

func TestInviteJoinService_RegisterAndJoin_PendingWhenApprovalRequired(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, true)

	res, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "waiter", Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Status != JoinStatusPending {
		t.Fatalf("status = %q, want pending", res.Status)
	}
	if _, err := repository.NewDomainRepository(db).GetMember(d.UUID, res.User.UUID); err != gorm.ErrRecordNotFound {
		t.Fatalf("pending user must not be a member: %v", err)
	}
	req, err := repository.NewDomainJoinRequestRepository(db).GetByDomainUser(d.UUID, res.User.UUID)
	if err != nil {
		t.Fatalf("join request missing: %v", err)
	}
	if req.Status != model.JoinRequestPending || req.InviteID != invite.ID {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestInviteJoinService_RegisterAndJoin_ExpiredCodeCreatesNoUser(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	_, invite := seedJoinInviteDomain(t, db, false)
	// 把码换成已过期的签名串（MaxUses/InviteID 保持）。
	key := DeriveInviteKey(testStorageKey, "jwt-key")
	expired, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf(invite.DomainUUID),
		InviteID:     uint32(invite.ID),
		ExpiresAt:    timeNowMinusHour(),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: expired, Username: "late", Password: "password123",
	})
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
	var count int64
	db.Model(&model.User{}).Where("name = ?", "late").Count(&count)
	if count != 0 {
		t.Fatalf("expired code must not create a user, found %d", count)
	}
}

func TestInviteJoinService_RegisterAndJoin_ExhaustedCode(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	_, invite := seedJoinInviteDomain(t, db, false)
	invite.MaxUses = 1
	invite.UsedCount = 1
	if err := db.Save(invite).Error; err != nil {
		t.Fatalf("exhaust invite: %v", err)
	}
	_, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "tooLate", Password: "password123",
	})
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
}

func TestInviteJoinService_RegisterAndJoin_RejectsDuplicateUsername(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	_, invite := seedJoinInviteDomain(t, db, false)
	if err := db.Create(&model.User{Name: "taken"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err := svc.RegisterAndJoin(&InviteRegisterRequest{
		Code: invite.Code, Username: "taken", Password: "password123",
	})
	if !pkg.IsCode(err, pkg.USERNAME_EXISTS) {
		t.Fatalf("err = %v, want USERNAME_EXISTS", err)
	}
}

func TestInviteJoinService_Join_ByCodeDirect(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, false)
	user := &model.User{Name: "existing"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	res, err := svc.Join(user.UUID, &InviteJoinRequest{Code: invite.Code})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if res.Status != JoinStatusJoined || res.Domain.UUID != d.UUID {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, err := repository.NewDomainRepository(db).GetMember(d.UUID, user.UUID); err != nil {
		t.Fatalf("member missing: %v", err)
	}
}

func TestInviteJoinService_Join_ByPublicDomainPending(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, _ := seedJoinInviteDomain(t, db, true)
	d.IsPublic = true
	if err := db.Save(d).Error; err != nil {
		t.Fatalf("make public: %v", err)
	}
	user := &model.User{Name: "publicSeeker"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	res, err := svc.Join(user.UUID, &InviteJoinRequest{DomainUUID: d.UUID})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if res.Status != JoinStatusPending {
		t.Fatalf("status = %q, want pending", res.Status)
	}
}

func TestInviteJoinService_Join_PrivateDomainRejected(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, _ := seedJoinInviteDomain(t, db, false)
	user := &model.User{Name: "sneaker"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err := svc.Join(user.UUID, &InviteJoinRequest{DomainUUID: d.UUID})
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
}

func TestInviteJoinService_Join_AlreadyMemberIsIdempotent(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, invite := seedJoinInviteDomain(t, db, false)
	user := &model.User{Name: "oldMember"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: d.UUID, UserUUID: user.UUID, RoleName: model.DomainRoleMember}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	res, err := svc.Join(user.UUID, &InviteJoinRequest{Code: invite.Code})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if res.Status != JoinStatusJoined {
		t.Fatalf("status = %q, want joined", res.Status)
	}
	if invite.UsedCount != 0 {
		t.Fatalf("already-member join must not consume the code, used=%d", invite.UsedCount)
	}
}

func TestInviteJoinService_MyRequestAndCancel(t *testing.T) {
	svc, _, db := setupInviteJoinService(t)
	d, _ := seedJoinInviteDomain(t, db, true)
	d.IsPublic = true
	if err := db.Save(d).Error; err != nil {
		t.Fatalf("make public: %v", err)
	}
	user := &model.User{Name: "canceller"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := svc.Join(user.UUID, &InviteJoinRequest{DomainUUID: d.UUID}); err != nil {
		t.Fatalf("join: %v", err)
	}
	view, err := svc.MyRequest(user.UUID, d.UUID)
	if err != nil {
		t.Fatalf("my request: %v", err)
	}
	if view.Status != model.JoinRequestPending || view.IsMember {
		t.Fatalf("unexpected view: %+v", view)
	}
	if err := svc.CancelRequest(user.UUID, d.UUID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	after, err := svc.MyRequest(user.UUID, d.UUID)
	if err != nil {
		t.Fatalf("my request after cancel: %v", err)
	}
	if after.Request != nil || after.Status != "" {
		t.Fatalf("cancel must clear the request: %+v", after)
	}
}
```

文件内还需要一个过期时间助手（放在测试文件末尾）：

```go
func timeNowMinusHour() time.Time {
	return time.Now().Add(-time.Hour).UTC()
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestInviteJoinService -v`
预期：FAIL，编译错误 `undefined: NewInviteJoinService`（及 `InviteRegisterRequest`、`JoinStatusJoined`、`timeNowMinusHour`）

---
- [ ] **步骤 3：编写实现**

创建 `app/server/internal/service/invite_join_service.go`：

```go
package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// 加入结果状态：joined 直接成为成员，pending 已提交待审申请。
const (
	JoinStatusJoined  = "joined"
	JoinStatusPending = "pending"
)

// maxJoinNicknameLen 域内昵称最大长度（按字符计）。
const maxJoinNicknameLen = 32

// InviteRegisterRequest 内联注册请求：注册 + 入域/提交申请。
type InviteRegisterRequest struct {
	Code      string `json:"code" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	EmailCode string `json:"email_code"`
}

// InviteJoinRequest 已登录用户加入域：code 与 domain_uuid 二选一。
type InviteJoinRequest struct {
	Code       string `json:"code"`
	DomainUUID string `json:"domain_uuid"`
}

// InviteJoinResponse 注册/加入的统一返回。
type InviteJoinResponse struct {
	Token        string        `json:"access_token,omitempty"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	ExpiresIn    int64         `json:"expires_in,omitempty"`
	User         *model.User   `json:"user,omitempty"`
	Domain       *model.Domain `json:"domain"`
	Status       string        `json:"status"`
}

// JoinRequestView 是 /invite/my-request 的返回：申请本体 + 是否已是成员。
type JoinRequestView struct {
	Request  *model.DomainJoinRequest `json:"request"`
	Status   string                   `json:"status"`
	IsMember bool                     `json:"is_member"`
}

// InviteJoinService 负责内联注册、已登录加入、申请查询与撤销。
// 不改动第一部分 InviteService 的构造函数，仅复用其校验能力。
type InviteJoinService struct {
	db          *gorm.DB
	inviteSvc   *InviteService
	joinReqSvc  *JoinRequestService
	inviteRepo  *repository.DomainInviteRepository
	domainRepo  *repository.DomainRepository
	userRepo    *repository.UserRepository
	emailConfig *EmailConfigService
	emailVerify *EmailVerificationService
}

func NewInviteJoinService(
	db *gorm.DB,
	inviteSvc *InviteService,
	joinReqSvc *JoinRequestService,
	inviteRepo *repository.DomainInviteRepository,
	domainRepo *repository.DomainRepository,
	userRepo *repository.UserRepository,
	emailConfig *EmailConfigService,
	emailVerify *EmailVerificationService,
) *InviteJoinService {
	return &InviteJoinService{
		db: db, inviteSvc: inviteSvc, joinReqSvc: joinReqSvc,
		inviteRepo: inviteRepo, domainRepo: domainRepo, userRepo: userRepo,
		emailConfig: emailConfig, emailVerify: emailVerify,
	}
}

// RegisterAndJoin 内联注册并按域策略入域或提交申请。
// 校验失败时（码无效/过期/耗尽、用户名重复、密码太短、邮箱验证失败）
// 不写任何 users 行：所有写操作都在最后一次事务里。
func (s *InviteJoinService) RegisterAndJoin(req *InviteRegisterRequest) (*InviteJoinResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "username is required")
	}
	if strings.HasPrefix(strings.ToLower(username), "guest_") {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "username cannot start with guest_")
	}
	if strings.HasPrefix(strings.ToLower(username), "bot_") {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS, "username prefix 'bot_' is reserved")
	}
	nickname := strings.TrimSpace(req.Nickname)
	if utf8.RuneCountInString(nickname) > maxJoinNicknameLen {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "nickname is too long")
	}

	validation, err := s.inviteSvc.Validate(req.Code)
	if err != nil {
		return nil, err
	}
	domain := validation.Domain

	existing, _ := s.userRepo.GetByName(username)
	if existing != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
	}
	if s.emailConfig != nil && s.emailConfig.IsVerificationAvailable() {
		if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.EmailCode) == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "email and email_code are required")
		}
		byEmail, err := s.userRepo.GetByEmail(req.Email)
		if err == nil && byEmail != nil {
			return nil, pkg.NewAppError(pkg.EMAIL_ALREADY_EXISTS)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if s.emailVerify == nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "email verification service is not initialized")
		}
		if err := s.emailVerify.VerifyRegisterEmail(req.Email, req.EmailCode); err != nil {
			return nil, err
		}
	}
	if err := validateNewPassword(req.Password); err != nil {
		return nil, err
	}
	hashed, err := authbossHasher.GenerateHash(req.Password)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	emailVerified := req.Email != "" && s.emailConfig != nil && s.emailConfig.IsVerificationAvailable()
	user := &model.User{
		Name:          username,
		Password:      hashed,
		Role:          "user",
		Email:         strings.TrimSpace(req.Email),
		EmailVerified: emailVerified,
	}

	status := JoinStatusJoined
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				if strings.Contains(strings.ToLower(err.Error()), "email") {
					return pkg.NewAppError(pkg.EMAIL_ALREADY_EXISTS)
				}
				return pkg.NewAppError(pkg.USERNAME_EXISTS)
			}
			return err
		}
		inviteRepo := repository.NewDomainInviteRepository(tx)
		ok, err := inviteRepo.ConsumeUse(validation.Invite.ID)
		if err != nil {
			return err
		}
		if !ok {
			reason := model.InviteFailExhausted
			if validation.Invite.IsDisabled() {
				reason = model.InviteFailDisabled
			}
			_ = inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
				InviteID: validation.Invite.ID, DomainUUID: domain.UUID, UserUUID: user.UUID,
				Code: validation.Invite.Code, Success: false, FailReason: reason,
			})
			return pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is no longer usable")
		}
		if err := inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
			InviteID: validation.Invite.ID, DomainUUID: domain.UUID, UserUUID: user.UUID,
			Code: validation.Invite.Code, Success: true,
		}); err != nil {
			return err
		}
		if domain.RequireJoinApproval {
			status = JoinStatusPending
			_, err := repository.NewDomainJoinRequestRepository(tx).Submit(domain.UUID, user.UUID, validation.Invite.ID)
			return err
		}
		return repository.NewDomainRepository(tx).AddMember(&model.DomainMember{
			DomainUUID: domain.UUID, UserUUID: user.UUID,
			Nickname: nickname, RoleName: model.DomainRoleMember,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}

	access, refresh, err := generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, err
	}
	return &InviteJoinResponse{
		Token: access, RefreshToken: refresh,
		ExpiresIn: pkg.AccessTokenExpiresIn(),
		User:      user, Domain: domain, Status: status,
	}, nil
}

// Join 已登录用户加入域。code 优先；只有 domain_uuid 时走公开域免码申请。
func (s *InviteJoinService) Join(userUUID string, req *InviteJoinRequest) (*InviteJoinResponse, error) {
	if userUUID == "" {
		return nil, pkg.NewAppError(pkg.TOKEN_NOT_EXIST, "user not authenticated")
	}
	code := strings.TrimSpace(req.Code)
	domainUUID := strings.TrimSpace(req.DomainUUID)
	if code == "" && domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "code or domain_uuid is required")
	}

	var domain *model.Domain
	inviteID := uint(0)
	if code != "" {
		validation, err := s.inviteSvc.Validate(code)
		if err != nil {
			return nil, err
		}
		domain = validation.Domain
		inviteID = validation.Invite.ID
	} else {
		d, err := s.domainRepo.GetByUUID(domainUUID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrDomainNotFound
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if !d.IsPublic {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "this domain requires an invite code")
		}
		domain = d
	}

	// 已是成员：幂等返回 joined，且不消耗邀请码额度。
	if _, err := s.domainRepo.GetMember(domain.UUID, userUUID); err == nil {
		return &InviteJoinResponse{Domain: domain, Status: JoinStatusJoined}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if domain.RequireJoinApproval {
		if _, err := s.joinReqSvc.Submit(domain.UUID, userUUID, inviteID); err != nil {
			return nil, err
		}
		return &InviteJoinResponse{Domain: domain, Status: JoinStatusPending}, nil
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		if inviteID != 0 {
			inviteRepo := repository.NewDomainInviteRepository(tx)
			ok, err := inviteRepo.ConsumeUse(inviteID)
			if err != nil {
				return err
			}
			if !ok {
				_ = inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
					InviteID: inviteID, DomainUUID: domain.UUID, UserUUID: userUUID,
					Code: code, Success: false, FailReason: model.InviteFailExhausted,
				})
				return pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is no longer usable")
			}
			if err := inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
				InviteID: inviteID, DomainUUID: domain.UUID, UserUUID: userUUID,
				Code: code, Success: true,
			}); err != nil {
				return err
			}
		}
		return repository.NewDomainRepository(tx).AddMember(&model.DomainMember{
			DomainUUID: domain.UUID, UserUUID: userUUID, RoleName: model.DomainRoleMember,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}
	return &InviteJoinResponse{Domain: domain, Status: JoinStatusJoined}, nil
}

// MyRequest 返回用户在某域的申请状态；无申请时 Status 为空串。
func (s *InviteJoinService) MyRequest(userUUID, domainUUID string) (*JoinRequestView, error) {
	if userUUID == "" || domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	view := &JoinRequestView{}
	if _, err := s.domainRepo.GetMember(domainUUID, userUUID); err == nil {
		view.IsMember = true
	}
	req, err := s.joinReqSvc.Find(domainUUID, userUUID)
	if err != nil {
		return nil, err
	}
	if req != nil {
		view.Request = req
		view.Status = req.Status
	}
	return view, nil
}

// CancelRequest 撤销自己的待审申请。
func (s *InviteJoinService) CancelRequest(userUUID, domainUUID string) error {
	if userUUID == "" || domainUUID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	return s.joinReqSvc.Cancel(domainUUID, userUUID)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestInviteJoinService -v`
预期：全部 PASS（10 个测试函数）

> 测试文件 import 块需含 `"testing"`、`"time"`、`"GOSpeak/internal/model"`、`"GOSpeak/internal/pkg"`、`"GOSpeak/internal/repository"`、`"github.com/glebarez/sqlite"`、`"gorm.io/gorm"`。

- [ ] **步骤 5：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/service/invite_join_service.go app/server/internal/service/invite_join_service_test.go
git commit -m "feat(invite): 内联注册与加入服务（同事务入域/待审）"
```

---

## 任务 5：注册与加入 Handler、preview 合并

**文件：**
- 创建：`app/server/internal/handler/invite_join_handler.go`
- 创建：`app/server/internal/handler/invite_join_handler_test.go`
- 修改：`app/server/internal/router/routes/invite/routes.go`（`RegisterPublic` 加限流、`RegisterAuth` 新增）

> **preview 合并结论（重要）：** 第一部分已建好 `POST /api/v1/invite/preview`，其响应为 `{domain:{uuid,name,icon_url,description}, requires_approval, remaining_uses}`，`requires_approval` 直接读 `result.Domain.RequireJoinApproval`。任务 1 已给 `model.Domain` 加上该字段，因此**不需要第二个 preview 端点，也不需要改响应结构**。本任务只做两件事：给该端点补 20/h/IP 限流，并让落地页按同一响应消费。

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/handler/invite_join_handler_test.go`：

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type inviteJoinHandlerEnv struct {
	router    *gin.Engine
	db        *gorm.DB
	inviteSvc *service.InviteService
}

func newInviteJoinHandlerEnv(t *testing.T) *inviteJoinHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainInvite{}, &model.DomainInviteRedemption{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	userRepo := repository.NewUserRepository(db)
	inviteRepo := repository.NewDomainInviteRepository(db)
	inviteSvc := service.NewInviteService(db, inviteRepo, domainRepo, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "jwt-key")
	joinReqSvc := service.NewJoinRequestService(db, repository.NewDomainJoinRequestRepository(db), domainRepo, userRepo)
	joinSvc := service.NewInviteJoinService(db, inviteSvc, joinReqSvc, inviteRepo, domainRepo, userRepo, nil, nil)
	h := NewInviteJoinHandler(joinSvc, defaultAuthCookieConfig())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if uuid := c.GetHeader("X-User-UUID"); uuid != "" {
			c.Set("user_uuid", uuid)
		}
		c.Next()
	})
	r.POST("/api/v1/invite/register", h.Register)
	r.POST("/api/v1/invite/join", h.Join)
	r.POST("/api/v1/invite/my-request", h.MyRequest)
	r.POST("/api/v1/invite/cancel-request", h.CancelRequest)
	return &inviteJoinHandlerEnv{router: r, db: db, inviteSvc: inviteSvc}
}

func (e *inviteJoinHandlerEnv) seedDomainInvite(t *testing.T, requireApproval bool) (*model.Domain, *model.DomainInvite) {
	t.Helper()
	d := &model.Domain{Name: "JoinRealm", OwnerUUID: "owner-1", RequireJoinApproval: requireApproval}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	invite, err := e.inviteSvc.Create(d.UUID, "测试", nil, 0, "owner-1")
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	return d, invite
}

func (e *inviteJoinHandlerEnv) post(t *testing.T, path, body string, headers ...map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeJoinResp(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	return resp
}

func TestInviteJoinHandler_Register_DirectJoinSetsCookie(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	_, invite := env.seedDomainInvite(t, false)

	rec := env.post(t, "/api/v1/invite/register",
		`{"code":"`+invite.Code+`","username":"handler1","password":"password123","nickname":"甲"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeJoinResp(t, rec)
	data, _ := resp["data"].(map[string]interface{})
	if data["status"] != "joined" {
		t.Fatalf("status = %v, want joined", data["status"])
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("register must set auth cookies")
	}
}

func TestInviteJoinHandler_Register_PendingWhenApprovalRequired(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	_, invite := env.seedDomainInvite(t, true)

	rec := env.post(t, "/api/v1/invite/register",
		`{"code":"`+invite.Code+`","username":"handler2","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeJoinResp(t, rec)
	data, _ := resp["data"].(map[string]interface{})
	if data["status"] != "pending" {
		t.Fatalf("status = %v, want pending", data["status"])
	}
}

func TestInviteJoinHandler_Register_MissingCodeReturns2001(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/register", `{"username":"x","password":"password123"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("code = %d, want 2001", code)
	}
}

func TestInviteJoinHandler_Register_InvalidCodeReturns2001(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/register",
		`{"code":"`+strings.Repeat("A", 36)+`","username":"handler3","password":"password123"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 2001 {
		t.Fatalf("code = %d, want 2001", code)
	}
	if strings.Contains(rec.Body.String(), "JoinRealm") {
		t.Fatal("invalid code must not leak domain info")
	}
}

func TestInviteJoinHandler_Join_RequiresAuth(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/join", `{"domain_uuid":"whatever"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code == 0 {
		t.Fatalf("join without user must fail, got code 0")
	}
}

func TestInviteJoinHandler_Join_ByCode(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	_, invite := env.seedDomainInvite(t, false)
	if err := env.db.Create(&model.User{Name: "joiner"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var user model.User
	if err := env.db.Where("name = ?", "joiner").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	rec := env.post(t, "/api/v1/invite/join", `{"code":"`+invite.Code+`"}`,
		map[string]string{"X-User-UUID": user.UUID})
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
}

func TestInviteJoinHandler_MyRequestAndCancel(t *testing.T) {
	env := newInviteJoinHandlerEnv(t)
	d, _ := env.seedDomainInvite(t, true)
	d.IsPublic = true
	if err := env.db.Save(d).Error; err != nil {
		t.Fatalf("make public: %v", err)
	}
	if err := env.db.Create(&model.User{Name: "waiter2"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var user model.User
	if err := env.db.Where("name = ?", "waiter2").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	headers := map[string]string{"X-User-UUID": user.UUID}

	rec := env.post(t, "/api/v1/invite/join", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("join code = %d; body=%s", code, rec.Body.String())
	}

	rec = env.post(t, "/api/v1/invite/my-request", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	resp := decodeJoinResp(t, rec)
	data, _ := resp["data"].(map[string]interface{})
	if data["status"] != "pending" {
		t.Fatalf("status = %v, want pending", data["status"])
	}

	rec = env.post(t, "/api/v1/invite/cancel-request", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("cancel code = %d; body=%s", code, rec.Body.String())
	}

	rec = env.post(t, "/api/v1/invite/my-request", `{"domain_uuid":"`+d.UUID+`"}`, headers)
	resp = decodeJoinResp(t, rec)
	data, _ = resp["data"].(map[string]interface{})
	if data["status"] != "" {
		t.Fatalf("status after cancel = %v, want empty", data["status"])
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/handler/ -run TestInviteJoinHandler -v`
预期：FAIL，编译错误 `undefined: NewInviteJoinHandler`

- [ ] **步骤 3：编写 handler**

创建 `app/server/internal/handler/invite_join_handler.go`：

```go
package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// InviteJoinHandler 内联注册、加入、申请查询与撤销。
type InviteJoinHandler struct {
	joinSvc *service.InviteJoinService
	cookie  *AuthCookieConfig
}

func NewInviteJoinHandler(joinSvc *service.InviteJoinService, cookie *AuthCookieConfig) *InviteJoinHandler {
	if cookie == nil {
		cookie = defaultAuthCookieConfig()
	}
	return &InviteJoinHandler{joinSvc: joinSvc, cookie: cookie}
}

// Register 内联注册：验码 → 建用户 → 按域策略入域或提交申请 → 签发 Cookie。
// @Summary      邀请码内联注册
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Param        request  body  service.InviteRegisterRequest  true  "邀请码与注册信息"
// @Success      200  {object}  pkg.Response
// @Router       /invite/register [post]
func (h *InviteJoinHandler) Register(c *gin.Context) {
	var req service.InviteRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.joinSvc.RegisterAndJoin(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	h.cookie.Set(c, resp.Token, resp.RefreshToken)
	pkg.Success(c, resp)
}

type inviteJoinRequest struct {
	Code       string `json:"code"`
	DomainUUID string `json:"domain_uuid"`
}

// Join 已登录用户加入域（邀请码或公开域免码）。
// @Summary      加入域
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  inviteJoinRequest  true  "邀请码或域名 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /invite/join [post]
func (h *InviteJoinHandler) Join(c *gin.Context) {
	var req inviteJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.joinSvc.Join(currentUserUUID(c), &service.InviteJoinRequest{
		Code: req.Code, DomainUUID: req.DomainUUID,
	})
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, resp)
}

type domainUUIDOnlyRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

// MyRequest 查询自己在该域的申请状态。
// @Summary      查询加入申请状态
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  domainUUIDOnlyRequest  true  "域名 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /invite/my-request [post]
func (h *InviteJoinHandler) MyRequest(c *gin.Context) {
	var req domainUUIDOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	view, err := h.joinSvc.MyRequest(currentUserUUID(c), req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, view)
}

// CancelRequest 撤销自己的待审申请。
// @Summary      撤销加入申请
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  domainUUIDOnlyRequest  true  "域名 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /invite/cancel-request [post]
func (h *InviteJoinHandler) CancelRequest(c *gin.Context) {
	var req domainUUIDOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.joinSvc.CancelRequest(currentUserUUID(c), req.DomainUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"ok": true})
}
```

- [ ] **步骤 4：给 preview 补限流**

修改 `app/server/internal/router/routes/invite/routes.go`：只改 `RegisterPublic` 一行，**不改函数签名**（`/invite/register` 与认证端点在任务 7 的 `invite_join` 包里注册，避免同一路径两处定义）：

```go
package invite

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册邀请码管理端点（protected 组，权限在 handler 内按域校验）。
func Register(r *gin.RouterGroup, h *handler.InviteHandler) {
	r.POST("/domain/invites/list", h.List)
	r.POST("/domain/invites/create", h.Create)
	r.POST("/domain/invites/disable", h.Disable)
	r.POST("/domain/invites/redemptions", h.Redemptions)
}

// RegisterPublic 注册公开预览端点（无需登录），按 IP 限流 20 次/小时。
func RegisterPublic(r *gin.RouterGroup, h *handler.InviteHandler) {
	r.POST("/preview", middleware.RateLimit(20, time.Hour), h.Preview)
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`cd app/server && go test ./internal/handler/ -run 'TestInviteJoinHandler' -v`
预期：全部 PASS（7 个测试函数）

> 本任务不动 `router.go`，`RegisterPublic` 签名不变，因此 `go build ./...` 也应通过。

- [ ] **步骤 6：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/handler/invite_join_handler.go app/server/internal/handler/invite_join_handler_test.go app/server/internal/router/routes/invite/routes.go
git commit -m "feat(invite): 内联注册与加入端点及限流"
```

---

## 任务 6：审核管理 Handler

**文件：**
- 创建：`app/server/internal/handler/join_request_handler.go`
- 创建：`app/server/internal/handler/join_request_handler_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/handler/join_request_handler_test.go`：

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// stubReviewPerm 是 JoinRequestHandler 依赖的域权限判决桩。
type stubReviewPerm struct{ granted bool }

func (s *stubReviewPerm) HasDomainPermission(domainUUID, userUUID, permCode string) bool {
	return s.granted
}

type joinRequestHandlerEnv struct {
	router *gin.Engine
	db     *gorm.DB
	svc    *service.JoinRequestService
}

func newJoinRequestHandlerEnv(t *testing.T, granted bool) *joinRequestHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Domain{}, &model.DomainMember{},
		&model.DomainRole{}, &model.DomainRolePermission{},
		&model.DomainJoinRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	svc := service.NewJoinRequestService(db, repository.NewDomainJoinRequestRepository(db), domainRepo, repository.NewUserRepository(db))
	h := NewJoinRequestHandler(svc, &stubReviewPerm{granted: granted})

	r := gin.New()
	// 伪造登录态：与生产 middleware.JWTAuth 注入的 key 保持一致。
	// domain_uuid 由 handler 从请求体自取，这里不预置（本 handler 也不读 context 里的 domain_uuid）。
	r.Use(func(c *gin.Context) {
		userUUID := c.GetHeader("X-User-UUID")
		if userUUID == "" {
			userUUID = "admin-1"
		}
		c.Set("user_uuid", userUUID)
		c.Set("username", userUUID)
		c.Next()
	})
	r.POST("/api/v1/domain/join-requests/list", h.List)
	r.POST("/api/v1/domain/join-requests/review", h.Review)
	r.POST("/api/v1/domain/join-approval/config", h.Config)
	return &joinRequestHandlerEnv{router: r, db: db, svc: svc}
}

func (e *joinRequestHandlerEnv) seedDomain(t *testing.T) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "ReviewRealm", OwnerUUID: "owner-1", RequireJoinApproval: true}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func (e *joinRequestHandlerEnv) seedApplicant(t *testing.T, name string) *model.User {
	t.Helper()
	u := &model.User{Name: name, DisplayName: name}
	if err := e.db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func (e *joinRequestHandlerEnv) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestJoinRequestHandler_List_ReturnsItemsAndCounts(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "applicant")
	if _, err := env.svc.Submit(d.UUID, u.UUID, 1); err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/list", `{"domain_uuid":"`+d.UUID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items  []model.DomainJoinRequestInfo `json:"items"`
			Total  int64                         `json:"total"`
			Counts map[string]int64              `json:"counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 0 || resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data.Counts[model.JoinRequestPending] != 1 {
		t.Fatalf("pending count = %d, want 1", resp.Data.Counts[model.JoinRequestPending])
	}
	if resp.Data.Items[0].Name != "applicant" {
		t.Fatalf("applicant name = %q", resp.Data.Items[0].Name)
	}
}

func TestJoinRequestHandler_List_ForbiddenWithoutPermission(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, false)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/join-requests/list", `{"domain_uuid":"`+d.UUID+`"}`)
	resp := decodeJoinResp(t, rec)
	if code := intCode(resp["code"]); code != 1013 {
		t.Fatalf("code = %d, want 1013", code)
	}
}

func TestJoinRequestHandler_List_FiltersByStatus(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	a := env.seedApplicant(t, "a")
	b := env.seedApplicant(t, "b")
	if _, err := env.svc.Submit(d.UUID, a.UUID, 1); err != nil {
		t.Fatalf("submit a: %v", err)
	}
	reqB, err := env.svc.Submit(d.UUID, b.UUID, 1)
	if err != nil {
		t.Fatalf("submit b: %v", err)
	}
	if _, err := env.svc.Review(d.UUID, reqB.ID, true, "admin-1"); err != nil {
		t.Fatalf("approve b: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/list",
		`{"domain_uuid":"`+d.UUID+`","status":"approved"}`)
	var resp struct {
		Data struct {
			Items []model.DomainJoinRequestInfo `json:"items"`
			Total int64                         `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Items[0].UserUUID != b.UUID {
		t.Fatalf("unexpected filtered response: %+v", resp.Data)
	}
}

func TestJoinRequestHandler_Review_ApproveAddsMember(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "approveMe")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/review",
		`{"domain_uuid":"`+d.UUID+`","request_id":`+itoa(req.ID)+`,"approve":true}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
	if _, err := repository.NewDomainRepository(env.db).GetMember(d.UUID, u.UUID); err != nil {
		t.Fatalf("approved user must be a member: %v", err)
	}
}

func TestJoinRequestHandler_Review_RejectDoesNotAddMember(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "rejectMe")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/review",
		`{"domain_uuid":"`+d.UUID+`","request_id":`+itoa(req.ID)+`,"approve":false}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
	if _, err := repository.NewDomainRepository(env.db).GetMember(d.UUID, u.UUID); err != gorm.ErrRecordNotFound {
		t.Fatalf("rejected user must not be a member: %v", err)
	}
}

func TestJoinRequestHandler_Review_TwiceReturnsError(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "doubleReview")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	body := `{"domain_uuid":"` + d.UUID + `","request_id":` + itoa(req.ID) + `,"approve":true}`
	env.post(t, "/api/v1/domain/join-requests/review", body)
	rec := env.post(t, "/api/v1/domain/join-requests/review", body)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code == 0 {
		t.Fatalf("second review must fail, got code 0")
	}
}

func TestJoinRequestHandler_Review_ForbiddenWithoutPermission(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, false)
	d := env.seedDomain(t)
	u := env.seedApplicant(t, "noPerm")
	req, err := env.svc.Submit(d.UUID, u.UUID, 1)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/join-requests/review",
		`{"domain_uuid":"`+d.UUID+`","request_id":`+itoa(req.ID)+`,"approve":true}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 1013 {
		t.Fatalf("code = %d, want 1013", code)
	}
}

func TestJoinRequestHandler_Config_TogglesApproval(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, true)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/join-approval/config",
		`{"domain_uuid":"`+d.UUID+`","require_join_approval":false}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 0 {
		t.Fatalf("code = %d; body=%s", code, rec.Body.String())
	}
	var got model.Domain
	if err := env.db.Where("uuid = ?", d.UUID).First(&got).Error; err != nil {
		t.Fatalf("reload domain: %v", err)
	}
	if got.RequireJoinApproval {
		t.Fatal("require_join_approval must be false after toggle")
	}
}

func TestJoinRequestHandler_Config_RequiresManagePermission(t *testing.T) {
	env := newJoinRequestHandlerEnv(t, false)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/join-approval/config",
		`{"domain_uuid":"`+d.UUID+`","require_join_approval":true}`)
	if code := intCode(decodeJoinResp(t, rec)["code"]); code != 1013 {
		t.Fatalf("code = %d, want 1013", code)
	}
}

// itoa 是测试内的 uint -> 十进制串助手，避免再引 strconv。
func itoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/handler/ -run TestJoinRequestHandler -v`
预期：FAIL，编译错误 `undefined: NewJoinRequestHandler`

- [ ] **步骤 3：编写 handler**

创建 `app/server/internal/handler/join_request_handler.go`：

```go
package handler

import (
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// JoinRequestHandler 域加入申请的管理端端点。
type JoinRequestHandler struct {
	svc       *service.JoinRequestService
	domainSvc domainPermissionChecker
	permSvc   globalPermissionChecker
}

func NewJoinRequestHandler(svc *service.JoinRequestService, domainSvc domainPermissionChecker) *JoinRequestHandler {
	return &JoinRequestHandler{svc: svc, domainSvc: domainSvc}
}

// SetGlobalPermissionChecker 注入平台权限判决依赖（平台管理员跨域审核时使用）。
func (h *JoinRequestHandler) SetGlobalPermissionChecker(permSvc globalPermissionChecker) {
	h.permSvc = permSvc
}

func (h *JoinRequestHandler) canReview(c *gin.Context, domainUUID string) bool {
	return domainPermissionGranted(c, domainUUID, permcode.PermDomainMemberReview, h.domainSvc, h.permSvc)
}

func (h *JoinRequestHandler) canManage(c *gin.Context, domainUUID string) bool {
	return domainPermissionGranted(c, domainUUID, permcode.PermDomainManage, h.domainSvc, h.permSvc)
}

type listJoinRequestsRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	Status     string `json:"status"`
	Keyword    string `json:"keyword"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// List 分页查询域内加入申请，附带各状态计数。
// @Summary      加入申请列表
// @Tags         域加入审核
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  listJoinRequestsRequest  true  "查询条件"
// @Success      200  {object}  pkg.Response
// @Router       /domain/join-requests/list [post]
func (h *JoinRequestHandler) List(c *gin.Context) {
	var req listJoinRequestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canReview(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain member review permission")
		return
	}
	items, total, err := h.svc.List(req.DomainUUID, req.Status, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	counts, err := h.svc.Counts(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total, "counts": counts})
}

type reviewJoinRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	RequestID  uint   `json:"request_id" binding:"required"`
	Approve    bool   `json:"approve"`
}

// Review 通过或拒绝一条加入申请。
// @Summary      审核加入申请
// @Tags         域加入审核
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  reviewJoinRequest  true  "审核决定"
// @Success      200  {object}  pkg.Response
// @Router       /domain/join-requests/review [post]
func (h *JoinRequestHandler) Review(c *gin.Context) {
	var req reviewJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canReview(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain member review permission")
		return
	}
	updated, err := h.svc.Review(req.DomainUUID, req.RequestID, req.Approve, currentUserUUID(c))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, updated)
}

type joinApprovalConfigRequest struct {
	DomainUUID          string `json:"domain_uuid" binding:"required"`
	RequireJoinApproval bool   `json:"require_join_approval"`
}

// Config 读写域级「加入需审核」开关。
// @Summary      加入审核开关
// @Tags         域加入审核
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  joinApprovalConfigRequest  true  "开关取值"
// @Success      200  {object}  pkg.Response
// @Router       /domain/join-approval/config [post]
func (h *JoinRequestHandler) Config(c *gin.Context) {
	var req joinApprovalConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "not domain owner or missing permission")
		return
	}
	domain, err := h.svc.SetRequireApproval(req.DomainUUID, req.RequireJoinApproval)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{
		"uuid":                  domain.UUID,
		"require_join_approval": domain.RequireJoinApproval,
	})
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/handler/ -run TestJoinRequestHandler -v`
预期：全部 PASS（10 个测试函数）

- [ ] **步骤 5：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/handler/join_request_handler.go app/server/internal/handler/join_request_handler_test.go
git commit -m "feat(domain): 加入申请审核端点（列表/审核/开关）"
```

---

## 任务 7：路由与依赖装配

**文件：**
- 创建：`app/server/internal/router/routes/invite_join/routes.go`
- 修改：`app/server/internal/router/router.go:41-66`、`:96-140`
- 修改：`app/server/server/gin.go:131-135`、`:364-370`、`:465-480`
- 修改：`app/server/internal/service/domain_service.go`（`SetInviteCreator` 之后加 `OnMemberAdded`）

> 本任务不改 `invite/routes.go`（任务 5 已改）。新路由统一放进独立的 `invite_join` 包，避免与第一部分的邀请码管理路由混在一起。

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/router/routes/invite_join/routes_test.go`（断言三条新路径已注册，且限流中间件挂在 register 上）：

```go
package invite_join

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRoutes_Registered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	pub := r.Group("/api/v1/invite")
	RegisterPublic(pub, nil)
	prot := r.Group("/api/v1/invite")
	RegisterProtected(prot, nil)
	admin := r.Group("/api/v1/domain")
	RegisterDomainAdmin(admin, nil)

	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/invite/register"},
		{http.MethodPost, "/api/v1/invite/join"},
		{http.MethodPost, "/api/v1/invite/my-request"},
		{http.MethodPost, "/api/v1/invite/cancel-request"},
		{http.MethodPost, "/api/v1/domain/join-requests/list"},
		{http.MethodPost, "/api/v1/domain/join-requests/review"},
		{http.MethodPost, "/api/v1/domain/join-approval/config"},
	}
	registered := map[string]bool{}
	for _, route := range r.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, want := range cases {
		if !registered[want.method+" "+want.path] {
			t.Fatalf("route %s %s is not registered", want.method, want.path)
		}
	}
}

func TestRoutes_RegisterIsRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterPublic(r.Group("/api/v1/invite"), nil)
	// 20 次以内放行；第 21 次命中 1017。handler 为 nil 会在放行后 panic，
	// 因此只断言「限流生效时返回 1017 而不是 500」。
	for i := 0; i < 21; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/invite/register", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.1.2.3:1234"
		func() {
			defer func() { _ = recover() }()
			r.ServeHTTP(rec, req)
		}()
		if i == 20 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("21st request status = %d, want 429", rec.Code)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/router/routes/invite_join/ -v`
预期：FAIL，编译错误 `undefined: RegisterPublic`（包内还没有 routes.go）

- [ ] **步骤 3：新建路由文件**

创建 `app/server/internal/router/routes/invite_join/routes.go`：

```go
package invite_join

import (
	"time"

	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterPublic 注册公开的内联注册端点，按 IP 限流 20 次/小时。
func RegisterPublic(r *gin.RouterGroup, h *handler.InviteJoinHandler) {
	r.POST("/register", middleware.RateLimit(20, time.Hour), h.Register)
}

// RegisterProtected 注册已登录用户的加入与申请端点。
func RegisterProtected(r *gin.RouterGroup, h *handler.InviteJoinHandler) {
	r.POST("/join", h.Join)
	r.POST("/my-request", h.MyRequest)
	r.POST("/cancel-request", h.CancelRequest)
}

// RegisterDomainAdmin 注册域管理端的加入审核端点（protected 组，权限在 handler 内按域校验）。
func RegisterDomainAdmin(r *gin.RouterGroup, h *handler.JoinRequestHandler) {
	r.POST("/join-requests/list", h.List)
	r.POST("/join-requests/review", h.Review)
	r.POST("/join-approval/config", h.Config)
}
```

- [ ] **步骤 4：接线到 router.go**

修改 `app/server/internal/router/router.go`：

1. import 块追加（与 `inviteRoutes` 相邻）：

```go
	inviteRoutes "GOSpeak/internal/router/routes/invite"
	inviteJoinRoutes "GOSpeak/internal/router/routes/invite_join"
```

2. `Handlers` 结构体在 `Invite` 之后追加两个字段：

```go
	Invite       *handler.InviteHandler
	InviteJoin   *handler.InviteJoinHandler
	JoinRequest  *handler.JoinRequestHandler
```

3. 公开路由区，在现有 `inviteRoutes.RegisterPublic(...)` 调用**之后**追加（保持原有 preview 注册不变）：

```go
	if h.Invite != nil {
		inviteRoutes.RegisterPublic(api.Group("/invite"), h.Invite)
	}
	if h.InviteJoin != nil {
		inviteJoinRoutes.RegisterPublic(api.Group("/invite"), h.InviteJoin)
	}
```

4. protected 路由区，在 `inviteRoutes.Register(protected.Group(""), h.Invite)` 之后追加：

```go
	if h.InviteJoin != nil {
		inviteJoinRoutes.RegisterProtected(protected.Group("/invite"), h.InviteJoin)
	}
	if h.JoinRequest != nil {
		inviteJoinRoutes.RegisterDomainAdmin(protected.Group("/domain"), h.JoinRequest)
	}
```

- [ ] **步骤 5：装配到 gin.go**

在 `inviteSvc := service.NewInviteService(...)` 之后追加：

```go
	joinRequestSvc := service.NewJoinRequestService(
		db,
		repository.NewDomainJoinRequestRepository(db),
		domainRepo,
		userRepo,
	)
	joinRequestSvc.SetMemberAddedHook(domainSvc.OnMemberAdded)
	inviteJoinSvc := service.NewInviteJoinService(
		db, inviteSvc, joinRequestSvc,
		repository.NewDomainInviteRepository(db), domainRepo, userRepo,
		emailConfigSvc, emailVerificationSvc,
	)
```

在 `inviteH := handler.NewInviteHandler(inviteSvc)` 之后追加：

```go
	inviteJoinH := handler.NewInviteJoinHandler(inviteJoinSvc, authCookieCfg)
	joinRequestH := handler.NewJoinRequestHandler(joinRequestSvc, domainSvc)
	joinRequestH.SetGlobalPermissionChecker(permSvc)
```

在 `Handlers{...}` 字面量中 `Invite: inviteH,` 那一行之后追加两行（不要重复写 `Invite` 行）：

```go
		Invite:       inviteH,
		InviteJoin:   inviteJoinH,
		JoinRequest:  joinRequestH,
```

- [ ] **步骤 6：给 DomainService 补成员变更回调**

在 `app/server/internal/service/domain_service.go` 追加（`SetInviteCreator` 之后）：

```go
// OnMemberAdded 清理成员缓存并广播失效；供加入审核通过后复用。
func (s *DomainService) OnMemberAdded(domainUUID, userUUID string) {
	s.invalidateMembership(domainUUID, userUUID, true)
}
```

- [ ] **步骤 7：编译与测试**

运行：

```bash
cd app/server && go build ./... && go vet ./... && go test ./internal/handler/ ./internal/service/ ./internal/repository/ -count=1
```
预期：`go build` / `go vet` 无输出；三个包测试全 PASS

- [ ] **步骤 8：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/router/routes/invite_join/routes.go app/server/internal/router/routes/invite_join/routes_test.go app/server/internal/router/router.go app/server/server/gin.go app/server/internal/service/domain_service.go
git commit -m "feat(invite): 加入审核路由与依赖装配"
```

---

## 任务 8：后端集成测试

**文件：**
- 创建：`test/invite/invite.test.ts`（助手函数写在用例文件内，不改 `test/helpers/api.ts`，避免影响既有用例）

- [ ] **步骤 1：编写失败的测试**

创建 `test/invite/invite.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import { api, assertSuccess, createDomain, registerUser, unique } from "../helpers";

interface DomainInviteDTO {
  id: number;
  domain_uuid: string;
  code: string;
  max_uses: number;
  used_count: number;
  disabled_at?: string | null;
}

async function createInvite(
  token: string,
  domainUUID: string,
  body: Record<string, unknown> = {},
): Promise<DomainInviteDTO> {
  const res = await api<DomainInviteDTO>("/api/v1/domain/invites/create", {
    token,
    body: { domain_uuid: domainUUID, label: unique("invite"), ...body },
  });
  return assertSuccess(res);
}

async function listInvites(token: string, domainUUID: string): Promise<DomainInviteDTO[]> {
  const res = await api<{ invites: DomainInviteDTO[] }>("/api/v1/domain/invites/list", {
    token,
    body: { domain_uuid: domainUUID },
  });
  return assertSuccess(res).invites;
}

async function setRequireApproval(token: string, domainUUID: string, require: boolean): Promise<void> {
  const res = await api("/api/v1/domain/join-approval/config", {
    token,
    body: { domain_uuid: domainUUID, require_join_approval: require },
  });
  expect(res.code).toBe(0);
}

describe("invite registration and approval", () => {
  it("creates a domain with a default invite code", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invites = await listInvites(owner.access_token, domain.uuid);
    expect(invites.length).toBeGreaterThanOrEqual(1);
    const code = invites[0].code;
    expect(code.length).toBe(36);
    expect(/^[A-HJ-NP-Z2-9]{36}$/.test(code)).toBe(true);
  });

  it("previews an invite without leaking the code", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);

    const preview = await api<{
      domain: { uuid: string; name: string };
      requires_approval: boolean;
      remaining_uses: number;
    }>("/api/v1/invite/preview", { body: { code: invite.code } });
    expect(preview.code).toBe(0);
    expect(preview.data?.domain.uuid).toBe(domain.uuid);
    expect(preview.data?.requires_approval).toBe(false);
    expect(preview.raw.includes(invite.code)).toBe(false);
  });

  it("registers and joins directly when approval is off", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);

    const username = unique("newbie");
    const res = await api<{ status: string; user: { uuid: string }; domain: { uuid: string } }>(
      "/api/v1/invite/register",
      { body: { code: invite.code, username, password: "password123", nickname: "新人" } },
    );
    const data = assertSuccess(res);
    expect(data.status).toBe("joined");
    expect(data.domain.uuid).toBe(domain.uuid);

    const members = await api<{ members: Array<{ user_uuid: string; role_name: string }> }>(
      "/api/v1/domain/members",
      { token: owner.access_token, body: { domain_uuid: domain.uuid } },
    );
    const joined = members.data?.members.find((m) => m.user_uuid === data.user.uuid);
    expect(joined?.role_name).toBe("member");

    // 免审核域不得留下任何申请记录（§8.2）
    const pendingList = await api<{ items: Array<{ user_uuid: string }>; total: number }>(
      "/api/v1/domain/join-requests/list",
      { token: owner.access_token, body: { domain_uuid: domain.uuid } },
    );
    expect(pendingList.data?.total).toBe(0);
  });

  it("registers into pending when approval is on", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);
    await setRequireApproval(owner.access_token, domain.uuid, true);

    const username = unique("waiter");
    const res = await api<{ status: string; user: { uuid: string }; access_token: string }>(
      "/api/v1/invite/register",
      { body: { code: invite.code, username, password: "password123" } },
    );
    const data = assertSuccess(res);
    expect(data.status).toBe("pending");

    const mine = await api<{ status: string; is_member: boolean }>("/api/v1/invite/my-request", {
      token: data.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(mine.data?.status).toBe("pending");
    expect(mine.data?.is_member).toBe(false);
  });

  it("approves a pending request and the user becomes a member", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid);
    await setRequireApproval(owner.access_token, domain.uuid, true);

    const username = unique("approvee");
    const reg = await api<{ user: { uuid: string }; access_token: string }>(
      "/api/v1/invite/register",
      { body: { code: invite.code, username, password: "password123" } },
    );
    const registered = assertSuccess(reg);

    const list = await api<{ items: Array<{ id: number; user_uuid: string }> }>(
      "/api/v1/domain/join-requests/list",
      { token: owner.access_token, body: { domain_uuid: domain.uuid, status: "pending" } },
    );
    const request = list.data?.items.find((item) => item.user_uuid === registered.user.uuid);
    expect(request).toBeTruthy();

    const review = await api("/api/v1/domain/join-requests/review", {
      token: owner.access_token,
      body: { domain_uuid: domain.uuid, request_id: request!.id, approve: true },
    });
    expect(review.code).toBe(0);

    const mine = await api<{ status: string; is_member: boolean }>("/api/v1/invite/my-request", {
      token: registered.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(mine.data?.is_member).toBe(true);
    expect(mine.data?.status).toBe("approved");
  });

  // 真过期需要等到 expires_at 过去，集成测试无法在合理时间内构造；
  // 过期分支由 service/invite_join_service_test.go 的
  // TestInviteJoinService_RegisterAndJoin_ExpiredCodeCreatesNoUser 覆盖。
  // 这里覆盖「无效码不建用户、不泄露域信息」。
  it("rejects an invalid invite code without creating a user", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));

    const res = await api("/api/v1/invite/register", {
      body: { code: "A".repeat(36), username: unique("bad"), password: "password123" },
    });
    expect(res.code).toBe(2001);
    expect(res.raw.includes(domain.name)).toBe(false);
  });

  it("enforces the usage limit", async () => {
    const owner = await registerUser("inv_owner");
    const domain = await createDomain(owner.access_token, unique("inv_domain"));
    const invite = await createInvite(owner.access_token, domain.uuid, { max_uses: 1 });

    const first = await api("/api/v1/invite/register", {
      body: { code: invite.code, username: unique("first"), password: "password123" },
    });
    expect(first.code).toBe(0);

    const second = await api("/api/v1/invite/register", {
      body: { code: invite.code, username: unique("second"), password: "password123" },
    });
    expect(second.code).toBe(2001);
  });

  it("keeps domains isolated", async () => {
    const ownerA = await registerUser("inv_a");
    const ownerB = await registerUser("inv_b");
    const domainA = await createDomain(ownerA.access_token, unique("inv_a"));
    const domainB = await createDomain(ownerB.access_token, unique("inv_b"));
    const inviteA = await createInvite(ownerA.access_token, domainA.uuid);

    const res = await api<{ domain: { uuid: string } }>("/api/v1/invite/register", {
      body: { code: inviteA.code, username: unique("cross"), password: "password123" },
    });
    const joined = assertSuccess(res);
    expect(joined.domain.uuid).toBe(domainA.uuid);
    expect(joined.domain.uuid).not.toBe(domainB.uuid);
  });

  it("lets a logged-in user join a public domain without a code", async () => {
    const owner = await registerUser("inv_pub");
    const created = await api<{ uuid: string }>("/api/v1/domain/create", {
      token: owner.access_token,
      body: { name: unique("pub_domain"), is_public: true },
    });
    const domain = assertSuccess(created);
    const joiner = await registerUser("inv_joiner");

    const res = await api<{ status: string; domain: { uuid: string } }>("/api/v1/invite/join", {
      token: joiner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    const data = assertSuccess(res);
    expect(data.status).toBe("joined");
    expect(data.domain.uuid).toBe(domain.uuid);
  });

  it("puts public-domain joins into pending when approval is on", async () => {
    const owner = await registerUser("inv_pub2");
    const created = await api<{ uuid: string }>("/api/v1/domain/create", {
      token: owner.access_token,
      body: { name: unique("pub_domain"), is_public: true },
    });
    const domain = assertSuccess(created);
    await setRequireApproval(owner.access_token, domain.uuid, true);
    const joiner = await registerUser("inv_joiner2");

    const res = await api<{ status: string }>("/api/v1/invite/join", {
      token: joiner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(assertSuccess(res).status).toBe("pending");
  });

  it("refuses code-less joins to private domains", async () => {
    const owner = await registerUser("inv_priv");
    const domain = await createDomain(owner.access_token, unique("priv_domain"));
    const joiner = await registerUser("inv_joiner3");

    const res = await api("/api/v1/invite/join", {
      token: joiner.access_token,
      body: { domain_uuid: domain.uuid },
    });
    expect(res.code).toBe(2001);
  });

  it("requires JWT for list-public (discover is not anonymous)", async () => {
    const res = await api("/api/v1/domain/list-public", { body: {} });
    expect(res.code).toBe(1001);
  });
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm test:server -- test/invite/`
预期：FAIL，`/api/v1/domain/invites/create` 返回 404 或 `code !== 0`（装配未完成时）——若任务 7 已完成，则本条应为 PASS；此时改为先跑 `pnpm test:server -- test/invite/` 确认新用例被收集（vitest 输出包含 `test/invite/invite.test.ts`）即为通过本步骤。

- [ ] **步骤 3：确认无回归**

运行：`pnpm test:server`
预期：全部通过（含既有 `test/domain`、`test/room`、`test/message`）

- [ ] **步骤 4：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add test/invite/invite.test.ts
git commit -m "test(invite): 内联注册与加入审核集成测试"
```

---

## 任务 9：前端 API 层 `inviteJoin.ts`

**文件：**
- 创建：`app/web/src/api/inviteJoin.ts`
- 测试：`app/web/src/api/inviteJoin.spec.ts`

> 与第一部分的 `api/invite.ts` 分工：`invite.ts` 只放邀请码管理（list/create/disable/redemptions）；`inviteJoin.ts` 放落地页与审核链路。两者不互相 import。

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/api/inviteJoin.spec.ts`：

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";

const post = vi.fn();
vi.mock("./apiClient", () => ({
	default: { post: (...args: unknown[]) => post(...args) },
}));

import {
	cancelJoinRequest,
	getJoinRequest,
	joinByCode,
	joinByDomain,
	previewInvite,
	registerViaInvite,
	setJoinApproval,
} from "./inviteJoin";

describe("inviteJoin api", () => {
	beforeEach(() => {
		post.mockReset();
	});

	it("previewInvite posts code to /api/v1/invite/preview without toast", async () => {
		post.mockResolvedValue({
			domain: { uuid: "d1", name: "Realm", icon_url: "", description: "" },
			requires_approval: true,
			remaining_uses: 3,
		});
		const got = await previewInvite("CODE");
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/invite/preview",
			data: { code: "CODE" },
			skipErrorToast: true,
		});
		expect(got.requires_approval).toBe(true);
	});

	it("registerViaInvite posts the inline registration payload", async () => {
		post.mockResolvedValue({ status: "joined", domain: { uuid: "d1" } });
		await registerViaInvite({
			code: "CODE",
			username: "u1",
			password: "password123",
			nickname: "小明",
		});
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/invite/register",
			data: {
				code: "CODE",
				username: "u1",
				password: "password123",
				nickname: "小明",
				email: "",
				email_code: "",
			},
			skipErrorToast: true,
		});
	});

	it("joinByCode and joinByDomain hit the same endpoint with different bodies", async () => {
		post.mockResolvedValue({ status: "joined", domain: { uuid: "d1" } });
		await joinByCode("CODE");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/join",
			data: { code: "CODE" },
			skipErrorToast: true,
		});
		await joinByDomain("d1");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/join",
			data: { domain_uuid: "d1" },
			skipErrorToast: true,
		});
	});

	it("getJoinRequest and cancelJoinRequest use domain_uuid", async () => {
		post.mockResolvedValue({ request: null, status: "", is_member: false });
		await getJoinRequest("d1");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/my-request",
			data: { domain_uuid: "d1" },
			skipErrorToast: true,
		});
		post.mockResolvedValue({ ok: true });
		await cancelJoinRequest("d1");
		expect(post).toHaveBeenLastCalledWith({
			url: "/api/v1/invite/cancel-request",
			data: { domain_uuid: "d1" },
		});
	});

	it("setJoinApproval posts to the domain config endpoint", async () => {
		post.mockResolvedValue({ uuid: "d1", require_join_approval: true });
		const got = await setJoinApproval("d1", true);
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/domain/join-approval/config",
			data: { domain_uuid: "d1", require_join_approval: true },
		});
		expect(got.require_join_approval).toBe(true);
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/web && pnpm vitest run src/api/inviteJoin.spec.ts`
预期：FAIL，`Failed to resolve import "./inviteJoin"`

- [ ] **步骤 3：编写实现**

创建 `app/web/src/api/inviteJoin.ts`：

```ts
import apiClient from "./apiClient";

/** 落地页 preview 返回的域摘要（不含邀请码本身）。 */
export interface InvitePreviewDomain {
	uuid: string;
	name: string;
	icon_url: string;
	description: string;
}

export interface InvitePreview {
	domain: InvitePreviewDomain;
	requires_approval: boolean;
	remaining_uses: number;
}

export type InviteJoinStatus = "joined" | "pending";

export interface InviteRegisterInput {
	code: string;
	username: string;
	password: string;
	nickname?: string;
	email?: string;
	email_code?: string;
}

export interface InviteJoinResult {
	access_token?: string;
	refresh_token?: string;
	expires_in?: number;
	user?: {
		id: number;
		uuid: string;
		name: string;
		display_name: string;
		avatar: string;
		role: string;
	};
	domain: InvitePreviewDomain & { is_public?: boolean };
	status: InviteJoinStatus;
}

export interface JoinRequestDTO {
	id: number;
	domain_uuid: string;
	user_uuid: string;
	invite_id: number;
	status: "pending" | "approved" | "rejected";
	reviewed_by: string;
	reviewed_at?: string | null;
	created_at: string;
	updated_at: string;
}

export interface JoinRequestView {
	request: JoinRequestDTO | null;
	status: "" | "pending" | "approved" | "rejected";
	is_member: boolean;
}

export interface JoinRequestListItem extends JoinRequestDTO {
	name: string;
	display_name: string;
}

export interface JoinRequestListResult {
	items: JoinRequestListItem[];
	total: number;
	counts: Record<string, number>;
}

/** 验码并取域信息。邀请码无效是高频可预期错误：页面内就地展示中文提示。 */
export async function previewInvite(code: string): Promise<InvitePreview> {
	const data = await apiClient.post<InvitePreview>({
		url: "/api/v1/invite/preview",
		data: { code },
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite preview is missing");
	return data;
}

/** 内联注册：注册成功即入域或提交申请，并下发 Cookie。 */
export async function registerViaInvite(
	input: InviteRegisterInput,
): Promise<InviteJoinResult> {
	const data = await apiClient.post<InviteJoinResult>({
		url: "/api/v1/invite/register",
		data: {
			code: input.code,
			username: input.username,
			password: input.password,
			nickname: input.nickname ?? "",
			email: input.email ?? "",
			email_code: input.email_code ?? "",
		},
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite register response is missing");
	return data;
}

/** 已登录用户凭邀请码加入。 */
export async function joinByCode(code: string): Promise<InviteJoinResult> {
	const data = await apiClient.post<InviteJoinResult>({
		url: "/api/v1/invite/join",
		data: { code },
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite join response is missing");
	return data;
}

/** 已登录用户免码申请加入公开域（仍受域级审核约束）。 */
export async function joinByDomain(domainUUID: string): Promise<InviteJoinResult> {
	const data = await apiClient.post<InviteJoinResult>({
		url: "/api/v1/invite/join",
		data: { domain_uuid: domainUUID },
		skipErrorToast: true,
	});
	if (!data) throw new Error("invite join response is missing");
	return data;
}

/** 查询自己在该域的申请状态。 */
export async function getJoinRequest(domainUUID: string): Promise<JoinRequestView> {
	const data = await apiClient.post<JoinRequestView>({
		url: "/api/v1/invite/my-request",
		data: { domain_uuid: domainUUID },
		skipErrorToast: true,
	});
	if (!data) throw new Error("join request response is missing");
	return data;
}

/** 撤销自己的待审申请。 */
export async function cancelJoinRequest(domainUUID: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/invite/cancel-request",
		data: { domain_uuid: domainUUID },
	});
}

export interface ListJoinRequestsInput {
	domain_uuid: string;
	status?: "" | "pending" | "approved" | "rejected";
	keyword?: string;
	page?: number;
	page_size?: number;
}

/** 管理端：分页查询域内加入申请。 */
export async function listJoinRequests(
	input: ListJoinRequestsInput,
): Promise<JoinRequestListResult> {
	const data = await apiClient.post<JoinRequestListResult>({
		url: "/api/v1/domain/join-requests/list",
		data: input,
	});
	if (!data) throw new Error("join request list is missing");
	return data;
}

/** 管理端：通过或拒绝一条申请。 */
export async function reviewJoinRequest(
	domainUUID: string,
	requestID: number,
	approve: boolean,
): Promise<JoinRequestDTO> {
	const data = await apiClient.post<JoinRequestDTO>({
		url: "/api/v1/domain/join-requests/review",
		data: { domain_uuid: domainUUID, request_id: requestID, approve },
	});
	if (!data) throw new Error("join request review response is missing");
	return data;
}

/** 管理端：读写域级「加入需审核」开关。 */
export async function setJoinApproval(
	domainUUID: string,
	require: boolean,
): Promise<{ uuid: string; require_join_approval: boolean }> {
	const data = await apiClient.post<{ uuid: string; require_join_approval: boolean }>({
		url: "/api/v1/domain/join-approval/config",
		data: { domain_uuid: domainUUID, require_join_approval: require },
	});
	if (!data) throw new Error("join approval config response is missing");
	return data;
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/web && pnpm vitest run src/api/inviteJoin.spec.ts`
预期：5 个用例 PASS

- [ ] **步骤 5：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/web/src/api/inviteJoin.ts app/web/src/api/inviteJoin.spec.ts
git commit -m "feat(web): 邀请注册与加入审核 API 客户端"
```

---

## 任务 10：前端内联注册表单与状态卡片

**文件：**
- 创建：`app/web/src/components/invite/InviteRegisterForm.tsx`
- 创建：`app/web/src/components/invite/JoinRequestStatusCard.tsx`

> 这两个是纯展示组件，不含数据请求；状态与提交回调由落地页（任务 11）持有，因此本任务用「渲染断言」的组件测试验收，不依赖路由。

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/components/invite/inviteComponents.test.tsx`：

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { InviteRegisterForm } from "./InviteRegisterForm";
import { JoinRequestStatusCard } from "./JoinRequestStatusCard";

describe("InviteRegisterForm", () => {
	it("renders the direct-join button label when approval is off", () => {
		render(
			<InviteRegisterForm
				domainName="Realm"
				requiresApproval={false}
				submitting={false}
				error=""
				onSubmit={() => {}}
				onSendEmailCode={() => {}}
				emailRequired={false}
				codeSending={false}
				codeCountdown={0}
			/>,
		);
		expect(screen.getByRole("button", { name: "注册并加入" })).toBeTruthy();
	});

	it("renders the pending button label when approval is on", () => {
		render(
			<InviteRegisterForm
				domainName="Realm"
				requiresApproval
				submitting={false}
				error=""
				onSubmit={() => {}}
				onSendEmailCode={() => {}}
				emailRequired={false}
				codeSending={false}
				codeCountdown={0}
			/>,
		);
		expect(screen.getByRole("button", { name: "注册并提交申请" })).toBeTruthy();
	});

	it("blocks submit and shows inline errors when fields are empty", () => {
		const onSubmit = vi.fn();
		render(
			<InviteRegisterForm
				domainName="Realm"
				requiresApproval={false}
				submitting={false}
				error=""
				onSubmit={onSubmit}
				onSendEmailCode={() => {}}
				emailRequired={false}
				codeSending={false}
				codeCountdown={0}
			/>,
		);
		fireEvent.click(screen.getByRole("button", { name: "注册并加入" }));
		expect(onSubmit).not.toHaveBeenCalled();
		expect(screen.getByText("用户名 是必填项")).toBeTruthy();
		expect(screen.getByText("密码 是必填项")).toBeTruthy();
	});

	it("submits the entered values", () => {
		const onSubmit = vi.fn();
		render(
			<InviteRegisterForm
				domainName="Realm"
				requiresApproval={false}
				submitting={false}
				error=""
				onSubmit={onSubmit}
				onSendEmailCode={() => {}}
				emailRequired={false}
				codeSending={false}
				codeCountdown={0}
			/>,
		);
		fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "newbie" } });
		fireEvent.change(screen.getByLabelText("密码"), { target: { value: "password123" } });
		fireEvent.click(screen.getByRole("button", { name: "注册并加入" }));
		expect(onSubmit).toHaveBeenCalledWith({
			username: "newbie",
			password: "password123",
			nickname: "",
			email: "",
			emailCode: "",
		});
	});

	it("shows the server error banner", () => {
		render(
			<InviteRegisterForm
				domainName="Realm"
				requiresApproval={false}
				submitting={false}
				error="邀请链接无效"
				onSubmit={() => {}}
				onSendEmailCode={() => {}}
				emailRequired={false}
				codeSending={false}
				codeCountdown={0}
			/>,
		);
		expect(screen.getByRole("alert").textContent).toContain("邀请链接无效");
	});
});

describe("JoinRequestStatusCard", () => {
	it("renders the pending state with a cancel action", () => {
		const onCancel = vi.fn();
		render(
			<JoinRequestStatusCard
				domainName="Realm"
				iconUrl=""
				status="pending"
				submittedAt="2026-11-10 12:00"
				cancelling={false}
				onCancel={onCancel}
				onReapply={() => {}}
			/>,
		);
		expect(screen.getByText("申请审核中")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "撤销申请" }));
		expect(onCancel).toHaveBeenCalled();
	});

	it("renders the rejected state with a reapply action", () => {
		const onReapply = vi.fn();
		render(
			<JoinRequestStatusCard
				domainName="Realm"
				iconUrl=""
				status="rejected"
				submittedAt="2026-11-10 12:00"
				cancelling={false}
				onCancel={() => {}}
				onReapply={onReapply}
			/>,
		);
		expect(screen.getByText("申请未通过")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "重新申请" }));
		expect(onReapply).toHaveBeenCalled();
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/web && pnpm vitest run src/components/invite/inviteComponents.test.tsx`
预期：FAIL，`Failed to resolve import "./InviteRegisterForm"`

- [ ] **步骤 3：实现内联注册表单**

创建 `app/web/src/components/invite/InviteRegisterForm.tsx`：

```tsx
import { Button, Input } from "@gospeak/components";
import { Loader2 } from "lucide-react";
import { useState } from "react";
import PasswordInput from "@/components/common/passwordInput";

export interface InviteRegisterSubmit {
	username: string;
	password: string;
	nickname: string;
	email: string;
	emailCode: string;
}

export interface InviteRegisterFormProps {
	domainName: string;
	requiresApproval: boolean;
	submitting: boolean;
	error: string;
	emailRequired: boolean;
	codeSending: boolean;
	codeCountdown: number;
	onSubmit: (values: InviteRegisterSubmit) => void;
	onSendEmailCode: (email: string) => void;
}

export function InviteRegisterForm(props: InviteRegisterFormProps) {
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [nickname, setNickname] = useState("");
	const [email, setEmail] = useState("");
	const [emailCode, setEmailCode] = useState("");
	const [touched, setTouched] = useState({ username: false, password: false });

	const usernameError = touched.username && !username.trim() ? "用户名 是必填项" : "";
	const passwordError =
		touched.password && !password ? "密码 是必填项" : "";

	const submitLabel = props.requiresApproval ? "注册并提交申请" : "注册并加入";

	function handleSubmit(event: React.FormEvent) {
		event.preventDefault();
		setTouched({ username: true, password: true });
		if (!username.trim() || password.length < 8) return;
		if (props.emailRequired && (!email.trim() || !emailCode.trim())) return;
		props.onSubmit({
			username: username.trim(),
			password,
			nickname: nickname.trim(),
			email: email.trim(),
			emailCode: emailCode.trim(),
		});
	}

	return (
		<form onSubmit={handleSubmit} className="flex flex-col gap-3" noValidate>
			<p className="text-muted-foreground text-sm leading-6">
				注册后{props.requiresApproval ? "提交加入申请，管理员通过后即可进入" : "立即加入"}
				「{props.domainName}」。
			</p>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">用户名</span>
				<Input
					value={username}
					autoComplete="username"
					onChange={(e) => {
						setUsername(e.currentTarget.value);
						if (e.currentTarget.value.trim()) setTouched((t) => ({ ...t, username: false }));
					}}
				/>
				{usernameError && <span className="text-destructive mt-1 block text-xs">{usernameError}</span>}
			</label>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">密码</span>
				<PasswordInput
					value={password}
					autoComplete="new-password"
					placeholder="至少 8 位"
					onChange={(e) => setPassword(e.currentTarget.value)}
				/>
				{passwordError && <span className="text-destructive mt-1 block text-xs">{passwordError}</span>}
			</label>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">昵称（选填）</span>
				<Input
					value={nickname}
					maxLength={32}
					placeholder="不填则使用用户名"
					onChange={(e) => setNickname(e.currentTarget.value)}
				/>
			</label>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					邮箱{props.emailRequired ? "" : "（选填）"}
				</span>
				<Input
					type="email"
					value={email}
					autoComplete="email"
					onChange={(e) => setEmail(e.currentTarget.value)}
				/>
			</label>

			{email.trim() && (
				<div className="flex gap-2">
					<Input
						value={emailCode}
						placeholder="验证码"
						onChange={(e) => setEmailCode(e.currentTarget.value)}
					/>
					<Button
						type="button"
						variant="outline"
						disabled={props.codeSending || props.codeCountdown > 0}
						onClick={() => props.onSendEmailCode(email.trim())}
					>
						{props.codeSending
							? "发送中..."
							: props.codeCountdown > 0
								? `${props.codeCountdown}s 后重发`
								: "发送验证码"}
					</Button>
				</div>
			)}

			{props.error && (
				<p role="alert" className="border-destructive/50 bg-destructive/10 text-destructive rounded-sm border px-3 py-2 text-sm">
					{props.error}
				</p>
			)}

			<Button type="submit" size="lg" disabled={props.submitting}>
				{props.submitting ? <Loader2 className="size-4 animate-spin" /> : submitLabel}
			</Button>
		</form>
	);
}
```

- [ ] **步骤 4：实现申请状态卡片**

创建 `app/web/src/components/invite/JoinRequestStatusCard.tsx`：

```tsx
import { Button } from "@gospeak/components";
import { Clock, XCircle } from "lucide-react";

export interface JoinRequestStatusCardProps {
	domainName: string;
	iconUrl: string;
	status: "pending" | "rejected";
	submittedAt: string;
	cancelling: boolean;
	onCancel: () => void;
	onReapply: () => void;
}

export function JoinRequestStatusCard(props: JoinRequestStatusCardProps) {
	const pending = props.status === "pending";
	return (
		<div className="flex flex-col gap-4">
			<div className="flex items-center gap-3">
				{props.iconUrl ? (
					<img src={props.iconUrl} alt={props.domainName} className="size-14 shrink-0 rounded-xl object-cover" />
				) : (
					<div className="bg-primary text-primary-foreground flex size-14 shrink-0 items-center justify-center rounded-xl text-xl font-bold">
						{(Array.from(props.domainName.trim())[0] ?? "?").toUpperCase()}
					</div>
				)}
				<div className="min-w-0">
					<h1 className="truncate text-lg font-semibold">{props.domainName}</h1>
					<p className="text-muted-foreground text-xs">提交于 {props.submittedAt}</p>
				</div>
			</div>

			<div className="border-border flex items-start gap-2 rounded-lg border px-3 py-2.5 text-sm">
				{pending ? (
					<Clock size={16} className="text-warning mt-0.5 shrink-0" />
				) : (
					<XCircle size={16} className="text-destructive mt-0.5 shrink-0" />
				)}
				<div className="min-w-0">
					<p className="font-medium">{pending ? "申请审核中" : "申请未通过"}</p>
					<p className="text-muted-foreground mt-1 text-xs leading-relaxed">
						{pending
							? "管理员通过后即可进入该域。审核期间你暂时无法进入。"
							: "你可以重新提交申请，或联系域管理员了解原因。"}
					</p>
				</div>
			</div>

			<div className="flex gap-2">
				{pending ? (
					<Button variant="outline" className="flex-1" disabled={props.cancelling} onClick={props.onCancel}>
						{props.cancelling ? "撤销中..." : "撤销申请"}
					</Button>
				) : (
					<Button className="flex-1" onClick={props.onReapply}>
						重新申请
					</Button>
				)}
			</div>
		</div>
	);
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`cd app/web && pnpm vitest run src/components/invite/inviteComponents.test.tsx`
预期：7 个用例 PASS

> 若 `@gospeak/components` 未导出 `Input`，改读 `packages/components/src/index.ts` 后按实际导出名调整（`PasswordInput` 在 `app/web/src/components/common/passwordInput.tsx`，为 default 导出）。

- [ ] **步骤 6：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/web/src/components/invite/InviteRegisterForm.tsx app/web/src/components/invite/JoinRequestStatusCard.tsx app/web/src/components/invite/inviteComponents.test.tsx
git commit -m "feat(web): 内联注册表单与申请状态卡片"
```

---

## 任务 11：重写邀请落地页

**文件：**
- 修改（整体重写）：`app/web/src/pages/invite/d/$code.tsx`
- 测试：`app/web/src/pages/invite/__tests__/inviteLanding.test.tsx`

> **关键改造点：** 删除 `beforeLoad` 里的 `/guest` redirect（访客体系已在第一部分删除），改为**不拦截未登录用户**——未登录就渲染内联注册表单。`apiClient` 的公开白名单已包含 `path.startsWith("/invite/")`，因此 preview 与 register 的 401 不会触发整页跳登录。

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/pages/invite/__tests__/inviteLanding.test.tsx`：

```tsx
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const previewInvite = vi.fn();
const getJoinRequest = vi.fn();
const joinByCode = vi.fn();
const registerViaInvite = vi.fn();
const cancelJoinRequest = vi.fn();

vi.mock("@/api/inviteJoin", () => ({
	previewInvite: (...a: unknown[]) => previewInvite(...a),
	getJoinRequest: (...a: unknown[]) => getJoinRequest(...a),
	joinByCode: (...a: unknown[]) => joinByCode(...a),
	registerViaInvite: (...a: unknown[]) => registerViaInvite(...a),
	cancelJoinRequest: (...a: unknown[]) => cancelJoinRequest(...a),
}));

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
	createFileRoute: () => (opts: { component: () => JSX.Element }) => ({
		component: opts.component,
		useParams: () => ({ code: "CODE" }),
	}),
	Link: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
	useNavigate: () => navigate,
}));

const domainStoreState = {
	myDomainUUIDs: [] as string[],
	addDomain: vi.fn(),
	setCurrentDomain: vi.fn(),
	loadMembers: vi.fn(async () => {}),
	loadMyDomains: vi.fn(async () => {}),
};
vi.mock("@/stores/domainStore", () => ({
	domainStore: domainStoreState,
	useDomainStore: (selector: (s: typeof domainStoreState) => unknown) => selector(domainStoreState),
}));

const userState = { user: null as null | { uuid: string }, ensureSession: vi.fn(async () => true) };
vi.mock("@/stores/userStore", () => ({
	useUserStore: Object.assign(
		(selector: (s: typeof userState) => unknown) => selector(userState),
		{ getState: () => userState },
	),
}));

import { InviteLandingPage } from "../d/$code";

const previewPayload = {
	domain: { uuid: "d1", name: "Realm", icon_url: "", description: "desc" },
	requires_approval: false,
	remaining_uses: -1,
};

describe("invite landing page", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		userState.user = null;
		domainStoreState.myDomainUUIDs = [];
		previewInvite.mockResolvedValue(previewPayload);
	});

	it("shows the inline register form for anonymous visitors", async () => {
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByRole("button", { name: "注册并加入" })).toBeTruthy());
	});

	it("shows the join button for a logged-in non-member", async () => {
		userState.user = { uuid: "u1" };
		getJoinRequest.mockResolvedValue({ request: null, status: "", is_member: false });
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByRole("button", { name: "加入" })).toBeTruthy());
	});

	it("shows the enter-domain button for an existing member", async () => {
		userState.user = { uuid: "u1" };
		domainStoreState.myDomainUUIDs = ["d1"];
		getJoinRequest.mockResolvedValue({ request: null, status: "", is_member: true });
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByRole("button", { name: "进入域" })).toBeTruthy());
	});

	it("shows the pending status card when a request is awaiting review", async () => {
		userState.user = { uuid: "u1" };
		getJoinRequest.mockResolvedValue({
			request: { id: 1, status: "pending", created_at: "2026-11-10T04:00:00Z" },
			status: "pending",
			is_member: false,
		});
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByText("申请审核中")).toBeTruthy());
	});

	it("shows the rejected card with a reapply action", async () => {
		userState.user = { uuid: "u1" };
		getJoinRequest.mockResolvedValue({
			request: { id: 1, status: "rejected", created_at: "2026-11-10T04:00:00Z" },
			status: "rejected",
			is_member: false,
		});
		render(<InviteLandingPage />);
		await waitFor(() => expect(screen.getByText("申请未通过")).toBeTruthy());
	});

	it("shows a single error card without domain info when the code is invalid", async () => {
		previewInvite.mockRejectedValue(new Error("invalid"));
		render(<InviteLandingPage />);
		await waitFor(() =>
			expect(screen.getByText("邀请链接无效、已过期或已停用")).toBeTruthy(),
		);
		expect(screen.queryByText("Realm")).toBeNull();
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/web && pnpm vitest run src/pages/invite/__tests__/inviteLanding.test.tsx`
预期：FAIL，`InviteLandingPage` 未导出 / 渲染结果与断言不符

- [ ] **步骤 3：重写落地页**

把 `app/web/src/pages/invite/d/$code.tsx` 整体替换为：

```tsx
import { Button, toast } from "@gospeak/components";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { sendEmailCode } from "@/api/email";
import {
	cancelJoinRequest,
	getJoinRequest,
	type InviteJoinResult,
	type InvitePreview,
	joinByCode,
	previewInvite,
	registerViaInvite,
} from "@/api/inviteJoin";
import { InviteRegisterForm, type InviteRegisterSubmit } from "@/components/invite/InviteRegisterForm";
import { JoinRequestStatusCard } from "@/components/invite/JoinRequestStatusCard";
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { useUserStore } from "@/stores/userStore";
import { localizedApiMessage } from "@/utils/errorMessages";

export const Route = createFileRoute("/invite/d/$code")({
	component: InviteLandingPage,
	head: () => ({ meta: [{ title: "加入域 · GOSpeak" }] }),
});

type Phase = "loading" | "error" | "ready";

function formatSubmittedAt(raw: string): string {
	const date = new Date(raw);
	if (Number.isNaN(date.getTime())) return "";
	return date.toLocaleString("zh-CN", { hour12: false });
}

export function InviteLandingPage() {
	const { code } = Route.useParams();
	const navigate = useNavigate();

	const [phase, setPhase] = useState<Phase>("loading");
	const [preview, setPreview] = useState<InvitePreview | null>(null);
	const [error, setError] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [cancelling, setCancelling] = useState(false);
	const [requestStatus, setRequestStatus] = useState<"" | "pending" | "rejected">("");
	const [requestedAt, setRequestedAt] = useState("");
	const [codeSending, setCodeSending] = useState(false);
	const [codeCountdown, setCodeCountdown] = useState(0);

	const user = useUserStore((s) => s.user);
	const myDomainUUIDs = useDomainStore((s) => s.myDomainUUIDs);
	const joined = !!preview && myDomainUUIDs.includes(preview.domain.uuid);

	useEffect(() => {
		let cancelled = false;
		void (async () => {
			try {
				const data = await previewInvite(code);
				if (cancelled) return;
				setPreview(data);
				setPhase("ready");
			} catch {
				if (cancelled) return;
				setError("邀请链接无效、已过期或已停用");
				setPhase("error");
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [code]);

	useEffect(() => {
		if (!preview || !user) return;
		let cancelled = false;
		void (async () => {
			try {
				const view = await getJoinRequest(preview.domain.uuid);
				if (cancelled) return;
				// approved 不是落地页的展示态：审核通过后用户已是成员，
				// 由 joined（myDomainUUIDs）驱动「进入域」，这里归零即可。
				setRequestStatus(
					view.status === "pending" || view.status === "rejected" ? view.status : "",
				);
				setRequestedAt(view.request ? formatSubmittedAt(view.request.created_at) : "");
			} catch {
				// 申请状态是附加信息：拿不到不影响主流程（加入/进入域仍可用）
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [preview, user]);

	function enterDomain(uuid: string) {
		domainStore.setCurrentDomain(uuid);
		void domainStore.loadMembers(uuid);
		navigate({ to: "/domain/$domainUUID", params: { domainUUID: uuid } });
	}

	// 加入成功后先刷新「我的域」列表再跳转：domainStore.addDomain 需要完整 Domain，
	// 而 /invite/join 只回摘要字段，直接塞会缺 id/owner_uuid 等必填项。
	async function applyResult(result: InviteJoinResult) {
		if (result.status === "joined") {
			await domainStore.loadMyDomains().catch(() => {});
			toast.success(`已加入「${result.domain.name}」`);
			enterDomain(result.domain.uuid);
			return;
		}
		setRequestStatus("pending");
		setRequestedAt(formatSubmittedAt(new Date().toISOString()));
	}

	async function handleJoin() {
		if (!preview || submitting) return;
		if (joined) {
			enterDomain(preview.domain.uuid);
			return;
		}
		setSubmitting(true);
		setError("");
		try {
			await applyResult(await joinByCode(code));
		} catch (e) {
			setError(localizedApiMessage(e, "加入失败，请稍后重试"));
		} finally {
			setSubmitting(false);
		}
	}

	async function handleRegister(values: InviteRegisterSubmit) {
		if (!preview || submitting) return;
		setSubmitting(true);
		setError("");
		try {
			const result = await registerViaInvite({
				code,
				username: values.username,
				password: values.password,
				nickname: values.nickname,
				email: values.email,
				email_code: values.emailCode,
			});
			if (result.user) {
				await useUserStore.getState().login(result.user, result.expires_in);
			}
			await applyResult(result);
		} catch (e) {
			setError(localizedApiMessage(e, "注册失败，请稍后重试"));
		} finally {
			setSubmitting(false);
		}
	}

	async function handleSendEmailCode(email: string) {
		if (!email || codeSending || codeCountdown > 0) return;
		setCodeSending(true);
		try {
			const res = await sendEmailCode({ email, scene: "register" }, { skipErrorToast: true });
			toast.success("验证码已发送，请查收邮箱");
			setCodeCountdown(res.expires_in || 60);
		} catch (e) {
			setError(localizedApiMessage(e, "验证码发送失败"));
		} finally {
			setCodeSending(false);
		}
	}

	async function handleCancel() {
		if (!preview || cancelling) return;
		setCancelling(true);
		try {
			await cancelJoinRequest(preview.domain.uuid);
			setRequestStatus("");
			setRequestedAt("");
			toast.success("申请已撤销");
		} catch (e) {
			setError(localizedApiMessage(e, "撤销失败，请稍后重试"));
		} finally {
			setCancelling(false);
		}
	}

	async function handleReapply() {
		await handleJoin();
	}

	return (
		<div className="bg-muted flex min-h-full w-full items-center justify-center px-4">
			<div className="bg-card border-border w-full max-w-md rounded-lg border p-6 shadow-xl">
				<div className="mb-5 flex items-center gap-2">
					<span className="text-primary text-lg font-semibold">GOSpeak</span>
					<span className="text-faint-foreground text-xs">邀请</span>
				</div>

				{phase === "loading" && (
					<div className="flex flex-col items-center gap-3 py-10">
						<Loader2 className="text-muted-foreground size-6 animate-spin" />
						<p className="text-muted-foreground text-sm">正在打开邀请…</p>
					</div>
				)}

				{phase === "error" && (
					<div className="flex flex-col gap-4">
						<p className="text-destructive text-sm">{error}</p>
						<p className="text-muted-foreground text-sm">请向对方确认邀请链接是否已过期或停用。</p>
						<div className="flex gap-2">
							<Button
								variant="outline"
								onClick={() => {
									setPhase("loading");
									setError("");
									void (async () => {
										try {
											setPreview(await previewInvite(code));
											setPhase("ready");
										} catch {
											setError("邀请链接无效、已过期或已停用");
											setPhase("error");
										}
									})();
								}}
							>
								重试
							</Button>
							<Link to="/discover" className="text-muted-foreground hover:text-foreground inline-flex items-center px-2 text-sm">
								先去发现域
							</Link>
						</div>
					</div>
				)}

				{phase === "ready" && preview && (
					<div className="flex flex-col gap-5">
						<div className="flex items-center gap-3">
							{preview.domain.icon_url ? (
								<img src={preview.domain.icon_url} alt={preview.domain.name} className="size-14 shrink-0 rounded-xl object-cover" />
							) : (
								<div className="bg-primary text-primary-foreground flex size-14 shrink-0 items-center justify-center rounded-xl text-xl font-bold">
									{(Array.from(preview.domain.name.trim())[0] ?? "?").toUpperCase()}
								</div>
							)}
							<div className="min-w-0">
								<h1 className="truncate text-lg font-semibold">{preview.domain.name}</h1>
								<p className="text-muted-foreground text-xs">
									{joined ? "你已在这个域里" : "邀请你加入这个语音域"}
								</p>
							</div>
						</div>

						{preview.domain.description && (
							<p className="text-muted-foreground text-sm leading-6">{preview.domain.description}</p>
						)}

						{preview.requires_approval && !joined && requestStatus !== "pending" && (
							<p className="text-warning text-xs leading-relaxed">该域开启加入审核，加入后需等待管理员通过。</p>
						)}

						{requestStatus === "pending" && (
							<JoinRequestStatusCard
								domainName={preview.domain.name}
								iconUrl={preview.domain.icon_url}
								status="pending"
								submittedAt={requestedAt}
								cancelling={cancelling}
								onCancel={() => void handleCancel()}
								onReapply={() => void handleReapply()}
							/>
						)}

						{requestStatus === "rejected" && (
							<JoinRequestStatusCard
								domainName={preview.domain.name}
								iconUrl={preview.domain.icon_url}
								status="rejected"
								submittedAt={requestedAt}
								cancelling={cancelling}
								onCancel={() => void handleCancel()}
								onReapply={() => void handleReapply()}
							/>
						)}

						{requestStatus === "" && !user && (
							<>
								<InviteRegisterForm
									domainName={preview.domain.name}
									requiresApproval={preview.requires_approval}
									submitting={submitting}
									error={error}
									emailRequired={false}
									codeSending={codeSending}
									codeCountdown={codeCountdown}
									onSubmit={(values) => void handleRegister(values)}
									onSendEmailCode={(email) => void handleSendEmailCode(email)}
								/>
								<Link
									to="/login"
									className="text-muted-foreground hover:text-foreground text-center text-sm"
								>
									已有账号？登录
								</Link>
							</>
						)}

						{requestStatus === "" && user && (
							<div className="flex flex-col gap-3">
								{error && <p className="text-destructive text-sm">{error}</p>}
								<Button className="flex-1" disabled={submitting} onClick={() => void handleJoin()}>
									{submitting ? (
										<Loader2 className="size-4 animate-spin" />
									) : joined ? (
										"进入域"
									) : preview.requires_approval ? (
										"申请加入"
									) : (
										"加入"
									)}
								</Button>
								<Link to="/discover" className="text-muted-foreground hover:text-foreground text-center text-sm">
									取消
								</Link>
							</div>
						)}
					</div>
				)}
			</div>
		</div>
	);
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/web && pnpm vitest run src/pages/invite/__tests__/inviteLanding.test.tsx`
预期：6 个用例 PASS

- [ ] **步骤 5：类型检查**

运行：`cd app/web && pnpm typecheck`
预期：`tsr generate` 后 tsc 无错误（`routeTree.gen.ts` 会因删除 `beforeLoad` 重新生成）

- [ ] **步骤 6：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add "app/web/src/pages/invite/d/\$code.tsx" app/web/src/pages/invite/__tests__/inviteLanding.test.tsx app/web/src/routeTree.gen.ts
git commit -m "feat(web): 邀请落地页重写为五身份分流与内联注册"
```

---

## 任务 12：域管理页「加入申请」分区与审核开关

**文件：**
- 创建：`app/web/src/components/domain/JoinRequestTable.tsx`
- 修改：`app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainSettingsForm.tsx`
- 修改：`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`
- 修改：`app/web/src/api/domain.ts`（`Domain` 加 `require_join_approval`，`updateDomain` 入参加该字段）
- 测试：`app/web/src/components/domain/JoinRequestTable.test.tsx`

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/components/domain/JoinRequestTable.test.tsx`：

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { JoinRequestTable } from "./JoinRequestTable";

const baseProps = {
	items: [],
	total: 0,
	counts: { pending: 0, approved: 0, rejected: 0 },
	status: "pending" as const,
	keyword: "",
	page: 1,
	pageSize: 10,
	loading: false,
	error: "",
	reviewing: 0,
	onStatusChange: () => {},
	onKeywordChange: () => {},
	onSearch: () => {},
	onPageChange: () => {},
	onReview: () => {},
};

describe("JoinRequestTable", () => {
	it("shows the empty state", () => {
		render(<JoinRequestTable {...baseProps} />);
		expect(screen.getByText("暂无加入申请")).toBeTruthy();
	});

	it("renders rows with applicant names and review actions", () => {
		const onReview = vi.fn();
		render(
			<JoinRequestTable
				{...baseProps}
				total={1}
				counts={{ pending: 1, approved: 0, rejected: 0 }}
				items={[
					{
						id: 7,
						domain_uuid: "d1",
						user_uuid: "u1",
						invite_id: 1,
						status: "pending",
						reviewed_by: "",
						reviewed_at: null,
						created_at: "2026-11-10T04:00:00Z",
						updated_at: "2026-11-10T04:00:00Z",
						name: "alice",
						display_name: "爱丽丝",
					},
				]}
				onReview={onReview}
			/>,
		);
		expect(screen.getByText("爱丽丝")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "通过" }));
		expect(onReview).toHaveBeenCalledWith(7, true);
		fireEvent.click(screen.getByRole("button", { name: "拒绝" }));
		expect(onReview).toHaveBeenCalledWith(7, false);
	});

	it("hides review actions for already reviewed rows", () => {
		render(
			<JoinRequestTable
				{...baseProps}
				status="approved"
				total={1}
				items={[
					{
						id: 8,
						domain_uuid: "d1",
						user_uuid: "u2",
						invite_id: 1,
						status: "approved",
						reviewed_by: "admin-1",
						reviewed_at: "2026-11-10T05:00:00Z",
						created_at: "2026-11-10T04:00:00Z",
						updated_at: "2026-11-10T05:00:00Z",
						name: "bob",
						display_name: "鲍勃",
					},
				]}
			/>,
		);
		expect(screen.queryByRole("button", { name: "通过" })).toBeNull();
		expect(screen.getByText("已通过")).toBeTruthy();
	});

	it("shows the error banner", () => {
		render(<JoinRequestTable {...baseProps} error="加载失败" />);
		expect(screen.getByRole("alert").textContent).toContain("加载失败");
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/web && pnpm vitest run src/components/domain/JoinRequestTable.test.tsx`
预期：FAIL，`Failed to resolve import "./JoinRequestTable"`

- [ ] **步骤 3：实现表格组件**

创建 `app/web/src/components/domain/JoinRequestTable.tsx`：

```tsx
import { Button } from "@gospeak/components";
import { Loader2, Search } from "lucide-react";
import type { JoinRequestListItem } from "@/api/inviteJoin";

export type JoinRequestStatusFilter = "pending" | "approved" | "rejected";

const STATUS_LABELS: Record<JoinRequestStatusFilter, string> = {
	pending: "待审核",
	approved: "已通过",
	rejected: "已拒绝",
};

export interface JoinRequestTableProps {
	items: JoinRequestListItem[];
	total: number;
	counts: Record<string, number>;
	status: JoinRequestStatusFilter;
	keyword: string;
	page: number;
	pageSize: number;
	loading: boolean;
	error: string;
	reviewing: number;
	onStatusChange: (status: JoinRequestStatusFilter) => void;
	onKeywordChange: (keyword: string) => void;
	onSearch: () => void;
	onPageChange: (page: number) => void;
	onReview: (requestID: number, approve: boolean) => void;
}

function formatTime(raw: string): string {
	const date = new Date(raw);
	if (Number.isNaN(date.getTime())) return "";
	return date.toLocaleString("zh-CN", { hour12: false });
}

export function JoinRequestTable(props: JoinRequestTableProps) {
	const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize));

	return (
		<div className="flex min-w-0 flex-col gap-3">
			<div className="flex flex-wrap items-center gap-2">
				<div className="border-border inline-flex overflow-hidden rounded-sm border">
					{(["pending", "approved", "rejected"] as const).map((status) => (
						<button
							key={status}
							type="button"
							className={`px-3 py-1.5 text-xs transition-colors ${
								props.status === status
									? "bg-primary text-primary-foreground"
									: "text-muted-foreground hover:bg-muted"
							}`}
							onClick={() => props.onStatusChange(status)}
						>
							{STATUS_LABELS[status]}（{props.counts[status] ?? 0}）
						</button>
					))}
				</div>
				<div className="flex min-w-0 flex-1 items-center gap-2">
					<input
						className="border-input bg-card text-foreground focus-visible:ring-ring flex h-8 min-w-0 flex-1 rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2"
						placeholder="搜索申请人"
						value={props.keyword}
						onChange={(e) => props.onKeywordChange(e.currentTarget.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") props.onSearch();
						}}
					/>
					<Button variant="outline" size="sm" className="gap-1.5" onClick={props.onSearch}>
						<Search size={14} />
						搜索
					</Button>
				</div>
			</div>

			{props.error && (
				<p role="alert" className="border-destructive/50 bg-destructive/10 text-destructive rounded-sm border px-3 py-2 text-sm">
					{props.error}
				</p>
			)}

			{props.loading ? (
				<div className="text-faint-foreground flex items-center justify-center py-8">
					<Loader2 className="size-5 animate-spin" />
				</div>
			) : props.items.length === 0 ? (
				<div className="text-faint-foreground flex items-center justify-center py-8 text-sm">
					暂无加入申请
				</div>
			) : (
				<div className="min-w-0 overflow-x-auto">
					<table className="w-full min-w-[520px] border-collapse text-sm">
						<thead>
							<tr className="border-border bg-muted/50 text-muted-foreground border-b text-xs font-semibold">
								<th className="px-3 py-2 text-left">申请人</th>
								<th className="px-3 py-2 text-left">提交时间</th>
								<th className="px-3 py-2 text-left">状态</th>
								<th className="px-3 py-2 text-right">操作</th>
							</tr>
						</thead>
						<tbody>
							{props.items.map((item) => {
								const pending = item.status === "pending";
								return (
									<tr key={item.id} className="border-border hover:bg-muted/40 border-b">
										<td className="px-3 py-2">
											<div className="font-medium">{item.display_name || item.name}</div>
											<div className="text-faint-foreground text-xs">@{item.name}</div>
										</td>
										<td className="text-muted-foreground px-3 py-2 text-xs">
											{formatTime(item.created_at)}
										</td>
										<td className="px-3 py-2 text-xs">
											{item.status === "pending"
												? "待审核"
												: item.status === "approved"
													? "已通过"
													: "已拒绝"}
										</td>
										<td className="px-3 py-2 text-right">
											{pending && (
												<div className="inline-flex gap-1">
													<Button
														variant="outline"
														size="xs"
														disabled={props.reviewing === item.id}
														onClick={() => props.onReview(item.id, true)}
													>
														通过
													</Button>
													<Button
														variant="outline"
														size="xs"
														disabled={props.reviewing === item.id}
														onClick={() => props.onReview(item.id, false)}
													>
														拒绝
													</Button>
												</div>
											)}
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			)}

			{props.total > props.pageSize && (
				<div className="flex items-center justify-between text-xs">
					<span className="text-faint-foreground">
						第 {props.page} / {totalPages} 页，共 {props.total} 条
					</span>
					<div className="flex gap-2">
						<Button
							variant="outline"
							size="xs"
							disabled={props.page <= 1}
							onClick={() => props.onPageChange(props.page - 1)}
						>
							上一页
						</Button>
						<Button
							variant="outline"
							size="xs"
							disabled={props.page >= totalPages}
							onClick={() => props.onPageChange(props.page + 1)}
						>
							下一页
						</Button>
					</div>
				</div>
			)}
		</div>
	);
}
```

- [ ] **步骤 4：改 Domain 类型与 updateDomain**

修改 `app/web/src/api/domain.ts`：

1. `Domain` 接口在 `is_public: boolean;` 之后追加：

```ts
	is_public: boolean;
	require_join_approval?: boolean;
```

2. `updateDomain` 的入参类型加字段：

```ts
export async function updateDomain(data: {
	domain_uuid: string;
	name?: string;
	description?: string;
	icon_url?: string;
	is_public?: boolean;
	require_join_approval?: boolean;
}): Promise<Domain> {
```

- [ ] **步骤 5：给 DomainSettingsForm 加开关**

修改 `app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainSettingsForm.tsx`：

1. `DomainSettingsFormProps` 在 `setIsPublic` 之后追加：

```ts
	isPublic: boolean;
	setIsPublic: (value: boolean) => void;
	requireJoinApproval: boolean;
	setRequireJoinApproval: (value: boolean) => void;
```

2. 只读视图（`!props.canManage`）的 `<dl>` 里，在「公开状态」之后追加一项：

```tsx
					<div>
						<dt className="text-faint-foreground text-xs">加入审核</dt>
						<dd>{props.domain?.require_join_approval ? "需审核" : "无需审核"}</dd>
					</div>
```

3. 可编辑视图里，在「公开域」那个 `<label>` 之后追加一个同构的开关行：

```tsx
			<label className="border-border flex cursor-pointer items-center justify-between rounded-lg border px-3 py-2.5">
				<span className="flex items-center gap-2 text-sm">
					<ShieldCheck size={16} className="text-warning" />
					加入需审核
				</span>
				<Switch
					checked={props.requireJoinApproval}
					disabled={props.saving}
					onCheckedChange={props.setRequireJoinApproval}
				/>
			</label>
```

4. 顶部 import 追加 `ShieldCheck`：

```ts
import { Globe, Lock, Save, ShieldCheck } from "lucide-react";
```

- [ ] **步骤 6：运行测试验证通过**

运行：`cd app/web && pnpm vitest run src/components/domain/JoinRequestTable.test.tsx`
预期：4 个用例 PASS

- [ ] **步骤 7：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/web/src/components/domain/JoinRequestTable.tsx app/web/src/components/domain/JoinRequestTable.test.tsx app/web/src/api/domain.ts "app/web/src/pages/(app)/manage/domains/\$domainUUID/components/DomainSettingsForm.tsx"
git commit -m "feat(web): 加入申请表格组件与加入审核开关"
```

---

## 任务 13：域管理页接线

**文件：**
- 修改：`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`

> 本任务把任务 12 的组件挂进页面：新增「加入申请」`ManageSection`、审核开关 state、审核 handler，并把 `requireJoinApproval` 透传给 `DomainSettingsForm`。

- [ ] **步骤 1：编写失败的测试**

在 `app/web/src/components/domain/JoinRequestTable.test.tsx` 追加一个「开关 props 必须被透传」的守卫用例（本任务改动的是页面接线，用类型级断言防止 props 漏传）：

```tsx
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

it("wires the join-request section and approval toggle into the domain page", () => {
	const page = readFileSync(
		resolve(__dirname, "../../../pages/(app)/manage/domains/$domainUUID/index.tsx"),
		"utf8",
	);
	expect(page).toContain("JoinRequestTable");
	expect(page).toContain("requireJoinApproval={requireJoinApproval}");
	expect(page).toContain("handleToggleApproval");
	expect(page).toContain("fetchJoinRequests");
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/web && pnpm vitest run src/components/domain/JoinRequestTable.test.tsx`
预期：FAIL，断言 `JoinRequestTable` 未出现在域管理页源码中

- [ ] **步骤 3：补 import**

在 `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx` 顶部 import 区追加：

```ts
import {
	type JoinRequestListItem,
	listJoinRequests,
	reviewJoinRequest,
	setJoinApproval,
} from "@/api/inviteJoin";
import {
	JoinRequestTable,
	type JoinRequestStatusFilter,
} from "@/components/domain/JoinRequestTable";
```

- [ ] **步骤 4：补权限判定**

在 `const canManageRoles = ...` 之后追加：

```ts
	const canReviewRequests =
		isOwner ||
		currentRole === "admin" ||
		hasPermission("domain:member:review") ||
		hasDomainPermission(uuid, "domain:member:review");
```

- [ ] **步骤 5：补 state 与 handler**

在 `const [saving, setSaving] = useState(false);` 之后追加：

```ts
	const [requireJoinApproval, setRequireJoinApproval] = useState(false);
	const [approvalSaving, setApprovalSaving] = useState(false);

	// ── 加入申请 ──
	const [joinRequests, setJoinRequests] = useState<JoinRequestListItem[]>([]);
	const [joinRequestTotal, setJoinRequestTotal] = useState(0);
	const [joinRequestCounts, setJoinRequestCounts] = useState<Record<string, number>>({
		pending: 0,
		approved: 0,
		rejected: 0,
	});
	const [joinRequestStatus, setJoinRequestStatus] =
		useState<JoinRequestStatusFilter>("pending");
	const [joinRequestKeyword, setJoinRequestKeyword] = useState("");
	const [joinRequestPage, setJoinRequestPage] = useState(1);
	const [joinRequestLoading, setJoinRequestLoading] = useState(false);
	const [joinRequestError, setJoinRequestError] = useState("");
	const [reviewingRequestID, setReviewingRequestID] = useState(0);
```

在 `async function handleSave(e: React.FormEvent) {` 之前追加两个函数：

```ts
	async function fetchJoinRequests(
		status: JoinRequestStatusFilter,
		keyword: string,
		page: number,
	) {
		if (!canReviewRequests) return;
		setJoinRequestLoading(true);
		setJoinRequestError("");
		try {
			const result = await listJoinRequests({
				domain_uuid: uuid,
				status,
				keyword,
				page,
				page_size: 10,
			});
			setJoinRequests(result.items);
			setJoinRequestTotal(result.total);
			setJoinRequestCounts(result.counts);
			setJoinRequestPage(page);
		} catch (error) {
			setJoinRequestError(toastActionError(error, "加入申请加载失败"));
		} finally {
			setJoinRequestLoading(false);
		}
	}

	async function handleReviewRequest(requestID: number, approve: boolean) {
		setReviewingRequestID(requestID);
		try {
			await reviewJoinRequest(uuid, requestID, approve);
			toast.success(approve ? "已通过加入申请" : "已拒绝加入申请");
			await fetchJoinRequests(joinRequestStatus, joinRequestKeyword, joinRequestPage);
			if (approve) {
				await domainStore.loadMembers(uuid).catch(() => {});
			}
		} catch (error) {
			toastActionError(error, "审核失败，请稍后重试");
		} finally {
			setReviewingRequestID(0);
		}
	}

	async function handleToggleApproval(next: boolean) {
		setApprovalSaving(true);
		try {
			const result = await setJoinApproval(uuid, next);
			setRequireJoinApproval(result.require_join_approval);
			const cached = useDomainStore.getState().domainCache[uuid];
			if (cached) {
				domainStore.updateCachedDomain({
					...cached,
					require_join_approval: result.require_join_approval,
				});
			}
			toast.success(result.require_join_approval ? "已开启加入审核" : "已关闭加入审核");
		} catch (error) {
			toastActionError(error, "切换加入审核失败");
		} finally {
			setApprovalSaving(false);
		}
	}
```

- [ ] **步骤 6：域切换时加载申请与开关**

在域切换的 `useEffect`（`if (loadedUUID === uuid) return;` 那个）里，`void fetchRoles();` 之后追加：

```ts
		void fetchRoles();
		if (canReviewRequests) {
			void fetchJoinRequests("pending", "", 1);
		}
```

并在表单初始化 `useEffect`（`setIsPublic(domain.is_public);` 那个）里追加：

```ts
		setIsPublic(domain.is_public);
		setRequireJoinApproval(!!domain.require_join_approval);
```

- [ ] **步骤 7：`handleSave` 同步开关回填**

`handleSave` 里的 `updateDomain({...})` 调用**保持不变**（开关不走这里）。只在其成功分支的 `setIsPublic(updated.is_public);` 之后追加一行：

```ts
			setIsPublic(updated.is_public);
			setRequireJoinApproval(!!updated.require_join_approval);
```

> 开关本身走 `/domain/join-approval/config` 即时提交（`handleToggleApproval`），避免「改了开关忘了点保存」的不一致。`updateDomain` 的 `require_join_approval` 入参留给其他调用方（如创建域后立即设置），本页不使用。

- [ ] **步骤 8：把 props 传给 DomainSettingsForm**

把 `<DomainSettingsForm ... />` 的 props 列表改为：

```tsx
								<DomainSettingsForm
									canManage={canManage}
									domain={domain}
									name={name}
									setName={setName}
									nameError={nameError}
									setNameError={setNameError}
									description={description}
									setDescription={setDescription}
									isPublic={isPublic}
									setIsPublic={setIsPublic}
									requireJoinApproval={requireJoinApproval}
									setRequireJoinApproval={(next) => void handleToggleApproval(next)}
									saving={saving || approvalSaving}
									formError={formError}
									onSave={handleSave}
								/>
```

- [ ] **步骤 9：插入「加入申请」分区**

在「成员管理」`ManageSection`（`title="成员管理"`）之后、「角色与权限」之前插入：

```tsx
							{canReviewRequests && (
								<ManageSection
									title="加入申请"
									description={`${joinRequestCounts.pending ?? 0} 条待审核`}
									className="min-w-0"
								>
									<JoinRequestTable
										items={joinRequests}
										total={joinRequestTotal}
										counts={joinRequestCounts}
										status={joinRequestStatus}
										keyword={joinRequestKeyword}
										page={joinRequestPage}
										pageSize={10}
										loading={joinRequestLoading}
										error={joinRequestError}
										reviewing={reviewingRequestID}
										onStatusChange={(status) => {
											setJoinRequestStatus(status);
											void fetchJoinRequests(status, joinRequestKeyword, 1);
										}}
										onKeywordChange={setJoinRequestKeyword}
										onSearch={() => void fetchJoinRequests(joinRequestStatus, joinRequestKeyword, 1)}
										onPageChange={(page) =>
											void fetchJoinRequests(joinRequestStatus, joinRequestKeyword, page)
										}
										onReview={(id, approve) => void handleReviewRequest(id, approve)}
									/>
								</ManageSection>
							)}
```

- [ ] **步骤 10：类型检查与测试**

运行：

```bash
cd app/web && pnpm typecheck && pnpm vitest run src/components/domain/JoinRequestTable.test.tsx
```
预期：tsc 无错误；表格 5 个用例 PASS（4 个渲染用例 + 1 个接线守卫用例）

- [ ] **步骤 11：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add "app/web/src/pages/(app)/manage/domains/\$domainUUID/index.tsx" app/web/src/components/domain/JoinRequestTable.test.tsx
git commit -m "feat(web): 域管理页加入申请分区与审核开关接线"
```

---

## 任务 14：发现页加入流程与前端文案

**文件：**
- 修改：`app/web/src/pages/(app)/discover.tsx`
- 修改：`app/web/src/components/discover/DiscoverDomainGrid.tsx`（新增 `pendingUUIDs` prop）
- 修改：`app/web/src/utils/errorMessages.ts`
- 修改：`app/web/src/utils/permissions.ts`

> 发现页只改「加入」这一段：公开域卡片与粘贴邀请码的 preview 继续用 `previewDomainInvite`（任务 15 才删除），但**加入动作**改走 `/invite/join`，因为公开域免码加入必须携带 `domain_uuid` 而不是码。

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/utils/joinTarget.spec.ts`（把发现页的分流决策抽成纯函数，避免为了测页面而 mock 整棵组件树）：

```ts
import { describe, expect, it } from "vitest";
import { joinTarget } from "./joinTarget";

describe("joinTarget", () => {
	it("prefers domain_uuid for public-domain joins", () => {
		expect(joinTarget("CODE", "d1")).toEqual({ domain_uuid: "d1" });
	});

	it("falls back to the invite code when no domain_uuid is known", () => {
		expect(joinTarget("CODE", "")).toEqual({ code: "CODE" });
	});

	it("returns null when neither is available", () => {
		expect(joinTarget("", "")).toBeNull();
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/web && pnpm vitest run src/utils/joinTarget.spec.ts`
预期：FAIL，`Failed to resolve import "./joinTarget"`

- [ ] **步骤 3：实现分流助手并改加入动作**

创建 `app/web/src/utils/joinTarget.ts`：

```ts
/**
 * 决定 /invite/join 的请求体：
 * 从公开域卡片进来时带 domain_uuid（免码，仍受域级审核约束）；
 * 从粘贴邀请码进来时带 code。两者都没有时返回 null，调用方应放弃请求。
 */
export function joinTarget(
	code: string,
	domainUUID: string,
): { code: string } | { domain_uuid: string } | null {
	if (domainUUID) return { domain_uuid: domainUUID };
	if (code) return { code };
	return null;
}
```

修改 `app/web/src/pages/(app)/discover.tsx`：

1. import 追加：

```ts
import { getJoinRequest, joinByCode, joinByDomain } from "@/api/inviteJoin";
import { joinTarget } from "@/utils/joinTarget";
```

2. 新增 state（与 `previewCode` 相邻）：

```ts
	const [previewDomainUUID, setPreviewDomainUUID] = useState("");
```

3. `openPreview` 签名改为同时记录 domain_uuid 来源：

```ts
	async function openPreview(code: string, domainUUID = "") {
		if (previewLoading) return;
		setPreviewOpen(true);
		setPreviewLoading(true);
		setPreviewError("");
		setPreviewDomain(null);
		setPreviewCode(code);
		setPreviewDomainUUID(domainUUID);
		try {
			setPreviewDomain(await previewDomainInvite(code));
		} catch (e) {
			setPreviewError(localizedApiMessage(e, "邀请码无效"));
		} finally {
			setPreviewLoading(false);
		}
	}
```

4. `joinPreview` 改为按来源分流：

```ts
	async function joinPreview() {
		const domain = previewDomain;
		const code = previewCode;
		const domainUUID = previewDomainUUID;
		if (!domain) return;
		if (joining) return;
		if (isJoined(domain.uuid)) {
			setPreviewOpen(false);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: domain.uuid },
			});
			return;
		}
		const target = joinTarget(code, domainUUID);
		if (!target) {
			setPreviewError("未识别到邀请码或邀请链接");
			return;
		}
		setJoining(true);
		setPreviewError("");
		try {
			const result = "domain_uuid" in target
				? await joinByDomain(target.domain_uuid)
				: await joinByCode(target.code);
			if (result.status === "pending") {
				setPreviewError("已提交加入申请，等待管理员审核");
				return;
			}
			await domainStore.loadMyDomains().catch(() => {});
			domainStore.setCurrentDomain(result.domain.uuid);
			setPreviewOpen(false);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: result.domain.uuid },
			});
		} catch (e) {
			setPreviewError(localizedApiMessage(e, "加入失败，请稍后重试"));
		} finally {
			setJoining(false);
		}
	}
```

5. 公开域卡片的 `onOpen` 改为传 domain_uuid：

```tsx
							onOpen={(domain) => void openPreview(domain.invite_code, domain.uuid)}
```

6. 已提交待审的用户在卡片上显示「申请审核中」而不是「加入」（§6.5）。新增 state 与加载逻辑：

```ts
	const [pendingDomainUUIDs, setPendingDomainUUIDs] = useState<string[]>([]);

	// 与 publicDomains 同批加载：对每个公开域查一次自己的申请状态。
	// 域数量有分页上限（PAGE_SIZE=12），请求量可控。
	useEffect(() => {
		if (!publicDomains?.domains.length) return;
		let cancelled = false;
		void (async () => {
			const results = await Promise.all(
				publicDomains.domains.map(async (domain) => {
					try {
						const view = await getJoinRequest(domain.uuid);
						return view.status === "pending" ? domain.uuid : "";
					} catch {
						return "";
					}
				}),
			);
			if (cancelled) return;
			setPendingDomainUUIDs(results.filter(Boolean));
		})();
		return () => {
			cancelled = true;
		};
	}, [publicDomains]);
```

把 `pendingDomainUUIDs` 传给 `DiscoverDomainGrid`（新增同名 prop），卡片按钮按它显示「申请审核中」并禁用；`DiscoverDomainGrid` 内对应判断为：

```tsx
	pendingUUIDs.includes(domain.uuid) ? "申请审核中" : joined ? "进入域" : "加入"
```

- [ ] **步骤 4：补中文文案**

修改 `app/web/src/utils/errorMessages.ts`，在 `SERVER_MESSAGE_ZH` 的「域」区块末尾追加：

```ts
	"invite code format is invalid": "邀请链接无效",
	"invite code has expired": "邀请链接已过期",
	"invite code is disabled": "邀请链接已停用",
	"invite code has reached its usage limit": "邀请链接已达使用上限",
	"invite code is no longer usable": "邀请链接已不可用",
	"invite code is required": "请填写邀请码",
	"this domain requires an invite code": "该域需要邀请码才能加入",
	"missing domain member review permission": "你没有审核权限",
	"missing domain invite manage permission": "你没有邀请码管理权限",
	"join request not found": "加入申请不存在",
	"join request has already been reviewed": "该申请已被其他管理员处理",
	"join request is not pending": "该申请不在待审核状态",
	"applicant no longer exists": "申请人账号已不存在",
	"nickname is too long": "昵称过长",
	"email and email_code are required": "该服务器已开启邮箱验证，请填写邮箱与验证码",
	"username cannot start with guest_": "用户名不能以 guest_ 开头",
	"password must be at least 8 characters": "密码至少 8 位",
	"domain_uuid and user_uuid are required": "缺少必要的参数",
```

- [ ] **步骤 5：补前端权限兜底表**

修改 `app/web/src/utils/permissions.ts`：

1. `rolePermissions.admin` 里 `"domain:invite",` 之后追加两行：

```ts
		"domain:invite",
		"domain:invite:manage",
		"domain:member:review",
```

> 若第一部分已加过这两项，本步骤跳过（重复项会被 lint 拦下）。

- [ ] **步骤 6：类型检查**

运行：`cd app/web && pnpm typecheck`
预期：tsc 无错误

- [ ] **步骤 7：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add "app/web/src/pages/(app)/discover.tsx" app/web/src/components/discover/DiscoverDomainGrid.tsx app/web/src/utils/joinTarget.ts app/web/src/utils/joinTarget.spec.ts app/web/src/utils/errorMessages.ts app/web/src/utils/permissions.ts app/web/src/components/discover/DiscoverDomainGrid.tsx app/web/src/utils/errorMessages.ts app/web/src/utils/permissions.ts
git commit -m "feat(web): 发现页加入改走 invite/join 并补审核文案"
```

---

## 任务 15：删除旧加入端点与前端旧 API

**文件：**
- 删除：`app/server/internal/handler/domain_handler.go` 的 `Join` / `Preview` / `JoinDomainRequest`
- 删除：`app/server/internal/router/routes/domain/routes.go:23-24`（`/join`、`/preview` 两行）
- 删除：`app/web/src/api/domain.ts` 的 `previewDomainInvite` / `joinDomain`
- 修改：`app/web/src/pages/(app)/discover.tsx`（preview 改用 `previewInvite`）
- 修改：`test/helpers/api.ts`、`test/domain/domain.test.ts`、`test/message/message.test.ts`
- 修改：`app/server/internal/handler/domain_handler_test.go`（删 join/preview 用例）

> 这是**最后一个任务**：此时新链路已完整（任务 1–14 全部合并），旧端点无调用方。删完后 `go build ./...`、`go test ./...`、`pnpm typecheck`、`pnpm test:server` 必须全绿。

- [ ] **步骤 1：编写失败的测试**

在 `test/invite/invite.test.ts` 末尾追加两个「旧端点必须已消失」的用例：

```ts
describe("legacy join endpoints removed", () => {
  it("no longer serves /api/v1/domain/join", async () => {
    const owner = await registerUser("legacy");
    const domain = await createDomain(owner.access_token, unique("legacy_domain"));
    const res = await api("/api/v1/domain/join", {
      token: owner.access_token,
      body: { invite_code: "ANY" },
    });
    expect(res.status).toBe(404);
  });

  it("no longer serves /api/v1/domain/preview", async () => {
    const res = await api("/api/v1/domain/preview", { body: { invite_code: "ANY" } });
    expect(res.status).toBe(404);
  });
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm test:server -- test/invite/`
预期：FAIL，两个用例的 `status` 仍为 200（旧端点还在）

- [ ] **步骤 3：确认无残留调用方**

运行：

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
grep -rn 'domain/join\|domain/preview\|joinDomain\|previewDomainInvite' --include=*.go --include=*.ts --include=*.tsx app/ test/ | grep -v node_modules
```
预期输出中只剩本任务要删的符号（`handler/domain_handler.go`、`routes/domain/routes.go`、`api/domain.ts`、`discover.tsx`、`test/helpers/api.ts`、`test/domain/domain.test.ts`、`test/message/message.test.ts`、`handler/domain_handler_test.go`）。若出现其他文件，先在那里改完再继续。

- [ ] **步骤 4：删后端 handler 与路由**

1. 从 `app/server/internal/handler/domain_handler.go` 删除 `JoinDomainRequest` 类型、`Join` 方法、`Preview` 方法（整段删，含 swagger 注释）。
2. 从 `app/server/internal/router/routes/domain/routes.go` 删除这两行：

```go
	r.POST("/join", h.Join)
	r.POST("/preview", h.Preview)
```

3. 从 `app/server/internal/handler/domain_handler_test.go` 删除 `TestDomainHandler_Join_*`、`TestDomainHandler_Preview_*` 用例，以及路由注册里的：

```go
	rg.POST("/join", h.Join)
	rg.POST("/preview", h.Preview)
```

- [ ] **步骤 5：删前端旧 API**

1. 从 `app/web/src/api/domain.ts` 删除 `previewDomainInvite`（L122–131）与 `joinDomain`（L133–141）两个函数。
2. `app/web/src/pages/(app)/discover.tsx`：
   - import 从 `@/api/domain` 移除 `joinDomain`、`previewDomainInvite`，并从 `@/api/inviteJoin` 追加 `previewInvite`；
   - `openPreview` 改为：

```ts
	async function openPreview(code: string, domainUUID = "") {
		if (previewLoading) return;
		setPreviewOpen(true);
		setPreviewLoading(true);
		setPreviewError("");
		setPreviewDomain(null);
		setPreviewCode(code);
		setPreviewDomainUUID(domainUUID);
		try {
			const data = await previewInvite(code);
			setPreviewDomain({
				id: 0,
				uuid: data.domain.uuid,
				name: data.domain.name,
				icon_url: data.domain.icon_url,
				description: data.domain.description,
				owner_uuid: "",
				is_public: true,
				require_join_approval: data.requires_approval,
				created_at: "",
			});
		} catch (e) {
			setPreviewError(localizedApiMessage(e, "邀请码无效"));
		} finally {
			setPreviewLoading(false);
		}
	}
```

- [ ] **步骤 6：改集成测试助手与既有用例**

1. `test/helpers/api.ts`：
   - `createDomain` 返回值去掉 `invite_code`（域创建后默认码在 `domain_invites` 表里，需另查）；
   - 删除 `joinDomain` 函数；
   - 新增两个助手：

```ts
export async function createDomainInvite(
  token: string,
  domainUUID: string,
): Promise<{ id: number; code: string; max_uses: number; used_count: number }> {
  const result = await api<{ id: number; code: string; max_uses: number; used_count: number }>(
    "/api/v1/domain/invites/create",
    { token, body: { domain_uuid: domainUUID } },
  );
  return assertSuccess(result);
}

export async function joinViaInvite(
  token: string,
  code: string,
): Promise<{ status: string; domain: { uuid: string } }> {
  const result = await api<{ status: string; domain: { uuid: string } }>("/api/v1/invite/join", {
    token,
    body: { code },
  });
  return assertSuccess(result);
}
```

2. `test/domain/domain.test.ts`：把 `domain.invite_code` 的用法改成 `const invite = await createDomainInvite(owner.access_token, domain.uuid);` 再用 `invite.code`；`joinDomain(...)` 改成 `joinViaInvite(...)`；删除 `/api/v1/domain/preview` 那一段（preview 由 `test/invite/invite.test.ts` 覆盖）。
3. `test/message/message.test.ts`：`joinDomain(reader.access_token, domain.invite_code)` 改成 `joinViaInvite(reader.access_token, (await createDomainInvite(author.access_token, domain.uuid)).code)`；删除访客用例（`/api/v1/auth/guest`，第一部分已删）。

- [ ] **步骤 7：全量验证**

运行：

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
cd app/server && go build ./... && go vet ./... && go test ./internal/... -count=1
cd ../.. && pnpm typecheck && pnpm test:web
pnpm test:server
```
预期：全部通过，无 `domain/join` / `domain/preview` 残留引用

- [ ] **步骤 8：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add -A
git commit -m "refactor(invite): 删除旧的 domain/join 与 domain/preview 端点"
```

---

## 规格问题与实现偏差记录

写作本计划时对照规格逐节核出以下 5 处，实施者按此处理（已在对应任务内落地）：

| # | 规格位置 | 问题 | 本计划的处理 |
|---|---------|------|------------|
| 1 | §6.1 内联注册表单 | 规格要求「邮箱 + 验证码：服务端开启邮箱验证时必填」，但**前端没有公开接口能读到 `IsVerificationAvailable()`**（只有管理端 `/api/v1/email/config`，需 `email_config:read` 权限） | 表单始终渲染邮箱与验证码字段，`emailRequired` 传 `false`，由后端在开启邮箱验证时返回 `2001 "email and email_code are required"`，前端用 `errorMessages.ts` 的新词条翻译成中文。**未新增公开配置端点**（超出本部分范围） |
| 2 | §5.2 `/invite/preview` | 规格把 preview 列为第二部分「公开路由」，但第一部分已实现同名端点 | 不新建第二份：沿用第一部分的端点与响应结构（已含 `requires_approval`/`remaining_uses`），第二部分只补 `RateLimit(20, time.Hour)`。已在任务 5 明确写明 |
| 3 | §4.3 状态机「`approved` 再次申请」 | 规格只描述「校验 `domain_members` 是否仍存在」，未说明「仍是成员」时的返回 | `JoinRequestService.Submit` 先查成员：是成员返回 `ErrAlreadyMember`（HTTP 层 3002）；`InviteJoinService.Join` 对已是成员走幂等返回 `status=joined` 且**不消耗邀请码额度**（任务 3/4 各有对应用例） |
| 4 | §5.5 步骤 2「复用 AuthService.Register 逻辑」 | `AuthService.Register` 自身开事务外写库并签发 token，无法并入「建用户 + 消耗码 + 入域」同一事务 | `InviteJoinService.RegisterAndJoin` 复刻其校验顺序（guest_/bot_ 前缀 → 用户名唯一 → 邮箱验证 → 密码强度）但把写操作全部放进自己的一次事务；`AuthService` 不改 |
| 5 | §7.1「审核通过时用户已被删除」 | 规格说「拒绝并标记申请 rejected」，但没定义错误码 | 先条件更新为 `rejected` 再返回 `NOT_FOUND "applicant no longer exists"`（3001），前端映射为「申请人账号已不存在」 |

---

## 验收标准

- [ ] `cd app/server && go build ./... && go vet ./...` 无输出
- [ ] `cd app/server && go test ./internal/... -count=1` 全通过（新增 5 个测试文件：`domain_join_request_test.go`、`domain_join_request_repo_test.go`、`join_request_service_test.go`、`invite_join_service_test.go`、`invite_join_handler_test.go`、`join_request_handler_test.go`）
- [ ] `pnpm test:server -- test/invite/` 通过；`pnpm test:server` 无回归
- [ ] `cd app/web && pnpm typecheck` 无错误
- [ ] `cd app/web && pnpm vitest run src/api/inviteJoin.spec.ts src/components/invite/inviteComponents.test.tsx src/components/domain/JoinRequestTable.test.tsx src/pages/invite/__tests__/inviteLanding.test.tsx` 全通过
- [ ] `pnpm lint` 通过
- [ ] 全仓无 `domain/join`、`domain/preview`、`joinDomain`、`previewDomainInvite` 残留
- [ ] 手工验证 1：新建域 → 邀请码列表有默认码 → 无痕窗口打开 `/invite/d/<code>` → 看到域信息与内联注册表单 → 注册 → 直接进域
- [ ] 手工验证 2：域管理页开启「加入需审核」→ 无痕窗口注册 → 看到「申请审核中」卡片（进不了域）
- [ ] 手工验证 3：域管理页「加入申请」显示 1 条待审 → 点「通过」→ 该用户刷新落地页 → 看到「进入域」
- [ ] 手工验证 4：同一用户点「撤销申请」→ 卡片消失 → 可重新注册/申请
- [ ] 手工验证 5：公开域（免审核）从发现页加入 → 直接进域
- [ ] 手工验证 6：公开域（需审核）从发现页加入 → 提示「已提交加入申请，等待管理员审核」
- [ ] 手工验证 7：私有域从发现页免码加入 → 提示需要邀请码
- [ ] 手工验证 8：邀请码设 1 次上限 → 第二人注册 → 提示已达使用上限
- [ ] 手工验证 9：无效码打开落地页 → 单一错误卡片，不显示任何域名信息
