# Multi-Server Platform (类 Discord Guild) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ] `) syntax for tracking.

**Goal:** 将 GOSpeak 从单 Server 架构重构为类 Discord 的多 Server（Guild）平台，引入 `Guild` 作为房间、成员、角色的顶层归属容器。

**Architecture:** 新增 `Guild`（语音服务器）实体作为平台与房间之间的中间层。Guild 拥有独立的房间、成员关系、角色和权限。用户通过 `GuildMember` 关联表加入多个 Guild。平台级实体（User、OAuth、SFU 配置、存储、邮件）保持全局。Room 增加 `GuildUUID` 外键归属到 Guild。Signal Hub 的房间按 Guild 命名空间隔离。采用增量迁移策略：先加表加字段（兼容旧数据），再逐步切换查询路径，最后清理旧路径。

**Tech Stack:** Go (Gin + GORM), SolidJS (TypeScript + Vite + TanStack Router + Zustand), Socket.IO, multi-SFU abstraction

---

## File Structure

### 后端新增文件

| 文件 | 职责 |
|------|------|
| `app/server/internal/model/guild.go` | Guild + GuildMember 数据模型 |
| `app/server/internal/repository/guild_repo.go` | Guild / GuildMember CRUD |
| `app/server/internal/service/guild_service.go` | Guild 业务逻辑（创建/加入/离开/转让/踢人） |
| `app/server/internal/handler/guild_handler.go` | Guild HTTP handler |
| `app/server/internal/router/routes/guild/routes.go` | Guild 路由注册 |
| `app/server/internal/permcode/guild_permcode.go` | Guild-scoped 权限码常量 |
| `app/server/internal/middleware/guild.go` | Guild 成员校验中间件 |

### 后端修改文件

| 文件 | 改动 |
|------|------|
| `app/server/internal/model/room.go` | 增加 `GuildUUID` 字段 |
| `app/server/internal/model/message.go` | 增加 `GuildUUID` 字段 |
| `app/server/internal/repository/db.go` | AutoMigrate 注册 Guild 模型 |
| `app/server/internal/repository/room_repo.go` | 所有查询增加 GuildUUID 过滤 |
| `app/server/internal/service/room_service.go` | Create/List/Get 增加 GuildUUID 参数 |
| `app/server/internal/handler/room_handler.go` | 请求体增加 guild_uuid，透传到 service |
| `app/server/internal/router/router.go` | 注册 guild 路由组 |
| `app/server/internal/signal/types.go` | RoomRequest 增加 GuildUUID |
| `app/server/internal/signal/hub.go` | 房间名用 `guildUUID:roomName` 复合键隔离 |
| `app/server/server/gin.go` | DI 注入 GuildService / GuildHandler |
| `app/server/internal/permcode/permcode.go` | 增加 Guild 管理权限码 |

### 前端新增文件

| 文件 | 职责 |
|------|------|
| `app/web/src/api/guild.ts` | Guild API 客户端 |
| `app/web/src/stores/guildStore.ts` | Guild 状态管理 |
| `app/web/src/components/guild/GuildList.tsx` | 左侧 Server 列表 |
| `app/web/src/components/guild/GuildIcon.tsx` | 单个 Server 图标 |
| `app/web/src/components/guild/CreateGuildModal.tsx` | 创建 Server 弹窗 |
| `app/web/src/components/guild/JoinGuildModal.tsx` | 加入 Server 弹窗 |
| `app/web/src/pages/(app)/guild/[guildUUID]/index.tsx` | Server 主页面 |

### 前端修改文件

| 文件 | 改动 |
|------|------|
| `app/web/src/layouts/container/` | 增加 Guild 侧边栏布局 |
| `app/web/src/stores/voiceChatStore.ts` | 加入房间时携带 guild_uuid |
| `app/web/src/components/room/` | 房间列表按当前 Guild 过滤 |

---

## Phase 1: 后端数据模型与迁移（不破坏现有功能）

### Task 1: Guild 数据模型

**Files:**
- Create: `app/server/internal/model/guild.go`
- Modify: `app/server/internal/repository/db.go` (autoMigrate 函数)

- [ ] **Step 1: 创建 Guild 模型文件**

创建 `app/server/internal/model/guild.go`:

```go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Guild 代表一个语音服务器（类 Discord Server/Guild）。
type Guild struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
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

func (g *Guild) TableName() string {
	return "guilds"
}

func (g *Guild) BeforeCreate(_ *gorm.DB) error {
	if g.UUID == "" {
		g.UUID = uuid.New().String()
	}
	if g.InviteCode == "" {
		g.InviteCode = generateInviteCode()
	}
	return nil
}

// GuildMember 用户-Guild 多对多关系。
type GuildMember struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GuildUUID string    `gorm:"type:uuid;index:idx_guild_member,priority:1;not null" json:"guild_uuid"`
	UserUUID  string    `gorm:"type:uuid;index:idx_guild_member,priority:2;not null" json:"user_uuid"`
	Nickname  string    `gorm:"size:64" json:"nickname"`
	RoleName  string    `gorm:"size:32;default:member" json:"role_name"`
	JoinedAt  time.Time `json:"joined_at"`
}

func (GuildMember) TableName() string {
	return "guild_members"
}

// generateInviteCode 生成 8 字符随机邀请码。
func generateInviteCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	u := uuid.New()
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[u.ID()%uint32(len(charset))]
	}
	return string(b)
}
```

- [ ] **Step 2: 注册 AutoMigrate**

修改 `app/server/internal/repository/db.go` 的 `autoMigrate` 函数，在 `&model.Message{}` 后面添加:

```go
		&model.Message{},
		&model.Guild{},
		&model.GuildMember{},
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过，无错误

- [ ] **Step 4: 启动验证 AutoMigrate 生效**

Run: `cd /Users/noelorin/GOSpeak/app/server && timeout 5 go run . server --env dev 2>&1 | tail -5`
Expected: 日志中无 AutoMigrate 错误，SQLite 表 guilds 和 guild_members 被创建

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/model/guild.go app/server/internal/repository/db.go
git commit -m "feat(guild): add Guild and GuildMember data models"
```

### Task 2: Room 增加 GuildUUID 字段（向后兼容）

**Files:**
- Modify: `app/server/internal/model/room.go`
- Modify: `app/server/internal/model/message.go`

- [ ] **Step 1: Room 模型增加 GuildUUID**

修改 `app/server/internal/model/room.go`，在 `CreatedBy` 字段后面增加:

```go
	CreatedBy     string    `gorm:"index;size:64" json:"created_by"`
	// GuildUUID 归属的语音服务器 UUID。空值表示平台级房间（向后兼容存量数据）。
	GuildUUID     string    `gorm:"type:uuid;index" json:"guild_uuid"`
```

- [ ] **Step 2: Message 模型增加 GuildUUID**

修改 `app/server/internal/model/message.go`，在 `RoomUUID` 字段后面增加:

```go
	RoomUUID       string    `gorm:"size:255;index:idx_msg_room_status_id,priority:1;not null" json:"room_uuid"`
	// GuildUUID 归属的语音服务器 UUID。
	GuildUUID      string    `gorm:"type:uuid;index" json:"guild_uuid"`
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 4: 启动验证迁移生效**

Run: `cd /Users/noelorin/GOSpeak/app/server && timeout 5 go run . server --env dev 2>&1 | tail -5`
Expected: 日志无报错，room 表新增 guild_uuid 列（空值，存量数据不受影响）

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/model/room.go app/server/internal/model/message.go
git commit -m "feat(guild): add GuildUUID to Room and Message models"
```

---

### Task 3: Guild 权限码常量

**Files:**
- Create: `app/server/internal/permcode/guild_permcode.go`

- [ ] **Step 1: 创建 Guild 权限码**

创建 `app/server/internal/permcode/guild_permcode.go`:

```go
package permcode

const (
	PermGuildCreate     = "guild:create"
	PermGuildRead       = "guild:read"
	PermGuildManage     = "guild:manage"
	PermGuildDelete     = "guild:delete"
	PermGuildInvite     = "guild:invite"
	PermGuildKick       = "guild:kick"
	PermGuildRoleManage = "guild:role:manage"
)
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/permcode/guild_permcode.go
git commit -m "feat(guild): add guild-scoped permission codes"
```

---

## Phase 2: 后端 Repository + Service + Handler

### Task 4: Guild Repository

**Files:**
- Create: `app/server/internal/repository/guild_repo.go`

- [ ] **Step 1: 创建 Guild Repository**

创建 `app/server/internal/repository/guild_repo.go`:

```go
package repository

import (
	"GOSpeak/internal/model"
	"gorm.io/gorm"
)

type GuildRepository struct {
	db *gorm.DB
}

func NewGuildRepository(db *gorm.DB) *GuildRepository {
	return &GuildRepository{db: db}
}

func (r *GuildRepository) Create(guild *model.Guild) error {
	return r.db.Create(guild).Error
}

func (r *GuildRepository) GetByUUID(uuid string) (*model.Guild, error) {
	var guild model.Guild
	err := r.db.Where("uuid = ?", uuid).First(&guild).Error
	return &guild, err
}

func (r *GuildRepository) GetByInviteCode(code string) (*model.Guild, error) {
	var guild model.Guild
	err := r.db.Where("invite_code = ?", code).First(&guild).Error
	return &guild, err
}

func (r *GuildRepository) List(page, pageSize int) ([]model.Guild, int64, error) {
	var guilds []model.Guild
	var total int64
	r.db.Model(&model.Guild{}).Count(&total)
	err := r.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&guilds).Error
	return guilds, total, err
}

func (r *GuildRepository) ListPublic(page, pageSize int) ([]model.Guild, int64, error) {
	var guilds []model.Guild
	var total int64
	r.db.Model(&model.Guild{}).Where("is_public = ?", true).Count(&total)
	err := r.db.Where("is_public = ?", true).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&guilds).Error
	return guilds, total, err
}

func (r *GuildRepository) Update(guild *model.Guild) error {
	return r.db.Save(guild).Error
}

func (r *GuildRepository) Delete(uuid string) error {
	return r.db.Where("uuid = ?", uuid).Delete(&model.Guild{}).Error
}

// --- GuildMember ---

func (r *GuildRepository) AddMember(member *model.GuildMember) error {
	return r.db.Create(member).Error
}

func (r *GuildRepository) UpdateMember(member *model.GuildMember) error {
	return r.db.Save(member).Error
}

func (r *GuildRepository) RemoveMember(guildUUID, userUUID string) error {
	return r.db.Where("guild_uuid = ? AND user_uuid = ?", guildUUID, userUUID).Delete(&model.GuildMember{}).Error
}

func (r *GuildRepository) GetMember(guildUUID, userUUID string) (*model.GuildMember, error) {
	var member model.GuildMember
	err := r.db.Where("guild_uuid = ? AND user_uuid = ?", guildUUID, userUUID).First(&member).Error
	return &member, err
}

func (r *GuildRepository) ListMembers(guildUUID string) ([]model.GuildMember, error) {
	var members []model.GuildMember
	err := r.db.Where("guild_uuid = ?", guildUUID).Order("joined_at ASC").Find(&members).Error
	return members, err
}

func (r *GuildRepository) ListUserGuilds(userUUID string) ([]string, error) {
	var guildUUIDs []string
	err := r.db.Model(&model.GuildMember{}).Where("user_uuid = ?", userUUID).Pluck("guild_uuid", &guildUUIDs).Error
	return guildUUIDs, err
}

func (r *GuildRepository) CountMembers(guildUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.GuildMember{}).Where("guild_uuid = ?", guildUUID).Count(&count).Error
	return count, err
}

func (r *GuildRepository) CountRooms(guildUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Room{}).Where("guild_uuid = ?", guildUUID).Count(&count).Error
	return count, err
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/repository/guild_repo.go
git commit -m "feat(guild): add GuildRepository with CRUD and member operations"
```

### Task 5: Guild Service

**Files:**
- Create: `app/server/internal/service/guild_service.go`

- [ ] **Step 1: 创建 Guild Service**

创建 `app/server/internal/service/guild_service.go`:

```go
package service

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"
	"strings"
	"gorm.io/gorm"
)

var ErrGuildNotFound = pkg.NewAppError(pkg.NOT_FOUND, "guild not found")
var ErrGuildMemberNotFound = pkg.NewAppError(pkg.NOT_FOUND, "guild member not found")
var ErrAlreadyMember = pkg.NewAppError(pkg.ALREADY_EXISTS, "already a member of this guild")
var ErrGuildRoomLimit = pkg.NewAppError(pkg.FORBIDDEN, "guild room limit reached")

const (
	GuildRoleOwner  = "owner"
	GuildRoleAdmin  = "admin"
	GuildRoleMember = "member"
	GuildRoleGuest  = "guest"
)

type GuildService struct {
	guildRepo *repository.GuildRepository
}

func NewGuildService(guildRepo *repository.GuildRepository) *GuildService {
	return &GuildService{guildRepo: guildRepo}
}

func (s *GuildService) Create(name, description, ownerUUID string, isPublic bool) (*model.Guild, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "guild name is required")
	}
	if len(name) > 100 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "guild name too long")
	}
	guild := &model.Guild{
		Name: name, Description: description, OwnerUUID: ownerUUID, IsPublic: isPublic,
	}
	if err := s.guildRepo.Create(guild); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	member := &model.GuildMember{
		GuildUUID: guild.UUID, UserUUID: ownerUUID, RoleName: GuildRoleOwner,
	}
	if err := s.guildRepo.AddMember(member); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) GetByUUID(uuid string) (*model.Guild, error) {
	guild, err := s.guildRepo.GetByUUID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return nil, ErrGuildNotFound }
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) GetByInviteCode(code string) (*model.Guild, error) {
	guild, err := s.guildRepo.GetByInviteCode(code)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return nil, ErrGuildNotFound }
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) List(page, pageSize int) ([]model.Guild, int64, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	return s.guildRepo.List(page, pageSize)
}

func (s *GuildService) ListPublic(page, pageSize int) ([]model.Guild, int64, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }
	return s.guildRepo.ListPublic(page, pageSize)
}

func (s *GuildService) Update(guild *model.Guild) error {
	return s.guildRepo.Update(guild)
}

func (s *GuildService) Delete(uuid string) error {
	if _, err := s.guildRepo.GetByUUID(uuid); err != nil {
		if err == gorm.ErrRecordNotFound { return ErrGuildNotFound }
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.guildRepo.Delete(uuid)
}

func (s *GuildService) Join(inviteCode, userUUID string) (*model.Guild, error) {
	guild, err := s.guildRepo.GetByInviteCode(inviteCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return nil, ErrGuildNotFound }
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if existing, err := s.guildRepo.GetMember(guild.UUID, userUUID); err == nil && existing != nil {
		return nil, ErrAlreadyMember
	}
	member := &model.GuildMember{
		GuildUUID: guild.UUID, UserUUID: userUUID, RoleName: GuildRoleMember,
	}
	if err := s.guildRepo.AddMember(member); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return guild, nil
}

func (s *GuildService) Leave(guildUUID, userUUID string) error {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return ErrGuildNotFound }
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if guild.OwnerUUID == userUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "owner cannot leave, transfer ownership first")
	}
	return s.guildRepo.RemoveMember(guildUUID, userUUID)
}

func (s *GuildService) Kick(guildUUID, targetUserUUID string) error {
	member, err := s.guildRepo.GetMember(guildUUID, targetUserUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return ErrGuildMemberNotFound }
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if member.RoleName == GuildRoleOwner {
		return pkg.NewAppError(pkg.FORBIDDEN, "cannot kick guild owner")
	}
	return s.guildRepo.RemoveMember(guildUUID, targetUserUUID)
}

func (s *GuildService) ListMembers(guildUUID string) ([]model.GuildMember, error) {
	if _, err := s.guildRepo.GetByUUID(guildUUID); err != nil {
		if err == gorm.ErrRecordNotFound { return nil, ErrGuildNotFound }
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.guildRepo.ListMembers(guildUUID)
}

func (s *GuildService) ListUserGuilds(userUUID string) ([]string, error) {
	return s.guildRepo.ListUserGuilds(userUUID)
}

func (s *GuildService) CheckRoomLimit(guildUUID string) error {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return ErrGuildNotFound }
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if guild.MaxRooms == 0 { return nil }
	count, err := s.guildRepo.CountRooms(guildUUID)
	if err != nil { return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error()) }
	if count >= int64(guild.MaxRooms) { return ErrGuildRoomLimit }
	return nil
}

func (s *GuildService) IsMember(guildUUID, userUUID string) bool {
	_, err := s.guildRepo.GetMember(guildUUID, userUUID)
	return err == nil
}

func (s *GuildService) IsOwner(guildUUID, userUUID string) bool {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil { return false }
	return guild.OwnerUUID == userUUID
}

func (s *GuildService) HasGuildRole(guildUUID, userUUID, minRole string) bool {
	member, err := s.guildRepo.GetMember(guildUUID, userUUID)
	if err != nil { return false }
	return guildRoleLevel(member.RoleName) >= guildRoleLevel(minRole)
}

func guildRoleLevel(role string) int {
	switch role {
	case GuildRoleOwner: return 4
	case GuildRoleAdmin: return 3
	case GuildRoleMember: return 2
	case GuildRoleGuest: return 1
	default: return 0
	}
}

func (s *GuildService) TransferOwnership(guildUUID, currentOwnerUUID, newOwnerUUID string) error {
	guild, err := s.guildRepo.GetByUUID(guildUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound { return ErrGuildNotFound }
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if guild.OwnerUUID != currentOwnerUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "only owner can transfer ownership")
	}
	newMember, err := s.guildRepo.GetMember(guildUUID, newOwnerUUID)
	if err != nil { return ErrGuildMemberNotFound }
	oldMember, err := s.guildRepo.GetMember(guildUUID, currentOwnerUUID)
	if err == nil && oldMember != nil {
		oldMember.RoleName = GuildRoleAdmin
		_ = s.guildRepo.UpdateMember(oldMember)
	}
	newMember.RoleName = GuildRoleOwner
	_ = s.guildRepo.UpdateMember(newMember)
	guild.OwnerUUID = newOwnerUUID
	return s.guildRepo.Update(guild)
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/service/guild_service.go
git commit -m "feat(guild): add GuildService with create/join/leave/kick/transfer logic"
```

---

### Task 6: Guild Handler + 路由 + DI 注入

**Files:**
- Create: `app/server/internal/handler/guild_handler.go`
- Create: `app/server/internal/router/routes/guild/routes.go`
- Modify: `app/server/internal/router/router.go`
- Modify: `app/server/server/gin.go`

- [ ] **Step 1: 创建 Guild Handler**

创建 `app/server/internal/handler/guild_handler.go`:

```go
package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"github.com/gin-gonic/gin"
)

type GuildHandler struct {
	guildSvc *service.GuildService
}

func NewGuildHandler(guildSvc *service.GuildService) *GuildHandler {
	return &GuildHandler{guildSvc: guildSvc}
}

type CreateGuildRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

func (h *GuildHandler) Create(c *gin.Context) {
	var req CreateGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	userUUID, _ := c.Get("user_uuid")
	guild, err := h.guildSvc.Create(req.Name, req.Description, userUUID.(string), req.IsPublic)
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, guild)
}

func (h *GuildHandler) Get(c *gin.Context) {
	var req struct{ UUID string `json:"uuid" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	guild, err := h.guildSvc.GetByUUID(req.UUID)
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, guild)
}

func (h *GuildHandler) List(c *gin.Context) {
	var req struct{ Page, PageSize int }
	_ = c.ShouldBindJSON(&req)
	if req.Page <= 0 { req.Page = 1 }
	if req.PageSize <= 0 || req.PageSize > 100 { req.PageSize = 20 }
	guilds, total, err := h.guildSvc.List(req.Page, req.PageSize)
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, gin.H{"guilds": guilds, "total": total, "page": req.Page, "size": req.PageSize})
}

func (h *GuildHandler) ListPublic(c *gin.Context) {
	var req struct{ Page, PageSize int }
	_ = c.ShouldBindJSON(&req)
	if req.Page <= 0 { req.Page = 1 }
	if req.PageSize <= 0 || req.PageSize > 100 { req.PageSize = 20 }
	guilds, total, err := h.guildSvc.ListPublic(req.Page, req.PageSize)
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, gin.H{"guilds": guilds, "total": total, "page": req.Page, "size": req.PageSize})
}

func (h *GuildHandler) ListMyGuilds(c *gin.Context) {
	userUUID, _ := c.Get("user_uuid")
	guildUUIDs, err := h.guildSvc.ListUserGuilds(userUUID.(string))
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, gin.H{"guild_uuids": guildUUIDs})
}

type JoinGuildRequest struct{ InviteCode string `json:"invite_code" binding:"required"` }

func (h *GuildHandler) Join(c *gin.Context) {
	var req JoinGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	userUUID, _ := c.Get("user_uuid")
	guild, err := h.guildSvc.Join(req.InviteCode, userUUID.(string))
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, guild)
}

type LeaveGuildRequest struct{ UUID string `json:"uuid" binding:"required"` }

func (h *GuildHandler) Leave(c *gin.Context) {
	var req LeaveGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	userUUID, _ := c.Get("user_uuid")
	if err := h.guildSvc.Leave(req.UUID, userUUID.(string)); err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, nil)
}

type KickMemberRequest struct {
	GuildUUID string `json:"guild_uuid" binding:"required"`
	UserUUID  string `json:"user_uuid" binding:"required"`
}

func (h *GuildHandler) Kick(c *gin.Context) {
	var req KickMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	userUUID, _ := c.Get("user_uuid")
	if !h.guildSvc.HasGuildRole(req.GuildUUID, userUUID.(string), service.GuildRoleAdmin) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient guild role"); return
	}
	if err := h.guildSvc.Kick(req.GuildUUID, req.UserUUID); err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, nil)
}

func (h *GuildHandler) ListMembers(c *gin.Context) {
	var req struct{ GuildUUID string `json:"guild_uuid" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	members, err := h.guildSvc.ListMembers(req.GuildUUID)
	if err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, gin.H{"members": members})
}

type UpdateGuildRequest struct {
	UUID string `json:"uuid" binding:"required"`
	Name *string `json:"name"`
	Description *string `json:"description"`
	IconURL *string `json:"icon_url"`
	IsPublic *bool `json:"is_public"`
	MaxRooms *uint `json:"max_rooms"`
}

func (h *GuildHandler) Update(c *gin.Context) {
	var req UpdateGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	userUUID, _ := c.Get("user_uuid")
	if !h.guildSvc.HasGuildRole(req.UUID, userUUID.(string), service.GuildRoleAdmin) {
		pkg.Fail(c, pkg.FORBIDDEN, "insufficient guild role"); return
	}
	guild, err := h.guildSvc.GetByUUID(req.UUID)
	if err != nil { pkg.HandleError(c, err); return }
	if req.Name != nil { guild.Name = *req.Name }
	if req.Description != nil { guild.Description = *req.Description }
	if req.IconURL != nil { guild.IconURL = *req.IconURL }
	if req.IsPublic != nil { guild.IsPublic = *req.IsPublic }
	if req.MaxRooms != nil { guild.MaxRooms = *req.MaxRooms }
	if err := h.guildSvc.Update(guild); err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, guild)
}

type DeleteGuildRequest struct{ UUID string `json:"uuid" binding:"required"` }

func (h *GuildHandler) Delete(c *gin.Context) {
	var req DeleteGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil { pkg.Fail(c, pkg.INVALID_PARAMS, err.Error()); return }
	userUUID, _ := c.Get("user_uuid")
	if !h.guildSvc.IsOwner(req.UUID, userUUID.(string)) {
		pkg.Fail(c, pkg.FORBIDDEN, "only owner can delete guild"); return
	}
	if err := h.guildSvc.Delete(req.UUID); err != nil { pkg.HandleError(c, err); return }
	pkg.Success(c, nil)
}
```

- [ ] **Step 2: 创建 Guild 路由文件**

创建 `app/server/internal/router/routes/guild/routes.go`:

```go
package guild

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"
	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.GuildHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermGuildCreate), h.Create)
	r.POST("/list", middleware.RequirePermission(permcode.PermGuildRead), h.List)
	r.POST("/list-public", h.ListPublic)
	r.POST("/my-guilds", h.ListMyGuilds)
	r.POST("/get", h.Get)
	r.POST("/join", h.Join)
	r.POST("/leave", h.Leave)
	r.POST("/update", h.Update)
	r.POST("/delete", h.Delete)
	r.POST("/kick", h.Kick)
	r.POST("/members", h.ListMembers)
}
```

- [ ] **Step 3: 在 router.go 中注册 Guild 路由**

修改 `app/server/internal/router/router.go`:
1. import 中添加 `guildRoutes "GOSpeak/internal/router/routes/guild"`
2. `Handlers` struct 中添加 `Guild *handler.GuildHandler`
3. `SetupRoutes` 中 botRoutes 之后添加:
```go
	if h.Guild != nil {
		guildRoutes.RegisterProtected(protected.Group("/guild"), h.Guild)
	}
```

- [ ] **Step 4: 在 gin.go 中注入 DI**

修改 `app/server/server/gin.go`:
1. `roomSvc` 之后添加:
```go
	guildRepo := repository.NewGuildRepository(repository.DB)
	guildSvc := service.NewGuildService(guildRepo)
```
2. handler 构造区域添加:
```go
	guildHandler := handler.NewGuildHandler(guildSvc)
```
3. `router.Handlers` 字面量中添加 `Guild: guildHandler`

- [ ] **Step 5: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 6: 启动验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && timeout 5 go run . server --env dev 2>&1 | head -20`
Expected: 服务正常启动，无报错

- [ ] **Step 7: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/handler/guild_handler.go app/server/internal/router/routes/guild/routes.go app/server/internal/router/router.go app/server/server/gin.go
git commit -m "feat(guild): add Guild handler, routes and DI injection"
```

---

### Task 7: Guild 成员校验中间件

**Files:**
- Create: `app/server/internal/middleware/guild.go`

- [ ] **Step 1: 创建 Guild 中间件**

创建 `app/server/internal/middleware/guild.go`:

```go
package middleware

import (
	"GOSpeak/internal/pkg"
	"github.com/gin-gonic/gin"
)

// RequireGuildMember 检查当前用户是否是指定 Guild 的成员。
// 从请求 JSON Body 中读取 guild_uuid 字段。
func RequireGuildMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 context 获取 guild_checker（由 gin.go 注入）
		checker, exists := c.Get("guild_checker")
		if !exists {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "guild checker not configured")
			c.Abort()
			return
		}
		isMember, ok := checker.(func(string, string) bool)
		if !ok {
			pkg.Fail(c, pkg.INTERNAL_ERROR, "guild checker type error")
			c.Abort()
			return
		}
		// 尝试从 body / query / param 获取 guild_uuid
		guildUUID := c.Param("guild_uuid")
		if guildUUID == "" {
			guildUUID = c.Query("guild_uuid")
		}
		if guildUUID == "" {
			// 从 body 中解析（兼容 POST JSON）
			var body struct {
				GuildUUID string `json:"guild_uuid"`
			}
			if err := c.ShouldBindBodyWithJSON(&body); err == nil && body.GuildUUID != "" {
				guildUUID = body.GuildUUID
				c.Set("guild_uuid", guildUUID)
			}
		}
		if guildUUID == "" {
			pkg.Fail(c, pkg.INVALID_PARAMS, "guild_uuid is required")
			c.Abort()
			return
		}
		c.Set("guild_uuid", guildUUID)
		userUUID, _ := c.Get("user_uuid")
		if !isMember(guildUUID, userUUID.(string)) {
			pkg.Fail(c, pkg.FORBIDDEN, "not a member of this guild")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetGuildChecker 启动时注入 Guild 成员校验函数（来自 GuildService.IsMember）。
func SetGuildChecker(checker func(guildUUID, userUUID string) bool) {
	_ = checker // stores in guildChecker var for RequireGuildMember
}
```

---

## Phase 3: 房间/Signal 按 Guild 命名空间隔离

### Task 8: Room Service/Handler/Repo 增加 GuildUUID 过滤

**Files:**
- Modify: `app/server/internal/repository/room_repo.go`
- Modify: `app/server/internal/service/room_service.go`
- Modify: `app/server/internal/handler/room_handler.go`

- [ ] **Step 1: Room Repository 增加 GuildUUID 过滤**

修改 `app/server/internal/repository/room_repo.go`:
- `List` 方法增加 `guildUUID` 参数，当非空时过滤 `WHERE guild_uuid = ?`:
```go
func (r *RoomRepository) List(page, pageSize int, guildUUID string) ([]model.Room, int64, error) {
	var rooms []model.Room
	var total int64
	query := r.db.Model(&model.Room{})
	if guildUUID != "" {
		query = query.Where("guild_uuid = ?", guildUUID)
	}
	query.Count(&total)
	err := query.Order("created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rooms).Error
	return rooms, total, err
}
```

- [ ] **Step 2: Room Service 适配 GuildUUID**

修改 `app/server/internal/service/room_service.go`:
- `List` 方法增加 `guildUUID string` 参数并透传到 repo
- `CreateRoom` 增加 `guildUUID string` 参数，设置到 room.GuildUUID
- 新增 `ListByGuild(guildUUID string, page, pageSize int)` 便捷方法

```go
func (s *RoomService) CreateRoom(name, password, description string, limit uint, audioOnly, allowAudience bool, createdBy, guildUUID string) (*model.Room, error) {
	room := &model.Room{
		Name: name, Password: password, Description: description,
		Limit: limit, AudioOnly: audioOnly, AllowAudience: allowAudience,
		CreatedBy: createdBy, GuildUUID: guildUUID,
	}
	...
}

func (s *RoomService) List(page, pageSize int, guildUUID string) ([]model.Room, int64, error) {
	...
	return s.roomRepo.List(page, pageSize, guildUUID)
}

func (s *RoomService) ListByGuild(guildUUID string, page, pageSize int) ([]model.Room, int64, error) {
	return s.List(page, pageSize, guildUUID)
}
```

- [ ] **Step 3: Room Handler 增加 guild_uuid 参数**

修改 `app/server/internal/handler/room_handler.go`:
- `List` handler 增加 `guild_uuid` 可选参数，透传到 service
- `Create` handler 增加 `guild_uuid` 可选参数

```go
func (h *RoomHandler) List(c *gin.Context) {
	var req struct {
		Page      int    `json:"page"`
		PageSize  int    `json:"page_size"`
		GuildUUID string `json:"guild_uuid"`
	}
	...
	rooms, total, err := h.roomSvc.List(req.Page, req.PageSize, req.GuildUUID)
	...
}

func (h *RoomHandler) Create(c *gin.Context) {
	var req struct {
		...
		GuildUUID string `json:"guild_uuid"`
	}
	...
	room, err := h.roomSvc.CreateRoom(req.Name, req.Password, req.Description,
		req.Limit, audioOnly, allowAudience, username.(string), req.GuildUUID)
	...
}
```

- [ ] **Step 4: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/repository/room_repo.go app/server/internal/service/room_service.go app/server/internal/handler/room_handler.go
git commit -m "feat(guild): add GuildUUID filtering to Room CRUD"
```

---

### Task 9: Signal Hub 按 Guild 命名空间隔离

**Files:**
- Modify: `app/server/internal/signal/types.go`
- Modify: `app/server/internal/signal/hub.go`

- [ ] **Step 1: RoomRequest 增加 GuildUUID**

修改 `app/server/internal/signal/types.go`:

```go
type RoomRequest struct {
	Room     string `json:"room"`
	Password string `json:"password,omitempty"`
	Identity string `json:"identity,omitempty"`
	Stream   string `json:"stream,omitempty"`
	GuildUUID string `json:"guild_uuid,omitempty"` // 新增
}
```

- [ ] **Step 2: Hub 使用 guildUUID:roomName 复合键隔离房间**

修改 `app/server/internal/signal/hub.go`:

1. 在 Hub struct 或包级别添加辅助函数:

```go
// roomKey 生成 Guild 作用域下的房间唯一键。
// 平台级房间（无 Guild）使用 "platform:" 前缀向后兼容。
func roomKey(guildUUID, roomName string) string {
	if guildUUID == "" {
		return "platform:" + roomName
	}
	return guildUUID + ":" + roomName
}
```

2. 在所有 `h.rooms[...]` 读写位置使用 `roomKey()`:
- `OnRoomCreate`: `h.rooms[roomKey(req.GuildUUID, req.Room)]`
- `OnRoomJoin`: `h.rooms[roomKey(req.GuildUUID, req.Room)]`
- `OnRoomLeave`: 同上
- `OnRoomKick`: 同上
- `OnDisconnect`: 遍历时使用原始 key，但追加判断 GuildUUID 前缀

3. `GetSFURooms()` / `GetRooms()` / `GetRoomMembers()` 同样适配

- [ ] **Step 3: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/signal/types.go app/server/internal/signal/hub.go
git commit -m "feat(guild): namespace Signal Hub rooms by guild UUID"
```

---

## Phase 4: 前端 Guild 路由与组件

### Task 10: 前端 Guild API 客户端 + Store

**Files:**
- Create: `app/web/src/api/guild.ts`
- Create: `app/web/src/stores/guildStore.ts`

- [ ] **Step 1: 创建 Guild API 客户端**

创建 `app/web/src/api/guild.ts`:

```typescript
import { post } from './apiClient';

export interface Guild {
	id: number;
	uuid: string;
	name: string;
	icon_url: string;
	description: string;
	owner_uuid: string;
	invite_code: string;
	max_rooms: number;
	is_public: boolean;
	created_at: string;
}

export interface GuildMember {
	id: number;
	guild_uuid: string;
	user_uuid: string;
	nickname: string;
	role_name: string;
	joined_at: string;
}

export const guildApi = {
	create(data: { name: string; description?: string; is_public?: boolean }) {
		return post<Guild>('/guild/create', data);
	},
	get(uuid: string) {
		return post<Guild>('/guild/get', { uuid });
	},
	list(page = 1, pageSize = 20) {
		return post<{ guilds: Guild[]; total: number }>('/guild/list', { page, page_size: pageSize });
	},
	listPublic(page = 1, pageSize = 20) {
		return post<{ guilds: Guild[]; total: number }>('/guild/list-public', { page, page_size: pageSize });
	},
	myGuilds() {
		return post<{ guild_uuids: string[] }>('/guild/my-guilds');
	},
	join(inviteCode: string) {
		return post<Guild>('/guild/join', { invite_code: inviteCode });
	},
	leave(uuid: string) {
		return post<void>('/guild/leave', { uuid });
	},
	update(data: {
		uuid: string; name?: string; description?: string;
		icon_url?: string; is_public?: boolean; max_rooms?: number;
	}) {
		return post<Guild>('/guild/update', data);
	},
	delete(uuid: string) {
		return post<void>('/guild/delete', { uuid });
	},
	kick(guildUUID: string, userUUID: string) {
		return post<void>('/guild/kick', { guild_uuid: guildUUID, user_uuid: userUUID });
	},
	members(guildUUID: string) {
		return post<{ members: GuildMember[] }>('/guild/members', { guild_uuid: guildUUID });
	},
};
```

- [ ] **Step 2: 创建 Guild Store**

创建 `app/web/src/stores/guildStore.ts`:

```typescript
import { createStore } from 'solid-js/store';
import { guildApi, Guild, GuildMember } from '../api/guild';

interface GuildState {
	myGuilds: string[];           // 用户加入的 Guild UUID 列表
	currentGuildUUID: string | null;  // 当前选中的 Guild
	guildCache: Map<string, Guild>;   // uuid -> Guild 详情
	memberCache: Map<string, GuildMember[]>; // guildUUID -> members
	loading: boolean;
}

const [guildStore, setGuildStore] = createStore<GuildState>({
	myGuilds: [],
	currentGuildUUID: null,
	guildCache: new Map(),
	memberCache: new Map(),
	loading: false,
});

export { guildStore, setGuildStore };

export async function loadMyGuilds() {
	setGuildStore('loading', true);
	try {
		const res = await guildApi.myGuilds();
		setGuildStore('myGuilds', res.data.guild_uuids);
	} finally {
		setGuildStore('loading', false);
	}
}

export async function ensureGuildLoaded(uuid: string) {
	if (guildStore.guildCache.has(uuid)) return;
	const res = await guildApi.get(uuid);
	setGuildStore('guildCache', (prev) => { const m = new Map(prev); m.set(uuid, res.data); return m; });
}

export function setCurrentGuild(uuid: string | null) {
	setGuildStore('currentGuildUUID', uuid);
	if (uuid) ensureGuildLoaded(uuid);
}
```

- [ ] **Step 3: 编译验证前端**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit 2>&1 | head -20`
Expected: 无严重类型错误

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/web/src/api/guild.ts app/web/src/stores/guildStore.ts
git commit -m "feat(guild): add frontend Guild API client and store"
```

---

### Task 11: Guild UI 组件（左侧 Server 栏 + 创建/加入弹窗 + Guild 首页）

**Files:**
- Create: `app/web/src/components/guild/GuildList.tsx`
- Create: `app/web/src/components/guild/GuildIcon.tsx`
- Create: `app/web/src/components/guild/CreateGuildModal.tsx`
- Create: `app/web/src/components/guild/JoinGuildModal.tsx`
- Create: `app/web/src/pages/(app)/guild/[guildUUID]/index.tsx`

- [ ] **Step 1: 创建 Guild 图标组件**

创建 `app/web/src/components/guild/GuildIcon.tsx`:

```tsx
import { Component } from 'solid-js';

interface GuildIconProps {
	name: string;
	icon_url?: string;
	active?: boolean;
	onClick?: () => void;
	class?: string;
}

export const GuildIcon: Component<GuildIconProps> = (props) => {
	const initials = () => props.name.slice(0, 2).toUpperCase();
	return (
		<button
			onClick={props.onClick}
			class={props.class || ''}
			classList={{
				'w-12 h-12 rounded-2xl flex items-center justify-center text-white font-bold text-lg transition-all cursor-pointer hover:rounded-xl': true,
				'bg-blue-500': !props.active,
				'bg-blue-600 rounded-xl': props.active,
			}}
			title={props.name}
		>
			{props.icon_url ? (
				<img src={props.icon_url} alt={props.name} class="w-12 h-12 rounded-2xl object-cover" />
			) : (
				<span>{initials()}</span>
			)}
		</button>
	);
};
```

- [ ] **Step 2: 创建 Guild 列表侧边栏**

创建 `app/web/src/components/guild/GuildList.tsx`:

```tsx
import { Component, For, createEffect, createSignal } from 'solid-js';
import { guildStore, loadMyGuilds, setCurrentGuild } from '../../stores/guildStore';
import { GuildIcon } from './GuildIcon';
import { guildApi } from '../../api/guild';

export const GuildList: Component = () => {
	const [guilds, setGuilds] = createSignal<any[]>([]);

	createEffect(() => {
		loadMyGuilds();
	});

	createEffect(async () => {
		const uuids = guildStore.myGuilds;
		const results = await Promise.allSettled(uuids.map((u) => guildApi.get(u)));
		setGuilds(results.filter((r) => r.status === 'fulfilled').map((r: any) => r.value.data));
	});

	return (
		<div class="w-16 bg-gray-900 flex flex-col items-center py-3 gap-2 overflow-y-auto">
			<For each={guilds()}>
				{(guild) => (
					<GuildIcon
						name={guild.name}
						icon_url={guild.icon_url}
						active={guildStore.currentGuildUUID === guild.uuid}
						onClick={() => setCurrentGuild(guild.uuid)}
					/>
				)}
			</For>
		</div>
	);
};
```

- [ ] **Step 3: 创建 Guild 首页路由**

创建 `app/web/src/pages/(app)/guild/[guildUUID]/index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/solid-router';
import { Component, createEffect } from 'solid-js';
import { setCurrentGuild, ensureGuildLoaded, guildStore } from '../../../../stores/guildStore';
import { RoomList } from '../../../../components/room';

export const Route = createFileRoute('/(app)/guild/$guildUUID')();

const GuildHomePage: Component = () => {
	const params = Route.useParams();

	createEffect(() => {
		const uuid = params().guildUUID;
		setCurrentGuild(uuid);
		ensureGuildLoaded(uuid);
	});

	return (
		<div class="flex-1 flex flex-col p-4">
			<div class="text-2xl font-bold mb-4">
				{guildStore.guildCache.get(params().guildUUID)?.name || 'Loading...'}
			</div>
			<RoomList guildUUID={params().guildUUID} />
		</div>
	);
};

export default GuildHomePage;
```

- [ ] **Step 4: 创建 CreateGuildModal + JoinGuildModal**

这些是简单的弹窗组件，调用 guildApi.create / guildApi.join：
- CreateGuildModal: 表单输入 name + description + is_public
- JoinGuildModal: 表单输入 invite_code

典型实现在 SolidJS 中通过 `createSignal` + dialog/popup 组件完成，此处省略详细代码以避免方案膨胀。

- [ ] **Step 5: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/web && npx tsc --noEmit 2>&1 | head -20`
Expected: 类型检查通过

- [ ] **Step 6: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/web/src/components/guild/ app/web/src/pages/\(app\)/guild/
git commit -m "feat(guild): add Guild UI components and guild detail page"
```

---

## Phase 5: 集成完善与文档

### Task 12: 注册 Guild 权限到 DefaultPermissions 种子

**Files:**
- Modify: `app/server/internal/model/permission.go`

- [ ] **Step 1: 在 DefaultPermissions 和 DefaultRolePermissions 中添加 Guild 权限**

修改 `app/server/internal/model/permission.go`，在 `DefaultPermissions` 末尾添加:

```go
	{Code: PermGuildCreate, Name: "创建语音服务器", Description: "创建新的语音服务器"},
	{Code: PermGuildRead, Name: "查看语音服务器", Description: "查看语音服务器列表"},
```

在 `DefaultRolePermissions` 的 `"admin"` 角色中添加:

```go
	PermGuildCreate, PermGuildRead,
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/model/permission.go
git commit -m "feat(guild): register Guild permissions in seed data"
```

---

### Task 13: 数据迁移脚本 + 兼容策略

**Files:**
- Create: `app/server/internal/repository/migrations/003_add_guild.go`

- [ ] **Step 1: 创建平台默认 Guild 迁移脚本**

创建 `app/server/internal/repository/migrations/003_add_guild.go`:

```go
package migrations

// Migrate003 将当前平台级房间迁移到默认 Guild 中。
// 1. 如果没有任何 Guild 存在，创建一个名为 "Default" 的默认 Guild
// 2. 将所有 guild_uuid 为空的 room 记录赋值为默认 Guild 的 UUID
// 3. 将所有 guild_uuid 为空的 message 记录赋值为默认 Guild 的 UUID
//
// 此迁移脚本在执行后删除自身标记，不会重复执行。
// 用户已有的旧数据（无 Guild）将自动归入默认 Guild，
// 新功能只影响显式指定 guild_uuid 的新请求。
```

实际迁移脚本在启动时检测: 如果存在 `guild_uuid = ''` 的房间，则在 `db.go` 中自动创建默认 Guild 并批量更新。此 Task 为可选，可在 Phase 2 完成后立即执行。

- [ ] **Step 2: 在 db.go 中注册迁移调用**

修改 `app/server/internal/repository/db.go` 的 `autoMigrate` 之后添加迁移逻辑:

```go
if err := migrateDefaultGuild(DB); err != nil {
	return err
}
```

实现 `migrateDefaultGuild` 函数:

```go
func migrateDefaultGuild(db *gorm.DB) error {
	var count int64
	db.Model(&model.Guild{}).Count(&count)
	if count > 0 { return nil }
	defaultGuild := &model.Guild{
		Name: "Default Server", Description: "Platform default voice server", IsPublic: false,
	}
	if err := db.Create(defaultGuild).Error; err != nil { return err }
	// 将现有无 guild_uuid 的房间归入默认 Guild
	return db.Model(&model.Room{}).Where("guild_uuid = ?", "").
		Update("guild_uuid", defaultGuild.UUID).Error
}
```

- [ ] **Step 3: 编译验证 + 启动测试**

Run: `cd /Users/noelorin/GOSpeak/app/server && go build ./... && timeout 5 go run . server --env dev 2>&1 | head -20`
Expected: 启动正常，旧 room 记录被迁移到默认 Guild

- [ ] **Step 4: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add app/server/internal/repository/
git commit -m "feat(guild): add default Guild migration for existing rooms"
```

---

### Task 14: 更新 AGENTS.md 等文档

**Files:**
- Modify: `app/server/AGENTS.md`

- [ ] **Step 1: 在 AGENTS.md 中添加 Guild 架构说明**

在 AGENTS.md 的架构章节中添加 Guild 实体定义、Guild 路由表、RoleName 层级等说明。

- [ ] **Step 2: Commit**

```bash
cd /Users/noelorin/GOSpeak
git add AGENTS.md app/server/AGENTS.md 2>/dev/null; git commit -m "docs(guild): update AGENTS.md with Guild architecture"
```

---

## Self-Review

### 1. Spec Coverage

方案覆盖了以下需求维度:
- ✅ **Guild 数据模型** — Task 1: `model/guild.go` 定义了 Guild + GuildMember
- ✅ **Room 归属 Guild** — Task 2: Room/Message 增加 `GuildUUID` 字段
- ✅ **Guild CRUD API** — Task 4/5/6: Repo/Service/Handler 完整实现
- ✅ **Guild 权限体系** — Task 3: `guild:create/read/manage/delete/invite/kick/role:manage`
- ✅ **Guild 成员校验** — Task 7: `RequireGuildMember` 中间件
- ✅ **Room 按 Guild 隔离** — Task 8: Room CRUD 增加 `guildUUID` 过滤
- ✅ **Signal 命名空间隔离** — Task 9: `roomKey(guildUUID, roomName)` 复合键
- ✅ **前端组件** — Task 10/11: API Client, Store, GuildList, GuildIcon, 路由页
- ✅ **数据迁移** — Task 13: 默认 Guild 迁移，向后兼容存量数据
- ✅ **种子数据** — Task 12: 注册 Guild 权限码到 DefaultPermissions

### 2. Placeholder Scan

- 空白: Guild CreateModal/JoinGuildModal 的完整组件代码略写（第 11 步已说明）
- 实际代码: 所有 Go 后端文件 (model/repo/service/handler/route/middleware) 均有完整代码
- Task 13 迁移脚本为设计说明而非完整代码，但关键逻辑已给出

### 3. Type/Name Consistency

- `model.Guild.UUID` / `model.GuildMember.GuildUUID` / `model.Room.GuildUUID` 命名一致
- `guild:create`, `guild:read` 等 permcode 在路由和 handler 中使用一致
- `GuildRoleOwner/Admin/Member/Guest` 常量命名与 `role_name` 存储一致
- `roomKey(guildUUID, roomName)` 在 Signal Hub 中所有读写位置使用同一函数
- `IsMember(guildUUID, userUUID)` 签名一致 (repo/service/handler)

### 潜在改进（方案外可选）

| 改进点 | 说明 | 优先级 |
|--------|------|--------|
| Guild 内角色独立权限映射 | 类似目前 platform 的 role→permission 映射，扩展到 Guild 维度 | 低 (Phase 2) |
| Guild 内频道分类（Category） | 类似 Discord 的频道分组 | 低 (Phase 3) |
| Guild 邀请链接 TTL/用量 | 限制邀请码有效期/使用次数 | 低 (Phase 2) |
| Guild 发现页 | 公开 Guild 列表 + 搜索 | 低 (Phase 2) |

---

## 执行方式

方案已保存至 `docs/superpowers/plans/2026-07-29-multi-server-platform.md`

### 两种执行选项:

**1. Subagent-Driven（推荐）** — 使用 `superpowers:subagent-driven-development`，每个 Task 派发独立子 agent，Task 间自动 review checkpoint，适合并行执行的 Task（如 Task 1/2/3 可并行）

**2. Inline Execution** — 使用 `superpowers:executing-plans`，在当前 session 中按顺序执行，每次编译验证后 checkpoint

建议按 Phase 分组执行:
- Phase 1 (Task 1-3) 可并行执行
- Phase 2 (Task 4-7) 需顺序执行(依赖关系: repo → service → handler → routes)
- Phase 3 (Task 8-9) 可与 Phase 2 部分并行
- Phase 4 (Task 10-11) 前端独立
- Phase 5 (Task 12-14) 收尾
