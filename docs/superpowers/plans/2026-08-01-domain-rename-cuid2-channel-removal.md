# Domain 重构、CUID2 强迁移与移除 /channel 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 “Guild” 全量改名为 “Domain（域）”，所有 Domain 标识强迁移为 CUID2，并移除 `/channel?id=...` 设计，全部进入 `/domain/:domainUUID` 管理。

**Architecture:** 后端继续沿用 Gin + GORM + Signal Hub，但模型、API、数据库表、信令字段统一改名为 Domain；Domain 公开 ID 改为 CUID2，并增加一次性数据库迁移。前端继续使用 TanStack Solid Router，但删除 `/channel` 路由，将 Domain 工作区和 `/manage/domains/:domainUUID` 作为唯一入口。

**Tech Stack:** Go 1.26、Gin、GORM、SQLite/PostgreSQL/MySQL、`github.com/nrednav/cuid2`、SolidJS、TanStack Router、Vite、Vitest、Biome。

**约定：**
- 不做旧 UUID 兼容，不做 `/guild`、`guild` 命名兼容。
- 不做 `/channel` 跳转兼容，删除后该路径 404。
- 仓库 `AGENTS.md` 禁止未经要求提交代码；本计划不包含 commit 步骤。
- 工作区已有用户改动，尤其 `app/web/src/stores/socketStore.ts` 等文件，实施时只做本计划范围内的改名，不回退其他改动。

---

## 文件地图

### 后端需要移动/重命名的文件
| 旧路径 | 新路径 |
|---|---|
| `app/server/internal/model/guild.go` | `app/server/internal/model/domain.go` |
| `app/server/internal/model/guild_test.go` | `app/server/internal/model/domain_test.go` |
| `app/server/internal/permcode/guild_permcode.go` | `app/server/internal/permcode/domain_permcode.go` |
| `app/server/internal/repository/guild_repo.go` | `app/server/internal/repository/domain_repo.go` |
| `app/server/internal/repository/guild_repo_test.go` | `app/server/internal/repository/domain_repo_test.go` |
| `app/server/internal/repository/guild_repo_search_test.go` | `app/server/internal/repository/domain_repo_search_test.go` |
| `app/server/internal/repository/default_guild_test.go` | `app/server/internal/repository/default_domain_test.go` |
| `app/server/internal/service/guild_service.go` | `app/server/internal/service/domain_service.go` |
| `app/server/internal/service/guild_service_test.go` | `app/server/internal/service/domain_service_test.go` |
| `app/server/internal/service/guild_service_search_test.go` | `app/server/internal/service/domain_service_search_test.go` |
| `app/server/internal/service/room_service_guild_test.go` | `app/server/internal/service/room_service_domain_test.go` |
| `app/server/internal/handler/guild_handler.go` | `app/server/internal/handler/domain_handler.go` |
| `app/server/internal/handler/guild_handler_test.go` | `app/server/internal/handler/domain_handler_test.go` |
| `app/server/internal/handler/guild_handler_phase0_test.go` | `app/server/internal/handler/domain_handler_phase0_test.go` |
| `app/server/internal/handler/guild_handler_preview_test.go` | `app/server/internal/handler/domain_handler_preview_test.go` |
| `app/server/internal/middleware/guild.go` | `app/server/internal/middleware/domain.go` |
| `app/server/internal/middleware/guild_test.go` | `app/server/internal/middleware/domain_test.go` |
| `app/server/internal/router/routes/guild/` | `app/server/internal/router/routes/domain/` |
| `app/server/internal/signal/hub_guild_test.go` | `app/server/internal/signal/hub_domain_test.go` |
| `app/server/internal/signal/hub_guild_filter_test.go` | `app/server/internal/signal/hub_domain_filter_test.go` |
| `app/server/internal/signal/hub_guild_ws_test.go` | `app/server/internal/signal/hub_domain_ws_test.go` |

### 前端需要移动/重命名的文件
| 旧路径 | 新路径 |
|---|---|
| `app/web/src/api/guild.ts` | `app/web/src/api/domain.ts` |
| `app/web/src/api/guildApi.spec.ts` | `app/web/src/api/domainApi.spec.ts` |
| `app/web/src/stores/guildStore.ts` | `app/web/src/stores/domainStore.ts` |
| `app/web/src/stores/guildStore.spec.ts` | `app/web/src/stores/domainStore.spec.ts` |
| `app/web/src/utils/guildInvite.ts` | `app/web/src/utils/domainInvite.ts` |
| `app/web/src/utils/guildInvite.test.ts` | `app/web/src/utils/domainInvite.test.ts` |
| `app/web/src/components/guild/` | `app/web/src/components/domain/` |
| `app/web/src/pages/(app)/guild/$guildUUID/` | `app/web/src/pages/(app)/domain/$domainUUID/` |
| `app/web/src/pages/(app)/invite/g/$code/` | `app/web/src/pages/(app)/invite/d/$code/` |
| `app/web/src/pages/(app)/channel/index.tsx` | 删除 |
| `app/web/src/pages/(app)/domain/$domainUUID/manage.tsx` | 移到 `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx` |

---

## Task 1: 添加 CUID2 依赖并建立 Domain 模型

**Files:**
- Modify: `app/server/go.mod`, `app/server/go.sum`
- Create: `app/server/internal/model/domain.go`
- Delete: `app/server/internal/model/guild.go`
- Modify: `app/server/internal/model/room.go`
- Modify: `app/server/internal/repository/db.go`

- [ ] **Step 1: 添加依赖**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/server
go get github.com/nrednav/cuid2@v1.1.0
```

Expected: `go.mod` 增加 `github.com/nrednav/cuid2 v1.1.0`，`go.sum` 更新。

- [ ] **Step 2: 创建 `internal/model/domain.go`**

Use `apply_patch` to replace `internal/model/guild.go` with:

```go
package model

import (
	"time"

	"github.com/nrednav/cuid2"
	"gorm.io/gorm"
)

// Domain 代表一个语音域（原 Guild/Server 语义）。
type Domain struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"size:32;uniqueIndex" json:"uuid"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	IconURL     string    `gorm:"size:512" json:"icon_url"`
	Description string    `gorm:"size:500" json:"description"`
	OwnerUUID   string    `gorm:"type:uuid;index;not null" json:"owner_uuid"`
	InviteCode  string    `gorm:"size:32;uniqueIndex" json:"invite_code"`
	MaxRooms    uint      `gorm:"default:0" json:"max_rooms"`
	IsPublic    bool      `gorm:"default:false" json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (d *Domain) TableName() string {
	return "domains"
}

func (d *Domain) BeforeCreate(_ *gorm.DB) error {
	if d.UUID == "" {
		d.UUID = cuid2.Generate()
	}
	if d.InviteCode == "" {
		d.InviteCode = generateInviteCode()
	}
	return nil
}

// DomainMember 用户-Domain 多对多关系。
type DomainMember struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DomainUUID string    `gorm:"size:32;index:idx_domain_member,priority:1;not null" json:"domain_uuid"`
	UserUUID   string    `gorm:"type:uuid;index:idx_domain_member,priority:2;not null" json:"user_uuid"`
	Nickname   string    `gorm:"size:64" json:"nickname"`
	RoleName   string    `gorm:"size:32;default:member" json:"role_name"`
	JoinedAt   time.Time `json:"joined_at"`
}

func (DomainMember) TableName() string {
	return "domain_members"
}

// generateInviteCode 生成 8 字符随机邀请码。
func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	u := cuid2.Generate()
	var b [8]byte
	for i := range b {
		b[i] = charset[int(u[i])%len(charset)]
	}
	return string(b[:])
}
```

- [ ] **Step 3: 删除旧模型文件**

Run:
```bash
rm /Users/noelorin/GOSpeak/app/server/internal/model/guild.go
```

- [ ] **Step 4: 修改 `internal/model/room.go`**

Change this field:

```go
// DomainUUID 归属的语音域 CUID2。空值表示平台级房间（仅迁移期兼容）。
DomainUUID string `gorm:"size:32;index" json:"domain_uuid"`
```

- [ ] **Step 5: 修改 `repository/db.go` 的 AutoMigrate**

Replace `&model.Guild{},` and `&model.GuildMember{},` with:

```go
&model.Domain{},
&model.DomainMember{},
```

- [ ] **Step 6: 运行模型测试占位检查**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/server
go test ./internal/model -run TestDomain -count=1
```

Expected: test binary compiles but may fail because `domain_test.go` 尚未更新；这是后续任务会处理的失败，不是阻塞。

---

## Task 2: 全量机械改名 Domain

**Files:** 所有 `app/server`、`app/web/src` 中含 `guild` 的文件。

- [ ] **Step 1: 移动后端文件**

Run from repo root:

```bash
cd /Users/noelorin/GOSpeak
mv app/server/internal/model/guild_test.go app/server/internal/model/domain_test.go
mv app/server/internal/permcode/guild_permcode.go app/server/internal/permcode/domain_permcode.go
mv app/server/internal/repository/guild_repo.go app/server/internal/repository/domain_repo.go
mv app/server/internal/repository/guild_repo_test.go app/server/internal/repository/domain_repo_test.go
mv app/server/internal/repository/guild_repo_search_test.go app/server/internal/repository/domain_repo_search_test.go
mv app/server/internal/repository/default_guild_test.go app/server/internal/repository/default_domain_test.go
mv app/server/internal/service/guild_service.go app/server/internal/service/domain_service.go
mv app/server/internal/service/guild_service_test.go app/server/internal/service/domain_service_test.go
mv app/server/internal/service/guild_service_search_test.go app/server/internal/service/domain_service_search_test.go
mv app/server/internal/service/room_service_guild_test.go app/server/internal/service/room_service_domain_test.go
mv app/server/internal/handler/guild_handler.go app/server/internal/handler/domain_handler.go
mv app/server/internal/handler/guild_handler_test.go app/server/internal/handler/domain_handler_test.go
mv app/server/internal/handler/guild_handler_phase0_test.go app/server/internal/handler/domain_handler_phase0_test.go
mv app/server/internal/handler/guild_handler_preview_test.go app/server/internal/handler/domain_handler_preview_test.go
mv app/server/internal/middleware/guild.go app/server/internal/middleware/domain.go
mv app/server/internal/middleware/guild_test.go app/server/internal/middleware/domain_test.go
mv app/server/internal/router/routes/guild app/server/internal/router/routes/domain
mv app/server/internal/signal/hub_guild_test.go app/server/internal/signal/hub_domain_test.go
mv app/server/internal/signal/hub_guild_filter_test.go app/server/internal/signal/hub_domain_filter_test.go
mv app/server/internal/signal/hub_guild_ws_test.go app/server/internal/signal/hub_domain_ws_test.go
```

- [ ] **Step 2: 移动前端文件**

Run from repo root:

```bash
cd /Users/noelorin/GOSpeak
mv app/web/src/api/guild.ts app/web/src/api/domain.ts
mv app/web/src/api/guildApi.spec.ts app/web/src/api/domainApi.spec.ts
mv app/web/src/stores/guildStore.ts app/web/src/stores/domainStore.ts
mv app/web/src/stores/guildStore.spec.ts app/web/src/stores/domainStore.spec.ts
mv app/web/src/utils/guildInvite.ts app/web/src/utils/domainInvite.ts
mv app/web/src/utils/guildInvite.test.ts app/web/src/utils/domainInvite.test.ts
mv app/web/src/components/guild app/web/src/components/domain
mv 'app/web/src/pages/(app)/guild/$guildUUID' 'app/web/src/pages/(app)/domain/$domainUUID'
mv 'app/web/src/pages/(app)/invite/g/$code' 'app/web/src/pages/(app)/invite/d/$code'
rm 'app/web/src/pages/(app)/channel/index.tsx'
```

- [ ] **Step 3: 全量替换标识符**

Run:

```bash
cd /Users/noelorin/GOSpeak
perl -pi -e '
s/GuildMember/DomainMember/g;
s/GuildHandler/DomainHandler/g;
s/GuildRepository/DomainRepository/g;
s/GuildService/DomainService/g;
s/GuildRole/DomainRole/g;
s/GuildState/DomainState/g;
s/GuildMutation/DomainMutation/g;
s/GuildApiError/DomainApiError/g;
s/GuildPage/DomainPage/g;
s/Guild/Domain/g;
s/guildUUIDs/domainUUIDs/g;
s/guild_uuids/domain_uuids/g;
s/guild_uuid/domain_uuid/g;
s/guildUUID/domainUUID/g;
s/my-guilds/my-domains/g;
s/guild/domain/g;
' $(rg -l -i 'guild' app/server app/web/src 2>/dev/null)
```

- [ ] **Step 4: 修复 Go 包声明和导入**

Run:

```bash
cd /Users/noelorin/GOSpeak
rg -n "package domain|routes/domain|handler.DomainHandler|service.DomainService|repository.DomainRepository" app/server/internal/router app/server/server app/server/internal/handler app/server/internal/service app/server/internal/repository | sed -n '1,120p'
```

Expected output should show `domainRoutes "GOSpeak/internal/router/routes/domain"`、`Domain` handler/service/repository names。手动修正任何残留导入路径。

- [ ] **Step 5: 运行 Go 编译检查**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
```

Expected: 编译通过；如果失败，按 `go build` 报错逐个修复类型名和字段名。

---

## Task 3: 数据库强迁移

**Files:**
- Modify: `app/server/internal/repository/db.go`
- Test: `app/server/internal/repository/default_domain_test.go`

- [ ] **Step 1: 在 `InitDB` 中调用表结构迁移**

In `InitDB` before `autoMigrate()`, add:

```go
if err := migrateGuildToDomainSchema(DB); err != nil {
	return err
}
if err := migrateGuildPermissions(DB); err != nil {
	return err
}
```

After `autoMigrate()`, add:

```go
if err := migrateDomainCUID2(DB); err != nil {
	return err
}
```

- [ ] **Step 2: 添加三个迁移函数**

Add to `app/server/internal/repository/db.go`:

```go
func migrateGuildToDomainSchema(db *gorm.DB) error {
	if db.Migrator().HasTable("guilds") {
		if err := db.Exec("ALTER TABLE guilds RENAME TO domains").Error; err != nil {
			return fmt.Errorf("rename guilds to domains: %w", err)
		}
	}
	if db.Migrator().HasTable("guild_members") {
		if err := db.Exec("ALTER TABLE guild_members RENAME TO domain_members").Error; err != nil {
			return fmt.Errorf("rename guild_members to domain_members: %w", err)
		}
	}
	if err := renameDomainColumn(db, "domain_members", "guild_uuid", "domain_uuid"); err != nil {
		return err
	}
	if err := renameDomainColumn(db, "rooms", "guild_uuid", "domain_uuid"); err != nil {
		return err
	}
	return nil
}

func renameDomainColumn(db *gorm.DB, table, oldName, newName string) error {
	if !db.Migrator().HasColumn(table, oldName) {
		return nil
	}
	var err error
	switch db.Dialector.Name() {
	case "mysql":
		err = db.Exec("ALTER TABLE " + table + " CHANGE COLUMN " + oldName + " " + newName + " varchar(32)").Error
	default:
		err = db.Exec("ALTER TABLE " + table + " RENAME COLUMN " + oldName + " TO " + newName).Error
	}
	if err != nil {
		return fmt.Errorf("rename %s.%s to %s: %w", table, oldName, newName, err)
	}
	return nil
}

func migrateGuildPermissions(db *gorm.DB) error {
	if db.Migrator().HasTable("permissions") {
		if err := db.Exec("UPDATE permissions SET code = REPLACE(code, 'guild:', 'domain:') WHERE code LIKE 'guild:%'").Error; err != nil {
			return fmt.Errorf("migrate permission codes: %w", err)
		}
		if err := db.Exec("UPDATE permissions SET name = REPLACE(name, '语音服务器', '域'), description = REPLACE(description, '语音服务器', '域') WHERE code LIKE 'domain:%'").Error; err != nil {
			return fmt.Errorf("migrate permission labels: %w", err)
		}
	}
	if db.Migrator().HasTable("bot_tokens") {
		if err := db.Exec("UPDATE bot_tokens SET permissions = REPLACE(permissions, 'guild:', 'domain:') WHERE permissions LIKE '%guild:%'").Error; err != nil {
			return fmt.Errorf("migrate bot permissions: %w", err)
		}
	}
	return nil
}

func migrateDomainCUID2(db *gorm.DB) error {
	type domainRow struct {
		ID   uint
		UUID string
	}
	var rows []domainRow
	if err := db.Table("domains").Select("id, uuid").Find(&rows).Error; err != nil {
		return fmt.Errorf("load domains for cuid migration: %w", err)
	}
	for _, row := range rows {
		if !strings.Contains(row.UUID, "-") {
			continue
		}
		next := cuid2.Generate()
		if err := db.Table("domains").Where("id = ?", row.ID).Update("uuid", next).Error; err != nil {
			return fmt.Errorf("migrate domain id %d: %w", row.ID, err)
		}
		if err := db.Table("domain_members").Where("domain_uuid = ?", row.UUID).Update("domain_uuid", next).Error; err != nil {
			return fmt.Errorf("migrate domain_members for %s: %w", row.UUID, err)
		}
		if err := db.Table("rooms").Where("domain_uuid = ?", row.UUID).Update("domain_uuid", next).Error; err != nil {
			return fmt.Errorf("migrate rooms for %s: %w", row.UUID, err)
		}
	}
	return nil
}
```

Add import `"github.com/nrednav/cuid2"` to `db.go` if not already present.

- [ ] **Step 3: 更新默认域迁移测试**

In `default_domain_test.go`, replace `EnsureDefaultGuild` with `EnsureDefaultDomain`, replace `model.Guild` with `model.Domain`, `model.GuildMember` with `model.DomainMember`, and rename seed name `"Default Server"` to `"默认域"`。测试里保留旧 `guilds`/`guild_members` 表迁移断言，新增断言：迁移后 `domains.uuid` 不含 `-`。

---

## Task 4: 后端 Domain 权限、路由、Handler、Service、Signal 改名

**Files:**
- Modify: `app/server/internal/permcode/domain_permcode.go`
- Modify: `app/server/internal/model/permission.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/internal/router/routes/domain/routes.go`
- Modify: `app/server/server/gin.go`
- Modify: `app/server/internal/middleware/domain.go`
- Modify: `app/server/internal/signal/hub.go`
- Modify: `app/server/internal/signal/state_sync.go`
- Modify: `app/server/internal/signal/types.go`
- Modify: `app/server/internal/ws/fanout.go`

- [ ] **Step 1: 权限码改为 `domain:*`**

Replace `app/server/internal/permcode/domain_permcode.go` with:

```go
package permcode

const (
	PermDomainCreate     = "domain:create"
	PermDomainRead       = "domain:read"
	PermDomainManage     = "domain:manage"
	PermDomainDelete     = "domain:delete"
	PermDomainInvite     = "domain:invite"
	PermDomainKick       = "domain:kick"
	PermDomainRoleManage = "domain:role:manage"
)
```

- [ ] **Step 2: 更新权限种子**

In `app/server/internal/model/permission.go`, rename constants and descriptions:

```go
PermDomainCreate     = permcode.PermDomainCreate
PermDomainRead       = permcode.PermDomainRead
PermDomainManage     = permcode.PermDomainManage
PermDomainDelete     = permcode.PermDomainDelete
PermDomainInvite     = permcode.PermDomainInvite
PermDomainKick       = permcode.PermDomainKick
PermDomainRoleManage = permcode.PermDomainRoleManage
```

Replace descriptions “语音服务器” with “域”，例如 `{Code: PermDomainCreate, Name: "创建域", Description: "创建新的语音域"}`。

- [ ] **Step 3: 路由组改为 `/api/v1/domain`**

In `app/server/internal/router/router.go`:

```go
domainRoutes "GOSpeak/internal/router/routes/domain"
```

and

```go
Domain *handler.DomainHandler
```

and

```go
if h.Domain != nil {
	domainRoutes.Register(protected.Group("/domain"), h.Domain)
}
```

In `app/server/internal/router/routes/domain/routes.go`, endpoints become:

```go
r.POST("/create", middleware.RequirePermission(permcode.PermDomainCreate), h.Create)
r.POST("/get", middleware.RequireDomainMember(), h.Get)
r.POST("/list", middleware.RequirePermission(permcode.PermDomainRead), h.List)
r.POST("/list-public", h.ListPublic)
r.POST("/my-domains", h.MyDomains)
r.POST("/update", middleware.RequireDomainMember(), h.Update)
r.POST("/delete", middleware.RequireDomainMember(), h.Delete)
r.POST("/join", h.Join)
r.POST("/preview", h.Preview)
r.POST("/leave", middleware.RequireDomainMember(), h.Leave)
r.POST("/kick", middleware.RequireDomainMember(), h.Kick)
r.POST("/members", middleware.RequireDomainMember(), h.Members)
```

- [ ] **Step 4: Gin 装配改名**

In `app/server/server/gin.go`:

```go
domainSvc := service.NewDomainService(repository.NewDomainRepository(repository.DB))
middleware.SetDomainChecker(domainSvc.IsMember)
signalHub.SetDomainChecker(domainSvc.IsMember)
domainH := handler.NewDomainHandler(domainSvc, permSvc)
domainH.SetOnDomainDelete(signalHub.OnDomainDelete)
```

and pass `Domain: domainH` in `router.Handlers`.

- [ ] **Step 5: 中间件和 Signal 字段统一**

In `app/server/internal/middleware/domain.go`, rename all public API to `SetDomainChecker`、`IsDomainMember`、`RequireDomainMember`、`RequireDomainMemberIfProvided`。JSON body 字段改为 `domain_uuid`，不再回退 `uuid` 字段。

In `app/server/internal/signal/hub.go`、`state_sync.go`、`types.go`、`ws/fanout.go`，把 `guild_uuid` 改为 `domain_uuid`，`GuildUUID` 改为 `DomainUUID`，`guildRoomKey` 改为 `domainRoomKey`，前缀 `"__guild:"` 改为 `"__domain:"`。

- [ ] **Step 6: 编译并跑后端测试**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
go test ./... -count=1
```

Expected: 编译和测试通过；若测试字符串仍使用 `guild-a` 或 `guild_uuid`，手动更新断言为 `domain-a` / `domain_uuid`。

---

## Task 5: 后端文档与测试夹具清理

**Files:**
- Modify: `app/server/testutil/user_factory.go`
- Modify: `app/server/docs/docs.go`
- Modify: `app/server/docs/swagger.json`
- Modify: `app/server/docs/swagger.yaml`
- Modify: `app/server/AGENTS.md`
- Modify: `app/server/internal/*/AGENTS.md`

- [ ] **Step 1: 更新测试夹具**

In `app/server/testutil/user_factory.go`，用 `DomainOption`、`WithDomainInviteCode`、`WithDomainMaxRooms`、`CreateTestDomain`、`AddTestDomainMember` 替换原 Guild 名称。

- [ ] **Step 2: 批量更新生成文档**

Run:
```bash
cd /Users/noelorin/GOSpeak
perl -pi -e 's/guild_uuid/domain_uuid/g; s/guildUUID/domainUUID/g; s/Guild/Domain/g; s/guild/domain/g' \
  app/server/docs/docs.go app/server/docs/swagger.json app/server/docs/swagger.yaml
perl -pi -e 's#/guild#/domain#g; s/guild/domain/g; s/Guild/Domain/g' \
  app/server/AGENTS.md app/server/internal/model/AGENTS.md app/server/internal/repository/AGENTS.md app/server/internal/service/AGENTS.md app/server/internal/handler/AGENTS.md app/server/internal/router/AGENTS.md app/server/internal/pkg/AGENTS.md
```

- [ ] **Step 3: 校验文档残留**

Run:
```bash
rg -n -i 'guild' app/server --glob '!go.sum' --glob '!gospeak' --glob '!tmp/**'
```

Expected: 无 `guild` 残留；如生成文档结构因文本替换不合法，直接运行 `go test ./...` 修复。

---

## Task 6: 前端 Domain API、Store、类型和工具改名

**Files:**
- Modify: `app/web/src/api/domain.ts`
- Modify: `app/web/src/stores/domainStore.ts`
- Modify: `app/web/src/socket/roomState.ts`
- Modify: `app/web/src/socket/types.ts`
- Modify: `app/web/src/types/room.ts`
- Modify: `app/web/src/api/room.ts`
- Modify: `app/web/src/utils/permissions.ts`
- Modify: `app/web/src/utils/domainInvite.ts`

- [ ] **Step 1: 更新 `api/domain.ts`**

API 路径统一为 `/api/v1/domain/*`，类型统一为：

```ts
export interface Domain { ... uuid: string; ... }
export interface DomainMember { ... domain_uuid: string; ... }
export interface DomainPage { domains: Domain[]; total: number; }
```

`myDomains()` 解析 `{ domain_uuids: string[] }`，`listPublicDomains()` 解析 `{ domains }`，踢人和成员接口使用 `domain_uuid`。

- [ ] **Step 2: 更新 `domainStore.ts`**

状态名改为：

```ts
myDomainUUIDs: string[]
currentDomainUUID: string | null
domainCache: Record<string, Domain>
memberCache: Record<string, DomainMember[]>
domainLoading: Record<string, boolean>
memberLoading: Record<string, boolean>
domainErrors: Record<string, string | null>
memberErrors: Record<string, string | null>
```

方法名改为 `loadMyDomains`、`ensureDomainLoaded`、`setCurrentDomain`、`loadMembers`、`addDomain`、`updateCachedDomain`、`removeDomain`、`leaveAndClear`、`deleteAndClear`。

- [ ] **Step 3: 更新权限兜底**

In `app/web/src/utils/permissions.ts`，所有 `guild:*` 改为 `domain:*`，例如 `user: ["room:create", "room:read", "domain:create", "user:read", "role:read"]`。

- [ ] **Step 4: 更新 WS 类型**

In `roomState.ts`、`socket/types.ts`、`types/room.ts`、`api/room.ts`，把 `guild_uuid` 改为 `domain_uuid`，`guildUUID` 改为 `domainUUID`，`GuildUUID` 改为 `DomainUUID`。

---

## Task 7: 移除 `/channel`，Domain 工作区接管

**Files:**
- Delete: `app/web/src/pages/(app)/channel/index.tsx`
- Modify: `app/web/src/components/common/dynamicRender.tsx`
- Modify: `app/web/src/layouts/layout.tsx`
- Modify: `app/web/src/layouts/common/sidebar.tsx`
- Modify: `app/web/src/components/dashboard/quick-actions.tsx`
- Modify: `app/web/src/components/modal/createRoomModal.tsx`
- Modify: `app/web/src/components/room/roomList.tsx`
- Modify: `app/web/src/pages/(app)/domain/$domainUUID/index.tsx`

- [ ] **Step 1: 更新 `dynamicRender.tsx`**

Change:

```ts
const PREFIX_MAP: [string, (...args: any[]) => JSX.Element][] = [
	["/manage", ManageNav],
	["/domain", RoomList],
	["/chat", ConversationList],
	["/", HomePage],
];
```

- [ ] **Step 2: 更新 `layout.tsx`**

Replace `isChannel` with:

```ts
const isDomain = () => location().pathname.startsWith("/domain");
```

Replace every `isChannel()` with `isDomain()`，并在文件顶部引入 `domainStore`。

Mobile bottom nav 中“频道”改为“域”：

```ts
<button
	type="button"
	class={itemClass(isDomain())}
	onClick={() => {
		const uuid = domainStore.state.currentDomainUUID;
		if (uuid) navigate({ to: "/domain/$domainUUID", params: { domainUUID: uuid } });
		else navigate({ to: "/discover" });
	}}
>
	<Headphones size={20} strokeWidth={2.1} />
	<span>域</span>
</button>
```

- [ ] **Step 3: 更新 `sidebar.tsx`**

删除“频道” `OptionSquare`，保留“首页”“聊天”“发现域”“设置”“管理”。域图标点击改为：

```ts
const handleSelect = async (uuid: string) => {
	const previousUUID = state.currentDomainUUID;
	setCurrentDomain(uuid);
	try {
		await navigate({ to: "/domain/$domainUUID", params: { domainUUID: uuid } });
	} catch {
		setCurrentDomain(previousUUID);
	}
};
```

“发现服务器”改为“发现域”。

- [ ] **Step 4: 更新 `quick-actions.tsx`**

`handleJoinRandom` 改为：

```ts
const handleJoinRandom = () => {
	const rooms = socketStore.rooms().filter((room) => room.count < room.limit);
	if (rooms.length === 0) {
		showToast("当前没有可加入的房间", { type: "warning" });
		return;
	}
	const target = [...rooms].sort((a, b) => b.count - a.count)[0];
	socketStore.selectRoom(target);
	const domainUUID = target.domain_uuid;
	if (domainUUID) navigate({ to: "/domain/$domainUUID", params: { domainUUID } });
	else navigate({ to: "/discover" });
};
```

“前往频道列表”改为“前往域”，同样优先跳当前 Domain，否则 `/discover`。

- [ ] **Step 5: 更新 `createRoomModal.tsx`**

`joinAfterCreate` 后改为：

```ts
if (payload.joinAfterCreate) {
	const domainUUID = payload.domainUUID;
	if (domainUUID) navigate({ to: "/domain/$domainUUID", params: { domainUUID } });
	else navigate({ to: "/discover" });
}
```

- [ ] **Step 6: 更新 `roomList.tsx`**

在 `RoomListHeader` 显示当前 Domain 名称，并提供“管理”入口：

```tsx
const currentDomain = createMemo(
	() => domainStore.state.domainCache[socketStore.currentDomainUUID() ?? ""],
);
```

Header 内：

```tsx
<Show when={currentDomain()}>
	<button
		type="button"
		class="btn btn-xs btn-ghost"
		onClick={() =>
			navigate({
				to: "/manage/domains/$domainUUID",
				params: { domainUUID: currentDomain()!.uuid },
			})
		}
	>
		管理
	</button>
</Show>
```

- [ ] **Step 7: 更新 Domain 首页路由**

In `app/web/src/pages/(app)/domain/$domainUUID/index.tsx`，路由改为：

```ts
export const Route = createFileRoute("/(app)/domain/$domainUUID/")({
	component: RouteComponent,
	staticData: { title: "语音域", icon: "icon-channel" },
});
```

所有 `guild`、`服务器` 文案按“域”语义更新。

---

## Task 8: Domain 管理迁入 `/manage/domains`

**Files:**
- Move: `app/web/src/pages/(app)/domain/$domainUUID/manage.tsx` → `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`
- Create: `app/web/src/pages/(app)/manage/domains/index.tsx`
- Modify: `app/web/src/components/manage/manageNav.tsx`
- Modify: `app/web/src/pages/(app)/manage/index.tsx`

- [ ] **Step 1: 移动并改写管理页**

Run:
```bash
cd /Users/noelorin/GOSpeak
mkdir -p 'app/web/src/pages/(app)/manage/domains/$domainUUID'
mv 'app/web/src/pages/(app)/domain/$domainUUID/manage.tsx' 'app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx'
```

改该文件：
- `createFileRoute("/(app)/manage/domains/$domainUUID/")`
- `staticData.title = "域管理"`
- 返回链接改为 `/domain/$domainUUID`
- imports 使用 `@/api/domain`、`@/components/domain/DomainMemberTable`、`@/stores/domainStore`
- 文案“服务器”改为“域”。

- [ ] **Step 2: 创建管理列表页**

Create `app/web/src/pages/(app)/manage/domains/index.tsx`:

```tsx
import { createFileRoute, Link } from "@tanstack/solid-router";
import { createResource, For, Show } from "solid-js";
import { getDomain, myDomains } from "@/api/domain";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";

export const Route = createFileRoute("/(app)/manage/domains/")({
	component: RouteComponent,
	staticData: { title: "域管理", icon: "icon-manage" },
});

function RouteComponent() {
	const [uuids] = createResource(myDomains);
	const [domains] = createResource(uuids, async (ids) => {
		const settled = await Promise.allSettled(ids.map((id) => getDomain(id)));
		return settled
			.filter(
				(r): r is PromiseFulfilledResult<Awaited<ReturnType<typeof getDomain>>> =>
					r.status === "fulfilled",
			)
			.map((r) => r.value);
	});

	return (
		<ManagePage>
			<ManageHeader title="域管理" description="管理我加入的语音域" />
			<ManageSection title="我的域" padded={false}>
				<Show
					when={domains()?.length}
					fallback={<div class="p-5 text-sm text-base-content/50">暂无域</div>}
				>
					<div class="divide-y divide-base-200">
						<For each={domains()}>
							{(domain) => (
								<div class="flex items-center justify-between gap-3 px-4 py-3">
									<div class="min-w-0">
										<div class="font-medium truncate">{domain.name}</div>
										<div class="text-xs text-base-content/50 truncate">
											{domain.description || "暂无描述"}
										</div>
									</div>
									<Link
										to="/manage/domains/$domainUUID"
										params={{ domainUUID: domain.uuid }}
										class="btn btn-sm btn-outline"
									>
										管理
									</Link>
								</div>
							)}
						</For>
					</div>
				</Show>
			</ManageSection>
		</ManagePage>
	);
}
```

- [ ] **Step 3: 管理导航增加“域”**

In `app/web/src/components/manage/manageNav.tsx`，增加：

```ts
type ManagePath = "domains" | "permission" | "sfu" | "users" | "mute" | "ban" | "storage" | "email" | "monitor" | "apikey" | "oauth" | "bot-plugins";
```

加一条 tab：

```ts
{
	path: "domains",
	to: "/manage/domains",
	label: "域",
	icon: ServerCog,
	permissions: ["domain:read"],
},
```

在 `app/web/src/pages/(app)/manage/index.tsx` 的 `MANAGE_PATHS` 中加入 `"domains"`。

---

## Task 9: 前端 UI 文案与测试更新

**Files:** 所有 `app/web/src` 中含 Domain 相关中文“服务器”文案的页面。

- [ ] **Step 1: 批量替换 Domain 专属文案**

Run:

```bash
cd /Users/noelorin/GOSpeak
perl -pi -e '
s/创建语音服务器/创建域/g;
s/语音服务器/语音域/g;
s/发现服务器/发现域/g;
s/新建服务器/新建域/g;
s/加入服务器/加入域/g;
s/进入服务器/进入域/g;
s/服务器管理/域管理/g;
s/服务器名称/域名称/g;
s/公开服务器/公开域/g;
s/服务器设置/域设置/g;
s/离开服务器/离开域/g;
s/删除服务器/删除域/g;
s/移出服务器/移出域/g;
s/服务器列表加载失败/域列表加载失败/g;
s/暂无公开服务器/暂无公开域/g;
s/加入语音域/加入域/g;
s/邀请链接无效或服务器不存在/邀请链接无效或域不存在/g;
' $(rg -l 'guild|语音服务器|服务器管理|发现服务器|新建服务器|加入服务器|进入服务器' app/web/src 2>/dev/null)
```

注意：不要全局替换 `server` 模块、SFU、SMTP 等“服务器”文案；只替换 Domain 专属文案。

- [ ] **Step 2: 更新前端测试**

更新 `domainApi.spec.ts`、`domainStore.spec.ts`、`domainInvite.test.ts`、`DomainInvitePreview.spec.tsx`、`DomainMemberTable.spec.tsx`、`permissions.test.ts` 中所有 `guild`、`/api/v1/guild`、`guild_uuid`、`/guild` 为 domain 对应值。

- [ ] **Step 3: 更新前端文档**

Run:
```bash
cd /Users/noelorin/GOSpeak
perl -pi -e 's#/channel#/domain/$domainUUID#g; s/guild/domain/g; s/Guild/Domain/g' \
  app/web/docs/design/AGENTS-pages.md app/web/docs/lib/tanstack-router.md
```

然后手动删除文档中残留的 `/channel` 示例，改为 `/domain/:domainUUID`。

---

## Task 10: 生成路由树与最终验证

**Files:**
- Regenerate: `app/web/src/routeTree.gen.ts`

- [ ] **Step 1: 重新生成路由树**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/web
pnpm exec tsr generate
```

如果 `tsr generate` 不可用，运行 `pnpm build` 让 Vite 插件重新生成。

- [ ] **Step 2: 检查残留**

Run:
```bash
cd /Users/noelorin/GOSpeak
rg -n -i 'guild' app/server app/web/src app/web/docs app/server/docs --glob '!go.sum' --glob '!gospeak' --glob '!tmp/**'
rg -n -F '/channel' app/web/src app/web/docs
```

Expected: 无 `guild` 残留，无 `/channel` 残留。

- [ ] **Step 3: 后端验证**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
go test ./... -count=1
```

Expected: 全部通过。

- [ ] **Step 4: 前端验证**

Run:
```bash
cd /Users/noelorin/GOSpeak/app/web
pnpm test
pnpm build
pnpm check
```

Expected: Vitest、Vite build、Biome check 全部通过。

- [ ] **Step 5: 手动冒烟**

1. 启动后端 `cd app/server && go run main.go server -e dev`。
2. 启动前端 `cd app/web && pnpm dev`。
3. 打开 `/domain/<cuid>` 应直接进入 Domain 工作区，左侧显示该域房间。
4. 打开 `/manage/domains/<cuid>` 应显示域设置和成员管理。
5. 打开 `/channel` 应 404，不再出现旧频道页。
6. 创建新域后，新 Domain ID 应以 `cuid2.Generate()` 格式生成且不含 `-`。

---

## Self-Review

- 覆盖了所有 `guild` 命名：后端模型、Repository、Service、Handler、Middleware、Router、Signal、前端 API/Store/Component/Route/文案/测试/文档。
- 覆盖了 CUID2 强迁移：新模型生成、旧表改名、旧列改名、旧 UUID 转 CUID2、权限码迁移。
- 覆盖了 `/channel` 移除：删除路由、移除所有引用、文档更新。
- 覆盖了 Domain 管理：`/manage/domains` 列表与 `/manage/domains/:domainUUID` 详情。
- 未提供兼容路由、未保留旧 UUID、未保留旧命名。
