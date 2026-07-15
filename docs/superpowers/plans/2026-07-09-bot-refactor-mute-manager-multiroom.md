# Bot 体系重构 + 禁言管理 + 多房间支持 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除旧 `gk_` BotAPIKey 体系，改为 Bot 以 `users` 表记录 (IsBot=true) 写入 + JWT 同构认证，同时使 Bot 具备禁言管理能力和多房间接入能力。

**Architecture:** Bot 不再使用独立的 API Key 签名体系（bcrypt + gk_ 前缀），而是直接作为 `users` 表中一条 `IsBot=true` 的记录，通过 `pkg.GenerateToken()` 签发标准 JWT，与用户 token 同构（同一套 Claims/签名/中间件）。新增 `BotToken` 表作为管理层记录（只记元信息，不记 token 值）。禁言管理使用现有 `POST /mute/create` 等端点，Bot 的 `user_uuid` 可直接查到。多房间支持通过扩展 Bot Node 框架的 `socketClient`/`botRunner` 实现，服务端 `hub.go` 不变。

**Tech Stack:** Go (Gin/GORM), TypeScript (socket.io-client v2), pnpm monorepo

---

## File Structure

### Server — 删除旧体系 (4 files)

| Action | File | Responsibility |
|--------|------|----------------|
| DELETE | `app/server/internal/model/bot_apikey.go` | 旧 gk_ + bcrypt + AdminOnlyPermissions 模型 |
| DELETE | `app/server/internal/repository/bot_apikey_repo.go` | 旧 DAO：Create/ListActive/GetByUUID/Revoke/TouchLastUsed |
| DELETE | `app/server/internal/service/bot_apikey_service.go` | 旧 Resolve(bcrypt 比对) + Create/List/Revoke 逻辑 |
| DELETE | `app/server/internal/handler/bot_apikey_handler.go` | 旧 HTTP 端点 (/bot/key/create, /bot/key/list, /bot/key/revoke) |

### Server — 修改 (7 files)

| Action | File | Responsibility |
|--------|------|----------------|
| MODIFY | `app/server/internal/model/user.go` | 加 `IsBot bool` 字段 |
| MODIFY | `app/server/internal/middleware/auth.go` | 删除 BotKeyAuth/checkBotPermissions/BotKeyResolver/BotKeyInfo；保留 JWTAuth/WSAuth/RequirePermission 不变 |
| MODIFY | `app/server/internal/router/router.go` | 移除 `protected.Use(BotKeyAuth)`，`SetupRoutes` 签名去掉 botKeySvc 参数 |
| MODIFY | `app/server/internal/server/gin.go` | 删除旧 botKeyRepo/svc/handler 实例化，接入新 BotService/BotHandler |
| MODIFY | `app/server/internal/repository/db.go` | AutoMigrate 中 `BotAPIKey` → `BotToken` |
| MODIFY | `app/server/internal/handler/auth_handler.go` | Register 中拦截 bot 前缀用户名 |
| MODIFY | `app/server/internal/router/routes/mute/routes.go` | `/mute/status` 中间件 `JWTAuth()` → `RequirePermission(PermMuteManage)` |

### Server — 新建 (5 files)

| Action | File | Responsibility |
|--------|------|----------------|
| CREATE | `app/server/internal/model/bot_token.go` | BotToken 结构体：Name, UserUUID, Role, ExpiresAt, Revoked |
| CREATE | `app/server/internal/repository/bot_token_repo.go` | DAO：Create/List/ListActive/GetByUUID/Revoke |
| CREATE | `app/server/internal/service/bot_service.go` | Create(创建 User(IsBot=true) + BotToken + 签发 JWT)/Revoke/List |
| CREATE | `app/server/internal/handler/bot_handler.go` | HTTP 端点：POST /bot/create, /bot/list, /bot/revoke |
| CREATE | `app/server/internal/router/routes/bot/routes.go` | 路由注册，全在 protected 组下，PermBotManage 保护 |

### Bot Node — 修改 (5 files)

| Action | File | Responsibility |
|--------|------|----------------|
| MODIFY | `packages/bot/src/core/context.ts` | RoomClient 扩展 join/leave/jointed 方法 |
| MODIFY | `packages/bot/src/runtime/apiClient.ts` | 新增 getUserByIdentity/muteUser/unmuteUser/listMutes/getSFUToken |
| MODIFY | `packages/bot/src/runtime/socketClient.ts` | 增加 joinedRooms Map、rooms getter、joinRoomSFU、幂等保护 |
| MODIFY | `packages/bot/src/runtime/botRunner.ts` | 增加 joinRoom/leaveRoom/jointedRooms；buildPluginCtx 扩展 rooms 接口 |
| MODIFY | `packages/bot/src/plugins/builtin/index.ts` | 导出 MuteManagerPlugin |

### Bot Node — 新建 (1 file)

| Action | File | Responsibility |
|--------|------|----------------|
| CREATE | `packages/bot/src/plugins/builtin/mute-manager/index.ts` | 禁言管理插件：/mute, /unmute, /mute list, /mute status 四个命令 |

---

## Tasks

### Task 1: 删除旧 BotAPIKey 体系（清理 4 个文件 + 2 处引用）

**Files:**
- Delete: `app/server/internal/model/bot_apikey.go`
- Delete: `app/server/internal/repository/bot_apikey_repo.go`
- Delete: `app/server/internal/service/bot_apikey_service.go`
- Delete: `app/server/internal/handler/bot_apikey_handler.go`
- Modify: `app/server/internal/repository/db.go:74` — 删除 `&model.BotAPIKey{}` 行
- Modify: `app/server/internal/router/router.go` — 后续任务中处理

- [ ] **Step 1: 删除 4 个旧文件**

```bash
cd /Users/noelorin/GOSpeak
rm app/server/internal/model/bot_apikey.go
rm app/server/internal/repository/bot_apikey_repo.go
rm app/server/internal/service/bot_apikey_service.go
rm app/server/internal/handler/bot_apikey_handler.go
```

验证：`ls app/server/internal/model/bot_apikey.go` → `No such file`

- [ ] **Step 2: 从 DB auto-migrate 中移除 BotAPIKey**

`app/server/internal/repository/db.go:74`:
```go
// 删除这一行：
&model.BotAPIKey{},
```

- [ ] **Step 3: 确认 IDE/编译器无残留引用**

```bash
cd /Users/noelorin/GOSpeak/app/server
rg "bot_apikey" --type go
```
预期：只输出 `model/bot_token.go` 和无关行，无旧包名导入。

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: remove legacy BotAPIKey system (gk_ + bcrypt)"
```

---

### Task 2: User 模型加 IsBot 字段

**Files:**
- Modify: `app/server/internal/model/user.go:12` — 新增 IsBot 字段

- [ ] **Step 1: 修改 User 结构体**

`app/server/internal/model/user.go`，在 `EmailVerified` 之后加一行：

```go
type User struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UUID          string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name          string    `gorm:"uniqueIndex" json:"name"`
	DisplayName   string    `gorm:"default:''" json:"display_name"`
	Avatar        string    `gorm:"default:''" json:"avatar"`
	Email         string    `gorm:"size:128;index" json:"email"`
	EmailVerified bool      `gorm:"default:false" json:"email_verified"`
	IsBot         bool      `gorm:"default:false" json:"is_bot"`   // ← 新增
	Password      string    `json:"-"`
	Role          string    `gorm:"default:user" json:"role"`
	TokenVersion  uint      `gorm:"default:0" json:"token_version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: 提交**

```bash
git add -A
git commit -m "feat: add IsBot field to User model"
```

---

### Task 3: 新建 BotToken 模型 + Repository

**Files:**
- Create: `app/server/internal/model/bot_token.go`
- Create: `app/server/internal/repository/bot_token_repo.go`
- Modify: `app/server/internal/repository/db.go` — auto-migrate 注册新表

- [ ] **Step 1: 创建 BotToken 模型**

`app/server/internal/model/bot_token.go`:

```go
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BotToken 管理记录，不存储实际 token 值（JWT 由 pkg.GenerateToken 签发）。
type BotToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UUID      string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name      string    `gorm:"size:64;uniqueIndex" json:"name"`
	UserUUID  string    `gorm:"type:uuid;index" json:"user_uuid"` // FK → users.uuid（IsBot=true 的记录）
	Role      string    `gorm:"size:32;default:user" json:"role"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BotToken) TableName() string {
	return "bot_tokens"
}

func (bt *BotToken) BeforeCreate(_ *gorm.DB) error {
	if bt.UUID == "" {
		bt.UUID = uuid.New().String()
	}
	return nil
}
```

- [ ] **Step 2: 创建 BotToken Repository**

`app/server/internal/repository/bot_token_repo.go`:

```go
package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type BotTokenRepository struct {
	db *gorm.DB
}

func NewBotTokenRepository(db *gorm.DB) *BotTokenRepository {
	return &BotTokenRepository{db: db}
}

func (r *BotTokenRepository) Create(token *model.BotToken) error {
	return r.db.Create(token).Error
}

func (r *BotTokenRepository) List() ([]model.BotToken, error) {
	var tokens []model.BotToken
	err := r.db.Order("created_at DESC").Find(&tokens).Error
	return tokens, err
}

func (r *BotTokenRepository) GetByUUID(uuid string) (*model.BotToken, error) {
	var token model.BotToken
	err := r.db.Where("uuid = ?", uuid).First(&token).Error
	return &token, err
}

func (r *BotTokenRepository) Revoke(uuid string) error {
	return r.db.Model(&model.BotToken{}).
		Where("uuid = ?", uuid).
		Update("revoked", true).Error
}
```

- [ ] **Step 3: 注册到 auto-migrate**

`app/server/internal/repository/db.go`，在 autoMigrate 函数中增加 `&model.BotToken{}`：

```go
func autoMigrate() error {
	return DB.AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.Room{},
		&model.UserGroup{},
		&model.OAuthProvider{},
		&model.OAuthAccount{},
		&model.EmailConfig{},
		&model.EmailVerificationCode{},
		&model.SFUConfig{},
		&model.SFUActiveProvider{},
		&model.Permission{},
		&model.RolePermission{},
		&model.Mute{},
		&model.StorageConfig{},
		&model.BotToken{},   // ← 替换旧的 &model.BotAPIKey{}
	)
}
```

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "feat: add BotToken model + repo for bot management"
```

---

### Task 4: 新建 BotService — 核心业务逻辑

**Files:**
- Create: `app/server/internal/service/bot_service.go`

- [ ] **Step 1: 创建 service 文件**

`app/server/internal/service/bot_service.go`:

```go
package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const botNamePrefix = "bot_"

type CreateBotRequest struct {
	Name      string `json:"name" binding:"required"`
	Role      string `json:"role" binding:"required"`
	ExpiresIn string `json:"expires_in"` // 如 "720h"、"30d"，空=不过期
}

type CreateBotResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

type BotService struct {
	userRepo    *repository.UserRepository
	botRepo     *repository.BotTokenRepository
}

func NewBotService(userRepo *repository.UserRepository, botRepo *repository.BotTokenRepository) *BotService {
	return &BotService{userRepo: userRepo, botRepo: botRepo}
}

func (s *BotService) Create(req *CreateBotRequest) (*CreateBotResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "name is required")
	}
	if strings.HasPrefix(name, botNamePrefix) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "name should not contain 'bot_' prefix")
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}

	botUsername := botNamePrefix + name
	existing, _ := s.userRepo.GetByName(botUsername)
	if existing != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS, "bot name already exists")
	}

	// 随机密码 — 禁止密码登录
	randomPwd := randomHex(32)
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	displayName := fmt.Sprintf("Bot-%s", name)
	user := &model.User{
		Name:        botUsername,
		DisplayName: displayName,
		Password:    string(hashedPwd),
		Role:        role,
		IsBot:       true,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	expiresAt, err := parseBotExpiry(req.ExpiresIn)
	if err != nil {
		// 用户已创建，标记删除或直接返回错误
		_ = s.userRepo.Delete(user.ID)
		return nil, err
	}

	botToken := &model.BotToken{
		UUID:      uuid.New().String(),
		Name:      name,
		UserUUID:  user.UUID,
		Role:      role,
		ExpiresAt: expiresAt,
	}
	if err := s.botRepo.Create(botToken); err != nil {
		_ = s.userRepo.Delete(user.ID)
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 签发 JWT（和用户完全同构）
	token, err := pkg.GenerateToken(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return &CreateBotResult{Token: token, User: user}, nil
}

func (s *BotService) List() ([]model.BotToken, error) {
	return s.botRepo.List()
}

func (s *BotService) Revoke(uuid string) error {
	token, err := s.botRepo.GetByUUID(uuid)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "bot token not found")
	}
	if err := s.botRepo.Revoke(uuid); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 可选择同时吊销 users 记录 — 但保留记录用于审计
	// 仅标记 bot_tokens.revoked=true
	_ = token // 可用于后续操作
	return nil
}

func parseBotExpiry(expiresIn string) (time.Time, error) {
	expiresIn = strings.TrimSpace(expiresIn)
	if expiresIn == "" {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), nil
	}
	d, err := time.ParseDuration(expiresIn)
	if err != nil {
		return time.Time{}, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid expires_in, use Go duration like 720h or 30d")
	}
	if d <= 0 {
		return time.Time{}, pkg.NewAppError(pkg.INVALID_PARAMS, "expires_in must be positive")
	}
	return time.Now().Add(d), nil
}

func randomHex(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 2: 提交**

```bash
git add -A
git commit -m "feat: add BotService — create bot user + issue JWT"
```

---

### Task 5: 新建 BotHandler + Bot Routes

**Files:**
- Create: `app/server/internal/handler/bot_handler.go`
- Create: `app/server/internal/router/routes/bot/routes.go`（覆盖旧文件）

- [ ] **Step 1: 创建 BotHandler**

`app/server/internal/handler/bot_handler.go`:

```go
package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type BotHandler struct {
	svc *service.BotService
}

func NewBotHandler(svc *service.BotService) *BotHandler {
	return &BotHandler{svc: svc}
}

// Create
// @Summary      创建 Bot
// @Description  创建 Bot 用户并签发 JWT（token 同构，与用户一致）
// @Tags         Bot
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body service.CreateBotRequest true "Bot 配置"
// @Success      200 {object} pkg.Response{data=service.CreateBotResult}
// @Router       /bot/create [post]
func (h *BotHandler) Create(c *gin.Context) {
	var req service.CreateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	result, err := h.svc.Create(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, result)
}

// List
// @Summary      列出 Bot
// @Description  列出所有 Bot Token 管理记录
// @Tags         Bot
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} pkg.Response
// @Router       /bot/list [post]
func (h *BotHandler) List(c *gin.Context) {
	tokens, err := h.svc.List()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, tokens)
}

// Revoke
// @Summary      吊销 Bot
// @Description  吊销指定 Bot Token（标记 revoked=true）
// @Tags         Bot
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body object{uuid=string} true "Bot UUID"
// @Success      200 {object} pkg.Response
// @Router       /bot/revoke [post]
func (h *BotHandler) Revoke(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.svc.Revoke(req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
```

- [ ] **Step 2: 覆盖旧 bot/routes.go**

`app/server/internal/router/routes/bot/routes.go`:

```go
package bot

import (
	"GOSpeak/internal/handler"
	"GOSpeak/internal/middleware"
	"GOSpeak/internal/permcode"

	"github.com/gin-gonic/gin"
)

func RegisterProtected(r *gin.RouterGroup, h *handler.BotHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermBotManage), h.Create)
	r.POST("/list", middleware.RequirePermission(permcode.PermBotManage), h.List)
	r.POST("/revoke", middleware.RequirePermission(permcode.PermBotManage), h.Revoke)
}
```

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "feat: add BotHandler + bot routes"
```

---

### Task 6: 清理中间件 — 删除 BotKeyAuth

**Files:**
- Modify: `app/server/internal/middleware/auth.go` — 删除约 80 行

- [ ] **Step 1: 从 auth.go 删除以下全部内容**

从 `auth.go` 中删除：

1. `checkBotPermissions` 函数（约 15 行）
2. `BotKeyAuth` 函数（约 35 行）
3. `BotKeyResolver` 接口定义（约 5 行）
4. `BotKeyInfo` 结构体（8 行，如果存在于 auth.go）

同时删除 `auth.go` 中 import 的 `os` 包（如果不再使用）。

保留的函数：`VerifyToken`、`setClaimsContext`、`JWTAuth`、`WSAuth`、`RequireRole`、`RequirePermission`、`RequireOwnerOrPermission`、`BanCheck`、`CORS`、`SetPermissionChecker`、`SetTokenVersionChecker`。

- [ ] **Step 2: 确认编译通过**（此时 router.go 还有 botKeySvc 引用，暂时可以）

```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./... 2>&1 | head -5
```
预期：`undefined: BotKeyAuth` 等错误 — 正常，下个任务修。

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "refactor: remove BotKeyAuth middleware (no longer needed)"
```

---

### Task 7: 更新 Router — 移除 BotKeyAuth + 接入新 BotHandler

**Files:**
- Modify: `app/server/internal/router/router.go`

- [ ] **Step 1: 修改 SetupRoutes 签名**

删除 `botKeySvc *service.BotAPIKeyService` 参数：

```go
// 改前
func SetupRoutes(r *gin.Engine, h *Handlers, botKeySvc *service.BotAPIKeyService) *gin.Engine {

// 改后
func SetupRoutes(r *gin.Engine, h *Handlers) *gin.Engine {
```

- [ ] **Step 2: 删除 protected 组的 BotKeyAuth**

```go
// 删除这一整行：
protected.Use(middleware.BotKeyAuth(botKeySvc))
```

protected 组现在只保留 `middleware.BanCheck()`：

```go
protected := api.Group("")
protected.Use(middleware.BanCheck())
```

- [ ] **Step 3: 更新 Handlers 结构体**

```go
type Handlers struct {
	Auth        *handler.AuthHandler
	User        *handler.UserHandler
	Signal      *handler.SignalHandler
	OAuth       *handler.OAuthHandler
	Role        *handler.RoleHandler
	Room        *handler.RoomHandler
	Permission  *handler.PermissionHandler
	Mute        *handler.MuteHandler
	SFUConfig   *handler.SFUConfigHandler
	Storage     *handler.StorageHandler
	Email       *handler.EmailVerificationHandler
	EmailConfig *handler.EmailConfigHandler
	Monitor     *handler.MonitorHandler
	SRSCallback *handler.SRSCallbackHandler
	Bot         *handler.BotHandler   // 类型从 *handler.BotAPIKeyHandler 改为 *handler.BotHandler
}
```

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "refactor: remove BotKeyAuth from router, wire BotHandler"
```

---

### Task 8: 更新 Gin 注册 — 接入新 BotService

**Files:**
- Modify: `app/server/internal/server/gin.go`

- [ ] **Step 1: 替换旧 bot 初始化代码**

在 gin.go 中找到旧 bot 代码段：

```go
// 删除以下全部：
botKeyRepo := repository.NewBotAPIKeyRepository(repository.DB)
// ... 旧的 botKeySvc
botKeySvc := service.NewBotAPIKeyService(botKeyRepo)
// ... 旧的 botKeyH
botKeyH := handler.NewBotAPIKeyHandler(botKeySvc)
```

替换为：

```go
botTokenRepo := repository.NewBotTokenRepository(repository.DB)
botSvc := service.NewBotService(userRepo, botTokenRepo)
botH := handler.NewBotHandler(botSvc)
```

- [ ] **Step 2: 更新 Router.SetupRoutes 调用**

```go
// 改前
router.SetupRoutes(r, &router.Handlers{
	...
	Bot: botKeyH,
}, botKeySvc)

// 改后
router.SetupRoutes(r, &router.Handlers{
	...
	Bot: botH,
})
```

将 Handlers 中的 `Bot: botKeyH` 改为 `Bot: botH`。

- [ ] **Step 3: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
```
预期：无错误。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "refactor: wire new BotService/BotHandler in gin.go"
```

---

### Task 9: 注册接口拦截 Bot 前缀用户名

**Files:**
- Modify: `app/server/internal/handler/auth_handler.go` — Register 加前缀检查

- [ ] **Step 1: 在 Register handler 中加前缀保护**

`app/server/internal/handler/auth_handler.go`，在 `Register` 函数开头、解析 body 之后：

```go
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	// 新增：拦截 bot_ 前缀用户名
	if strings.HasPrefix(req.Username, "bot_") {
		pkg.Fail(c, pkg.USERNAME_EXISTS, "username prefix 'bot_' is reserved")
		return
	}

	resp, err := h.authService.Register(&req)
	// ... 后续不变
}
```

需要在文件顶部 import 中加入 `"strings"`：

```go
import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/redis"
	"GOSpeak/internal/service"
	"strings"  // ← 新增

	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
```
预期：无错误。

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "feat: block bot_ prefix in user registration"
```

---

### Task 10: 修复 /mute/status 路由中间件

**Files:**
- Modify: `app/server/internal/router/routes/mute/routes.go`

- [ ] **Step 1: 修改中间件**

```go
func RegisterProtected(r *gin.RouterGroup, h *handler.MuteHandler) {
	r.POST("/create", middleware.RequirePermission(permcode.PermMuteManage), h.CreateMute)
	r.POST("/cancel", middleware.RequirePermission(permcode.PermMuteManage), h.CancelMute)
	r.POST("/status", middleware.RequirePermission(permcode.PermMuteManage), h.GetMuteStatus)  // ← 改这里
	r.POST("/list", middleware.RequirePermission(permcode.PermMuteManage), h.ListMutes)
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
```
预期：无错误。

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "fix: use RequirePermission for /mute/status (was JWTAuth-only)"
```

---

### Task 11: Bot Node — Context 扩展 RoomClient 接口

**Files:**
- Modify: `packages/bot/src/core/context.ts`

- [ ] **Step 1: 扩展 RoomClient 接口**

`packages/bot/src/core/context.ts`:

```typescript
export interface RoomClient {
	listRooms(): Promise<RoomRef[]>;
	getMembers(roomId: string): Promise<MemberRef[]>;
	createRoom(name: string, limit?: number): Promise<RoomRef>;
	// 新增：
	join(name: string, opts?: { sfu?: boolean }): Promise<void>;
	leave(name: string): void;
	joined(): string[];
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
npx tsc --noEmit 2>&1
```
预期：`apiClient.ts` 和 `botRunner.ts` 中未实现新接口的错误 — 正常，后续任务实现。

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "feat: extend RoomClient interface with join/leave/joined"
```

---

### Task 12: Bot Node — apiClient 新增禁言 + SFU token + 用户查询

**Files:**
- Modify: `packages/bot/src/runtime/apiClient.ts`

- [ ] **Step 1: 新增 5 个方法**

在 `packages/bot/src/runtime/apiClient.ts` 的 `GOSpeakApiClient` 类中新增：

```typescript
// ── 用户查询 ──
async getUserByIdentity(identity: string): Promise<{ ID: number; Name: string; Role: string }> {
	return this.request("POST", "/api/v1/user/info", { identity });
}

// ── 禁言管理 ──
async muteUser(userId: number, duration: number, permanent: boolean, reason?: string): Promise<void> {
	await this.request("POST", "/api/v1/mute/create", {
		user_id: userId,
		duration,
		permanent,
		reason: reason ?? "",
	});
}

async unmuteUser(userId: number): Promise<void> {
	await this.request("POST", "/api/v1/mute/cancel", { user_id: userId });
}

async listMutes(): Promise<unknown[]> {
	return this.request("POST", "/api/v1/mute/list");
}

async getMuteStatus(userId: number): Promise<unknown | null> {
	const data = await this.request<unknown | null>("POST", "/api/v1/mute/status", { user_id: userId });
	return data;
}

// ── SFU Token ──
async getSFUToken(room: string, identity: string): Promise<{ token: string; serverUrl: string; stream?: string }> {
	return this.request("POST", "/api/v1/signal/token", { room, identity });
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
npx tsc --noEmit 2>&1
```
预期：`RoomClient` 接口未完全实现的错误 — 下个任务修复。

- [ ] **Step 3: 提交**

```bash
git add -A
git commit -m "feat: add mute/sfu/user API methods to apiClient"
```

---

### Task 13: Bot Node — socketClient 多房间状态 + joinRoomSFU

**Files:**
- Modify: `packages/bot/src/runtime/socketClient.ts`

- [ ] **Step 1: 增加多房间状态管理**

在 `GOSpeakSocketClient` 类中新增：

```typescript
export class GOSpeakSocketClient {
	private opts: SocketClientOptions;
	private socket: SocketIONamespace | null = null;
	private onEvent: ((event: BotEvent) => void) | null = null;
	private connected = false;
	private logger: Logger;
	private joinedRooms: Map<string, { identity: string }> = new Map();  // ← 新增

	// ... 现有构造函数 ...

	get isConnected(): boolean {
		return this.connected;
	}

	// ← 新增
	get rooms(): string[] {
		return [...this.joinedRooms.keys()];
	}

	// ← 新增
	isInRoom(room: string): boolean {
		return this.joinedRooms.has(room);
	}
```

- [ ] **Step 2: 修改 joinRoom 为幂等**

```typescript
	joinRoom(room: string, identity: string): void {
		if (this.joinedRooms.has(room)) {
			this.logger.debug(`Already in room ${room}, skipping join`);
			return;
		}
		this.socket?.emit("room:join", { room, identity });
		this.joinedRooms.set(room, { identity });
	}
```

- [ ] **Step 3: 修改 leaveRoom 清理状态**

```typescript
	leaveRoom(room: string): void {
		this.socket?.emit("room:leave", { room });
		this.joinedRooms.delete(room);
	}
```

- [ ] **Step 4: 新增 joinRoomSFU（媒体面确认）**

```typescript
	joinRoomSFU(room: string, identity: string, stream?: string): Promise<{ ok: boolean; members: unknown[] }> {
		return new Promise((resolve, reject) => {
			if (!this.socket) {
				reject(new Error("socket not connected"));
				return;
			}
			this.socket.emit("room:join:sfu", { room, identity, stream }, (ack: unknown) => {
				const resp = ack as Record<string, unknown>;
				if (resp?.error) {
					reject(new Error(String(resp.error)));
				} else {
					this.joinedRooms.set(room, { identity });
					resolve(resp as { ok: boolean; members: unknown[] });
				}
			});
		});
	}
```

- [ ] **Step 5: disconnect 时清空状态**

在 `disconnect` 方法中：

```typescript
	disconnect(): void {
		if (this.socket) {
			this.socket.disconnect();
			this.socket = null;
		}
		this.connected = false;
		this.joinedRooms.clear();  // ← 新增
	}
```

- [ ] **Step 6: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
npx tsc --noEmit 2>&1
```
预期：`RoomClient` 接口未实现的错误 — 下个任务修复。

- [ ] **Step 7: 提交**

```bash
git add -A
git commit -m "feat: add multi-room state + joinRoomSFU to socketClient"
```

---

### Task 14: Bot Node — BotRunner 多房间 API + ctx 扩展

**Files:**
- Modify: `packages/bot/src/runtime/botRunner.ts`

- [ ] **Step 1: 新增 joinRoom/leaveRoom/joinedRooms**

在 `BotRunner` 类中新增：

```typescript
	async joinRoom(roomName: string, opts?: { sfu?: boolean }): Promise<void> {
		const identity = this.config.identity;

		if (opts?.sfu) {
			// 获取 SFU token
			const sfuToken = await this.api.getSFUToken(roomName, identity);
			// WS 信号面加入 + SFU 媒体面确认
			await this.socket.joinRoomSFU(roomName, identity, sfuToken.stream);
		} else {
			// 仅信号面加入
			this.socket.joinRoom(roomName, identity);
		}
	}

	leaveRoom(roomName: string): void {
		this.socket.leaveRoom(roomName);
	}

	get joinedRooms(): string[] {
		return this.socket.rooms;
	}
```

- [ ] **Step 2: 扩展 buildPluginCtx 中的 rooms 接口**

在 `BotRunner.buildPluginCtx` 方法中，扩展 `rooms` 属性：

```typescript
	private buildPluginCtx(pluginName: string): BotContext {
		return {
			logger: this.logger,
			config: this.config.pluginConfigs?.[pluginName] ?? {},
			pluginName,
			chat: this.api,
			rooms: {
				...this.api,
				join: (name, opts) => this.joinRoom(name, opts),
				leave: (name) => this.leaveRoom(name),
				joined: () => this.joinedRooms,
			},
			voice: this.api,
			kv: createKVStore(),
			hasPermission: (_level, _member) => true,
		};
	}
```

注意：`RoomClient.join` 的签名是 `(name: string, opts?: { sfu?: boolean }) => Promise<void>`，所以 `ctx.rooms.join(name)` 兼容。

- [ ] **Step 3: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
npx tsc --noEmit 2>&1
```
预期：无错误。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "feat: add joinRoom/leaveRoom/joinedRooms to BotRunner + extend ctx"
```

---

### Task 15: Bot Node — 新建禁言管理插件

**Files:**
- Create: `packages/bot/src/plugins/builtin/mute-manager/index.ts`
- Modify: `packages/bot/src/plugins/builtin/index.ts`

- [ ] **Step 1: 创建插件文件**

`packages/bot/src/plugins/builtin/mute-manager/index.ts`:

```typescript
/**
 * 禁言管理插件（Bot 有 mute:manage + user:read 权限时可用）
 *
 * 命令：
 *   /mute <user> <duration> [reason]   — 禁言用户
 *   /unmute <user>                     — 取消禁言
 *   /mute list                          — 列出生效禁言
 *   /mute status <user>                 — 查询指定用户禁言状态
 *
 * duration 格式：1h / 30m / 7d / permanent / 数字秒数
 */
import { Plugin } from "../../../core/plugin";
import { RegisterPlugin } from "../../../decorators/register";
import { Command, On } from "../../../decorators/handlers";
import { PermissionFilter } from "../../../filters/index";
import { EventType } from "../../../core/types";
import type { MessageEvent } from "../../../core/types";

interface MuteRecord {
	id: number;
	user_id: number;
	permanent: boolean;
	duration: number;
	reason: string;
	expires_at: string | null;
	created_at: string;
}

function parseDuration(input: string): { duration: number; permanent: boolean } | string {
	if (input === "permanent") {
		return { duration: 0, permanent: true };
	}
	const match = input.match(/^(\d+)([smhd])$/);
	if (match) {
		const num = parseInt(match[1], 10);
		switch (match[2]) {
			case "s": return { duration: num, permanent: false };
			case "m": return { duration: num * 60, permanent: false };
			case "h": return { duration: num * 3600, permanent: false };
			case "d": return { duration: num * 86400, permanent: false };
		}
	}
	const num = parseInt(input, 10);
	if (!isNaN(num) && num > 0) {
		return { duration: num, permanent: false };
	}
	return "invalid duration, use: 30m, 1h, 7d, permanent, or seconds";
}

function fmtRemaining(record: MuteRecord): string {
	if (record.permanent) return "permanent";
	if (!record.expires_at) return "unknown";
	const remaining = new Date(record.expires_at).getTime() - Date.now();
	if (remaining <= 0) return "expired";
	const secs = Math.floor(remaining / 1000);
	if (secs < 60) return `${secs}s`;
	if (secs < 3600) return `${Math.floor(secs / 60)}m`;
	if (secs < 86400) return `${Math.floor(secs / 3600)}h`;
	return `${Math.floor(secs / 86400)}d`;
}

@RegisterPlugin({
	name: "mute-manager",
	author: "gospeak",
	desc: "禁言管理：禁言/解禁/列表/状态查询",
	version: "1.0.0",
})
export class MuteManagerPlugin extends Plugin {
	@Command("mute", { desc: "禁言管理" })
	async onMute(event: MessageEvent): Promise<void> {
		const sub = event.rawCommand?.args[0];
		const args = event.rawCommand?.args.slice(1) ?? [];

		switch (sub) {
			case "list":
				return this.handleList(event);
			case "status":
				return this.handleStatus(event, args);
			default:
				// /mute <user> <duration> [reason]
				if (sub && args.length >= 1) {
					return this.handleCreate(event, sub, args);
				}
				await this.ctx.chat.reply(event,
					"用法: /mute <user> <duration> [reason] | /unmute <user> | /mute list | /mute status <user>");
		}
	}

	@Command("unmute", { desc: "取消禁言" })
	async onUnmute(event: MessageEvent): Promise<void> {
		const username = event.rawCommand?.args[0];
		if (!username) {
			await this.ctx.chat.reply(event, "用法: /unmute <user>");
			return;
		}
		try {
			const user = await this.ctx.voice.muteMember("", username, false);
			await this.ctx.chat.reply(event, `已取消 ${username} 的禁言`);
		} catch (err) {
			await this.ctx.chat.reply(event, `取消失败: ${(err as Error).message}`);
		}
	}

	private async handleCreate(event: MessageEvent, username: string, args: string[]): Promise<void> {
		const durationStr = args[0];
		const reason = args.slice(1).join(" ") || "";

		const parsed = parseDuration(durationStr);
		if (typeof parsed === "string") {
			await this.ctx.chat.reply(event, parsed);
			return;
		}

		try {
			const user = await this.ctx.rooms.getMembers(event.room.id);
			const target = user.find(m => m.identity === username || m.name === username);
			if (!target) {
				await this.ctx.chat.reply(event, `用户 ${username} 不存在`);
				return;
			}
			// Lookup user_id via API
			const userInfo = await (this.ctx as any).api.getUserByIdentity(username);
			await (this.ctx as any).api.muteUser(userInfo.ID, parsed.duration, parsed.permanent, reason);
			await this.ctx.chat.reply(event, `已禁言 ${username} (${durationStr})${reason ? ` 原因: ${reason}` : ""}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `禁言失败: ${(err as Error).message}`);
		}
	}

	private async handleList(event: MessageEvent): Promise<void> {
		try {
			const mutes = await (this.ctx as any).api.listMutes() as MuteRecord[];
			if (!mutes || mutes.length === 0) {
				await this.ctx.chat.reply(event, "当前没有生效禁言");
				return;
			}
			const lines = mutes.map(m => {
				const remaining = fmtRemaining(m);
				return `  - user#${m.user_id} ${remaining}${m.reason ? ` (${m.reason})` : ""}`;
			});
			await this.ctx.chat.reply(event, `生效禁言 (${mutes.length}):\n${lines.join("\n")}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}

	private async handleStatus(event: MessageEvent, args: string[]): Promise<void> {
		const username = args[0];
		if (!username) {
			await this.ctx.chat.reply(event, "用法: /mute status <user>");
			return;
		}
		try {
			const userInfo = await (this.ctx as any).api.getUserByIdentity(username);
			const status = await (this.ctx as any).api.getMuteStatus(userInfo.ID) as MuteRecord | null;
			if (!status) {
				await this.ctx.chat.reply(event, `${username} 未被禁言`);
				return;
			}
			const remaining = fmtRemaining(status);
			await this.ctx.chat.reply(event,
				`${username} 禁言中: ${remaining}${status.reason ? `, 原因: ${status.reason}` : ""}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}
}
```

- [ ] **Step 2: 注册插件到索引**

`packages/bot/src/plugins/builtin/index.ts`:

```typescript
export { RoomManagerPlugin } from "./room-manager";
export { KeywordReplyPlugin } from "./keyword-reply";
export { ModerationPlugin } from "./moderation";
export { WelcomePlugin } from "./welcome";
export { MuteManagerPlugin } from "./mute-manager";  // ← 新增
```

- [ ] **Step 3: 编译验证**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
npx tsc --noEmit 2>&1
```
预期：无错误（可能有 `(this.ctx as any)` 相关的类型警告但不报错）。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "feat: add mute-manager builtin plugin"
```

---

### Task 16: 全量编译验证

- [ ] **Step 1: 服务端编译**

```bash
cd /Users/noelorin/GOSpeak/app/server
go build ./...
```
预期：无错误。

- [ ] **Step 2: Bot Node 编译**

```bash
cd /Users/noelorin/GOSpeak/packages/bot
npx tsc --noEmit 2>&1
```
预期：无错误。

- [ ] **Step 3: 启动服务端做冒烟测试**

```bash
cd /Users/noelorin/GOSpeak/app/server
pnpm dev
```
等待就绪后，另开终端：

```bash
# 1. 管理员登录获取 token
curl -s -X POST http://localhost:8998/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
# 提取 admin_token

# 2. 创建 Bot
curl -s -X POST http://localhost:8998/api/v1/bot/create \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $admin_token" \
  -d '{"name":"test-moderator","role":"admin","expires_in":"720h"}'
# 返回 { token, user }

# 3. 用 Bot JWT 调用禁言列表
curl -s -X POST http://localhost:8998/api/v1/mute/list \
  -H "Authorization: Bearer $bot_token"
# 预期：{"code":0,"msg":"success","data":[]}

# 4. Bot 列房间
curl -s -X GET http://localhost:8998/api/v1/signal/rooms \
  -H "Authorization: Bearer $bot_token"
# 预期：正常返回房间列表
```

- [ ] **Step 4: 提交最终 commit**

```bash
git add -A
git commit -m "chore: full build verification - bot system refactor complete"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task | Status |
|------------|------|--------|
| 删除旧 gk_ BotAPIKey 体系 | Task 1 | ✅ |
| User 加 IsBot 字段 | Task 2 | ✅ |
| BotToken 管理表（不存 token 值） | Task 3 | ✅ |
| BotService: Create(User + BotToken + 签发 JWT) | Task 4 | ✅ |
| BotHandler: Create/List/Revoke | Task 5 | ✅ |
| Bot Routes 注册 | Task 5 | ✅ |
| 删除 BotKeyAuth 中间件 | Task 6 | ✅ |
| Router 清理 + 新 BotHandler | Task 7 | ✅ |
| Gin 注册新 BotService | Task 8 | ✅ |
| 拦截 bot_ 前缀注册 | Task 9 | ✅ |
| /mute/status 路由修复 | Task 10 | ✅ |
| Bot Node Context 扩展 | Task 11 | ✅ |
| Bot Node apiClient 新方法 | Task 12 | ✅ |
| Bot Node socketClient 多房间 | Task 13 | ✅ |
| Bot Node BotRunner 多房间 | Task 14 | ✅ |
| Bot Node mute-manager 插件 | Task 15 | ✅ |
| 全量编译 + 冒烟测试 | Task 16 | ✅ |

### Placeholder Scan

无 "TBD"、"TODO"、"implement later"、"fill in details" 等占位符。所有代码块包含完整实现。

### Type Consistency

- `BotService.Create` 返回 `*CreateBotResult` — 包含 `Token string` + `User *model.User`
- `BotHandler.Create` 返回 `pkg.Success(c, result)` — 匹配
- `RoomClient` 接口 `join(name, opts?)` / `leave(name)` / `joined()` — `botRunner` 的 `buildPluginCtx` 实现签名一致
- `GOSpeakSocketClient.joinRoomSFU` 返回 `Promise<{ok, members}>` — 对应服务端 `OnRoomJoinSFU` 的 ack 格式 `marshalAck`
- `apiClient` 的 `muteUser` 调用 `POST /api/v1/mute/create { user_id, duration, permanent, reason }` — 和 `MuteHandler.CreateMute` 接收字段一致

---

Plan complete and saved to `docs/superpowers/plans/2026-07-09-bot-refactor-mute-manager-multiroom.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
