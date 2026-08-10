# 每个域独立角色/权限管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让每个语音域拥有独立的角色与权限配置：域 owner/admin 可在域内创建自定义角色、为角色分配域内权限、调整成员角色，并且踢人、房间管理、消息删除等操作按“全局权限码 → 域角色权限 → 资源归属/创建者”的优先级生效。

**Architecture:** 后端新增 `domain_roles` 与 `domain_role_permissions` 两张表，每个域创建时自动 seed 系统角色 `owner/admin/member/guest`；`DomainService` 增加 `HasDomainPermission` 与角色 CRUD/成员角色分配能力，`DomainHandler` 暴露 `/api/v1/domain/roles/*`、`/members/update-role`、`/my-permissions` 路由，并改造现有 `Update/Kick/RoomHandler/MessageHandler/Signal message_bridge` 的判定。前端在 `/manage/domains/$domainUUID` 新增“角色与权限”面板，成员表支持下拉改角色，`domainStore` 缓存当前用户在每个域的有效权限。

**Tech Stack:** Go 1.26 · Gin · GORM · SQLite/PostgreSQL/MySQL · SolidJS · TypeScript · TanStack Solid Router · Tailwind v4 · daisyUI · lucide-solid · Vitest

---

## Global Constraints

- 当前工作树已有用户未提交改动：`app/server/internal/config/config*.go`、`app/server/internal/repository/db.go`、`app/server/internal/repository/db_test.go`。这些文件**不得**被本计划触碰；每个任务 commit 只暂存该任务列出的文件，禁止 `git add .`。
- 域角色权限只能使用 `model.AssignableDomainPermissions` 白名单；平台管理权限（`user:*`、`role:*`、`sfu:*`、`bot:*`、`storage:*`、`oauth:*`、`cluster:*`、`plugin:*`、`email_config:*`、`domain:create`、`domain:delete` 等）不允许分配给域角色。
- `owner` 为系统角色，不可创建/删除/改名/编辑权限/被分配；其有效权限恒等于 `AssignableDomainPermissions`。
- `admin`、`member`、`guest` 为系统角色，不可删除/改名，但权限可编辑。自定义角色可创建/删除，暂不支持重命名。
- 成员不能通过 API 修改自己的域角色；不能把成员改成 `owner`；owner 成员的角色不可被修改。
- 删除角色前若已有成员使用该角色，返回 `ALREADY_EXISTS` 并拒绝删除。
- HTTP 判定优先级固定：全局权限码 → 域角色权限 → 资源归属/创建者；`permSvc`/`domainSvc`/`roomSvc` 必须 nil 安全。
- WS `message:delete` 的域权限判定只信任 payload 中携带的 `domain_uuid`（现有前端已带该字段）；未带 `domain_uuid` 时保持现状只查全局权限。
- 后端测试命令：`cd app/server && go test ./...`；前端测试命令：`cd app/web && pnpm test`。
- 接口与字段命名沿用现有 snake_case；Go 类型 PascalCase；前端组件名 PascalCase；不加非必要注释、不使用 emoji。

## File Map

Create:
- `app/server/internal/model/domain_role.go`
- `app/server/internal/model/domain_role_test.go`
- `app/server/internal/repository/domain_role_repo.go`
- `app/server/internal/repository/domain_role_repo_test.go`
- `app/server/internal/handler/message_handler_domain_test.go`
- `app/web/src/utils/domainPermissions.ts`
- `app/web/src/utils/domainPermissions.test.ts`
- `app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainRolePanel.tsx`

Modify:
- `app/server/internal/repository/db_migrations.go`
- `app/server/internal/repository/domain_repo.go`
- `app/server/internal/repository/domain_repo_test.go`
- `app/server/internal/service/domain_service.go`
- `app/server/internal/service/domain_service_test.go`
- `app/server/internal/handler/domain_handler.go`
- `app/server/internal/handler/domain_handler_test.go`
- `app/server/internal/handler/room_handler.go`
- `app/server/internal/handler/room_handler_test.go`
- `app/server/internal/handler/message_handler.go`
- `app/server/internal/signal/hub.go`
- `app/server/internal/signal/message_bridge.go`
- `app/server/internal/signal/message_bridge_test.go`
- `app/server/server/gin.go`
- `app/server/internal/router/routes/domain/routes.go`
- `app/web/src/api/domain.ts`
- `app/web/src/api/domainApi.spec.ts`
- `app/web/src/stores/domainStore.ts`
- `app/web/src/stores/domainStore.spec.ts`
- `app/web/src/components/domain/DomainMemberTable.tsx`
- `app/web/src/components/domain/DomainMemberTable.spec.tsx`
- `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`

---

### Task 1: 域角色与域角色权限模型

**Files:**
- Create: `app/server/internal/model/domain_role.go`
- Create: `app/server/internal/model/domain_role_test.go`

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/model/domain_role_test.go`：

```go
package model

import "testing"

func TestAssignableDomainPermissionsAreScoped(t *testing.T) {
	allowed := AssignableDomainPermissionsSet()
	platformOnly := []string{
		PermUserRead, PermRoleRead, PermSFUManage, PermBotManage,
		PermStorageRead, PermOAuthRead, PermClusterRead,
		PermEmailConfigRead, PermPluginRead, PermDomainCreate, PermDomainDelete,
	}
	for _, code := range platformOnly {
		if _, ok := allowed[code]; ok {
			t.Errorf("platform-only permission %q must not be assignable to domain roles", code)
		}
	}
}

func TestDefaultDomainRolePermissionsUseAssignableSet(t *testing.T) {
	allowed := AssignableDomainPermissionsSet()
	for role, codes := range DefaultDomainRolePermissions {
		if IsSystemDomainRole(role) == false {
			t.Errorf("role %q must be system", role)
		}
		if role == DomainRoleOwner {
			t.Fatalf("owner permissions must not be stored, got %d codes", len(codes))
		}
		for _, code := range codes {
			if _, ok := allowed[code]; !ok {
				t.Errorf("role %q uses non-assignable permission %q", role, code)
			}
		}
	}
}

func TestIsSystemDomainRole(t *testing.T) {
	for _, role := range []string{DomainRoleOwner, DomainRoleAdmin, DomainRoleMember, DomainRoleGuest} {
		if !IsSystemDomainRole(role) {
			t.Errorf("expected %q to be system", role)
		}
	}
	if IsSystemDomainRole("moderator") {
		t.Error("custom role must not be system")
	}
}
```

预期：编译失败，`AssignableDomainPermissionsSet`、`DefaultDomainRolePermissions`、`IsSystemDomainRole` 不存在。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/model/ -run DomainRole -v`
Expected: FAIL（类型/函数未定义）

- [ ] **Step 3: 实现模型**

创建 `app/server/internal/model/domain_role.go`：

```go
package model

import "time"

const (
	DomainRoleOwner  = "owner"
	DomainRoleAdmin  = "admin"
	DomainRoleMember = "member"
	DomainRoleGuest  = "guest"
)

// DomainRole 每个域独立角色。
type DomainRole struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DomainUUID string    `gorm:"size:32;uniqueIndex:idx_domain_roles_uuid_name,priority:1" json:"domain_uuid"`
	Name       string    `gorm:"size:32;uniqueIndex:idx_domain_roles_uuid_name,priority:2" json:"name"`
	IsSystem   bool      `gorm:"default:false" json:"is_system"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (DomainRole) TableName() string {
	return "domain_roles"
}

// DomainRolePermission 域角色-权限码关联表。权限码必须属于 AssignableDomainPermissions。
type DomainRolePermission struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	DomainUUID     string `gorm:"size:32;uniqueIndex:idx_domain_role_perms,priority:1" json:"domain_uuid"`
	RoleName       string `gorm:"size:32;uniqueIndex:idx_domain_role_perms,priority:2" json:"role_name"`
	PermissionCode string `gorm:"size:64;uniqueIndex:idx_domain_role_perms,priority:3" json:"permission_code"`
}

func (DomainRolePermission) TableName() string {
	return "domain_role_permissions"
}

// AssignableDomainPermissions 域角色可分配的权限码白名单。
var AssignableDomainPermissions = []string{
	PermDomainManage, PermDomainInvite, PermDomainKick, PermDomainRoleManage,
	PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
	PermMessageSend, PermMessageRead, PermMessageDeleteOthers,
}

func AssignableDomainPermissionsSet() map[string]struct{} {
	set := make(map[string]struct{}, len(AssignableDomainPermissions))
	for _, code := range AssignableDomainPermissions {
		set[code] = struct{}{}
	}
	return set
}

// DefaultDomainRolePermissions 每个域创建时 seed 的系统角色权限。owner 不存权限行。
var DefaultDomainRolePermissions = map[string][]string{
	DomainRoleAdmin: {
		PermDomainManage, PermDomainInvite, PermDomainKick, PermDomainRoleManage,
		PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
		PermMessageSend, PermMessageRead, PermMessageDeleteOthers,
	},
	DomainRoleMember: {
		PermRoomCreate, PermRoomRead, PermMessageSend, PermMessageRead,
	},
	DomainRoleGuest: {
		PermRoomRead, PermMessageRead,
	},
}

func IsSystemDomainRole(name string) bool {
	switch name {
	case DomainRoleOwner, DomainRoleAdmin, DomainRoleMember, DomainRoleGuest:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/model/ -run DomainRole -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/model/domain_role.go app/server/internal/model/domain_role_test.go
git commit -m "feat: add per-domain role models"
```

---

### Task 2: 域角色仓储与默认角色 seed

**Files:**
- Create: `app/server/internal/repository/domain_role_repo.go`
- Create: `app/server/internal/repository/domain_role_repo_test.go`

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/repository/domain_role_repo_test.go`：

```go
package repository

import (
	"testing"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newDomainRoleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Domain{},
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSeedDefaultDomainRoles_CreatesSystemRoles(t *testing.T) {
	db := newDomainRoleTestDB(t)
	domain := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("SeedDefaultDomainRoles: %v", err)
	}

	repo := NewDomainRoleRepository(db)
	roles, err := repo.ListRoles(domain.UUID)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("expected 4 system roles, got %d", len(roles))
	}
	for _, name := range []string{model.DomainRoleAdmin, model.DomainRoleMember, model.DomainRoleGuest} {
		codes, err := repo.GetRolePermissions(domain.UUID, name)
		if err != nil {
			t.Fatalf("GetRolePermissions(%s): %v", name, err)
		}
		if len(codes) == 0 {
			t.Errorf("role %s must have seeded permissions", name)
		}
	}
	ownerCodes, err := repo.GetRolePermissions(domain.UUID, model.DomainRoleOwner)
	if err != nil {
		t.Fatalf("owner permissions: %v", err)
	}
	if len(ownerCodes) != 0 {
		t.Errorf("owner must not have stored permission rows, got %v", ownerCodes)
	}
}

func TestDomainRoleRepository_CreateAndDelete(t *testing.T) {
	db := newDomainRoleTestDB(t)
	repo := NewDomainRoleRepository(db)

	role := &model.DomainRole{DomainUUID: "domain-a", Name: "moderator"}
	if err := repo.CreateRoleWithPermissions(role, []string{model.PermRoomRead, model.PermMessageRead}); err != nil {
		t.Fatalf("CreateRoleWithPermissions: %v", err)
	}
	codes, err := repo.GetRolePermissions("domain-a", "moderator")
	if err != nil {
		t.Fatalf("GetRolePermissions: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("expected 2 permissions, got %v", codes)
	}

	inUse, err := repo.RoleInUse("domain-a", "moderator")
	if err != nil {
		t.Fatalf("RoleInUse: %v", err)
	}
	if inUse {
		t.Fatal("moderator must not be in use")
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: "domain-a", UserUUID: "u-1", RoleName: "moderator",
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	inUse, err = repo.RoleInUse("domain-a", "moderator")
	if err != nil {
		t.Fatalf("RoleInUse after member: %v", err)
	}
	if !inUse {
		t.Fatal("moderator must be in use after member assignment")
	}

	if err := repo.DeleteRole("domain-a", "moderator"); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	if _, err := repo.GetRole("domain-a", "moderator"); err == nil {
		t.Fatal("expected role to be deleted")
	}
}

func TestDomainRoleRepository_SyncPermissions(t *testing.T) {
	db := newDomainRoleTestDB(t)
	repo := NewDomainRoleRepository(db)
	role := &model.DomainRole{DomainUUID: "domain-b", Name: "viewer"}
	if err := repo.CreateRoleWithPermissions(role, []string{model.PermRoomRead}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.SyncRolePermissions("domain-b", "viewer", []string{model.PermMessageRead, model.PermRoomCreate}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	codes, err := repo.GetRolePermissions("domain-b", "viewer")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("expected 2 codes after sync, got %v", codes)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/repository/ -run DomainRole -v`
Expected: FAIL（`DomainRoleRepository`、`SeedDefaultDomainRoles` 未定义）

- [ ] **Step 3: 实现仓储**

创建 `app/server/internal/repository/domain_role_repo.go`：

```go
package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type DomainRoleRepository struct {
	db *gorm.DB
}

func NewDomainRoleRepository(db *gorm.DB) *DomainRoleRepository {
	return &DomainRoleRepository{db: db}
}

func (r *DomainRoleRepository) ListRoles(domainUUID string) ([]model.DomainRole, error) {
	var roles []model.DomainRole
	err := r.db.Where("domain_uuid = ?", domainUUID).Order("id ASC").Find(&roles).Error
	return roles, err
}

func (r *DomainRoleRepository) GetRole(domainUUID, name string) (*model.DomainRole, error) {
	var role model.DomainRole
	err := r.db.Where("domain_uuid = ? AND name = ?", domainUUID, name).First(&role).Error
	return &role, err
}

func (r *DomainRoleRepository) GetRolePermissions(domainUUID, roleName string) ([]string, error) {
	var codes []string
	err := r.db.Model(&model.DomainRolePermission{}).
		Where("domain_uuid = ? AND role_name = ?", domainUUID, roleName).
		Order("permission_code ASC").
		Pluck("permission_code", &codes).Error
	return codes, err
}

func (r *DomainRoleRepository) CreateRoleWithPermissions(role *model.DomainRole, permissions []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		for _, code := range permissions {
			rp := model.DomainRolePermission{
				DomainUUID:     role.DomainUUID,
				RoleName:       role.Name,
				PermissionCode: code,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *DomainRoleRepository) SyncRolePermissions(domainUUID, roleName string, permissions []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_uuid = ? AND role_name = ?", domainUUID, roleName).
			Delete(&model.DomainRolePermission{}).Error; err != nil {
			return err
		}
		for _, code := range permissions {
			rp := model.DomainRolePermission{
				DomainUUID:     domainUUID,
				RoleName:       roleName,
				PermissionCode: code,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *DomainRoleRepository) DeleteRole(domainUUID, name string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("domain_uuid = ? AND role_name = ?", domainUUID, name).
			Delete(&model.DomainRolePermission{}).Error; err != nil {
			return err
		}
		return tx.Where("domain_uuid = ? AND name = ?", domainUUID, name).
			Delete(&model.DomainRole{}).Error
	})
}

func (r *DomainRoleRepository) RoleInUse(domainUUID, name string) (bool, error) {
	var count int64
	err := r.db.Model(&model.DomainMember{}).
		Where("domain_uuid = ? AND role_name = ?", domainUUID, name).
		Count(&count).Error
	return count > 0, err
}

// SeedDefaultDomainRoles 为域创建系统角色；重复调用幂等。owner 不存权限行。
func SeedDefaultDomainRoles(db *gorm.DB, domainUUID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, name := range []string{
			model.DomainRoleOwner,
			model.DomainRoleAdmin,
			model.DomainRoleMember,
			model.DomainRoleGuest,
		} {
			var count int64
			if err := tx.Model(&model.DomainRole{}).
				Where("domain_uuid = ? AND name = ?", domainUUID, name).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			role := model.DomainRole{DomainUUID: domainUUID, Name: name, IsSystem: true}
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
			for _, code := range model.DefaultDomainRolePermissions[name] {
				rp := model.DomainRolePermission{
					DomainUUID:     domainUUID,
					RoleName:       name,
					PermissionCode: code,
				}
				if err := tx.Create(&rp).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/repository/ -run DomainRole -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/repository/domain_role_repo.go app/server/internal/repository/domain_role_repo_test.go
git commit -m "feat: add domain role repository and seed"
```

---

### Task 3: 注册迁移并在创建域/默认域时 seed 系统角色

**Files:**
- Modify: `app/server/internal/repository/db_migrations.go:238-260`
- Modify: `app/server/internal/repository/db_migrations.go:328-380`
- Modify: `app/server/internal/repository/domain_repo.go:22-34`
- Modify: `app/server/internal/repository/domain_repo_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/repository/domain_repo_test.go` 末尾追加：

```go
func TestCreateWithOwner_SeedsDefaultDomainRoles(t *testing.T) {
	db := newDomainTestDB(t)
	if err := db.AutoMigrate(&model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate roles: %v", err)
	}
	repo := NewDomainRepository(db)
	domain := &model.Domain{Name: "Roles", OwnerUUID: "owner-1"}
	owner := &model.DomainMember{DomainUUID: "", UserUUID: "owner-1", RoleName: model.DomainRoleOwner}
	if err := repo.CreateWithOwner(domain, owner); err != nil {
		t.Fatalf("CreateWithOwner: %v", err)
	}

	roleRepo := NewDomainRoleRepository(db)
	roles, err := roleRepo.ListRoles(domain.UUID)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("expected 4 seeded roles, got %d", len(roles))
	}
}
```

`newDomainTestDB` 的 AutoMigrate 未包含新表，但测试中显式补充迁移；当前 `CreateWithOwner` 不 seed 角色，测试会失败（0 个角色）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/repository/ -run TestCreateWithOwner_SeedsDefaultDomainRoles -v`
Expected: FAIL（expected 4 seeded roles, got 0）

- [ ] **Step 3: 实现迁移与 seed**

在 `app/server/internal/repository/db_migrations.go` 的 `autoMigrate()` 中，把 `&model.DomainMember{},` 之后追加两行：

```go
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
```

在 `migrateDefaultDomain` 的末尾 `return nil` 前追加（`defaultDomain.UUID` 在该处必然已确定）：

```go
	if err := SeedDefaultDomainRoles(db, defaultDomain.UUID); err != nil {
		return fmt.Errorf("seed default domain roles: %w", err)
	}
	return nil
```

修改 `app/server/internal/repository/domain_repo.go` 的 `CreateWithOwner`：

```go
func (r *DomainRepository) CreateWithOwner(domain *model.Domain, owner *model.DomainMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(domain).Error; err != nil {
			return err
		}
		if owner != nil && owner.DomainUUID == "" {
			owner.DomainUUID = domain.UUID
		}
		if err := tx.Create(owner).Error; err != nil {
			return err
		}
		return SeedDefaultDomainRoles(tx, domain.UUID)
	})
}
```

在 `app/server/internal/repository/domain_repo_test.go` 的 `newDomainTestDB` 中把 AutoMigrate 列表改为包含新表：

```go
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.User{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/repository/ -run 'TestCreateWithOwner|TestDomainRepo' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/repository/db_migrations.go app/server/internal/repository/domain_repo.go app/server/internal/repository/domain_repo_test.go
git commit -m "feat: migrate and seed per-domain roles"
```

---

### Task 4: DomainService 域角色能力

**Files:**
- Modify: `app/server/internal/service/domain_service.go`
- Modify: `app/server/internal/service/domain_service_test.go`
- Modify: `app/server/server/gin.go:119-120`
- Modify: `app/server/internal/handler/domain_handler_test.go:22-34`
- Modify: `app/server/internal/handler/room_handler_test.go:315-330`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/service/domain_service_test.go` 末尾追加（同时把文件内所有 `NewDomainService(repo)` 改为 `NewDomainService(repo, roleRepo)`，`roleRepo` 由 `repository.NewDomainRoleRepository(db)` 创建）：

```go
func TestDomainService_HasDomainPermission(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Perm", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "owner-1", RoleName: model.DomainRoleOwner,
	}).Error; err != nil {
		t.Fatalf("seed owner member: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "admin-1", RoleName: model.DomainRoleAdmin,
	}).Error; err != nil {
		t.Fatalf("seed admin member: %v", err)
	}
	if !svc.HasDomainPermission(domain.UUID, "owner-1", model.PermRoomDelete) {
		t.Error("owner must have room:delete")
	}
	if !svc.HasDomainPermission(domain.UUID, "admin-1", model.PermDomainKick) {
		t.Error("admin must have domain:kick by default")
	}
	if svc.HasDomainPermission(domain.UUID, "admin-1", model.PermRoomDelete) {
		t.Error("admin must not have room:delete without explicit grant")
	}
}

func TestDomainService_CreateAndDeleteCustomRole(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Custom", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := svc.CreateDomainRole(domain.UUID, "moderator", []string{model.PermRoomRead}); err != nil {
		t.Fatalf("CreateDomainRole: %v", err)
	}
	codes, err := svc.GetDomainRolePermissions(domain.UUID, "moderator")
	if err != nil {
		t.Fatalf("GetDomainRolePermissions: %v", err)
	}
	if len(codes) != 1 {
		t.Fatalf("expected 1 permission, got %v", codes)
	}
	if err := svc.CreateDomainRole(domain.UUID, "admin", nil); err == nil {
		t.Fatal("must reject creating system role name")
	}
	if err := svc.DeleteDomainRole(domain.UUID, "admin"); err == nil {
		t.Fatal("must reject deleting system role")
	}
	if err := svc.DeleteDomainRole(domain.UUID, "moderator"); err != nil {
		t.Fatalf("DeleteDomainRole: %v", err)
	}
}

func TestDomainService_SetMemberRole(t *testing.T) {
	svc, db := setupDomainServiceTestDB(t)
	domain := &model.Domain{Name: "Members", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "u-2", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := svc.SetMemberRole(domain.UUID, "owner-1", "u-2", model.DomainRoleAdmin); err != nil {
		t.Fatalf("SetMemberRole: %v", err)
	}
	member, err := repository.NewDomainRepository(db).GetMember(domain.UUID, "u-2")
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if member.RoleName != model.DomainRoleAdmin {
		t.Fatalf("expected admin, got %s", member.RoleName)
	}
	if err := svc.SetMemberRole(domain.UUID, "u-2", "u-2", model.DomainRoleGuest); err == nil {
		t.Fatal("must reject changing own role")
	}
	if err := svc.SetMemberRole(domain.UUID, "owner-1", "u-2", model.DomainRoleOwner); err == nil {
		t.Fatal("must reject assigning owner role")
	}
}
```

修改 `domain_service_test.go` 现有的 `setupDomainServiceTestDB`（签名保持不变，避免改动既有调用）：

```go
func setupDomainServiceTestDB(t *testing.T) (*DomainService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewDomainRepository(db)
	roleRepo := repository.NewDomainRoleRepository(db)
	svc := NewDomainService(repo, roleRepo)
	return svc, db
}
```

新测试使用 `svc, db := setupDomainServiceTestDB(t)`；需要 `repo` 时用 `repository.NewDomainRepository(db)`，需要 `roleRepo` 时用 `repository.NewDomainRoleRepository(db)`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/service/ -run 'TestDomainService_HasDomainPermission|TestDomainService_CreateAndDeleteCustomRole|TestDomainService_SetMemberRole' -v`
Expected: FAIL（新方法未定义 + 构造函数签名不匹配）

- [ ] **Step 3: 实现 DomainService**

修改 `app/server/internal/service/domain_service.go`：

替换常量块：

```go
const (
	DomainRoleOwner  = model.DomainRoleOwner
	DomainRoleAdmin  = model.DomainRoleAdmin
	DomainRoleMember = model.DomainRoleMember
	DomainRoleGuest  = model.DomainRoleGuest
)
```

新增错误常量（放在 `ErrAlreadyMember` 后）：

```go
var ErrDomainRoleNotFound = pkg.NewAppError(pkg.NOT_FOUND, "domain role not found")
```

替换 struct 与构造函数：

```go
type DomainService struct {
	domainRepo *repository.DomainRepository
	roleRepo   *repository.DomainRoleRepository
}

func NewDomainService(domainRepo *repository.DomainRepository, roleRepo *repository.DomainRoleRepository) *DomainService {
	return &DomainService{domainRepo: domainRepo, roleRepo: roleRepo}
}
```

在文件末尾追加新方法：

```go
func (s *DomainService) HasDomainPermission(domainUUID, userUUID, permCode string) bool {
	member, err := s.domainRepo.GetMember(domainUUID, userUUID)
	if err != nil {
		return false
	}
	if member.RoleName == model.DomainRoleOwner {
		_, ok := model.AssignableDomainPermissionsSet()[permCode]
		return ok
	}
	codes, err := s.roleRepo.GetRolePermissions(domainUUID, member.RoleName)
	if err != nil {
		return false
	}
	for _, code := range codes {
		if code == permCode {
			return true
		}
	}
	return false
}

func (s *DomainService) ListDomainRoles(domainUUID string) ([]model.DomainRole, error) {
	return s.roleRepo.ListRoles(domainUUID)
}

func (s *DomainService) GetDomainRolePermissions(domainUUID, roleName string) ([]string, error) {
	if roleName == model.DomainRoleOwner {
		return append([]string(nil), model.AssignableDomainPermissions...), nil
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainRoleNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.roleRepo.GetRolePermissions(domainUUID, roleName)
}

func (s *DomainService) CreateDomainRole(domainUUID, name string, permissions []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "role name is required")
	}
	if len(name) > 32 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "role name too long")
	}
	if model.IsSystemDomainRole(name) {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot create system role")
	}
	if err := validateDomainPermissions(permissions); err != nil {
		return err
	}
	if _, err := s.roleRepo.GetRole(domainUUID, name); err == nil {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "domain role already exists")
	} else if err != gorm.ErrRecordNotFound {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	role := &model.DomainRole{DomainUUID: domainUUID, Name: name}
	if err := s.roleRepo.CreateRoleWithPermissions(role, permissions); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) UpdateDomainRolePermissions(domainUUID, roleName string, permissions []string) error {
	if roleName == model.DomainRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot modify owner role")
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainRoleNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := validateDomainPermissions(permissions); err != nil {
		return err
	}
	if err := s.roleRepo.SyncRolePermissions(domainUUID, roleName, permissions); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) DeleteDomainRole(domainUUID, roleName string) error {
	if model.IsSystemDomainRole(roleName) {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot delete system role")
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainRoleNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	inUse, err := s.roleRepo.RoleInUse(domainUUID, roleName)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if inUse {
		return pkg.NewAppError(pkg.ALREADY_EXISTS, "role is assigned to members")
	}
	if err := s.roleRepo.DeleteRole(domainUUID, roleName); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) SetMemberRole(domainUUID, operatorUUID, targetUserUUID, roleName string) error {
	if targetUserUUID == operatorUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot change your own role")
	}
	if roleName == model.DomainRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "owner role cannot be assigned")
	}
	target, err := s.domainRepo.GetMember(domainUUID, targetUserUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainMemberNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if target.RoleName == model.DomainRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot change owner role")
	}
	if _, err := s.roleRepo.GetRole(domainUUID, roleName); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDomainRoleNotFound
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	target.RoleName = roleName
	if err := s.domainRepo.UpdateMember(target); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *DomainService) MyDomainPermissions(domainUUID, userUUID string) (string, []string, error) {
	member, err := s.domainRepo.GetMember(domainUUID, userUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil, ErrDomainMemberNotFound
		}
		return "", nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	codes, err := s.GetDomainRolePermissions(domainUUID, member.RoleName)
	if err != nil {
		return "", nil, err
	}
	return member.RoleName, codes, nil
}

func validateDomainPermissions(codes []string) error {
	allowed := model.AssignableDomainPermissionsSet()
	for _, code := range codes {
		if _, ok := allowed[code]; !ok {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid domain permission: "+code)
		}
	}
	return nil
}
```

- [ ] **Step 4: 更新所有构造函数调用点**

`app/server/server/gin.go:119-120`：

```go
	domainRepo := repository.NewDomainRepository(db)
	domainRoleRepo := repository.NewDomainRoleRepository(db)
	domainSvc := service.NewDomainService(domainRepo, domainRoleRepo)
```

`app/server/internal/handler/domain_handler_test.go:22-34` 的 `setupDomainHandlerTestDB` 返回签名改为 `(*gorm.DB, *service.DomainService)` 保持，内部改为：

```go
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.Room{}, &model.User{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	domainRepo := repository.NewDomainRepository(db)
	domainRoleRepo := repository.NewDomainRoleRepository(db)
	domainSvc := service.NewDomainService(domainRepo, domainRoleRepo)
```

`app/server/internal/handler/room_handler_test.go:315-330` 中：

```go
		if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		domainRoleRepo := repository.NewDomainRoleRepository(db)
		domainSvc := service.NewDomainService(repository.NewDomainRepository(db), domainRoleRepo)
		if err := repository.SeedDefaultDomainRoles(db, g.UUID); err != nil {
			t.Fatalf("seed roles: %v", err)
		}
```

其余 `NewDomainService(...)` 调用点按相同模式补齐（`rg -n "NewDomainService\\(" app/server` 逐个核对，当前仅有上述三处 + service 测试）。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/service/ ./internal/handler/ -run 'TestDomainService|TestRoomHandler_Delete_RequiresManagePermission|TestDomainHandler' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/service/domain_service.go app/server/internal/service/domain_service_test.go app/server/server/gin.go app/server/internal/handler/domain_handler_test.go app/server/internal/handler/room_handler_test.go
git commit -m "feat: add per-domain role and permission service"
```

---

### Task 5: DomainHandler 角色/权限 API 与路由

**Files:**
- Modify: `app/server/internal/handler/domain_handler.go`
- Modify: `app/server/internal/router/routes/domain/routes.go`
- Modify: `app/server/internal/handler/domain_handler_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/domain_handler_test.go` 末尾追加：

```go
func TestDomainHandler_RoleManagement_OwnerSuccess(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Role API", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/roles/list",
		`{"domain_uuid":"`+domain.UUID+`"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("list roles: got %d, body %s", resp.Code, resp.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", body["code"])
	}

	resp = postDomainJSON(t, router, "/api/v1/domain/roles/create",
		`{"domain_uuid":"`+domain.UUID+`","name":"moderator","permissions":["room:read"]}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("create role: got %d, body %s", resp.Code, resp.Body.String())
	}

	resp = postDomainJSON(t, router, "/api/v1/domain/members/update-role",
		`{"domain_uuid":"`+domain.UUID+`","user_uuid":"member-1","role_name":"moderator"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("update member role: got %d, body %s", resp.Code, resp.Body.String())
	}
}

func TestDomainHandler_RoleManagement_NonManagerForbidden(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Role Denied", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "member-1", RoleName: model.DomainRoleMember,
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/roles/create",
		`{"domain_uuid":"`+domain.UUID+`","name":"hacker","permissions":[]}`,
		map[string]string{"X-User-UUID": "member-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected http status %d", resp.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"].(float64) != 1013 {
		t.Fatalf("expected FORBIDDEN 1013, got %v", body["code"])
	}
}

func TestDomainHandler_MyPermissions(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "My Perms", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "owner-1", RoleName: model.DomainRoleOwner,
	}).Error; err != nil {
		t.Fatalf("seed owner member: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/my-permissions",
		`{"domain_uuid":"`+domain.UUID+`"}`,
		map[string]string{"X-User-UUID": "owner-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("my permissions: got %d, body %s", resp.Code, resp.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", body["code"])
	}
}
```

测试中 `setupDomainHandlerRouter` 目前没有注册新路由，预期新接口 404；先按 Task 4 已更新构造。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run 'TestDomainHandler_RoleManagement|TestDomainHandler_MyPermissions' -v`
Expected: FAIL（404 或方法未定义）

- [ ] **Step 3: 实现 handler 方法**

在 `app/server/internal/handler/domain_handler.go` 的 `Members` 方法后追加：

```go
type DomainRoleListRequest struct {
}

type CreateDomainRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions"`
}

type UpdateDomainRoleRequest struct {
	RoleName    string   `json:"role_name" binding:"required"`
	Permissions []string `json:"permissions"`
}

type DeleteDomainRoleRequest struct {
	RoleName string `json:"role_name" binding:"required"`
}

type UpdateMemberRoleRequest struct {
	UserUUID string `json:"user_uuid" binding:"required"`
	RoleName string `json:"role_name" binding:"required"`
}

func (h *DomainHandler) canManageDomainRoles(c *gin.Context, domainUUID, userUUID string) bool {
	if h.domainSvc.IsOwner(domainUUID, userUUID) {
		return true
	}
	if h.hasPermission(c, permcode.PermDomainRoleManage) {
		return true
	}
	return h.domainSvc.HasDomainPermission(domainUUID, userUUID, permcode.PermDomainRoleManage)
}

func (h *DomainHandler) ListRoles(c *gin.Context) {
	domainUUID := domainUUIDFromContext(c)
	userUUID := currentUserUUID(c)
	if domainUUID == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	if !h.canManageDomainRoles(c, domainUUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role permission")
		return
	}
	roles, err := h.domainSvc.ListDomainRoles(domainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	views := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		codes, err := h.domainSvc.GetDomainRolePermissions(domainUUID, role.Name)
		if err != nil {
			pkg.HandleError(c, err)
			return
		}
		views = append(views, gin.H{
			"name":        role.Name,
			"is_system":   role.IsSystem,
			"permissions": codes,
		})
	}
	pkg.Success(c, gin.H{
		"roles":      views,
		"assignable": model.AssignableDomainPermissions,
	})
}

func (h *DomainHandler) CreateRole(c *gin.Context) {
	var req CreateDomainRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domainUUID := domainUUIDFromContext(c)
	userUUID := currentUserUUID(c)
	if domainUUID == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	if !h.canManageDomainRoles(c, domainUUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role permission")
		return
	}
	if err := h.domainSvc.CreateDomainRole(domainUUID, req.Name, req.Permissions); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *DomainHandler) UpdateRole(c *gin.Context) {
	var req UpdateDomainRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domainUUID := domainUUIDFromContext(c)
	userUUID := currentUserUUID(c)
	if domainUUID == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	if !h.canManageDomainRoles(c, domainUUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role permission")
		return
	}
	if err := h.domainSvc.UpdateDomainRolePermissions(domainUUID, req.RoleName, req.Permissions); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *DomainHandler) DeleteRole(c *gin.Context) {
	var req DeleteDomainRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domainUUID := domainUUIDFromContext(c)
	userUUID := currentUserUUID(c)
	if domainUUID == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	if !h.canManageDomainRoles(c, domainUUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role permission")
		return
	}
	if err := h.domainSvc.DeleteDomainRole(domainUUID, req.RoleName); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *DomainHandler) UpdateMemberRole(c *gin.Context) {
	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	domainUUID := domainUUIDFromContext(c)
	userUUID := currentUserUUID(c)
	if domainUUID == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	if !h.canManageDomainRoles(c, domainUUID, userUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role permission")
		return
	}
	if err := h.domainSvc.SetMemberRole(domainUUID, userUUID, req.UserUUID, req.RoleName); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *DomainHandler) MyPermissions(c *gin.Context) {
	domainUUID := domainUUIDFromContext(c)
	userUUID := currentUserUUID(c)
	if domainUUID == "" || userUUID == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "domain_uuid is required")
		return
	}
	roleName, codes, err := h.domainSvc.MyDomainPermissions(domainUUID, userUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"role_name": roleName, "permissions": codes})
}
```

在 `app/server/internal/handler/domain_handler.go` 的 import 块中新增 `"GOSpeak/internal/model"`；`currentUserUUID` 在 `room_handler.go` 已有同包函数，直接复用。

- [ ] **Step 4: 注册路由**

`app/server/internal/router/routes/domain/routes.go` 在 `r.POST("/members", ...)` 后追加：

```go
	r.POST("/roles/list", middleware.RequireDomainMember(), h.ListRoles)
	r.POST("/roles/create", middleware.RequireDomainMember(), h.CreateRole)
	r.POST("/roles/update", middleware.RequireDomainMember(), h.UpdateRole)
	r.POST("/roles/delete", middleware.RequireDomainMember(), h.DeleteRole)
	r.POST("/members/update-role", middleware.RequireDomainMember(), h.UpdateMemberRole)
	r.POST("/my-permissions", middleware.RequireDomainMember(), h.MyPermissions)
```

同步更新 `setupDomainHandlerRouter` 测试 helper 注册上述路由。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ -run 'TestDomainHandler' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/handler/domain_handler.go app/server/internal/handler/domain_handler_test.go app/server/internal/router/routes/domain/routes.go
git commit -m "feat: add domain role management APIs"
```

---

### Task 6: 现有域管理/房间判定接入域权限

**Files:**
- Modify: `app/server/internal/handler/domain_handler.go`
- Modify: `app/server/internal/handler/room_handler.go`
- Modify: `app/server/internal/handler/room_handler_test.go`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/room_handler_test.go` 的 `TestRoomHandler_Delete_RequiresManagePermission` 场景内，追加一个自定义域角色用例（复用该测试的 `newEnv` 结构，新增一个辅助构造）：

```go
func TestRoomHandler_Delete_AllowsDomainRolePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, g.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: "moderator-1", RoleName: "moderator"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		&model.DomainRole{DomainUUID: g.UUID, Name: "moderator"},
		[]string{model.PermRoomDelete},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	room := model.Room{Name: "lobby", DomainUUID: g.UUID, CreatedBy: "someone-else"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}

	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "moderator-1")
		c.Set("user_uuid", "moderator-1")
		c.Set("role", "user")
		c.Next()
	})
	h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
	r.POST("/api/v1/room/delete", h.Delete)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/room/delete", strings.NewReader(`{"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", w.Code, w.Body.String())
	}
}
```

同时新增：

```go
func TestDomainHandler_Update_AllowsDomainRolePermission(t *testing.T) {
	db, domainSvc := setupDomainHandlerTestDB(t)
	domain := &model.Domain{Name: "Update Role", OwnerUUID: "owner-1"}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if err := repository.SeedDefaultDomainRoles(db, domain.UUID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if err := db.Create(&model.DomainMember{
		DomainUUID: domain.UUID, UserUUID: "manager-1", RoleName: "manager",
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		&model.DomainRole{DomainUUID: domain.UUID, Name: "manager"},
		[]string{model.PermDomainManage},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	router := setupDomainHandlerRouter(t, domainSvc)

	resp := postDomainJSON(t, router, "/api/v1/domain/update",
		`{"domain_uuid":"`+domain.UUID+`","name":"Renamed"}`,
		map[string]string{"X-User-UUID": "manager-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", resp.Code, resp.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run 'TestRoomHandler_Delete_AllowsDomainRolePermission|TestDomainHandler_Update_AllowsDomainRolePermission' -v`
Expected: FAIL（当前实现只认全局权限或固定 admin 域角色/owner）

- [ ] **Step 3: 改造判定**

`app/server/internal/handler/domain_handler.go` 的 `Update` 中，替换权限判断：

```go
	if !h.hasPermission(c, permcode.PermDomainManage) &&
		!h.domainSvc.IsOwner(domainUUID, userUUID) &&
		!h.domainSvc.HasDomainPermission(domainUUID, userUUID, permcode.PermDomainManage) {
		pkg.Fail(c, pkg.FORBIDDEN, "not domain owner or missing permission")
		return
	}
```

`Kick` 中替换：

```go
	permOK := h.hasPermission(c, permcode.PermDomainKick)
	roleOK := h.domainSvc.HasDomainPermission(domainUUID, userUUID, permcode.PermDomainKick)
	if !permOK && !roleOK {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient domain role or permission")
		return
	}
```

`app/server/internal/handler/room_handler.go` 的 `canManageRoom` 中替换：

```go
	if room.DomainUUID != "" && h.domainSvc != nil &&
		h.domainSvc.HasDomainPermission(room.DomainUUID, currentUserUUID(c), perm) {
		return true
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ -run 'TestRoomHandler|TestDomainHandler' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/server/internal/handler/domain_handler.go app/server/internal/handler/room_handler.go app/server/internal/handler/room_handler_test.go
git commit -m "feat: enforce domain role permissions on domain and room management"
```

---

### Task 7: 消息删除他人消息的域权限判定

**Files:**
- Create: `app/server/internal/handler/message_handler_domain_test.go`
- Modify: `app/server/internal/handler/message_handler.go`
- Modify: `app/server/server/gin.go:395`
- Modify: `app/server/internal/signal/hub.go`
- Modify: `app/server/internal/signal/message_bridge.go`
- Modify: `app/server/internal/signal/message_bridge_test.go`

- [ ] **Step 1: 写失败测试**

创建 `app/server/internal/handler/message_handler_domain_test.go`：

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMessageHandler_Delete_AllowsDomainRolePermission(t *testing.T) {
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
	if err := db.Create(&model.DomainMember{DomainUUID: domain.UUID, UserUUID: "mod-1", RoleName: "moderator"}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := repository.NewDomainRoleRepository(db).CreateRoleWithPermissions(
		&model.DomainRole{DomainUUID: domain.UUID, Name: "moderator"},
		[]string{model.PermMessageDeleteOthers},
	); err != nil {
		t.Fatalf("create role: %v", err)
	}
	room := &model.Room{Name: "chat", DomainUUID: domain.UUID, Type: model.RoomTypeText}
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
	msg := &model.Message{
		RoomUUID: room.UUID, AuthorID: "other", AuthorUUID: "other-uuid",
		Content: "hello", ConversationType: model.ConversationTypeRoom,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	roomSvc := service.NewRoomService(repository.NewRoomRepository(db))
	domainSvc := service.NewDomainService(repository.NewDomainRepository(db), repository.NewDomainRoleRepository(db))
	h := NewMessageHandler(service.NewMessageService(repository.NewMessageRepository(db), repository.NewRoomRepository(db), domainSvc), nil, roomSvc, domainSvc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "mod-1")
		c.Set("user_uuid", "mod-1")
		c.Set("role", "user")
		c.Next()
	})
	r.POST("/delete", h.Delete)

	body, _ := json.Marshal(map[string]string{"room_uuid": room.UUID, "message_uuid": msg.UUID})
	req := httptest.NewRequest(http.MethodPost, "/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", w.Code, w.Body.String())
	}
}
```

在 `app/server/internal/signal/message_bridge_test.go` 末尾追加：

```go
func TestOnMessageDelete_DomainRoleCanDeleteOthers(t *testing.T) {
	store := &mockRoomStore{rooms: []model.Room{
		{UUID: "uuid-domain", Name: "text-chat", Type: model.RoomTypeText, DomainUUID: "domain-a"},
	}}
	h := newTestHub()
	h.roomStore = store
	h.permChecker = &mockPermChecker{rolePerms: map[string]map[string]bool{"user": {}}}
	h.domainPermChecker = func(domainUUID, userUUID, permCode string) bool {
		return domainUUID == "domain-a" && userUUID == "mod-1" && permCode == "message:delete_others"
	}
	msgSvc := &mockMessageSvc{}
	h.SetMessageService(msgSvc)

	conn := newMockClient("conn-1")
	conn.claims = &pkg.Claims{Username: "mod-1", UserUUID: "mod-1", Role: "user"}
	if _, err := h.OnRoomJoin(conn, `{"room":"text-chat","identity":"mod-1","domain_uuid":"domain-a"}`); err != nil {
		t.Fatalf("join: %v", err)
	}
	ack, err := h.OnMessageDelete(conn, `{"room":"text-chat","domain_uuid":"domain-a","message_uuid":"msg-1"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ackMap := decodeAck(t, ack)
	if ackMap["success"] != true {
		t.Fatalf("expected success, got %v", ackMap)
	}
	if msgSvc.lastCanDeleteOthers != true {
		t.Fatalf("expected canDeleteOthers true, got %v", msgSvc.lastCanDeleteOthers)
	}
}
```

若 `mockMessageSvc` 没有 `lastCanDeleteOthers` 字段，先在 mock struct 中添加并在 `Delete` 中记录。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ ./internal/signal/ -run 'TestMessageHandler_Delete_AllowsDomainRolePermission|TestOnMessageDelete_DomainRoleCanDeleteOthers' -v`
Expected: FAIL（MessageHandler 无 roomSvc/domainSvc 参数；Hub 无 `domainPermChecker`）

- [ ] **Step 3: 实现 HTTP MessageHandler 域权限**

`app/server/internal/handler/message_handler.go`：

```go
type MessageHandler struct {
	msgSvc    *service.MessageService
	permSvc   *service.PermissionService
	roomSvc   *service.RoomService
	domainSvc *service.DomainService
}

func NewMessageHandler(msgSvc *service.MessageService, permSvc *service.PermissionService, roomSvc *service.RoomService, domainSvc *service.DomainService) *MessageHandler {
	return &MessageHandler{msgSvc: msgSvc, permSvc: permSvc, roomSvc: roomSvc, domainSvc: domainSvc}
}
```

`Delete` 中的权限计算替换为：

```go
	canDeleteOthers := h.permSvc != nil && h.permSvc.HasPermission(roleStr, permcode.PermMessageDeleteOthers)
	if !canDeleteOthers && h.roomSvc != nil && h.domainSvc != nil {
		if room, roomErr := h.roomSvc.GetByUUID(req.RoomUUID); roomErr == nil && room != nil && room.DomainUUID != "" {
			canDeleteOthers = h.domainSvc.HasDomainPermission(room.DomainUUID, currentUserUUID(c), permcode.PermMessageDeleteOthers)
		}
	}
```

`app/server/server/gin.go:395`：

```go
	msgH := handler.NewMessageHandler(messageSvc, permSvc, roomSvc, domainSvc)
```

- [ ] **Step 4: 实现 Signal Hub 域权限判定**

`app/server/internal/signal/hub.go`：

在 Hub struct 的 `domainChecker` 字段后新增：

```go
	domainPermChecker       func(domainUUID, userUUID, permCode string) bool
```

在 `HubOptions` 中新增：

```go
	DomainPermissionChecker func(domainUUID, userUUID, permCode string) bool
```

在 `NewHubWithOptions` 中 `h.domainChecker = opts.DomainChecker` 后新增：

```go
	h.domainPermChecker = opts.DomainPermissionChecker
```

新增 setter（放在 `SetDomainChecker` 后）：

```go
func (h *Hub) SetDomainPermissionChecker(checker func(domainUUID, userUUID, permCode string) bool) {
	h.domainPermChecker = checker
}
```

`app/server/internal/signal/message_bridge.go` 的 `OnMessageDelete` 中替换：

```go
	canDeleteOthers := false
	if claims := c.Claims(); claims != nil {
		if h.permChecker != nil && h.permChecker.HasPermission(claims.Role, permcode.PermMessageDeleteOthers) {
			canDeleteOthers = true
		} else if h.domainPermChecker != nil && req.DomainUUID != "" && claims.UserUUID != "" {
			canDeleteOthers = h.domainPermChecker(req.DomainUUID, claims.UserUUID, permcode.PermMessageDeleteOthers)
		}
	}
```

`app/server/server/gin.go` 的 `HubOptions` 中新增：

```go
		DomainPermissionChecker: domainSvc.HasDomainPermission,
```

`app/server/internal/signal/hub_test.go` 的 `newTestHub` 不设置 checker，默认 nil 不改变现有行为。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ ./internal/signal/ ./internal/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/handler/message_handler.go app/server/internal/handler/message_handler_domain_test.go app/server/server/gin.go app/server/internal/signal/hub.go app/server/internal/signal/message_bridge.go app/server/internal/signal/message_bridge_test.go
git commit -m "feat: enforce domain permissions on message deletion"
```

---

### Task 8: 前端 API 与 store 支持

**Files:**
- Modify: `app/web/src/api/domain.ts`
- Modify: `app/web/src/api/domainApi.spec.ts`
- Modify: `app/web/src/stores/domainStore.ts`
- Modify: `app/web/src/stores/domainStore.spec.ts`
- Create: `app/web/src/utils/domainPermissions.ts`

- [ ] **Step 1: 写失败测试**

在 `app/web/src/api/domainApi.spec.ts` 末尾追加：

```ts
describe("domainRoleApi", () => {
	it("listDomainRoles calls correct endpoint", async () => {
		const data = { roles: [], assignable: ["room:read"] };
		(apiClient.post as any).mockResolvedValue(data);
		const result = await listDomainRoles("g-1");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/roles/list",
			data: { domain_uuid: "g-1" },
		});
		expect(result.assignable).toEqual(["room:read"]);
	});

	it("createDomainRole sends name and permissions", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await createDomainRole("g-1", "moderator", ["room:read"]);
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/roles/create",
			data: { domain_uuid: "g-1", name: "moderator", permissions: ["room:read"] },
		});
	});

	it("updateDomainMemberRole sends role name", async () => {
		(apiClient.post as any).mockResolvedValue(null);
		await updateDomainMemberRole("g-1", "u-2", "admin");
		expect(apiClient.post).toHaveBeenCalledWith({
			url: "/api/v1/domain/members/update-role",
			data: { domain_uuid: "g-1", user_uuid: "u-2", role_name: "admin" },
		});
	});

	it("myDomainPermissions returns role and codes", async () => {
		const data = { role_name: "admin", permissions: ["room:read"] };
		(apiClient.post as any).mockResolvedValue(data);
		const result = await myDomainPermissions("g-1");
		expect(result.role_name).toBe("admin");
	});
});
```

在 `app/web/src/stores/domainStore.spec.ts` 末尾追加：

```ts
describe("domainStore permissions", () => {
	it("loads and caches my domain permissions", async () => {
		const store = createDomainStore();
		myDomainPermissionsMock.mockResolvedValue({
			role_name: "admin",
			permissions: ["room:read", "room:delete"],
		});
		await store.loadMyPermissions("g-1");
		expect(store.state.myRolePermissions["g-1"]).toEqual(["room:read", "room:delete"]);
	});
});
```

同步在 spec 顶部 hoisted mock 中加入 `myDomainPermissionsMock` 并注册到 `vi.mock`。

创建 `app/web/src/utils/domainPermissions.test.ts`：

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";

vi.mock("@/stores/domainStore", () => ({
	default: {
		state: {
			myRolePermissions: {
				"g-1": ["room:delete"],
			},
		},
	},
}));

vi.mock("@/utils/permissions", () => ({
	hasPermission: (code: string) => code === "domain:manage",
}));

import { hasDomainPermission } from "./domainPermissions";

describe("hasDomainPermission", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("falls through to global permission", () => {
		expect(hasDomainPermission("g-1", "domain:manage")).toBe(true);
	});

	it("checks cached domain role permissions", () => {
		expect(hasDomainPermission("g-1", "room:delete")).toBe(true);
		expect(hasDomainPermission("g-1", "room:read")).toBe(false);
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm test -- --run domainApi.spec.ts domainStore.spec.ts domainPermissions.test.ts`
Expected: FAIL（API 函数、store 方法、`hasDomainPermission` 未定义）

- [ ] **Step 3: 实现 API**

`app/web/src/api/domain.ts` 在 `domainMembers` 后追加：

```ts
export interface DomainRole {
	name: string;
	is_system: boolean;
	permissions: string[];
}

export interface DomainRoleList {
	roles: DomainRole[];
	assignable: string[];
}

export async function listDomainRoles(domainUUID: string): Promise<DomainRoleList> {
	const result = await apiClient.post<DomainRoleList>({
		url: "/api/v1/domain/roles/list",
		data: { domain_uuid: domainUUID },
	});
	if (!result) throw new Error("domain role data is missing");
	return result;
}

export async function createDomainRole(
	domainUUID: string,
	name: string,
	permissions: string[],
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/roles/create",
		data: { domain_uuid: domainUUID, name, permissions },
	});
}

export async function updateDomainRolePermissions(
	domainUUID: string,
	roleName: string,
	permissions: string[],
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/roles/update",
		data: { domain_uuid: domainUUID, role_name: roleName, permissions },
	});
}

export async function deleteDomainRole(
	domainUUID: string,
	roleName: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/roles/delete",
		data: { domain_uuid: domainUUID, role_name: roleName },
	});
}

export async function updateDomainMemberRole(
	domainUUID: string,
	userUUID: string,
	roleName: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/domain/members/update-role",
		data: { domain_uuid: domainUUID, user_uuid: userUUID, role_name: roleName },
	});
}

export async function myDomainPermissions(
	domainUUID: string,
): Promise<{ role_name: string; permissions: string[] }> {
	const result = await apiClient.post<{ role_name: string; permissions: string[] }>({
		url: "/api/v1/domain/my-permissions",
		data: { domain_uuid: domainUUID },
	});
	if (!result) throw new Error("domain permission data is missing");
	return result;
}
```

同步更新 `app/web/src/api/domainApi.spec.ts` 顶部 import。

- [ ] **Step 4: 实现 store**

`app/web/src/stores/domainStore.ts`：

```ts
import {
	type Domain,
	type DomainMember,
	deleteDomain,
	domainMembers,
	getDomain,
	leaveDomain,
	myDomainPermissions,
	myDomains,
} from "@/api/domain";
```

`DomainState` 新增字段：

```ts
	myRolePermissions: Record<string, string[]>;
```

初始 state 新增：

```ts
		myRolePermissions: {},
```

`removeDomain` 中新增清理：

```ts
		setState("myRolePermissions", (prev) => ({ ...prev, [uuid]: undefined }));
```

在 `loadMembers` 后新增方法：

```ts
	const loadMyPermissions = async (domainUUID: string): Promise<string[]> => {
		const data = await myDomainPermissions(domainUUID);
		setState("myRolePermissions", domainUUID, data.permissions);
		return data.permissions;
	};
```

`activateDomain` 中在 `void loadMembers(uuid).catch(() => {});` 后追加 `void loadMyPermissions(uuid).catch(() => {});`，使普通域页面也自动加载域权限。

返回对象新增 `loadMyPermissions`。

- [ ] **Step 5: 实现前端域权限判定**

创建 `app/web/src/utils/domainPermissions.ts`：

```ts
import domainStore from "@/stores/domainStore";
import { hasPermission } from "@/utils/permissions";

export function hasDomainPermission(domainUUID: string, code: string): boolean {
	if (hasPermission(code)) return true;
	const perms = domainStore.state.myRolePermissions[domainUUID];
	return !!perms?.includes(code);
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd app/web && pnpm test -- --run domainApi.spec.ts domainStore.spec.ts domainPermissions.test.ts`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add app/web/src/api/domain.ts app/web/src/api/domainApi.spec.ts app/web/src/stores/domainStore.ts app/web/src/stores/domainStore.spec.ts app/web/src/utils/domainPermissions.ts app/web/src/utils/domainPermissions.test.ts
git commit -m "feat: add frontend domain role API and permission cache"
```

---

### Task 9: 前端域角色管理面板与成员角色下拉

**Files:**
- Create: `app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainRolePanel.tsx`
- Modify: `app/web/src/components/domain/DomainMemberTable.tsx`
- Modify: `app/web/src/components/domain/DomainMemberTable.spec.tsx`
- Modify: `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`

- [ ] **Step 1: 写失败测试**

在 `app/web/src/components/domain/DomainMemberTable.spec.tsx` 末尾追加：

```ts
describe("DomainMemberTable role select logic", () => {
	it("allows changing role for other non-owner members", () => {
		expect(
			canChangeMemberRole(member, "u-owner", "u-admin", true, ["admin", "member", "guest"]),
		).toBe(true);
	});

	it("hides role select for owner, self, or missing permission", () => {
		expect(
			canChangeMemberRole(owner, "u-owner", "u-admin", true, ["admin"]),
		).toBe(false);
		expect(
			canChangeMemberRole(member, "u-owner", "u-member", true, ["admin"]),
		).toBe(false);
		expect(
			canChangeMemberRole(member, "u-owner", "u-admin", false, ["admin"]),
		).toBe(false);
		expect(
			canChangeMemberRole(member, "u-owner", "u-admin", true, []),
		).toBe(false);
	});
});
```

在 spec 顶部 import 中新增 `canChangeMemberRole`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm test -- --run DomainMemberTable.spec.tsx`
Expected: FAIL（`canChangeRole`/`onChangeRole` prop 未实现）

- [ ] **Step 3: 扩展成员表**

`app/web/src/components/domain/DomainMemberTable.tsx`：

新增导出纯函数（放在 `roleLabel` 后）：

```ts
export function canChangeMemberRole(
	member: Pick<DomainMember, "user_uuid" | "role_name">,
	ownerUUID: string | undefined,
	currentUserUUID: string | undefined,
	canChangeRole: boolean,
	roles?: string[],
): boolean {
	return (
		canChangeRole &&
		!!roles?.length &&
		member.user_uuid !== ownerUUID &&
		member.user_uuid !== currentUserUUID &&
		member.role_name !== "owner"
	);
}
```

props 新增：

```ts
	roles?: string[];
	canChangeRole?: boolean;
	onChangeRole?: (userUUID: string, roleName: string) => void;
```

“角色”列中，对非 owner、非当前用户且 `canChangeRole` 的行渲染 select：

```tsx
<td>
	<Show
		when={canChangeMemberRole(
			member,
			props.ownerUUID,
			props.currentUserUUID,
			props.canChangeRole ?? false,
			props.roles,
		)}
		fallback={
			<span class="badge badge-ghost badge-sm">
				{roleLabel(member.role_name)}
			</span>
		}
	>
		<select
			class="select select-bordered select-xs"
			value={member.role_name}
			aria-label={`角色 ${memberDisplayName(member)}`}
			onChange={(e) =>
				props.onChangeRole?.(member.user_uuid, e.currentTarget.value)
			}
		>
			<For each={props.roles}>
				{(role) => <option value={role}>{role}</option>}
			</For>
		</select>
	</Show>
</td>
```

- [ ] **Step 4: 创建角色管理面板**

创建 `app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainRolePanel.tsx`：

```tsx
import { createSignal, For, Show } from "solid-js";
import Plus from "lucide-solid/icons/plus";
import Save from "lucide-solid/icons/save";
import Trash2 from "lucide-solid/icons/trash-2";
import type { DomainRole } from "@/api/domain";

export default function DomainRolePanel(props: {
	roles: DomainRole[];
	assignable: string[];
	loading: boolean;
	saving: boolean;
	error: string;
	onCreate: (name: string, permissions: string[]) => Promise<void>;
	onUpdate: (roleName: string, permissions: string[]) => Promise<void>;
	onDelete: (roleName: string) => Promise<void>;
}) {
	const [selected, setSelected] = createSignal<string>("");
	const [selectedCodes, setSelectedCodes] = createSignal<Set<string>>(new Set());
	const [newRoleName, setNewRoleName] = createSignal("");

	const selectRole = (role: DomainRole) => {
		setSelected(role.name);
		setSelectedCodes(new Set(role.permissions));
	};

	const toggle = (code: string, checked: boolean) => {
		const next = new Set(selectedCodes());
		if (checked) next.add(code);
		else next.delete(code);
		setSelectedCodes(next);
	};

	const save = async () => {
		const name = selected();
		if (!name) return;
		await props.onUpdate(name, Array.from(selectedCodes()));
	};

	return (
		<div class="min-w-0">
			<Show when={props.error}>
				<div role="alert" class="alert alert-error mb-3 text-sm">
					<span>{props.error}</span>
				</div>
			</Show>
			<div class="grid min-w-0 gap-4 md:grid-cols-[220px_minmax(0,1fr)]">
				<div class="flex flex-col gap-1">
					<For each={props.roles}>
						{(role) => (
							<button
								type="button"
								class={`btn btn-ghost btn-sm justify-start ${selected() === role.name ? "btn-active" : ""}`}
								onClick={() => selectRole(role)}
							>
								<span class="truncate">{role.name}</span>
								{role.is_system ? (
									<span class="badge badge-ghost badge-xs">系统</span>
								) : null}
							</button>
						)}
					</For>
					<div class="mt-3 flex gap-2">
						<input
							class="input input-bordered input-sm min-w-0 flex-1"
							placeholder="新角色名"
							value={newRoleName()}
							onInput={(e) => setNewRoleName(e.currentTarget.value)}
						/>
						<button
							type="button"
							class="btn btn-primary btn-sm"
							disabled={!newRoleName().trim() || props.saving}
							onClick={() => {
								void props.onCreate(newRoleName().trim(), Array.from(selectedCodes()));
								setNewRoleName("");
							}}
						>
							<Plus size={14} />
							创建
						</button>
					</div>
				</div>
				<div class="min-w-0">
					<Show when={selected()} fallback={<p class="text-sm text-base-content/50">选择角色进行权限配置</p>}>
						<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
							<h3 class="font-semibold">{selected()}</h3>
							<div class="flex gap-2">
								<Show when={!props.roles.find((r) => r.name === selected())?.is_system}>
									<button
										type="button"
										class="btn btn-outline btn-error btn-sm"
										disabled={props.saving}
										onClick={() => void props.onDelete(selected())}
									>
										<Trash2 size={14} />
										删除
									</button>
								</Show>
								<button
									type="button"
									class="btn btn-primary btn-sm"
									disabled={props.saving || selected() === "owner"}
									onClick={() => void save()}
								>
									<Save size={14} />
									保存权限
								</button>
							</div>
						</div>
						<div class="grid grid-cols-2 gap-2 xl:grid-cols-3 max-md:grid-cols-1">
							<For each={props.assignable}>
								{(code) => (
									<label class="flex cursor-pointer items-start gap-2 border border-base-300 px-3 py-2 text-sm">
										<input
											type="checkbox"
											class="checkbox checkbox-sm mt-0.5"
											checked={selectedCodes().has(code)}
											disabled={selected() === "owner"}
											onChange={(e) => toggle(code, e.currentTarget.checked)}
										/>
										<span class="break-all font-mono text-xs">{code}</span>
									</label>
								)}
							</For>
						</div>
					</Show>
				</div>
			</div>
		</div>
	);
}
```

- [ ] **Step 5: 接入域管理页**

`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`：

`domainStore` 解构改为：

```ts
	const { state, setCurrentDomain, loadMembers, loadMyPermissions, updateCachedDomain } =
		domainStore;
```

import 新增：

```ts
import {
	createDomainRole,
	deleteDomainRole,
	listDomainRoles,
	type DomainRole,
	updateDomainMemberRole,
	updateDomainRolePermissions,
} from "@/api/domain";
import { hasDomainPermission } from "@/utils/domainPermissions";
import DomainRolePanel from "./components/DomainRolePanel";
```

页面 state 新增：

```ts
	const [domainRoles, setDomainRoles] = createSignal<DomainRole[]>([]);
	const [assignableCodes, setAssignableCodes] = createSignal<string[]>([]);
	const [rolesLoading, setRolesLoading] = createSignal(true);
	const [rolesError, setRolesError] = createSignal("");
	const [rolesSaving, setRolesSaving] = createSignal(false);
```

加载角色列表（在 `createEffect` 中与 rooms 一起触发）：

```ts
	async function fetchRoles() {
		setRolesLoading(true);
		setRolesError("");
		try {
			const data = await listDomainRoles(uuid());
			setDomainRoles(data.roles);
			setAssignableCodes(data.assignable);
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesLoading(false);
		}
	}
```

调用 helper 与保存/创建/删除/改角色 handler：

```ts
	const canManageRoles = createMemo(
		() =>
			isOwner() ||
			hasPermission("domain:role:manage") ||
			hasDomainPermission(uuid(), "domain:role:manage"),
	);

	async function handleCreateRole(name: string, permissions: string[]) {
		setRolesSaving(true);
		setRolesError("");
		try {
			await createDomainRole(uuid(), name, permissions);
			showToast("角色已创建", { type: "success" });
			await fetchRoles();
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleUpdateRole(roleName: string, permissions: string[]) {
		setRolesSaving(true);
		setRolesError("");
		try {
			await updateDomainRolePermissions(uuid(), roleName, permissions);
			showToast("权限已保存", { type: "success" });
			await fetchRoles();
			await loadMyPermissions(uuid());
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleDeleteRole(roleName: string) {
		setRolesSaving(true);
		setRolesError("");
		try {
			await deleteDomainRole(uuid(), roleName);
			showToast("角色已删除", { type: "success" });
			await fetchRoles();
		} catch (error) {
			setRolesError(apiErrorMessage(error));
		} finally {
			setRolesSaving(false);
		}
	}

	async function handleMemberRoleChange(userUUID: string, roleName: string) {
		try {
			await updateDomainMemberRole(uuid(), userUUID, roleName);
			await loadMembers(uuid());
			showToast("成员角色已更新", { type: "success" });
		} catch (error) {
			const message = apiErrorMessage(error);
			showToast(message, { type: "error" });
		}
	}
```

`createEffect` 中调用 `void fetchRoles()` 与 `void loadMyPermissions(uuid()).catch(() => {})`；`domainStore` 解构新增 `loadMyPermissions`。

在“成员管理”`ManageSection` 中给 `DomainMemberTable` 传新 props：

```tsx
					roles={domainRoles().map((role) => role.name)}
					canChangeRole={canManageRoles()}
					onChangeRole={(userUUID, roleName) =>
						void handleMemberRoleChange(userUUID, roleName)
					}
```

在“成员管理” section 后新增“角色与权限” section：

```tsx
				<Show when={canManageRoles()}>
					<ManageSection
						title="角色与权限"
						description="配置域内角色的独立权限"
						padded={false}
						class="min-w-0"
						actions={
							<button
								type="button"
								class="btn btn-ghost btn-xs"
								disabled={rolesLoading()}
								onClick={() => void fetchRoles()}
							>
								刷新
							</button>
						}
					>
						<DomainRolePanel
							roles={domainRoles()}
							assignable={assignableCodes()}
							loading={rolesLoading()}
							saving={rolesSaving()}
							error={rolesError()}
							onCreate={handleCreateRole}
							onUpdate={handleUpdateRole}
							onDelete={handleDeleteRole}
						/>
					</ManageSection>
				</Show>
```

同时把 `canManage`/`canKick`/`canManageRooms` 改为同时检查 `hasDomainPermission`：

```ts
	const canManage = createMemo(
		() =>
			isOwner() ||
			hasPermission("domain:manage") ||
			hasDomainPermission(uuid(), "domain:manage"),
	);
	const canKick = createMemo(
		() =>
			isOwner() ||
			hasPermission("domain:kick") ||
			hasDomainPermission(uuid(), "domain:kick"),
	);
	const canManageRooms = createMemo(
		() =>
			isOwner() ||
			hasAnyPermission("room:update", "room:delete") ||
			hasDomainPermission(uuid(), "room:update") ||
			hasDomainPermission(uuid(), "room:delete"),
	);
```

- [ ] **Step 6: 运行测试与构建检查**

Run: `cd app/web && pnpm test`
Expected: PASS

Run: `cd app/web && pnpm build`
Expected: PASS（TypeScript 无类型错误）

- [ ] **Step 7: Commit**

```bash
git add "app/web/src/pages/(app)/manage/domains/$domainUUID/components/DomainRolePanel.tsx" "app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx" app/web/src/components/domain/DomainMemberTable.tsx app/web/src/components/domain/DomainMemberTable.spec.tsx
git commit -m "feat: add domain role management UI"
```

---

### Task 10: 集成测试与端到端验证

**Files:**
- Modify: `test/domain/domain.test.ts`
- Modify: `test/helpers/api.ts`

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/service/domain_service_test.go` 已覆盖单元行为的基础上，扩展集成测试。`test/helpers/api.ts` 的 `createDomain` 后追加：

```ts
export async function listDomainRoles(
	token: string,
	domainUUID: string,
): Promise<{ roles: Array<{ name: string; is_system: boolean; permissions: string[] }>; assignable: string[] }> {
	const result = await api(`/api/v1/domain/roles/list`, {
		token,
		body: { domain_uuid: domainUUID },
	});
	return assertSuccess(result);
}

export async function createDomainRole(
	token: string,
	domainUUID: string,
	name: string,
	permissions: string[],
): Promise<unknown> {
	const result = await api(`/api/v1/domain/roles/create`, {
		token,
		body: { domain_uuid: domainUUID, name, permissions },
	});
	return assertSuccess(result);
}

export async function updateDomainMemberRole(
	token: string,
	domainUUID: string,
	userUUID: string,
	roleName: string,
): Promise<unknown> {
	const result = await api(`/api/v1/domain/members/update-role`, {
		token,
		body: { domain_uuid: domainUUID, user_uuid: userUUID, role_name: roleName },
	});
	return assertSuccess(result);
}
```

在 `test/domain/domain.test.ts` 的 `describe("domains")` 内追加：

```ts
it("creates per-domain role and assigns member", async () => {
	const owner = await registerUser("domain_role_owner");
	const created = await api<{ uuid: string }>("/api/v1/domain/create", {
		token: owner.access_token,
		body: { name: unique("role_domain"), is_public: false },
	});
	const domain = assertSuccess(created);

	const roles = await listDomainRoles(owner.access_token, domain.uuid);
	expect(roles.roles.some((r) => r.name === "owner")).toBe(true);
	expect(roles.assignable).toContain("room:read");

	const member = await registerUser("domain_role_member");
	await createDomainRole(owner.access_token, domain.uuid, "moderator", ["room:read", "message:delete_others"]);
	await updateDomainMemberRole(owner.access_token, domain.uuid, member.user.uuid, "moderator");

	const after = await listDomainRoles(owner.access_token, domain.uuid);
	expect(after.roles.find((r) => r.name === "moderator")?.permissions).toEqual(
		expect.arrayContaining(["room:read", "message:delete_others"]),
	);
});
```

`registerUser` 返回的 `AuthData.user.uuid` 即用户 UUID，直接使用。

- [ ] **Step 2: 运行集成测试确认失败**

Run: `pnpm run test:integration -- -t "creates per-domain role"`
Expected: FAIL（新 API 不存在）

- [ ] **Step 3: 全量验证**

Run: `cd app/server && go test ./...`
Expected: PASS

Run: `cd app/web && pnpm test && pnpm build`
Expected: PASS

Run: `pnpm run test:integration`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add test/domain/domain.test.ts test/helpers/api.ts
git commit -m "test: cover per-domain role management flow"
```

---

## Self-Review

- 规格覆盖：域角色模型/seed、角色 CRUD、成员角色分配、现有 Kick/Update/Room/Message 判定接入、前端管理 UI、集成测试均落在 Task 1-10。
- 占位符扫描：每个任务均给出实际文件路径、测试代码、实现代码与命令；Task 6 的两个测试均已提供完整断言。
- 类型一致性：模型常量统一从 `model.DomainRole*` 导出，`service.DomainRole*` 改为别名；`NewDomainService(domainRepo, roleRepo)`、`NewMessageHandler(msgSvc, permSvc, roomSvc, domainSvc)`、`HubOptions.DomainPermissionChecker` 在 Task 4/7 中前后一致；前端 `hasDomainPermission(domainUUID, code)` 签名在 Task 8/9 中一致。
- 已知限制：WS `message:delete` 依赖 payload 中的 `domain_uuid`；自定义角色暂不支持重命名；域角色权限采用 DB 直查，不做缓存（域规模小时可接受）。
