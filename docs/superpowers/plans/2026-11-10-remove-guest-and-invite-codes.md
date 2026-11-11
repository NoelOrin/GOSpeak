# 下线访客体系与邀请码实体化 实现计划（第一部分）

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 删除访客体系全链路（后端 6 个文件 + 前端 5 个文件 + 8 个数据列 + 1 张表），并把邀请码从 `domains.invite_code` 一列升级为独立实体 `domain_invites`：自包含 HMAC 签名串、可配过期与使用上限、可禁用、可审计。

**架构：** 邀请码是**自包含签名串**——22 字节载荷（域前缀 4B + codeID 4B + 过期时间 4B + 次数上限 2B）+ HMAC-SHA256 前 8 字节，base32 编码成 36 字符。验签与判过期完全不查库，只有「查已用次数 / 禁用状态」需要一次 DB 查询。密钥从 `StorageEncryptKey` 派生，与存储加密用途隔离。访客体系整体删除，不保留降级路径。

**技术栈：** Go 1.26 + Gin + GORM（SQLite/PostgreSQL/MySQL）、`crypto/hmac` + `crypto/sha256` + `encoding/base32`（全部标准库）、React 19 + TypeScript + zustand + TanStack Router/Query、vitest。

**规格来源：** `docs/superpowers/specs/2026-11-10-invite-registration-approval-design.md` §2–§3、§6.3、§6.7

---

## 文件结构

### 后端（新建）

| 文件 | 职责 |
|------|------|
| `app/server/internal/service/invite_code.go` | 邀请码纯函数：密钥派生、编码、解码验签、过期判断。无 DB、无 gin 依赖 |
| `app/server/internal/service/invite_code_test.go` | 上述纯函数单测（生成/篡改/过期/边界值） |
| `app/server/internal/model/domain_invite.go` | `DomainInvite` + `DomainInviteRedemption` 两张表 |
| `app/server/internal/repository/domain_invite_repo.go` | 邀请码 DAO：创建、按 ID 查、条件消耗次数、禁用、列表、使用记录 |
| `app/server/internal/repository/domain_invite_repo_test.go` | 仓库单测（内存 SQLite，含并发超发用例） |
| `app/server/internal/service/invite_service.go` | 邀请码业务：创建（含默认码）、校验（验签 + 查库）、禁用、列表、审计记录 |
| `app/server/internal/service/invite_service_test.go` | 服务单测：全分支校验、次数耗尽、禁用、跨域隔离 |
| `app/server/internal/handler/invite_handler.go` | 4 个管理端点 + 1 个公开 preview |
| `app/server/internal/handler/invite_handler_test.go` | 参数校验、权限、错误码映射 |
| `app/server/internal/router/routes/invite/routes.go` | 路由注册 |

### 后端（修改）

| 文件 | 改动 |
|------|------|
| `app/server/internal/model/domain.go` | 删 5 个 guest 字段 + `InviteCode` 字段 + `DefaultGuestLimit` + `GenerateInviteCode` + `BeforeCreate` 中的邀请码分支；`DomainDetail.InviteCode` 删除 |
| `app/server/internal/model/user.go` | 删 `IsGuest`、`GuestNamePrefix`、`IsGuestName` |
| `app/server/internal/model/domain_role.go` | 删 `DomainRoleGuest`、权限矩阵条目、`IsSystemDomainRole` 分支；`AssignableDomainPermissions` 加 2 个新权限码 |
| `app/server/internal/repository/domain_role_repo.go` | seed 列表删 guest |
| `app/server/internal/repository/db_migrations.go` | AutoMigrate 删 `DomainGuestBan`，加两张新表 |
| `app/server/internal/repository/domain_repo.go` | 删 `GetByInviteCode`、`ResetInviteCode` |
| `app/server/internal/service/domain_service.go` | 删 `ResetInviteCode`、`GetByInviteCode`、`Join`、`DomainRoleGuest` 常量、`domainRoleLevel` guest 分支 |
| `app/server/internal/service/auth_service.go` | 删 `guest_` 前缀拦截（访客机器名约定已消失） |
| `app/server/internal/handler/domain_handler.go` | 删 `Join`、`Preview`、`ResetInvite` 三个方法及 `JoinDomainRequest` |
| `app/server/internal/router/router.go` | 删 guest 路由 import/字段/注册，加 invite 路由 |
| `app/server/server/gin.go` | 删 ~27 处 guest 装配，加 invite service/handler 装配 |
| `app/server/internal/signal/hub.go` | 删 3 个 guest 策略字段与 setter |
| `app/server/internal/signal/hub_room_join.go` | 删 2 处 guard 调用 + `enforceGuestSpeakPolicyLocked` + `rollbackGuestSFUJoin` |
| `app/server/internal/signal/message_bridge.go` | 删 `guestMessagePolicy` 分支 |
| `app/server/internal/service/sfu_service.go` | 删 `SetGuestPolicy`/`guestIs`/`guestCaps`/`canPublish` 分支 |
| `app/server/internal/handler/message_handler.go` | 删 `SetGuestPolicy`/`guestIs`/`guestCaps`/`guestMessagingAllowed` |
| `app/server/internal/middleware/auth.go` | 删 `GuestChecker` 依赖字段 |
| `app/server/internal/permcode/domain_permcode.go` | 加 `PermDomainInviteManage`、`PermDomainMemberReview` |
| `app/server/internal/model/permission.go` | 两个新权限码的注册与默认角色分配 |

### 后端（删除）

`app/server/internal/service/guest_service.go`、`service/guest_service_test.go`、`handler/guest_handler.go`、`handler/guest_handler_test.go`、`middleware/guest.go`、`model/guest_ban.go`、`model/guest_ban_test.go`、`repository/guest_ban_repo.go`、`repository/guest_ban_repo_test.go`、`router/routes/guest/routes.go`、`signal/hub_guest_guard_test.go`

### 前端（新建）

| 文件 | 职责 |
|------|------|
| `app/web/src/api/invite.ts` | 邀请码 API 客户端：preview/create/disable/list/redemptions |
| `app/web/src/api/invite.spec.ts` | API 层单测 |
| `app/web/src/components/domain/InviteCodeTable.tsx` | 邀请码列表（码/备注/过期/已用/状态/操作） |
| `app/web/src/components/domain/InviteCodeCreateModal.tsx` | 新建邀请码弹窗（有效期 + 次数上限） |
| `app/web/src/components/domain/InviteRedemptionsPanel.tsx` | 使用记录侧滑面板 |

### 前端（修改）

| 文件 | 改动 |
|------|------|
| `app/web/src/pages/(app)/link.tsx` | 改造为邀请码管理中心（列表 + 新建 + 禁用 + 使用记录） |
| `app/web/src/utils/domainInvite.ts` | 正则由 `{8}` 改 `{36}`，字符集排除 `I`/`O` |
| `app/web/src/utils/domainInvite.test.ts` | 同步测试 |
| `app/web/src/api/domain.ts` | 删 `previewDomainInvite`、`joinDomain`、`resetDomainInviteCode` |
| `app/web/src/api/domainApi.spec.ts` | 删对应用例 |
| `app/web/src/api/apiClient.ts` | 公开白名单删 `/guest` |
| `app/web/src/api/redirectToLogin.test.ts` | 删 `/guest` 断言 |
| `app/web/src/components/domain/DomainMenu.tsx` | 邀请码从 `domain.invite_code` 改为调 invite API |
| `app/web/src/components/domain/InviteShareModal.tsx` | 无改动（接受 inviteUrl 字符串），仅确认调用方传新码 |
| `app/web/src/pages/invite/d/$code.tsx` | 第一部分只改 redirect 目标（完整重写见第二部分） |
| `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx` | 删「访客访问」「访客封禁管理」两分区 |
| `app/web/src/components/userBar.tsx`、`components/textRoom/MessageInput.tsx`、`hooks/useGlobalShortcuts.ts`、`pages/(app)/index.tsx` | 删 `isGuest`/`guestCaps` 调用 |
| `app/web/src/stores/domainStore.ts` | 删 `syncGuestCaps` 及其调用 |
| `app/web/src/pages/(app)/discover.tsx` | 删 `joinDomain`/`previewDomainInvite` 用法（第二部分重写加入流程） |

### 前端（删除）

`app/web/src/pages/guest.tsx`、`stores/guestStore.ts`、`stores/guestStore.test.ts`、`components/domain/GuestAccessSettings.tsx`、`components/domain/GuestManageTable.tsx`、`api/guest.ts`、`api/guest.spec.ts`

### 桌面端

| 文件 | 改动 |
|------|------|
| `app/desktop/deeplink.go` | 邀请路径正则 `([A-Z2-9]{8})` → `([A-HJ-NP-Z2-9]{36})` |
| `app/desktop/deeplink_test.go` | 同步用例 |
| `app/desktop/README.md` | 深链示例更新 |

---

## 任务 1：邀请码签名编解码

**文件：**
- 创建：`app/server/internal/service/invite_code.go`
- 测试：`app/server/internal/service/invite_code_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/service/invite_code_test.go`：

```go
package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func testInviteKey() []byte {
	return DeriveInviteKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "jwt-fallback")
}

func TestDeriveInviteKey(t *testing.T) {
	withStorage := DeriveInviteKey("storage-key", "jwt-key")
	withJWT := DeriveInviteKey("", "jwt-key")

	if len(withStorage) != 32 {
		t.Fatalf("derived key length = %d, want 32", len(withStorage))
	}
	if string(withStorage) == string(withJWT) {
		t.Fatal("storage key and jwt fallback must derive different keys")
	}
	if string(DeriveInviteKey("storage-key", "other")) != string(withStorage) {
		t.Fatal("derivation must not depend on jwt key when storage key is set")
	}
	if string(DeriveInviteKey("", "a")) == string(DeriveInviteKey("", "b")) {
		t.Fatal("jwt fallback must depend on jwt key")
	}
}

func TestEncodeDecodeInviteCode_RoundTrip(t *testing.T) {
	key := testInviteKey()
	expires := time.Unix(1900000000, 0).UTC()
	code := InviteCode{
		DomainPrefix: DomainPrefixOf("domain-uuid-1"),
		InviteID:     42,
		ExpiresAt:    expires,
		MaxUses:      7,
	}

	raw, err := EncodeInviteCode(key, code)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(raw) != inviteCodeLength {
		t.Fatalf("code length = %d, want %d", len(raw), inviteCodeLength)
	}
	for _, r := range raw {
		if !strings.ContainsRune(inviteAlphabet, r) {
			t.Fatalf("code contains char %q outside alphabet", r)
		}
	}

	got, err := DecodeInviteCode(key, raw, time.Unix(1800000000, 0))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.InviteID != code.InviteID {
		t.Fatalf("invite id = %d, want %d", got.InviteID, code.InviteID)
	}
	if got.DomainPrefix != code.DomainPrefix {
		t.Fatal("domain prefix mismatch")
	}
	if got.MaxUses != code.MaxUses {
		t.Fatalf("max uses = %d, want %d", got.MaxUses, code.MaxUses)
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Fatalf("expires at = %v, want %v", got.ExpiresAt, expires)
	}
}

func TestEncodeDecodeInviteCode_NeverExpires(t *testing.T) {
	key := testInviteKey()
	raw, err := EncodeInviteCode(key, InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeInviteCode(key, raw, time.Unix(4000000000, 0))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expires at = %v, want zero", got.ExpiresAt)
	}
}

func TestDecodeInviteCode_RejectsMalformed(t *testing.T) {
	key := testInviteKey()
	cases := map[string]string{
		"empty":      "",
		"too short":  "ABCDEFGH",
		"too long":   strings.Repeat("A", inviteCodeLength+1),
		"bad chars":  strings.Repeat("I", inviteCodeLength),
		"lowercase":  strings.Repeat("a", inviteCodeLength),
		"space":      " " + strings.Repeat("A", inviteCodeLength-1),
	}
	for name, raw := range cases {
		if _, err := DecodeInviteCode(key, raw, time.Now()); !errors.Is(err, ErrInviteCodeMalformed) {
			t.Fatalf("%s: err = %v, want ErrInviteCodeMalformed", name, err)
		}
	}
}

func TestDecodeInviteCode_RejectsTamperedPayload(t *testing.T) {
	key := testInviteKey()
	raw, err := EncodeInviteCode(key, InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 9, MaxUses: 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// 改动第一个字符（落在域前缀里），HMAC 必须失配。
	tampered := []byte(raw)
	if tampered[0] == inviteAlphabet[0] {
		tampered[0] = inviteAlphabet[1]
	} else {
		tampered[0] = inviteAlphabet[0]
	}
	if _, err := DecodeInviteCode(key, string(tampered), time.Now()); !errors.Is(err, ErrInviteCodeInvalid) {
		t.Fatalf("err = %v, want ErrInviteCodeInvalid", err)
	}
}

func TestDecodeInviteCode_RejectsWrongKey(t *testing.T) {
	raw, err := EncodeInviteCode(testInviteKey(), InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 3})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	other := DeriveInviteKey("another-key", "jwt")
	if _, err := DecodeInviteCode(other, raw, time.Now()); !errors.Is(err, ErrInviteCodeInvalid) {
		t.Fatalf("err = %v, want ErrInviteCodeInvalid", err)
	}
}

func TestDecodeInviteCode_RejectsExpired(t *testing.T) {
	key := testInviteKey()
	raw, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf("d"),
		InviteID:     5,
		ExpiresAt:    time.Unix(1000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := DecodeInviteCode(key, raw, time.Unix(1001, 0)); !errors.Is(err, ErrInviteCodeExpired) {
		t.Fatalf("err = %v, want ErrInviteCodeExpired", err)
	}
	if _, err := DecodeInviteCode(key, raw, time.Unix(1000, 0)); err != nil {
		t.Fatalf("expiry boundary must be inclusive: %v", err)
	}
}

func TestEncodeInviteCode_MaxUsesBoundaries(t *testing.T) {
	key := testInviteKey()
	// MaxUses 是 uint16，上限由类型本身保证；这里只验证边界值可正常往返。
	for _, maxUses := range []uint16{0, 1, InviteMaxUsesLimit} {
		raw, err := EncodeInviteCode(key, InviteCode{DomainPrefix: DomainPrefixOf("d"), InviteID: 1, MaxUses: maxUses})
		if err != nil {
			t.Fatalf("max uses %d: encode: %v", maxUses, err)
		}
		got, err := DecodeInviteCode(key, raw, time.Now())
		if err != nil {
			t.Fatalf("max uses %d: decode: %v", maxUses, err)
		}
		if got.MaxUses != maxUses {
			t.Fatalf("max uses round trip = %d, want %d", got.MaxUses, maxUses)
		}
	}
}

func TestDomainPrefixOf_StableAndDistinct(t *testing.T) {
	a1 := DomainPrefixOf("domain-a")
	a2 := DomainPrefixOf("domain-a")
	b := DomainPrefixOf("domain-b")
	if a1 != a2 {
		t.Fatal("prefix must be deterministic")
	}
	if a1 == b {
		t.Fatal("different domains must produce different prefixes")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run 'TestDeriveInviteKey|TestEncodeDecodeInviteCode|TestDecodeInviteCode|TestDomainPrefixOf' -v`
预期：FAIL，编译错误 `undefined: DeriveInviteKey`（及 `InviteCode`、`inviteAlphabet`、`inviteCodeLength` 等）

- [ ] **步骤 3：编写实现**

创建 `app/server/internal/service/invite_code.go`：

```go
package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"time"

	"GOSpeak/internal/pkg"
)

// inviteAlphabet 邀请码字符集：32 个无歧义字符（去掉 I、O、0、1），
// 正好满足 base32 的字母表长度要求。
const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var inviteEncoding = base32.NewEncoding(inviteAlphabet).WithPadding(base32.NoPadding)

const (
	// invitePayloadSize 参与 HMAC 的载荷字节数：域前缀 4 + codeID 4 + 过期 4 + 次数上限 2。
	invitePayloadSize = 14
	// inviteMACSize HMAC 截断长度；64 位足以使碰撞概率可忽略。
	inviteMACSize = 8
	// inviteCodeBytes 未编码的完整字节数。
	inviteCodeBytes = invitePayloadSize + inviteMACSize
	// inviteCodeLength base32 编码后的字符数：ceil(22*8/5) = 36。
	inviteCodeLength = 36
	// inviteKeySalt 与存储加密用途隔离的固定 salt 前缀；算法升级时追加 v2。
	inviteKeySalt = "gospeak:invite:v1"
	// InviteMaxUsesLimit 次数上限的编码上限（uint16）。
	InviteMaxUsesLimit = 65535
)

var (
	// ErrInviteCodeMalformed 码格式错误（长度、字符集、base32 解码失败）。
	ErrInviteCodeMalformed = pkg.NewAppError(pkg.INVALID_PARAMS, "invite code format is invalid")
	// ErrInviteCodeInvalid 验签失败（伪造或篡改）。
	ErrInviteCodeInvalid = pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is invalid")
	// ErrInviteCodeExpired 码已过期。
	ErrInviteCodeExpired = pkg.NewAppError(pkg.INVALID_PARAMS, "invite code has expired")
)

// InviteCode 是邀请码验签后的明文载荷。零值 ExpiresAt 表示永不过期。
type InviteCode struct {
	DomainPrefix [4]byte
	InviteID     uint32
	ExpiresAt    time.Time
	MaxUses      uint16
}

// DeriveInviteKey 从存储加密密钥派生邀请码 HMAC 密钥。
// StorageEncryptKey 为空时回退 JWTKey（开发环境），生产必设 StorageEncryptKey。
func DeriveInviteKey(storageEncryptKey, jwtKey string) []byte {
	base := storageEncryptKey
	if base == "" {
		base = jwtKey
	}
	mac := hmac.New(sha256.New, []byte(base))
	mac.Write([]byte(inviteKeySalt))
	return mac.Sum(nil)
}

// DomainPrefixOf 返回域 UUID 的 4 字节前缀（SHA-256 截断），用于验签后定位域而不泄露 UUID。
func DomainPrefixOf(domainUUID string) [4]byte {
	sum := sha256.Sum256([]byte(domainUUID))
	var out [4]byte
	copy(out[:], sum[:4])
	return out
}

// EncodeInviteCode 生成 36 字符邀请码。
func EncodeInviteCode(key []byte, code InviteCode) (string, error) {
	// MaxUses 是 uint16，65535 上限由类型保证；服务层在接收 int 时另行校验。
	var payload [invitePayloadSize]byte
	copy(payload[0:4], code.DomainPrefix[:])
	binary.BigEndian.PutUint32(payload[4:8], code.InviteID)
	var expires uint32
	if !code.ExpiresAt.IsZero() {
		expires = uint32(code.ExpiresAt.Unix())
	}
	binary.BigEndian.PutUint32(payload[8:12], expires)
	binary.BigEndian.PutUint16(payload[12:14], code.MaxUses)

	full := make([]byte, inviteCodeBytes)
	copy(full, payload[:])
	mac := hmac.New(sha256.New, key)
	mac.Write(payload[:])
	copy(full[invitePayloadSize:], mac.Sum(nil)[:inviteMACSize])

	return inviteEncoding.EncodeToString(full), nil
}

// DecodeInviteCode 校验并解析邀请码。now 用于判断过期（零值 ExpiresAt 永不过期）。
func DecodeInviteCode(key []byte, raw string, now time.Time) (*InviteCode, error) {
	if len(raw) != inviteCodeLength {
		return nil, ErrInviteCodeMalformed
	}
	for _, r := range raw {
		if !containsInviteChar(r) {
			return nil, ErrInviteCodeMalformed
		}
	}
	full, err := inviteEncoding.DecodeString(raw)
	if err != nil || len(full) != inviteCodeBytes {
		return nil, ErrInviteCodeMalformed
	}

	payload := full[:invitePayloadSize]
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(full[invitePayloadSize:], mac.Sum(nil)[:inviteMACSize]) {
		return nil, ErrInviteCodeInvalid
	}

	code := &InviteCode{InviteID: binary.BigEndian.Uint32(payload[4:8])}
	copy(code.DomainPrefix[:], payload[0:4])
	if expires := binary.BigEndian.Uint32(payload[8:12]); expires != 0 {
		code.ExpiresAt = time.Unix(int64(expires), 0).UTC()
		if now.After(code.ExpiresAt) {
			return nil, ErrInviteCodeExpired
		}
	}
	code.MaxUses = binary.BigEndian.Uint16(payload[12:14])
	return code, nil
}

func containsInviteChar(r rune) bool {
	for _, c := range inviteAlphabet {
		if c == r {
			return true
		}
	}
	return false
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run 'TestDeriveInviteKey|TestEncodeDecodeInviteCode|TestDecodeInviteCode|TestDomainPrefixOf' -v`
预期：全部 PASS（10 个测试函数）

- [ ] **步骤 5：Commit**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
git add app/server/internal/service/invite_code.go app/server/internal/service/invite_code_test.go
git commit -m "feat(invite): 自包含签名邀请码的编解码与验签"
```

---

## 任务 2：邀请码数据模型与仓库

**文件：**
- 创建：`app/server/internal/model/domain_invite.go`
- 创建：`app/server/internal/repository/domain_invite_repo.go`
- 测试：`app/server/internal/repository/domain_invite_repo_test.go`
- 修改：`app/server/internal/repository/db_migrations.go:236-263`（AutoMigrate 列表）

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/repository/domain_invite_repo_test.go`：

```go
package repository

import (
	"fmt"
	"testing"
	"time"

	"GOSpeak/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupInviteRepoDB 使用共享缓存的匿名内存库：普通 :memory: 每个连接是独立库，
// 并发用例会因连接切换而看不到表。cache=shared 让所有连接共享同一个库。
func setupInviteRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:invite_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.DomainInvite{}, &model.DomainInviteRedemption{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDomainInviteRepository_CreateAndGetByID(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	invite := &model.DomainInvite{
		DomainUUID: "domain-1", Label: "默认邀请码", Code: "CODE1",
		ExpiresAt: &expires, MaxUses: 10, CreatedBy: "user-1",
	}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	if invite.ID == 0 {
		t.Fatal("create must assign primary key")
	}

	got, err := repo.GetByID(invite.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Code != "CODE1" || got.MaxUses != 10 || got.Label != "默认邀请码" {
		t.Fatalf("unexpected invite: %+v", got)
	}
}

func TestDomainInviteRepository_GetByID_NotFound(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	if _, err := repo.GetByID(999); err != gorm.ErrRecordNotFound {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestDomainInviteRepository_ConsumeUse_Unlimited(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C", MaxUses: 0}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 3; i++ {
		ok, err := repo.ConsumeUse(invite.ID)
		if err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("consume %d: unlimited invite must always succeed", i)
		}
	}
	got, _ := repo.GetByID(invite.ID)
	if got.UsedCount != 3 {
		t.Fatalf("used count = %d, want 3", got.UsedCount)
	}
}

func TestDomainInviteRepository_ConsumeUse_Exhausts(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C", MaxUses: 2}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 2; i++ {
		ok, err := repo.ConsumeUse(invite.ID)
		if err != nil || !ok {
			t.Fatalf("consume %d: ok=%v err=%v", i, ok, err)
		}
	}
	ok, err := repo.ConsumeUse(invite.ID)
	if err != nil {
		t.Fatalf("consume over limit: %v", err)
	}
	if ok {
		t.Fatal("consume beyond max_uses must report exhausted")
	}
	got, _ := repo.GetByID(invite.ID)
	if got.UsedCount != 2 {
		t.Fatalf("used count = %d, want 2 (over-limit consume must not increment)", got.UsedCount)
	}
}

func TestDomainInviteRepository_ConsumeUse_Disabled(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C"}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Disable(invite.ID, "user-1"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	ok, err := repo.ConsumeUse(invite.ID)
	if err != nil {
		t.Fatalf("consume disabled: %v", err)
	}
	if ok {
		t.Fatal("disabled invite must not be consumable")
	}
}

func TestDomainInviteRepository_Disable_Idempotent(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C"}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Disable(invite.ID, "u1"); err != nil {
		t.Fatalf("first disable: %v", err)
	}
	first, _ := repo.GetByID(invite.ID)
	if first.DisabledAt == nil {
		t.Fatal("disabled_at must be set")
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.Disable(invite.ID, "u2"); err != nil {
		t.Fatalf("second disable: %v", err)
	}
	second, _ := repo.GetByID(invite.ID)
	if !second.DisabledAt.Equal(*first.DisabledAt) {
		t.Fatal("disable must be idempotent (disabled_at unchanged)")
	}
}

func TestDomainInviteRepository_ListByDomain(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	for _, code := range []string{"A", "B"} {
		if err := repo.Create(&model.DomainInvite{DomainUUID: "d1", Code: code}); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	if err := repo.Create(&model.DomainInvite{DomainUUID: "d2", Code: "C"}); err != nil {
		t.Fatalf("create d2: %v", err)
	}
	list, err := repo.ListByDomain("d1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list length = %d, want 2", len(list))
	}
}

func TestDomainInviteRepository_Redemptions(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C"}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}
	records := []*model.DomainInviteRedemption{
		{InviteID: invite.ID, DomainUUID: "d", UserUUID: "u1", Code: "C", Success: true},
		{InviteID: invite.ID, DomainUUID: "d", UserUUID: "u2", Code: "C", Success: false, FailReason: "expired"},
	}
	for _, rec := range records {
		if err := repo.CreateRedemption(rec); err != nil {
			t.Fatalf("create redemption: %v", err)
		}
	}
	list, total, err := repo.ListRedemptions(invite.ID, 1, 10)
	if err != nil {
		t.Fatalf("list redemptions: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("total=%d len=%d, want 2/2", total, len(list))
	}
}

func TestDomainInviteRepository_ConcurrentConsume_NoOversell(t *testing.T) {
	db := setupInviteRepoDB(t)
	repo := NewDomainInviteRepository(db)
	invite := &model.DomainInvite{DomainUUID: "d", Code: "C", MaxUses: 5}
	if err := repo.Create(invite); err != nil {
		t.Fatalf("create: %v", err)
	}

	const workers = 20
	results := make(chan bool, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			ok, err := repo.ConsumeUse(invite.ID)
			results <- ok
			errs <- err
		}()
	}
	successes := 0
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent consume: %v", err)
		}
		if <-results {
			successes++
		}
	}
	if successes != 5 {
		t.Fatalf("successes = %d, want exactly 5 (conditional update must prevent oversell)", successes)
	}
	got, _ := repo.GetByID(invite.ID)
	if got.UsedCount != 5 {
		t.Fatalf("used count = %d, want 5", got.UsedCount)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/repository/ -run TestDomainInviteRepository -v`
预期：FAIL，编译错误 `undefined: model.DomainInvite` / `undefined: NewDomainInviteRepository`

- [ ] **步骤 3：创建模型**

创建 `app/server/internal/model/domain_invite.go`：

```go
package model

import "time"

// DomainInvite 是域邀请码。Code 是自包含签名串，ExpiresAt 与 MaxUses
// 在创建时写入码内，之后不可修改；UsedCount 是库侧权威计数。
type DomainInvite struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DomainUUID string     `gorm:"size:32;index:idx_domain_invite_domain,priority:1;not null" json:"domain_uuid"`
	Label      string     `gorm:"size:100" json:"label"`
	Code       string     `gorm:"size:64;uniqueIndex;not null" json:"code"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	MaxUses    int        `gorm:"default:0" json:"max_uses"`
	UsedCount  int        `gorm:"default:0" json:"used_count"`
	DisabledAt *time.Time `gorm:"index:idx_domain_invite_domain,priority:2" json:"disabled_at,omitempty"`
	CreatedBy  string     `gorm:"size:36;index" json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (DomainInvite) TableName() string { return "domain_invites" }

// IsDisabled 报告邀请码是否已被显式禁用。
func (i *DomainInvite) IsDisabled() bool { return i != nil && i.DisabledAt != nil }

// IsExhausted 报告已用次数是否达到上限（MaxUses 为 0 表示不限）。
func (i *DomainInvite) IsExhausted() bool {
	return i != nil && i.MaxUses > 0 && i.UsedCount >= i.MaxUses
}

// DomainInviteRedemption 是邀请码使用记录。失败尝试也记录，
// 不存 IP，不存码的修改历史。
type DomainInviteRedemption struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InviteID   uint      `gorm:"index:idx_invite_redemption_invite;not null" json:"invite_id"`
	DomainUUID string    `gorm:"size:32;index" json:"domain_uuid"`
	UserUUID   string    `gorm:"size:36;index" json:"user_uuid"`
	Code       string    `gorm:"size:64" json:"code"`
	Success    bool      `gorm:"default:false" json:"success"`
	FailReason string    `gorm:"size:32" json:"fail_reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (DomainInviteRedemption) TableName() string { return "domain_invite_redemptions" }

// 邀请码使用失败原因，写入 DomainInviteRedemption.FailReason。
const (
	InviteFailExpired       = "expired"
	InviteFailExhausted     = "exhausted"
	InviteFailInvalid       = "invalid"
	InviteFailDisabled      = "disabled"
	InviteFailRejected      = "rejected"
	InviteFailAlreadyMember = "already_member"
)
```

- [ ] **步骤 4：创建仓库**

创建 `app/server/internal/repository/domain_invite_repo.go`：

```go
package repository

import (
	"time"

	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type DomainInviteRepository struct {
	db *gorm.DB
}

func NewDomainInviteRepository(db *gorm.DB) *DomainInviteRepository {
	return &DomainInviteRepository{db: db}
}

func (r *DomainInviteRepository) Create(invite *model.DomainInvite) error {
	return r.db.Create(invite).Error
}

func (r *DomainInviteRepository) GetByID(id uint) (*model.DomainInvite, error) {
	var invite model.DomainInvite
	err := r.db.Where("id = ?", id).First(&invite).Error
	return &invite, err
}

// ConsumeUse 以条件更新原子消耗一次使用额度。
// 返回 false 表示码已禁用或次数已耗尽（调用方应回滚事务并报错）。
// 该条件更新本身即并发保护，不引入分布式锁。
func (r *DomainInviteRepository) ConsumeUse(id uint) (bool, error) {
	res := r.db.Model(&model.DomainInvite{}).
		Where("id = ?", id).
		Where("disabled_at IS NULL").
		Where("max_uses = 0 OR used_count < max_uses").
		UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Disable 禁用邀请码，幂等（已禁用时保持原 disabled_at 不变）。
func (r *DomainInviteRepository) Disable(id uint, operatorUUID string) error {
	now := time.Now()
	return r.db.Model(&model.DomainInvite{}).
		Where("id = ? AND disabled_at IS NULL", id).
		Updates(map[string]interface{}{"disabled_at": &now}).Error
}

func (r *DomainInviteRepository) ListByDomain(domainUUID string) ([]model.DomainInvite, error) {
	var list []model.DomainInvite
	err := r.db.Where("domain_uuid = ?", domainUUID).
		Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *DomainInviteRepository) CreateRedemption(rec *model.DomainInviteRedemption) error {
	return r.db.Create(rec).Error
}

func (r *DomainInviteRepository) ListRedemptions(inviteID uint, page, pageSize int) ([]model.DomainInviteRedemption, int64, error) {
	var list []model.DomainInviteRedemption
	var total int64
	q := r.db.Model(&model.DomainInviteRedemption{}).Where("invite_id = ?", inviteID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}
```

- [ ] **步骤 5：注册 AutoMigrate**

修改 `app/server/internal/repository/db_migrations.go`，在 `autoMigrate()` 的 AutoMigrate 列表中，把 `&model.DomainGuestBan{},` 这一行替换为两张新表（`DomainGuestBan` 在任务 6 删除）：

```go
		&model.AuditLog{},
		&model.DomainInvite{},
		&model.DomainInviteRedemption{},
		&model.Announcement{},
```

- [ ] **步骤 6：运行测试验证通过**

运行：`cd app/server && go test ./internal/repository/ -run TestDomainInviteRepository -v`
预期：全部 PASS（9 个测试函数），其中并发用例断言恰好 5 次成功

- [ ] **步骤 7：Commit**

```bash
git add app/server/internal/model/domain_invite.go app/server/internal/repository/domain_invite_repo.go app/server/internal/repository/domain_invite_repo_test.go app/server/internal/repository/db_migrations.go
git commit -m "feat(invite): 邀请码与使用记录数据模型及仓库"
```

---

## 任务 3：邀请码服务层

**文件：**
- 创建：`app/server/internal/service/invite_service.go`
- 测试：`app/server/internal/service/invite_service_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/service/invite_service_test.go`：

```go
package service

import (
	"fmt"
	"testing"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const testStorageKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupInviteService(t *testing.T) (*InviteService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.DomainInvite{}, &model.DomainInviteRedemption{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewInviteService(
		db,
		repository.NewDomainInviteRepository(db),
		repository.NewDomainRepository(db),
		testStorageKey,
		"jwt-key",
	)
	return svc, db
}

func seedInviteDomain(t *testing.T, db *gorm.DB) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "InviteRealm", OwnerUUID: "00000000-0000-0000-0000-000000000001"}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func TestInviteService_Create_GeneratesCode(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)

	invite, err := svc.Create(d.UUID, "测试码", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(invite.Code) != inviteCodeLength {
		t.Fatalf("code length = %d, want %d", len(invite.Code), inviteCodeLength)
	}
	if invite.ID == 0 {
		t.Fatal("invite must be persisted")
	}
}

func TestInviteService_Create_RejectsMaxUsesOverflow(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	if _, err := svc.Create(d.UUID, "", nil, InviteMaxUsesLimit+1, "user-1"); err == nil {
		t.Fatal("max_uses over limit must be rejected")
	}
}

func TestInviteService_Create_RejectsPastExpiry(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	past := time.Now().Add(-time.Hour)
	if _, err := svc.Create(d.UUID, "", &past, 0, "user-1"); err == nil {
		t.Fatal("expiry in the past must be rejected")
	}
}

func TestInviteService_CreateDefaultForDomain(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.CreateDefaultForDomain(d.UUID, "user-1")
	if err != nil {
		t.Fatalf("create default: %v", err)
	}
	if invite.Label != "默认邀请码" {
		t.Fatalf("label = %q, want 默认邀请码", invite.Label)
	}
	if invite.MaxUses != 0 || invite.ExpiresAt != nil {
		t.Fatal("default invite must be unlimited and never expire")
	}
}

func TestInviteService_Validate_Success(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 5, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Validate(invite.Code)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Domain.UUID != d.UUID {
		t.Fatalf("domain = %s, want %s", got.Domain.UUID, d.UUID)
	}
	if got.RemainingUses != 5 {
		t.Fatalf("remaining = %d, want 5", got.RemainingUses)
	}
}

func TestInviteService_Validate_RejectsExpired(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	// 直接构造一个已过期的码：绕过 Create 的过去时间校验，用底层编码器。
	key := DeriveInviteKey(testStorageKey, "jwt-key")
	expires := time.Now().Add(-time.Minute)
	raw, err := EncodeInviteCode(key, InviteCode{
		DomainPrefix: DomainPrefixOf(d.UUID), InviteID: 1, ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = svc.Validate(raw)
	if !pkg.IsCode(err, pkg.INVALID_PARAMS) {
		t.Fatalf("err = %v, want INVALID_PARAMS", err)
	}
}

func TestInviteService_Validate_RejectsDisabled(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Disable(invite.ID, "user-1"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := svc.Validate(invite.Code); err == nil {
		t.Fatal("disabled invite must be rejected")
	}
}

func TestInviteService_Validate_RejectsExhausted(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 1, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Consume(invite.Code, "u1", true); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := svc.Validate(invite.Code); err == nil {
		t.Fatal("exhausted invite must be rejected")
	}
}

func TestInviteService_Validate_RejectsTampered(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	tampered := []byte(invite.Code)
	if tampered[0] == inviteAlphabet[0] {
		tampered[0] = inviteAlphabet[1]
	} else {
		tampered[0] = inviteAlphabet[0]
	}
	if _, err := svc.Validate(string(tampered)); err == nil {
		t.Fatal("tampered code must be rejected")
	}
}

func TestInviteService_Consume_WritesRedemption(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 2, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	domain, err := svc.Consume(invite.Code, "u1", true)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if domain.UUID != d.UUID {
		t.Fatalf("domain = %s, want %s", domain.UUID, d.UUID)
	}
	var recs []model.DomainInviteRedemption
	if err := db.Find(&recs).Error; err != nil {
		t.Fatalf("query redemptions: %v", err)
	}
	if len(recs) != 1 || !recs[0].Success || recs[0].UserUUID != "u1" {
		t.Fatalf("unexpected redemptions: %+v", recs)
	}
}

func TestInviteService_Consume_FailureRecordsReason(t *testing.T) {
	svc, db := setupInviteService(t)
	d := seedInviteDomain(t, db)
	invite, err := svc.Create(d.UUID, "", nil, 1, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Consume(invite.Code, "u1", true); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := svc.Consume(invite.Code, "u2", true); err == nil {
		t.Fatal("second consume must fail")
	}
	var recs []model.DomainInviteRedemption
	if err := db.Order("id ASC").Find(&recs).Error; err != nil {
		t.Fatalf("query redemptions: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("redemptions = %d, want 2 (failure must be recorded)", len(recs))
	}
	if recs[1].Success || recs[1].FailReason != model.InviteFailExhausted {
		t.Fatalf("unexpected failure record: %+v", recs[1])
	}
}

func TestInviteService_CrossDomainIsolation(t *testing.T) {
	svc, db := setupInviteService(t)
	d1 := seedInviteDomain(t, db)
	d2 := seedInviteDomain(t, db)
	invite, err := svc.Create(d1.UUID, "", nil, 0, "user-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Validate(invite.Code)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Domain.UUID == d2.UUID {
		t.Fatal("invite must resolve to its own domain")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/service/ -run TestInviteService -v`
预期：FAIL，编译错误 `undefined: NewInviteService`

- [ ] **步骤 3：编写实现**

创建 `app/server/internal/service/invite_service.go`：

```go
package service

import (
	"log"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// InviteValidation 是邀请码校验成功后的结果。
type InviteValidation struct {
	Invite        *model.DomainInvite
	Domain        *model.Domain
	RemainingUses int
}

// InviteService 管理域邀请码的创建、校验、消耗与禁用。
type InviteService struct {
	db         *gorm.DB
	inviteRepo *repository.DomainInviteRepository
	domainRepo *repository.DomainRepository
	key        []byte
}

// NewInviteService 构造 InviteService。storageEncryptKey 为空时回退 jwtKey 并打 warn。
func NewInviteService(
	db *gorm.DB,
	inviteRepo *repository.DomainInviteRepository,
	domainRepo *repository.DomainRepository,
	storageEncryptKey, jwtKey string,
) *InviteService {
	if strings.TrimSpace(storageEncryptKey) == "" {
		log.Printf("[invite] STORAGE_ENCRYPT_KEY is empty; falling back to JWT_KEY for invite signing")
	}
	return &InviteService{
		db:         db,
		inviteRepo: inviteRepo,
		domainRepo: domainRepo,
		key:        DeriveInviteKey(storageEncryptKey, jwtKey),
	}
}

// Create 为域创建邀请码。expiresAt 为 nil 表示永不过期；maxUses 为 0 表示不限。
func (s *InviteService) Create(domainUUID, label string, expiresAt *time.Time, maxUses int, createdBy string) (*model.DomainInvite, error) {
	if domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	if maxUses < 0 || maxUses > InviteMaxUsesLimit {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "max_uses must be between 0 and 65535")
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "expires_at must be in the future")
	}
	if _, err := s.domainRepo.GetByUUID(domainUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// codeID 由自增主键决定，因此先插入占位行拿到 ID，再回填签名码。
	invite := &model.DomainInvite{
		DomainUUID: domainUUID,
		Label:      strings.TrimSpace(label),
		ExpiresAt:  expiresAt,
		MaxUses:    maxUses,
		CreatedBy:  createdBy,
	}
	if err := s.inviteRepo.Create(invite); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	code, err := EncodeInviteCode(s.key, InviteCode{
		DomainPrefix: DomainPrefixOf(domainUUID),
		InviteID:     uint32(invite.ID),
		ExpiresAt:    expiresAt,
		MaxUses:      uint16(maxUses),
	})
	if err != nil {
		return nil, err
	}
	invite.Code = code
	if err := s.inviteRepo.UpdateCode(invite.ID, code); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return invite, nil
}

// CreateDefaultForDomain 为新域生成默认邀请码（永不过期、不限次数）。
func (s *InviteService) CreateDefaultForDomain(domainUUID, createdBy string) (*model.DomainInvite, error) {
	return s.Create(domainUUID, "默认邀请码", nil, 0, createdBy)
}

// Validate 校验邀请码并返回其域。失败时统一返回 INVALID_PARAMS，
// 且不泄露任何域信息。
func (s *InviteService) Validate(raw string) (*InviteValidation, error) {
	code, err := DecodeInviteCode(s.key, strings.TrimSpace(raw), time.Now())
	if err != nil {
		return nil, err
	}
	invite, err := s.inviteRepo.GetByID(uint(code.InviteID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if invite.Code != strings.TrimSpace(raw) {
		return nil, ErrInviteCodeInvalid
	}
	if invite.IsDisabled() {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is disabled")
	}
	if invite.IsExhausted() {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite code has reached its usage limit")
	}
	domain, err := s.domainRepo.GetByUUID(invite.DomainUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	remaining := -1
	if invite.MaxUses > 0 {
		remaining = invite.MaxUses - invite.UsedCount
	}
	return &InviteValidation{Invite: invite, Domain: domain, RemainingUses: remaining}, nil
}

// Consume 在事务内消耗一次额度并写使用记录。recordSuccess 为 false 时只写失败记录。
func (s *InviteService) Consume(raw, userUUID string, recordSuccess bool) (*model.Domain, error) {
	code, decodeErr := DecodeInviteCode(s.key, strings.TrimSpace(raw), time.Now())
	if decodeErr != nil {
		return nil, decodeErr
	}
	invite, err := s.inviteRepo.GetByID(uint(code.InviteID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	var domain *model.Domain
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDomainInviteRepository(tx)
		ok, err := repo.ConsumeUse(invite.ID)
		if err != nil {
			return err
		}
		if !ok {
			reason := model.InviteFailExhausted
			if invite.IsDisabled() {
				reason = model.InviteFailDisabled
			}
			_ = repo.CreateRedemption(&model.DomainInviteRedemption{
				InviteID: invite.ID, DomainUUID: invite.DomainUUID, UserUUID: userUUID,
				Code: invite.Code, Success: false, FailReason: reason,
			})
			return pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is no longer usable")
		}
		d, err := s.domainRepo.GetByUUID(invite.DomainUUID)
		if err != nil {
			return err
		}
		domain = d
		return repo.CreateRedemption(&model.DomainInviteRedemption{
			InviteID: invite.ID, DomainUUID: invite.DomainUUID, UserUUID: userUUID,
			Code: invite.Code, Success: recordSuccess,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}
	return domain, nil
}

// Disable 禁用邀请码。
func (s *InviteService) Disable(inviteID uint, operatorUUID string) error {
	if _, err := s.inviteRepo.GetByID(inviteID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.NOT_FOUND, "invite code not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.inviteRepo.Disable(inviteID, operatorUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// List 列出域内全部邀请码。
func (s *InviteService) List(domainUUID string) ([]model.DomainInvite, error) {
	list, err := s.inviteRepo.ListByDomain(domainUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

// ListRedemptions 分页查询某邀请码的使用记录。
func (s *InviteService) ListRedemptions(inviteID uint, page, pageSize int) ([]model.DomainInviteRedemption, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	list, total, err := s.inviteRepo.ListRedemptions(inviteID, page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, total, nil
}
```

- [ ] **步骤 4：给仓库补 `UpdateCode`**

在 `app/server/internal/repository/domain_invite_repo.go` 追加：

```go
// UpdateCode 回填签名码（创建时先插行拿自增 ID，再生成依赖 ID 的码）。
func (r *DomainInviteRepository) UpdateCode(id uint, code string) error {
	return r.db.Model(&model.DomainInvite{}).
		Where("id = ?", id).
		UpdateColumn("code", code).Error
}
```

- [ ] **步骤 5：给 `pkg` 补错误码判定助手**

在 `app/server/internal/pkg/errors.go` 末尾追加：

```go
// IsCode 判断 err 是否为携带指定业务码的 AppError。
func IsCode(err error, code ErrCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}
```

- [ ] **步骤 6：运行测试验证通过**

运行：`cd app/server && go test ./internal/service/ -run TestInviteService -v`
预期：全部 PASS（11 个测试函数）

- [ ] **步骤 7：Commit**

```bash
git add app/server/internal/service/invite_service.go app/server/internal/service/invite_service_test.go app/server/internal/repository/domain_invite_repo.go app/server/internal/pkg/errors.go
git commit -m "feat(invite): 邀请码服务层（创建/校验/消耗/禁用/审计）"
```

---

## 任务 4：邀请码权限码

**文件：**
- 修改：`app/server/internal/permcode/domain_permcode.go:3-11`
- 修改：`app/server/internal/model/permission.go:27-33`、`:75`、`:128`
- 修改：`app/server/internal/model/domain_role.go:38-66`
- 测试：`app/server/internal/model/permission_test.go`、`app/server/internal/model/domain_role_test.go`

- [ ] **步骤 1：编写失败的测试**

在 `app/server/internal/model/permission_test.go` 追加：

```go
func TestInviteManagePermission_Registered(t *testing.T) {
	found := false
	for _, p := range DefaultPermissions {
		if p.Code == PermDomainInviteManage {
			found = true
			if p.Name == "" || p.Description == "" {
				t.Fatal("invite manage permission must have name and description")
			}
		}
	}
	if !found {
		t.Fatal("PermDomainInviteManage must be in DefaultPermissions seed")
	}
}

func TestInviteManagePermission_GrantedToAdmin(t *testing.T) {
	codes := DefaultRolePermissions["admin"]
	for _, want := range []string{PermDomainInviteManage, PermDomainMemberReview} {
		found := false
		for _, code := range codes {
			if code == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("admin must hold %s", want)
		}
	}
}
```

在 `app/server/internal/model/domain_role_test.go`（不存在则新建）写入：

```go
package model

import "testing"

func TestAssignableDomainPermissions_IncludesInvitePermissions(t *testing.T) {
	set := AssignableDomainPermissionsSet()
	for _, want := range []string{PermDomainInviteManage, PermDomainMemberReview} {
		if _, ok := set[want]; !ok {
			t.Fatalf("%s must be assignable", want)
		}
	}
}

func TestDefaultDomainRolePermissions_AdminHoldsInvitePermissions(t *testing.T) {
	codes := DefaultDomainRolePermissions[DomainRoleAdmin]
	for _, want := range []string{PermDomainInviteManage, PermDomainMemberReview} {
		found := false
		for _, code := range codes {
			if code == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("domain admin must hold %s", want)
		}
	}
}

func TestDefaultDomainRolePermissions_MemberLacksInvitePermissions(t *testing.T) {
	for _, code := range DefaultDomainRolePermissions[DomainRoleMember] {
		if code == PermDomainInviteManage || code == PermDomainMemberReview {
			t.Fatalf("member must not hold %s", code)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/model/ -run 'TestInviteManagePermission|TestAssignableDomainPermissions_IncludesInvitePermissions|TestDefaultDomainRolePermissions' -v`
预期：FAIL，编译错误 `undefined: PermDomainInviteManage`

- [ ] **步骤 3：新增权限码常量**

修改 `app/server/internal/permcode/domain_permcode.go`，把常量块改为：

```go
const (
	PermDomainCreate       = "domain:create"
	PermDomainRead         = "domain:read"
	PermDomainManage       = "domain:manage"
	PermDomainDelete       = "domain:delete"
	PermDomainInvite       = "domain:invite"
	PermDomainInviteManage = "domain:invite:manage"
	PermDomainMemberReview = "domain:member:review"
	PermDomainKick         = "domain:kick"
	PermDomainRoleManage   = "domain:role:manage"
)
```

- [ ] **步骤 4：注册到 model 别名与种子**

修改 `app/server/internal/model/permission.go`：在别名块（第 27–33 行附近）追加两行：

```go
	PermDomainInvite       = permcode.PermDomainInvite
	PermDomainInviteManage = permcode.PermDomainInviteManage
	PermDomainMemberReview = permcode.PermDomainMemberReview
	PermDomainKick         = permcode.PermDomainKick
```

在 `DefaultPermissions` 的 `PermDomainInvite` 那一行之后追加：

```go
	{Code: PermDomainInviteManage, Name: "管理邀请码", Description: "创建、禁用邀请码并查看使用记录"},
	{Code: PermDomainMemberReview, Name: "审核加入申请", Description: "审批或拒绝域加入申请"},
```

在 `DefaultRolePermissions["admin"]` 的 `PermDomainInvite, PermDomainKick, PermDomainRoleManage,` 那一行改为：

```go
		PermDomainInvite, PermDomainInviteManage, PermDomainMemberReview, PermDomainKick, PermDomainRoleManage,
```

- [ ] **步骤 5：更新域角色权限矩阵**

修改 `app/server/internal/model/domain_role.go`，把 `AssignableDomainPermissions` 与 `DefaultDomainRolePermissions` 改为：

```go
// AssignableDomainPermissions 域角色可分配的权限码白名单。
var AssignableDomainPermissions = []string{
	PermDomainManage, PermDomainInvite, PermDomainInviteManage, PermDomainMemberReview,
	PermDomainKick, PermDomainRoleManage,
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
		PermDomainManage, PermDomainInvite, PermDomainInviteManage, PermDomainMemberReview,
		PermDomainKick, PermDomainRoleManage,
		PermRoomCreate, PermRoomRead, PermRoomUpdate, PermRoomDelete,
		PermMessageSend, PermMessageRead, PermMessageDeleteOthers,
	},
	DomainRoleMember: {
		PermRoomCreate, PermRoomRead, PermMessageSend, PermMessageRead,
	},
}
```

注意：这里**同时删掉了 `DomainRoleGuest` 的权限条目**（访客角色在任务 6 删除）。若想让任务 4 与任务 6 解耦，可先保留 `DomainRoleGuest: {PermRoomRead, PermMessageRead},` 一行，由任务 6 删除；两种顺序都能编译。

- [ ] **步骤 6：运行测试验证通过**

运行：`cd app/server && go test ./internal/model/ -v -run 'TestInvite|TestAssignable|TestDefaultDomainRole'`
预期：全部 PASS

- [ ] **步骤 7：Commit**

```bash
git add app/server/internal/permcode/domain_permcode.go app/server/internal/model/permission.go app/server/internal/model/domain_role.go app/server/internal/model/permission_test.go app/server/internal/model/domain_role_test.go
git commit -m "feat(permission): 新增邀请码管理与加入审核权限码"
```

---

## 任务 5：邀请码 HTTP 端点与装配

**文件：**
- 创建：`app/server/internal/handler/invite_handler.go`
- 创建：`app/server/internal/handler/invite_handler_test.go`
- 创建：`app/server/internal/router/routes/invite/routes.go`
- 修改：`app/server/internal/router/router.go:28-66`、`:96-133`
- 修改：`app/server/server/gin.go:107`、`:134`、`:363`
- 修改：`app/server/internal/service/domain_service.go:116-140`（Create 后建默认码）

- [ ] **步骤 1：编写失败的测试**

创建 `app/server/internal/handler/invite_handler_test.go`：

```go
package handler

import (
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

type inviteHandlerEnv struct {
	router *gin.Engine
	db     *gorm.DB
	svc    *service.InviteService
}

func newInviteHandlerEnv(t *testing.T) *inviteHandlerEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainMember{}, &model.DomainInvite{}, &model.DomainInviteRedemption{}, &model.DomainRole{}, &model.DomainRolePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := service.NewInviteService(
		db,
		repository.NewDomainInviteRepository(db),
		repository.NewDomainRepository(db),
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"jwt-key",
	)
	h := NewInviteHandler(svc)
	router := gin.New()
	router.POST("/api/v1/domain/invites/create", h.Create)
	router.POST("/api/v1/domain/invites/list", h.List)
	router.POST("/api/v1/domain/invites/disable", h.Disable)
	router.POST("/api/v1/domain/invites/redemptions", h.Redemptions)
	router.POST("/api/v1/invite/preview", h.Preview)
	return &inviteHandlerEnv{router: router, db: db, svc: svc}
}

func (e *inviteHandlerEnv) seedDomain(t *testing.T) *model.Domain {
	t.Helper()
	d := &model.Domain{Name: "InviteRealm", OwnerUUID: "00000000-0000-0000-0000-000000000001"}
	if err := e.db.Create(d).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func (e *inviteHandlerEnv) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestInviteHandler_Create_ReturnsCode(t *testing.T) {
	env := newInviteHandlerEnv(t)
	d := env.seedDomain(t)
	rec := env.post(t, "/api/v1/domain/invites/create", `{"domain_uuid":"`+d.UUID+`","label":"测试"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int                 `json:"code"`
		Data model.DomainInvite  `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 0 || len(resp.Data.Code) != 36 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInviteHandler_Create_RejectsMissingDomain(t *testing.T) {
	env := newInviteHandlerEnv(t)
	rec := env.post(t, "/api/v1/domain/invites/create", `{}`)
	if rec.Code == http.StatusOK {
		t.Fatal("missing domain_uuid must fail")
	}
}

func TestInviteHandler_List_ReturnsInvites(t *testing.T) {
	env := newInviteHandlerEnv(t)
	d := env.seedDomain(t)
	if _, err := env.svc.Create(d.UUID, "a", nil, 0, "u1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	rec := env.post(t, "/api/v1/domain/invites/list", `{"domain_uuid":"`+d.UUID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Invites []model.DomainInvite `json:"invites"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Invites) != 1 {
		t.Fatalf("invites = %d, want 1", len(resp.Data.Invites))
	}
}

func TestInviteHandler_Preview_ReturnsDomainWithoutCode(t *testing.T) {
	env := newInviteHandlerEnv(t)
	d := env.seedDomain(t)
	invite, err := env.svc.Create(d.UUID, "", nil, 0, "u1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rec := env.post(t, "/api/v1/invite/preview", `{"code":"`+invite.Code+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), invite.Code) {
		t.Fatal("preview must not echo the invite code")
	}
}

func TestInviteHandler_Preview_RejectsInvalidCode(t *testing.T) {
	env := newInviteHandlerEnv(t)
	rec := env.post(t, "/api/v1/invite/preview", `{"code":"`+strings.Repeat("A", 36)+`"}`)
	if rec.Code == http.StatusOK {
		t.Fatal("invalid code must fail")
	}
	if strings.Contains(rec.Body.String(), "InviteRealm") {
		t.Fatal("invalid code must not leak domain info")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/server && go test ./internal/handler/ -run TestInviteHandler -v`
预期：FAIL，编译错误 `undefined: NewInviteHandler`

- [ ] **步骤 3：编写 handler**

创建 `app/server/internal/handler/invite_handler.go`：

```go
package handler

import (
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// InviteHandler 邀请码管理端与公开预览端点。
type InviteHandler struct {
	svc        *service.InviteService
	domainSvc  domainPermissionChecker
	permSvc    globalPermissionChecker
}

func NewInviteHandler(svc *service.InviteService) *InviteHandler {
	return &InviteHandler{svc: svc}
}

// SetPermissionCheckers 注入域/平台权限判决依赖（管理端点用）。
func (h *InviteHandler) SetPermissionCheckers(domainSvc domainPermissionChecker, permSvc globalPermissionChecker) {
	h.domainSvc = domainSvc
	h.permSvc = permSvc
}

func (h *InviteHandler) canManage(c *gin.Context, domainUUID string) bool {
	return domainPermissionGranted(c, domainUUID, permcode.PermDomainInviteManage, h.domainSvc, h.permSvc)
}

type createInviteRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	Label      string `json:"label"`
	ExpiresIn  int    `json:"expires_in_hours"`
	MaxUses    int    `json:"max_uses"`
}

func (h *InviteHandler) Create(c *gin.Context) {
	var req createInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &t
	}
	invite, err := h.svc.Create(req.DomainUUID, req.Label, expiresAt, req.MaxUses, currentUserUUID(c))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, invite)
}

type domainUUIDRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

func (h *InviteHandler) List(c *gin.Context) {
	var req domainUUIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	invites, err := h.svc.List(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"invites": invites})
}

type disableInviteRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	InviteID   uint   `json:"invite_id" binding:"required"`
}

func (h *InviteHandler) Disable(c *gin.Context) {
	var req disableInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	if err := h.svc.Disable(req.InviteID, currentUserUUID(c)); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

type redemptionsRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
	InviteID   uint   `json:"invite_id" binding:"required"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

func (h *InviteHandler) Redemptions(c *gin.Context) {
	var req redemptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.canManage(c, req.DomainUUID) {
		pkg.Fail(c, pkg.FORBIDDEN, "missing domain invite manage permission")
		return
	}
	items, total, err := h.svc.ListRedemptions(req.InviteID, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total})
}

type previewInviteRequest struct {
	Code string `json:"code" binding:"required"`
}

// Preview 是公开端点：验码后返回域名/图标/描述与是否需审核。
// 失败一律不泄露域信息。
func (h *InviteHandler) Preview(c *gin.Context) {
	var req previewInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invite code is required")
		return
	}
	result, err := h.svc.Validate(req.Code)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{
		"domain": gin.H{
			"uuid":        result.Domain.UUID,
			"name":        result.Domain.Name,
			"icon_url":    result.Domain.IconURL,
			"description": result.Domain.Description,
		},
		"requires_approval": result.Domain.RequireJoinApproval,
		"remaining_uses":    result.RemainingUses,
	})
}
```

注意：`RequireJoinApproval` 字段在第二部分任务中新增。若第一部分先落地，可在 Preview 中先省略该字段并返回 `requires_approval: false`；第二部分补上。

- [ ] **步骤 4：注册路由**

创建 `app/server/internal/router/routes/invite/routes.go`：

```go
package invite

import (
	"GOSpeak/internal/handler"

	"github.com/gin-gonic/gin"
)

// Register 注册邀请码管理端点（protected 组，权限在 handler 内按域校验）。
func Register(r *gin.RouterGroup, h *handler.InviteHandler) {
	r.POST("/domain/invites/list", h.List)
	r.POST("/domain/invites/create", h.Create)
	r.POST("/domain/invites/disable", h.Disable)
	r.POST("/domain/invites/redemptions", h.Redemptions)
}

// RegisterPublic 注册公开预览端点（无需登录）。
func RegisterPublic(r *gin.RouterGroup, h *handler.InviteHandler) {
	r.POST("/preview", h.Preview)
}
```

- [ ] **步骤 5：接线到 router.go**

修改 `app/server/internal/router/router.go`：

1. import 块追加：

```go
	inviteRoutes "GOSpeak/internal/router/routes/invite"
```

2. `Handlers` 结构体在 `Guest` 字段位置（任务 6 会删 `Guest`）追加：

```go
	Invite       *handler.InviteHandler
```

3. 公开路由区（`signalRoutes.Register(api.Group("/signal"), h.Signal)` 之后）追加：

```go
	if h.Invite != nil {
		inviteRoutes.RegisterPublic(api.Group("/invite"), h.Invite)
	}
```

4. protected 路由区（`systemRoutes.RegisterProtected(...)` 之前）追加：

```go
	if h.Invite != nil {
		inviteRoutes.Register(protected.Group(""), h.Invite)
	}
```

- [ ] **步骤 6：装配到 gin.go**

在 `app/server/server/gin.go` 的 `authSvc := service.NewAuthService(...)` 之后追加：

```go
	inviteSvc := service.NewInviteService(db, repository.NewDomainInviteRepository(db), domainRepo, cfg.StorageEncryptKey, cfg.JWTKey)
```

在 `guestH := handler.NewGuestHandler(...)` 附近（任务 6 删除 guestH 后仍保留这行）追加：

```go
	inviteH := handler.NewInviteHandler(inviteSvc)
	inviteH.SetPermissionCheckers(domainSvc, permSvc)
```

在 `Handlers{...}` 字面量中追加：

```go
		Invite:       inviteH,
```

- [ ] **步骤 7：新建域时自动生成默认邀请码**

修改 `app/server/internal/service/domain_service.go` 的 `Create`，在 `roleRepo.SeedDefaults` 之后、`return domain, nil` 之前插入：

```go
	if s.inviteCreator != nil {
		if _, err := s.inviteCreator.CreateDefaultForDomain(domain.UUID, ownerUUID); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
```

并在 `DomainService` 结构体加字段与 setter：

```go
type DomainService struct {
	// ... 既有字段
	inviteCreator *InviteService
}

// SetInviteCreator 注入邀请码创建器；nil 表示不自动建默认码（测试/降级）。
func (s *DomainService) SetInviteCreator(svc *InviteService) {
	s.inviteCreator = svc
}
```

在 `gin.go` 的 `domainSvc := service.NewDomainService(...)` 之后、`inviteSvc` 创建之后追加：

```go
	domainSvc.SetInviteCreator(inviteSvc)
```

- [ ] **步骤 8：运行测试验证通过**

运行：
```bash
cd app/server && go build ./... && go test ./internal/handler/ -run TestInviteHandler -v && go test ./internal/service/ -run TestDomainService_Create -v
```
预期：build 通过；invite handler 5 个用例 PASS；domain create 用例 PASS

- [ ] **步骤 9：Commit**

```bash
git add app/server/internal/handler/invite_handler.go app/server/internal/handler/invite_handler_test.go app/server/internal/router/routes/invite/routes.go app/server/internal/router/router.go app/server/server/gin.go app/server/internal/service/domain_service.go
git commit -m "feat(invite): 邀请码 HTTP 端点、路由与装配"
```

---

## 任务 6：删除访客后端全链路

**文件：**
- 删除：`app/server/internal/service/guest_service.go`、`service/guest_service_test.go`
- 删除：`app/server/internal/handler/guest_handler.go`、`handler/guest_handler_test.go`
- 删除：`app/server/internal/middleware/guest.go`
- 删除：`app/server/internal/model/guest_ban.go`、`model/guest_ban_test.go`
- 删除：`app/server/internal/repository/guest_ban_repo.go`、`repository/guest_ban_repo_test.go`
- 删除：`app/server/internal/router/routes/guest/routes.go`
- 删除：`app/server/internal/signal/hub_guest_guard_test.go`
- 修改：见下方逐文件清单

> 这是本计划最大的单个任务。**先删文件再逐个修编译错误**，比逐文件推演更快。
> 完整的「删除后必然编译报错的符号清单」在 `.local/invite-backend-facts.md` §21（296 条，可勾选）。

- [ ] **步骤 1：删除访客专属文件**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration/app/server
rm internal/service/guest_service.go internal/service/guest_service_test.go
rm internal/handler/guest_handler.go internal/handler/guest_handler_test.go
rm internal/middleware/guest.go
rm internal/model/guest_ban.go internal/model/guest_ban_test.go
rm internal/repository/guest_ban_repo.go internal/repository/guest_ban_repo_test.go
rm internal/router/routes/guest/routes.go
rm internal/signal/hub_guest_guard_test.go
rmdir internal/router/routes/guest
```

- [ ] **步骤 2：清理 model 层**

`app/server/internal/model/domain.go`：
- 删 5 个 guest 字段（`AllowGuest`、`GuestCanListen`、`GuestCanSpeak`、`GuestCanMessage`、`GuestLimit`）
- 删 `InviteCode` 字段
- 删 `DefaultGuestLimit` 常量
- 删 `GenerateInviteCode()` 函数
- `BeforeCreate` 只保留 UUID 生成
- `DomainDetail` 删 `InviteCode` 字段

改后的 `Domain` 结构体：

```go
// Domain 代表一个语音域（原 Domain/Server 语义）。
type Domain struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	UUID                string    `gorm:"size:32;uniqueIndex" json:"uuid"`
	Name                string    `gorm:"size:100;not null" json:"name"`
	IconURL             string    `gorm:"size:512" json:"icon_url"`
	Description         string    `gorm:"size:500" json:"description"`
	OwnerUUID           string    `gorm:"type:uuid;index;not null" json:"owner_uuid"`
	IsPublic            bool      `gorm:"default:false" json:"is_public"`
	RequireJoinApproval bool      `gorm:"default:false" json:"require_join_approval"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (d *Domain) TableName() string {
	return "domains"
}

func (d *Domain) BeforeCreate(_ *gorm.DB) error {
	if d.UUID == "" {
		d.UUID = cuid2.Generate()
	}
	return nil
}
```

注意：`RequireJoinApproval` 是第二部分的字段。若第一部分先落地而第二部分尚未实施，**保留该字段**（第二部分直接使用，不需要再改 model）——它是纯新增，不影响第一部分任何逻辑。

改后的 `DomainDetail`：

```go
// DomainDetail 我的语音域批量详情视图，附带成员数与房间数。
type DomainDetail struct {
	ID          uint      `json:"id"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	IconURL     string    `json:"icon_url"`
	Description string    `json:"description"`
	OwnerUUID   string    `json:"owner_uuid"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MemberCount int64     `json:"member_count"`
	RoomCount   int64     `json:"room_count"`
}
```

`app/server/internal/model/user.go`：删 `IsGuest` 字段、`GuestNamePrefix` 常量、`IsGuestName` 函数，以及不再使用的 `strings` import。

`app/server/internal/model/domain_role.go`：删 `DomainRoleGuest` 常量、`DefaultDomainRolePermissions` 中的 guest 条目、`IsSystemDomainRole` 的 guest 分支。

```go
func IsSystemDomainRole(name string) bool {
	switch name {
	case DomainRoleOwner, DomainRoleAdmin, DomainRoleMember:
		return true
	default:
		return false
	}
}
```

- [ ] **步骤 3：清理 repository 层**

`app/server/internal/repository/db_migrations.go`：AutoMigrate 列表删 `&model.DomainGuestBan{},`。

`app/server/internal/repository/domain_role_repo.go`：`SeedDefaultDomainRoles` 的角色列表删 `model.DomainRoleGuest,`，并把注释改为「owner/admin/member」。

`app/server/internal/repository/domain_repo.go`：删 `GetByInviteCode` 与 `ResetInviteCode` 两个方法（及不再使用的 `clause` import）。

- [ ] **步骤 4：清理 service 层**

`app/server/internal/service/domain_service.go`：
- 删 `DomainRoleGuest = model.DomainRoleGuest` 常量
- 删 `GetByInviteCode` 方法
- 删 `ResetInviteCode` 方法
- 删 `Join` 方法
- `domainRoleLevel` 删 guest 分支

```go
func domainRoleLevel(role string) int {
	switch role {
	case DomainRoleOwner:
		return 4
	case DomainRoleAdmin:
		return 3
	case DomainRoleMember:
		return 2
	default:
		return 0
	}
}
```

`app/server/internal/service/auth_service.go`：`Register` 开头删掉 guest_ 前缀拦截：

```go
	// 删除这三行：
	// if strings.HasPrefix(req.Username, model.GuestNamePrefix) {
	// 	return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "username cannot start with guest_")
	// }
```

若 `strings` 与 `model` 因此变成未使用 import，一并删除。

`app/server/internal/service/sfu_service.go`：
- 删 `guestIs`、`guestCaps` 字段
- 删 `SetGuestPolicy` 方法
- `canPublish`（或调用点）删 guest 分支

`app/server/internal/handler/message_handler.go`：
- 删 `guestIs`、`guestCaps` 字段
- 删 `SetGuestPolicy` 方法
- 删 `guestMessagingAllowed` 方法
- 删第 187–190 行的 guest 分支

- [ ] **步骤 5：清理 middleware 与 handler 层**

`app/server/internal/middleware/auth.go`：删 `GuestChecker guestChecker` 字段。

`app/server/internal/handler/domain_handler.go`：删 `Join`、`Preview`、`ResetInvite` 三个方法，以及 `JoinDomainRequest` 结构体。

`app/server/internal/router/routes/domain/routes.go`：删三行：

```go
	r.POST("/reset-invite", middleware.RequireDomainMember(), h.ResetInvite)
	r.POST("/join", h.Join)
	r.POST("/preview", h.Preview)
```

- [ ] **步骤 6：清理 signal 层**

`app/server/internal/signal/hub.go`：
- 删 `guestJoinGuard`、`guestSpeakPolicy`、`guestMessagePolicy` 三个字段
- 删 `GuestJoinGuard`、`GuestSpeakPolicy`、`GuestMessagePolicy` 三个 Options 字段与赋值
- 删 `SetGuestJoinGuard`、`SetGuestSpeakPolicy`、`SetGuestMessagePolicy` 三个 setter

`app/server/internal/signal/hub_room_join.go`：
- 删两处 `if h.guestJoinGuard != nil { ... }` 整块（约 104–120、363–379）
- 删 `h.enforceGuestSpeakPolicyLocked(...)` 调用
- 删 `enforceGuestSpeakPolicyLocked` 与 `rollbackGuestSFUJoin` 两个方法

`app/server/internal/signal/message_bridge.go`：删 `guestMessagePolicy` 分支（约 106–112）。

- [ ] **步骤 7：清理 router 与装配**

`app/server/internal/router/router.go`：
- 删 `guestRoutes "GOSpeak/internal/router/routes/guest"` import
- 删 `Handlers.Guest` 字段
- 删 `if h.Guest != nil { guestRoutes.Register(...) }`
- 删 `protected.Use(middleware.GuestGuard())`
- 删 `if h.Guest != nil { guestRoutes.RegisterProtected(...) }`

`app/server/server/gin.go`：删全部 guest 装配（`.local/invite-backend-facts.md` §2 列出 27 处），包括：
- `guestSvc := service.NewGuestService(...)`
- `signalHub.SetGuestMessagePolicy/SetGuestSpeakPolicy/SetGuestJoinGuard(...)` 三块
- `guestSvc.SetOnlineGuestResolver/SetOnlineCounter(...)`
- `sfuSvc.SetGuestPolicy(...)`
- `guestH := handler.NewGuestHandler(...)` 与 `guestH.SetOnDomainKick(...)`
- `msgH.SetGuestPolicy(...)`
- `GuestChecker: guestSvc`（middleware 依赖注入）
- `Guest: guestH`（Handlers 字面量）

- [ ] **步骤 8：编译并修复残留**

```bash
cd app/server && go build ./... 2>&1 | head -40
```

按报错逐个修复。预期残留类型：
- 未使用 import（`strings`、`model`、`errors`）
- 测试文件中引用已删符号

- [ ] **步骤 9：运行后端测试**

```bash
cd app/server && go vet ./... && go test ./internal/... 2>&1 | tail -30
```

预期：编译通过；测试全绿（访客测试已随文件删除）。若 `domain_service_test.go` 引用了 `ResetInviteCode`/`Join`，删除对应用例。

- [ ] **步骤 10：重新生成 swagger**

```bash
cd app/server && go run github.com/swaggo/swag/cmd/swag init -g main.go -o docs --parseDependency --parseInternal 2>&1 | tail -5
```

若 swag 未安装，跳过此步并在 PR 描述中注明「swagger 需在 CI 重新生成」。

- [ ] **步骤 11：Commit**

```bash
git add -A app/server
git commit -m "refactor(guest)!: 下线访客体系全链路"
```

---

## 任务 7：删除访客前端全链路

**文件：**
- 删除：`app/web/src/pages/guest.tsx`、`stores/guestStore.ts`、`stores/guestStore.test.ts`、`components/domain/GuestAccessSettings.tsx`、`components/domain/GuestManageTable.tsx`、`api/guest.ts`、`api/guest.spec.ts`
- 修改：见下方逐文件清单

> 完整引用点清单在 `.local/invite-frontend-facts.md` §5.7（逐条 `文件:行号 + 内容`）。

- [ ] **步骤 1：删除访客专属文件**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration/app/web
rm src/pages/guest.tsx
rm src/stores/guestStore.ts src/stores/guestStore.test.ts
rm src/components/domain/GuestAccessSettings.tsx src/components/domain/GuestManageTable.tsx
rm src/api/guest.ts src/api/guest.spec.ts
```

- [ ] **步骤 2：清理 `isGuest` / `guestCaps` 调用点**

`app/web/src/components/textRoom/MessageInput.tsx`：
- 删 `import { guestCaps, isGuest } from "@/stores/guestStore";`
- 删 `const guestMsgBlocked = isGuest() && !guestCaps().message;` 及其在发送按钮禁用条件中的引用

`app/web/src/components/userBar.tsx`：
- 删 import
- 删 `const guestSpeakBlocked = isGuest() && !guestCaps().speak;` 及引用

`app/web/src/hooks/useGlobalShortcuts.ts`：
- 删 import
- 删 `const guestBlocked = isGuest() && !guestCaps().speak;` 及引用

`app/web/src/pages/(app)/index.tsx`：
- 删 import
- 删 `if (isGuest()) throw redirect({ to: "/discover" });`（连同不再使用的 `redirect` import）

- [ ] **步骤 3：清理 domainStore**

`app/web/src/stores/domainStore.ts`：
- 删 `import { getGuestConfig } from "@/api/guest";`
- 删 `import { isGuest, setGuestCaps } from "@/stores/guestStore";`
- 删 `syncGuestCaps` 函数全文
- 删 `activateDomain` 中的 `if (isGuest()) syncGuestCaps(uuid);` 一行

- [ ] **步骤 4：清理域管理页**

`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`：
- 删 8 个 `@/api/guest` import
- 删 `GuestAccessSettings`、`GuestManageTable` 两个组件 import
- 删「访客访问」`ManageSection` 整块（含 `<GuestAccessSettings ... />`）
- 删「访客封禁管理」`ManageSection` 整块（含 `<GuestManageTable ... />`）
- 删相关 state：`guestConfig`、`guestBans`、`guestLoading` 等
- 删相关 handler：`loadGuestConfig`、`saveGuestConfig`、`banGuest`、`unbanGuest`、`cleanupGuests` 等
- 删「清理访客」确认弹窗

- [ ] **步骤 5：清理公开路径白名单**

`app/web/src/api/apiClient.ts:56-63`：`shouldSkipLoginRedirect` 删 `path === "/guest" ||` 一行，并更新注释（不再提 guest）。

`app/web/src/api/redirectToLogin.test.ts`：删 `expect(shouldSkipLoginRedirect("/guest")).toBe(true);` 断言与相关注释。

- [ ] **步骤 6：改邀请落地页的 redirect 目标**

`app/web/src/pages/invite/d/$code.tsx` 的 `beforeLoad`：访客分支已无目标页。第一部分改为「未登录不 redirect，由页面自己渲染内联注册表单」（第二部分完整重写）。最小改动：

```tsx
	beforeLoad: async ({ params }) => {
		const ok = await useUserStore.getState().ensureSession();
		const user = useUserStore.getState().user;
		if (!ok || !user) {
			throw redirect({
				to: "/login",
				search: { redirect: `/invite/d/${params.code}` },
			});
		}
	},
```

同时删掉 `user.is_guest` 判断与组件内对 `is_guest` 的引用。

- [ ] **步骤 7：类型检查**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
pnpm --filter @gospeak/web typecheck 2>&1 | tail -20
```

预期：无错误。若有残留引用，按报错逐个清理。

- [ ] **步骤 8：运行前端测试**

```bash
pnpm --filter @gospeak/web test 2>&1 | tail -25
```

预期：全绿。若有测试引用了已删的 guest 组件/store，删除对应用例。

- [ ] **步骤 9：Lint**

```bash
pnpm lint 2>&1 | tail -15
```

- [ ] **步骤 10：Commit**

```bash
git add -A app/web
git commit -m "refactor(web)!: 下线访客前端入口与能力开关"
```

---

## 任务 8：邀请码正则与 API 客户端

**文件：**
- 修改：`app/web/src/utils/domainInvite.ts:3-4`
- 修改：`app/web/src/utils/domainInvite.test.ts`
- 修改：`app/web/src/api/domain.ts:122-160`（删三个函数）
- 修改：`app/web/src/api/domainApi.spec.ts`
- 创建：`app/web/src/api/invite.ts`
- 创建：`app/web/src/api/invite.spec.ts`
- 修改：`app/desktop/deeplink.go:23`
- 修改：`app/desktop/deeplink_test.go`（若存在）
- 修改：`app/desktop/README.md`

- [ ] **步骤 1：编写失败的测试**

在 `app/web/src/utils/domainInvite.test.ts` 追加（若文件不存在则新建）：

```ts
import { describe, expect, it } from "vitest";
import {
	domainDeepLinkUrl,
	domainInviteUrl,
	extractDomainInviteCode,
	parseInviteLink,
} from "./domainInvite";

const VALID = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789AB";

describe("extractDomainInviteCode", () => {
	it("accepts a 36-char code", () => {
		expect(extractDomainInviteCode(VALID)).toBe(VALID);
	});

	it("lowercases are normalized to upper", () => {
		expect(extractDomainInviteCode(VALID.toLowerCase())).toBe(VALID);
	});

	it("rejects the legacy 8-char code", () => {
		expect(extractDomainInviteCode("ABCDEFGH")).toBeNull();
	});

	it("rejects codes containing I or O", () => {
		const withI = `${VALID.slice(0, 35)}I`;
		expect(extractDomainInviteCode(withI)).toBeNull();
		const withO = `${VALID.slice(0, 35)}O`;
		expect(extractDomainInviteCode(withO)).toBeNull();
	});

	it("extracts code from a full invite link", () => {
		expect(extractDomainInviteCode(`https://example.com/invite/d/${VALID}`)).toBe(VALID);
	});

	it("extracts code from a desktop deep link", () => {
		expect(extractDomainInviteCode(`gospeak://invite/d/${VALID}`)).toBe(VALID);
	});

	it("extracts code from a query parameter", () => {
		expect(extractDomainInviteCode(`https://example.com/x?code=${VALID}`)).toBe(VALID);
	});

	it("returns null for unrelated text", () => {
		expect(extractDomainInviteCode("hello world")).toBeNull();
	});
});

describe("parseInviteLink", () => {
	it("derives server origin from an https link", () => {
		const parsed = parseInviteLink(`https://example.com/invite/d/${VALID}`);
		expect(parsed).toEqual({ code: VALID, server: "https://example.com" });
	});

	it("omits server for loopback hosts", () => {
		const parsed = parseInviteLink(`http://127.0.0.1:3000/invite/d/${VALID}`);
		expect(parsed).toEqual({ code: VALID });
	});

	it("omits server for bare codes", () => {
		expect(parseInviteLink(VALID)).toEqual({ code: VALID });
	});
});

describe("domainInviteUrl / domainDeepLinkUrl", () => {
	it("builds a browser invite link", () => {
		expect(domainInviteUrl(VALID, "https://example.com")).toBe(
			`https://example.com/invite/d/${VALID}`,
		);
	});

	it("builds a desktop deep link", () => {
		expect(domainDeepLinkUrl(VALID)).toBe(`gospeak://invite/d/${VALID}`);
	});
});
```

创建 `app/web/src/api/invite.spec.ts`：

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";

const post = vi.fn();
vi.mock("@/api/apiClient", () => ({
	apiClient: { post: (...args: unknown[]) => post(...args) },
}));

const { createInvite, disableInvite, listInvites, listRedemptions, previewInvite } =
	await import("./invite");

describe("invite api", () => {
	beforeEach(() => post.mockReset());

	it("previewInvite posts the code with skipErrorToast", async () => {
		post.mockResolvedValue({ domain: { uuid: "d1" }, requires_approval: false, remaining_uses: -1 });
		const result = await previewInvite("CODE");
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/invite/preview",
			data: { code: "CODE" },
			skipErrorToast: true,
		});
		expect(result.requires_approval).toBe(false);
	});

	it("createInvite sends domain uuid and options", async () => {
		post.mockResolvedValue({ id: 1, code: "C" });
		await createInvite({ domain_uuid: "d1", label: "测试", expires_in_hours: 24, max_uses: 5 });
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/domain/invites/create",
			data: { domain_uuid: "d1", label: "测试", expires_in_hours: 24, max_uses: 5 },
		});
	});

	it("listInvites unwraps the invites array", async () => {
		post.mockResolvedValue({ invites: [{ id: 1 }] });
		const list = await listInvites("d1");
		expect(list).toHaveLength(1);
	});

	it("disableInvite posts invite id", async () => {
		post.mockResolvedValue(null);
		await disableInvite("d1", 7);
		expect(post).toHaveBeenCalledWith({
			url: "/api/v1/domain/invites/disable",
			data: { domain_uuid: "d1", invite_id: 7 },
		});
	});

	it("listRedemptions unwraps items and total", async () => {
		post.mockResolvedValue({ items: [{ id: 1 }], total: 1 });
		const result = await listRedemptions("d1", 3, 1, 20);
		expect(result.total).toBe(1);
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
pnpm --filter @gospeak/web test -- src/utils/domainInvite.test.ts src/api/invite.spec.ts 2>&1 | tail -25
```

预期：FAIL（`invite.ts` 不存在；正则仍是 8 位）

- [ ] **步骤 3：改邀请码正则**

`app/web/src/utils/domainInvite.ts` 第 3–4 行改为：

```ts
const INVITE_CODE_PATTERN = /^[A-HJ-NP-Z2-9]{36}$/i;
const INVITE_LINK_PATTERN = /\/invite\/d\/([A-HJ-NP-Z2-9]{36})/i;
```

同时更新文件顶部注释，说明新码是 36 字符自包含签名串。

- [ ] **步骤 4：创建邀请码 API 客户端**

创建 `app/web/src/api/invite.ts`：

```ts
import { apiClient } from "@/api/apiClient";

export interface InvitePreview {
	domain: {
		uuid: string;
		name: string;
		icon_url?: string;
		description?: string;
	};
	requires_approval: boolean;
	remaining_uses: number;
}

export interface DomainInvite {
	id: number;
	domain_uuid: string;
	label: string;
	code: string;
	expires_at?: string | null;
	max_uses: number;
	used_count: number;
	disabled_at?: string | null;
	created_by: string;
	created_at: string;
}

export interface InviteRedemption {
	id: number;
	invite_id: number;
	domain_uuid: string;
	user_uuid: string;
	code: string;
	success: boolean;
	fail_reason?: string;
	created_at: string;
}

export async function previewInvite(code: string): Promise<InvitePreview> {
	const result = await apiClient.post<InvitePreview>({
		url: "/api/v1/invite/preview",
		data: { code },
		skipErrorToast: true,
	});
	if (!result) throw new Error("invite preview is missing");
	return result;
}

export async function createInvite(params: {
	domain_uuid: string;
	label?: string;
	expires_in_hours?: number;
	max_uses?: number;
}): Promise<DomainInvite> {
	const result = await apiClient.post<DomainInvite>({
		url: "/api/v1/domain/invites/create",
		data: params,
	});
	if (!result) throw new Error("invite create returned no data");
	return result;
}

export async function listInvites(domainUUID: string): Promise<DomainInvite[]> {
	const result = await apiClient.post<{ invites: DomainInvite[] }>({
		url: "/api/v1/domain/invites/list",
		data: { domain_uuid: domainUUID },
	});
	return result?.invites ?? [];
}

export async function disableInvite(domainUUID: string, inviteID: number): Promise<void> {
	await apiClient.post<null>({
		url: "/api/v1/domain/invites/disable",
		data: { domain_uuid: domainUUID, invite_id: inviteID },
	});
}

export async function listRedemptions(
	domainUUID: string,
	inviteID: number,
	page = 1,
	pageSize = 20,
): Promise<{ items: InviteRedemption[]; total: number }> {
	const result = await apiClient.post<{ items: InviteRedemption[]; total: number }>({
		url: "/api/v1/domain/invites/redemptions",
		data: {
			domain_uuid: domainUUID,
			invite_id: inviteID,
			page,
			page_size: pageSize,
		},
	});
	return { items: result?.items ?? [], total: result?.total ?? 0 };
}

/** 邀请码状态：生效中 / 已过期 / 已用完 / 已禁用。 */
export function inviteStatus(invite: DomainInvite): "active" | "expired" | "exhausted" | "disabled" {
	if (invite.disabled_at) return "disabled";
	if (invite.expires_at && new Date(invite.expires_at).getTime() <= Date.now()) return "expired";
	if (invite.max_uses > 0 && invite.used_count >= invite.max_uses) return "exhausted";
	return "active";
}
```

- [ ] **步骤 5：删掉旧邀请码 API**

`app/web/src/api/domain.ts`：删 `previewDomainInvite`、`joinDomain`、`resetDomainInviteCode` 三个函数。

`app/web/src/api/domainApi.spec.ts`：删对应 import 与用例（`previewDomainInvite sends the normalized invite code`、`joinDomain calls with invite_code`，以及 reset 相关用例）。

- [ ] **步骤 6：改桌面端正则**

`app/desktop/deeplink.go:23`：

```go
var invitePathPattern = regexp.MustCompile(`(?i)/invite/d/([A-HJ-NP-Z2-9]{36})(?:[/?#]|$)`)
```

`app/desktop/README.md`：把深链示例中的 8 位码改成 36 位示例（例如 `gospeak://invite/d/ABCDEFGHJKLMNPQRSTUVWXYZ23456789AB`），并说明码长从 8 位变为 36 位。

若 `app/desktop/deeplink_test.go` 存在 8 位码用例，同步改成 36 位。

- [ ] **步骤 7：运行测试验证通过**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
pnpm --filter @gospeak/web test -- src/utils/domainInvite.test.ts src/api/invite.spec.ts 2>&1 | tail -20
cd app/desktop && go test ./... -run TestInvite 2>&1 | tail -10
```

预期：前端两个文件全绿；桌面端用例通过

- [ ] **步骤 8：Commit**

```bash
git add app/web/src/utils/domainInvite.ts app/web/src/utils/domainInvite.test.ts app/web/src/api/invite.ts app/web/src/api/invite.spec.ts app/web/src/api/domain.ts app/web/src/api/domainApi.spec.ts app/desktop/deeplink.go app/desktop/README.md
git commit -m "feat(invite): 前端邀请码 API 与 36 位码正则"
```

---

## 任务 9：`/link` 改造为邀请码管理中心

**文件：**
- 创建：`app/web/src/components/domain/InviteCodeTable.tsx`
- 创建：`app/web/src/components/domain/InviteCodeCreateModal.tsx`
- 创建：`app/web/src/components/domain/InviteRedemptionsPanel.tsx`
- 修改：`app/web/src/pages/(app)/link.tsx`（整体重写）
- 测试：`app/web/src/components/domain/InviteCodeTable.test.tsx`

- [ ] **步骤 1：编写失败的测试**

创建 `app/web/src/components/domain/InviteCodeTable.test.tsx`：

```tsx
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { DomainInvite } from "@/api/invite";
import InviteCodeTable from "./InviteCodeTable";

afterEach(cleanup);

function invite(overrides: Partial<DomainInvite> = {}): DomainInvite {
	return {
		id: 1,
		domain_uuid: "d1",
		label: "默认邀请码",
		code: "ABCDEFGHJKLMNPQRSTUVWXYZ23456789AB",
		expires_at: null,
		max_uses: 0,
		used_count: 0,
		disabled_at: null,
		created_by: "u1",
		created_at: "2026-11-10T00:00:00Z",
		...overrides,
	};
}

describe("InviteCodeTable", () => {
	it("renders an empty hint when there are no invites", () => {
		render(<InviteCodeTable invites={[]} onDisable={() => {}} onShowRedemptions={() => {}} />);
		expect(screen.getByText("还没有邀请码")).toBeTruthy();
	});

	it("renders label, usage and status for an active invite", () => {
		render(
			<InviteCodeTable
				invites={[invite({ max_uses: 10, used_count: 3 })]}
				onDisable={() => {}}
				onShowRedemptions={() => {}}
			/>,
		);
		expect(screen.getByText("默认邀请码")).toBeTruthy();
		expect(screen.getByText("3 / 10")).toBeTruthy();
		expect(screen.getByText("生效中")).toBeTruthy();
	});

	it("marks unlimited usage", () => {
		render(
			<InviteCodeTable invites={[invite()]} onDisable={() => {}} onShowRedemptions={() => {}} />,
		);
		expect(screen.getByText("0 / 不限")).toBeTruthy();
	});

	it("shows disabled status and hides the disable action", () => {
		render(
			<InviteCodeTable
				invites={[invite({ disabled_at: "2026-11-10T01:00:00Z" })]}
				onDisable={() => {}}
				onShowRedemptions={() => {}}
			/>,
		);
		expect(screen.getByText("已禁用")).toBeTruthy();
		expect(screen.queryByText("禁用")).toBeNull();
	});

	it("calls onDisable with the invite id", () => {
		const onDisable = vi.fn();
		render(
			<InviteCodeTable invites={[invite({ id: 9 })]} onDisable={onDisable} onShowRedemptions={() => {}} />,
		);
		screen.getByText("禁用").click();
		expect(onDisable).toHaveBeenCalledWith(9);
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

```bash
pnpm --filter @gospeak/web test -- src/components/domain/InviteCodeTable.test.tsx 2>&1 | tail -15
```

预期：FAIL，模块不存在

- [ ] **步骤 3：创建邀请码列表组件**

创建 `app/web/src/components/domain/InviteCodeTable.tsx`：

```tsx
import { Button } from "@gospeak/components";
import { Copy, History, Ban } from "lucide-react";
import { type DomainInvite, inviteStatus } from "@/api/invite";

const STATUS_LABEL: Record<ReturnType<typeof inviteStatus>, string> = {
	active: "生效中",
	expired: "已过期",
	exhausted: "已用完",
	disabled: "已禁用",
};

function usageText(invite: DomainInvite): string {
	const used = `${invite.used_count}`;
	return invite.max_uses > 0 ? `${used} / ${invite.max_uses}` : `${used} / 不限`;
}

function expiryText(invite: DomainInvite): string {
	if (!invite.expires_at) return "永久";
	return new Date(invite.expires_at).toLocaleString("zh-CN");
}

export default function InviteCodeTable({
	invites,
	onDisable,
	onShowRedemptions,
	onCopy,
}: {
	invites: DomainInvite[];
	onDisable: (inviteID: number) => void;
	onShowRedemptions: (invite: DomainInvite) => void;
	onCopy?: (invite: DomainInvite) => void;
}) {
	if (invites.length === 0) {
		return <p className="text-muted-foreground text-sm">还没有邀请码</p>;
	}
	return (
		<table className="w-full text-sm">
			<thead>
				<tr className="text-muted-foreground border-border border-b text-left text-xs">
					<th className="py-2">备注</th>
					<th className="py-2">邀请码</th>
					<th className="py-2">有效期</th>
					<th className="py-2">已用 / 上限</th>
					<th className="py-2">状态</th>
					<th className="py-2 text-right">操作</th>
				</tr>
			</thead>
			<tbody>
				{invites.map((invite) => {
					const status = inviteStatus(invite);
					return (
						<tr key={invite.id} className="border-border/60 border-b last:border-0">
							<td className="py-2">{invite.label || "—"}</td>
							<td className="py-2">
								<code className="text-xs">{`${invite.code.slice(0, 8)}…${invite.code.slice(-4)}`}</code>
							</td>
							<td className="py-2 text-xs">{expiryText(invite)}</td>
							<td className="py-2 text-xs">{usageText(invite)}</td>
							<td className="py-2 text-xs">{STATUS_LABEL[status]}</td>
							<td className="py-2 text-right">
								<div className="inline-flex gap-1">
									{onCopy && (
										<Button variant="ghost" size="sm" onClick={() => onCopy(invite)}>
											<Copy size={14} />
										</Button>
									)}
									<Button variant="ghost" size="sm" onClick={() => onShowRedemptions(invite)}>
										<History size={14} />
									</Button>
									{status !== "disabled" && (
										<Button variant="ghost" size="sm" onClick={() => onDisable(invite.id)}>
											<Ban size={14} />
											禁用
										</Button>
									)}
								</div>
							</td>
						</tr>
					);
				})}
			</tbody>
		</table>
	);
}
```

- [ ] **步骤 4：创建新建邀请码弹窗**

创建 `app/web/src/components/domain/InviteCodeCreateModal.tsx`：

```tsx
import {
	Button,
	Dialog,
	DialogContent,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	Input,
} from "@gospeak/components";
import { useState } from "react";
import { toastActionError } from "@/utils/toast";

const EXPIRY_OPTIONS = [
	{ label: "永久", hours: 0 },
	{ label: "1 小时", hours: 1 },
	{ label: "24 小时", hours: 24 },
	{ label: "7 天", hours: 24 * 7 },
	{ label: "30 天", hours: 24 * 30 },
] as const;

export default function InviteCodeCreateModal({
	open,
	onClose,
	onCreate,
}: {
	open: boolean;
	onClose: () => void;
	onCreate: (params: { label: string; expiresInHours: number; maxUses: number }) => Promise<void>;
}) {
	const [label, setLabel] = useState("");
	const [expiryHours, setExpiryHours] = useState(0);
	const [maxUses, setMaxUses] = useState(0);
	const [busy, setBusy] = useState(false);

	async function submit() {
		if (busy) return;
		if (maxUses < 0 || maxUses > 65535) {
			toastActionError(new Error("invalid max uses"), "使用上限必须在 0 到 65535 之间");
			return;
		}
		setBusy(true);
		try {
			await onCreate({ label: label.trim(), expiresInHours: expiryHours, maxUses });
			setLabel("");
			setExpiryHours(0);
			setMaxUses(0);
			onClose();
		} finally {
			setBusy(false);
		}
	}

	return (
		<Dialog open={open} onOpenChange={(next) => !next && onClose()}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>新建邀请码</DialogTitle>
				</DialogHeader>
				<div className="flex flex-col gap-4">
					<label className="flex flex-col gap-1 text-sm">
						备注名
						<Input
							value={label}
							onChange={(e) => setLabel(e.target.value)}
							placeholder="例如：KOL 推广"
						/>
					</label>
					<label className="flex flex-col gap-1 text-sm">
						有效期
						<select
							className="border-border bg-background h-9 rounded-sm border px-2 text-sm"
							value={expiryHours}
							onChange={(e) => setExpiryHours(Number(e.target.value))}
						>
							{EXPIRY_OPTIONS.map((opt) => (
								<option key={opt.hours} value={opt.hours}>
									{opt.label}
								</option>
							))}
						</select>
					</label>
					<label className="flex flex-col gap-1 text-sm">
						最大使用次数（0 表示不限，上限 65535）
						<Input
							type="number"
							min={0}
							max={65535}
							value={maxUses}
							onChange={(e) => setMaxUses(Number(e.target.value))}
						/>
					</label>
				</div>
				<DialogFooter>
					<Button variant="outline" onClick={onClose} disabled={busy}>
						取消
					</Button>
					<Button onClick={() => void submit()} disabled={busy}>
						{busy ? "创建中…" : "创建"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
```

- [ ] **步骤 5：创建使用记录面板**

创建 `app/web/src/components/domain/InviteRedemptionsPanel.tsx`：

```tsx
import { Button } from "@gospeak/components";
import { useEffect, useState } from "react";
import {
	type DomainInvite,
	type InviteRedemption,
	listRedemptions,
} from "@/api/invite";
import { toastActionError } from "@/utils/toast";

const FAIL_LABEL: Record<string, string> = {
	expired: "已过期",
	exhausted: "已达上限",
	invalid: "无效",
	disabled: "已禁用",
	rejected: "审核拒绝",
	already_member: "已是成员",
};

export default function InviteRedemptionsPanel({
	domainUUID,
	invite,
	onClose,
}: {
	domainUUID: string;
	invite: DomainInvite;
	onClose: () => void;
}) {
	const [items, setItems] = useState<InviteRedemption[]>([]);
	const [total, setTotal] = useState(0);
	const [page, setPage] = useState(1);
	const [loading, setLoading] = useState(false);

	useEffect(() => {
		let cancelled = false;
		setLoading(true);
		void listRedemptions(domainUUID, invite.id, page, 20)
			.then((result) => {
				if (cancelled) return;
				setItems(result.items);
				setTotal(result.total);
			})
			.catch((error) => {
				if (!cancelled) toastActionError(error, "加载使用记录失败");
			})
			.finally(() => {
				if (!cancelled) setLoading(false);
			});
		return () => {
			cancelled = true;
		};
	}, [domainUUID, invite.id, page]);

	const pageCount = Math.max(1, Math.ceil(total / 20));

	return (
		<div className="border-border bg-card flex h-full w-80 flex-col gap-3 border-l p-4">
			<div className="flex items-center justify-between">
				<h3 className="text-sm font-semibold">使用记录</h3>
				<Button variant="ghost" size="sm" onClick={onClose}>
					关闭
				</Button>
			</div>
			<p className="text-muted-foreground text-xs">
				{invite.label || invite.code.slice(0, 8)} · 共 {total} 条
			</p>
			{loading && <p className="text-muted-foreground text-xs">加载中…</p>}
			{!loading && items.length === 0 && (
				<p className="text-muted-foreground text-xs">还没有使用记录</p>
			)}
			<ul className="flex-1 overflow-auto">
				{items.map((item) => (
					<li key={item.id} className="border-border/60 border-b py-2 text-xs last:border-0">
						<div className="flex items-center justify-between">
							<span>{item.success ? "成功" : `失败 · ${FAIL_LABEL[item.fail_reason ?? ""] ?? item.fail_reason}`}</span>
							<span className="text-faint-foreground">
								{new Date(item.created_at).toLocaleString("zh-CN")}
							</span>
						</div>
						{item.user_uuid && (
							<p className="text-faint-foreground mt-1 truncate">{item.user_uuid}</p>
						)}
					</li>
				))}
			</ul>
			{pageCount > 1 && (
				<div className="flex items-center justify-between">
					<Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
						上一页
					</Button>
					<span className="text-muted-foreground text-xs">
						{page} / {pageCount}
					</span>
					<Button
						variant="outline"
						size="sm"
						disabled={page >= pageCount}
						onClick={() => setPage((p) => p + 1)}
					>
						下一页
					</Button>
				</div>
			)}
		</div>
	);
}
```

- [ ] **步骤 6：重写 `/link` 页**

`app/web/src/pages/(app)/link.tsx` 整体替换为：

```tsx
import { Button, toast } from "@gospeak/components";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { getMyDomainsDetailed } from "@/api/domain";
import {
	createInvite,
	type DomainInvite,
	disableInvite,
	listInvites,
} from "@/api/invite";
import DomainIcon from "@/components/domain/DomainIcon";
import InviteCodeCreateModal from "@/components/domain/InviteCodeCreateModal";
import InviteCodeTable from "@/components/domain/InviteCodeTable";
import InviteRedemptionsPanel from "@/components/domain/InviteRedemptionsPanel";
import InviteShareModal from "@/components/domain/InviteShareModal";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { domainStore } from "@/stores/domainStore";
import { domainInviteUrl } from "@/utils/domainInvite";
import { usePublicBaseUrl } from "@/utils/publicBaseUrl";
import { toastActionError } from "@/utils/toast";
import { useEffect } from "react";

export const Route = createFileRoute("/(app)/link")({
	validateSearch: (search: Record<string, unknown>) => ({
		domain: typeof search.domain === "string" ? search.domain : undefined,
	}),
	component: RouteComponent,
	staticData: { title: "邀请码" },
});

function RouteComponent() {
	const search = Route.useSearch();
	const publicBase = usePublicBaseUrl();
	const [activeDomain, setActiveDomain] = useState<string | undefined>(search.domain);
	const [invites, setInvites] = useState<DomainInvite[]>([]);
	const [loadingInvites, setLoadingInvites] = useState(false);
	const [createOpen, setCreateOpen] = useState(false);
	const [shareInvite, setShareInvite] = useState<DomainInvite | null>(null);
	const [redemptionsInvite, setRedemptionsInvite] = useState<DomainInvite | null>(null);

	useEffect(() => {
		void domainStore.loadMyDomains().catch(() => {});
	}, []);

	const { data: domains, isLoading } = useQuery({
		queryKey: ["my-domains-detailed-link"],
		queryFn: getMyDomainsDetailed,
	});

	useEffect(() => {
		if (!activeDomain && domains && domains.length > 0) {
			setActiveDomain(domains[0].uuid);
		}
	}, [activeDomain, domains]);

	useEffect(() => {
		if (!activeDomain) return;
		let cancelled = false;
		setLoadingInvites(true);
		void listInvites(activeDomain)
			.then((list) => {
				if (!cancelled) setInvites(list);
			})
			.catch((error) => {
				if (!cancelled) toastActionError(error, "加载邀请码失败");
			})
			.finally(() => {
				if (!cancelled) setLoadingInvites(false);
			});
		return () => {
			cancelled = true;
		};
	}, [activeDomain]);

	async function refreshInvites() {
		if (!activeDomain) return;
		try {
			setInvites(await listInvites(activeDomain));
		} catch (error) {
			toastActionError(error, "刷新邀请码失败");
		}
	}

	async function handleCreate(params: { label: string; expiresInHours: number; maxUses: number }) {
		if (!activeDomain) return;
		try {
			await createInvite({
				domain_uuid: activeDomain,
				label: params.label,
				expires_in_hours: params.expiresInHours,
				max_uses: params.maxUses,
			});
			toast.success("邀请码已创建");
			await refreshInvites();
		} catch (error) {
			toastActionError(error, "创建邀请码失败");
		}
	}

	async function handleDisable(inviteID: number) {
		if (!activeDomain) return;
		try {
			await disableInvite(activeDomain, inviteID);
			toast.success("邀请码已禁用");
			await refreshInvites();
		} catch (error) {
			toastActionError(error, "禁用邀请码失败");
		}
	}

	if (isLoading) return <ManageLoading />;

	const current = domains?.find((d) => d.uuid === activeDomain);
	const inviteUrl = shareInvite ? domainInviteUrl(shareInvite.code, publicBase) : "";

	return (
		<ManagePage>
			<ManageHeader title="邀请码" description="创建、禁用邀请码并查看使用记录" />

			<ManageSection title="选择域">
				<div className="flex flex-wrap gap-2">
					{(domains ?? []).map((d) => (
						<button
							key={d.uuid}
							type="button"
							onClick={() => setActiveDomain(d.uuid)}
							className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm ${
								d.uuid === activeDomain
									? "border-primary bg-primary/10"
									: "border-border hover:bg-muted"
							}`}
						>
							<DomainIcon domain={d} size={20} />
							{d.name}
						</button>
					))}
				</div>
			</ManageSection>

			{current && (
				<ManageSection
					title="邀请码"
					actions={
						<Button size="sm" onClick={() => setCreateOpen(true)}>
							新建邀请码
						</Button>
					}
				>
					<div className="flex gap-4">
						<div className="min-w-0 flex-1">
							{loadingInvites ? (
								<p className="text-muted-foreground text-sm">加载中…</p>
							) : (
								<InviteCodeTable
									invites={invites}
									onDisable={(id) => void handleDisable(id)}
									onShowRedemptions={(invite) => setRedemptionsInvite(invite)}
									onCopy={(invite) => setShareInvite(invite)}
								/>
							)}
						</div>
						{redemptionsInvite && (
							<InviteRedemptionsPanel
								domainUUID={current.uuid}
								invite={redemptionsInvite}
								onClose={() => setRedemptionsInvite(null)}
							/>
						)}
					</div>
				</ManageSection>
			)}

			<InviteCodeCreateModal
				open={createOpen}
				onClose={() => setCreateOpen(false)}
				onCreate={handleCreate}
			/>

			{shareInvite && (
				<InviteShareModal
					open={!!shareInvite}
					inviteUrl={inviteUrl}
					onClose={() => setShareInvite(null)}
				/>
			)}
		</ManagePage>
	);
}
```

注意：`ManageSection` 是否支持 `actions` prop 需按 `.local/invite-frontend-facts.md` §14 的实际签名调整；若只接受 `title` + `children`，把按钮移到 section 内容顶部。

- [ ] **步骤 7：类型检查与测试**

```bash
cd /Users/noelorin/GOSpeak/.worktrees/invite-registration
pnpm --filter @gospeak/web typecheck 2>&1 | tail -20
pnpm --filter @gospeak/web test -- src/components/domain/InviteCodeTable.test.tsx 2>&1 | tail -15
```

预期：typecheck 无错误；5 个用例 PASS

- [ ] **步骤 8：Commit**

```bash
git add app/web/src/components/domain/InviteCodeTable.tsx app/web/src/components/domain/InviteCodeCreateModal.tsx app/web/src/components/domain/InviteRedemptionsPanel.tsx app/web/src/components/domain/InviteCodeTable.test.tsx "app/web/src/pages/(app)/link.tsx"
git commit -m "feat(web): 邀请码管理中心（列表/新建/禁用/使用记录）"
```

---

## 任务 10：DomainMenu 与 InviteShareModal 适配新码

**文件：**
- 修改：`app/web/src/components/domain/DomainMenu.tsx`
- 修改：`app/web/src/components/domain/InviteShareModal.tsx`（仅在需要时）

- [ ] **步骤 1：改 DomainMenu 的邀请码来源**

`app/web/src/components/domain/DomainMenu.tsx` 当前从 `domain.invite_code` 读码（第 56 行）。`invite_code` 列已删除，改为在菜单打开时按需拉取该域的第一个可用邀请码：

```tsx
import { listInvites } from "@/api/invite";

// 组件内新增 state
const [inviteCode, setInviteCode] = useState("");

useEffect(() => {
	if (!open || !domainUUID) return;
	let cancelled = false;
	void listInvites(domainUUID)
		.then((list) => {
			if (cancelled) return;
			const usable = list.find((i) => !i.disabled_at);
			setInviteCode(usable?.code ?? "");
		})
		.catch(() => {
			if (!cancelled) setInviteCode("");
		});
	return () => {
		cancelled = true;
	};
}, [open, domainUUID]);
```

删除原第 56 行的 `const inviteCode = extractDomainInviteCode(domain?.invite_code || "") || "";`，以及不再使用的 `extractDomainInviteCode` import 与 `domain` 变量（若 `domain` 仅用于此处）。

- [ ] **步骤 2：类型检查**

```bash
pnpm --filter @gospeak/web typecheck 2>&1 | tail -15
```

- [ ] **步骤 3：Commit**

```bash
git add app/web/src/components/domain/DomainMenu.tsx
git commit -m "refactor(web): 域菜单邀请码改从邀请码 API 读取"
```

---

## 任务 11：删除 `domains.invite_code` 列与旧端点

**文件：**
- 修改：`app/server/internal/repository/db_migrations.go`（加显式 DROP COLUMN）
- 修改：`app/server/internal/service/domain_service.go`（确认无残留）
- 修改：`app/server/internal/repository/domain_repo.go`（确认无残留）

- [ ] **步骤 1：加显式删列迁移**

规格 §2.5 明确「不写迁移代码，通过移除 AutoMigrate 注册并显式 DROP 完成」。在 `app/server/internal/repository/db_migrations.go` 的 `autoMigrate()` 末尾、`return migrateConversationQueryIndexes(getDB())` 之前插入：

```go
	if err := dropLegacyGuestSchema(getDB()); err != nil {
		return err
	}
```

并在同文件新增函数：

```go
// dropLegacyGuestSchema 删除访客体系与旧邀请码列/表。
// 这些对象已无任何代码引用，AutoMigrate 不会自动删列，需显式 DROP。
// 幂等：列/表不存在时静默跳过。
func dropLegacyGuestSchema(db *gorm.DB) error {
	m := db.Migrator()
	columns := map[string][]string{
		"domains": {
			"allow_guest", "guest_can_listen", "guest_can_speak",
			"guest_can_message", "guest_limit", "invite_code",
		},
		"users": {"is_guest"},
	}
	for table, cols := range columns {
		if !m.HasTable(table) {
			continue
		}
		for _, col := range cols {
			if !m.HasColumn(table, col) {
				continue
			}
			if err := m.DropColumn(table, col); err != nil {
				return fmt.Errorf("drop column %s.%s: %w", table, col, err)
			}
		}
	}
	if m.HasTable("domain_guest_bans") {
		if err := m.DropTable("domain_guest_bans"); err != nil {
			return fmt.Errorf("drop table domain_guest_bans: %w", err)
		}
	}
	return nil
}
```

- [ ] **步骤 2：验证后端编译与测试**

```bash
cd app/server && go build ./... && go vet ./... && go test ./internal/... 2>&1 | tail -20
```

- [ ] **步骤 3：启动服务验证迁移幂等**

```bash
cd app/server && GIN_MODE=release DB_TYPE=SQLite DB_PATH=/tmp/gospeak-invite-migrate.db timeout 20 go run main.go server -e dev 2>&1 | grep -iE "migrat|error|listen" | head -20
```

预期：首次启动建表 + 删列成功；第二次启动不报错（幂等）。删除临时库文件。

- [ ] **步骤 4：Commit**

```bash
git add app/server/internal/repository/db_migrations.go
git commit -m "refactor(db): 显式清理访客表/列与旧邀请码列"
```

---

## 验收标准

- [ ] `cd app/server && go build ./... && go vet ./... && go test ./internal/...` 全通过
- [ ] `grep -rn "Guest\|guest" app/server --include=*.go | grep -v "_test.go"` 除 `docs/docs.go` 生成物外无结果
- [ ] `grep -rn "is_guest\|guestCaps\|isGuest" app/web/src` 无结果
- [ ] `pnpm --filter @gospeak/web typecheck && pnpm --filter @gospeak/web test` 通过
- [ ] `pnpm lint` 通过
- [ ] 新建域后自动生成一个「默认邀请码」（永不过期、不限次数）
- [ ] `/link` 页可列出邀请码、新建、禁用、查看使用记录
- [ ] 36 位邀请码可通过 `domainInviteUrl` 生成链接，且桌面端 `gospeak://invite/d/<36 位>` 能解析
- [ ] 旧 8 位码链接解析返回 `null`（不再被误判为有效码）
